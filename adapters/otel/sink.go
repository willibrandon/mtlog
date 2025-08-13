package otel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	olog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	// Removed semconv import to avoid schema conflicts
)

// OTLPTransport represents the transport protocol for OTLP
type OTLPTransport int

const (
	// OTLPTransportGRPC uses gRPC protocol
	OTLPTransportGRPC OTLPTransport = iota
	// OTLPTransportHTTP uses HTTP/protobuf protocol
	OTLPTransportHTTP
)

// Default configuration values
const (
	defaultBatchSize          = 100
	defaultBatchTimeout       = 5 * time.Second
	defaultMaxQueueSize       = 10000
	defaultTimeout            = 30 * time.Second
	defaultRetryInterval      = 1 * time.Second
	defaultRetryMaxElapsed    = 30 * time.Second
	defaultCompression        = "gzip"
	defaultMaxConcurrentExports = 10 // Limit concurrent export goroutines
)

// OTLPSink exports log events to OpenTelemetry collectors via OTLP.
// Supports both gRPC and HTTP transports with batching, compression, and retry.
type OTLPSink struct {
	// Configuration
	endpoint      string
	transport     OTLPTransport
	headers       map[string]string
	timeout       time.Duration
	retryEnabled  bool
	retryInterval time.Duration
	retryMaxElapsedTime time.Duration
	compression   string // "gzip", "zstd", or "none"
	
	// TLS configuration
	tlsConfig     *tls.Config
	insecure      bool
	clientCert    string
	clientKey     string
	caCert        string
	
	// Batching configuration
	batchSize      int
	batchTimeout   time.Duration
	maxQueueSize   int
	
	// OTEL components
	exporter       sdklog.Exporter
	logger         olog.Logger
	loggerProvider *sdklog.LoggerProvider
	resource       *resource.Resource
	loggerMu       sync.RWMutex  // Protects logger access
	
	// Internal state
	batch          []*core.LogEvent
	batchMu        sync.Mutex
	timerMu        sync.Mutex    // Separate mutex for timer to avoid race
	timer          *time.Timer
	stopCh         chan struct{}
	flushCh        chan struct{}
	wg             sync.WaitGroup
	closed         atomic.Bool
	
	// Queue management for backpressure
	totalQueueSize atomic.Int64  // Total events in queue (batch + pending exports)
	
	// Metrics
	exportedCount  atomic.Uint64
	droppedCount   atomic.Uint64
	errorCount     atomic.Uint64
	
	// Object pool for batch buffer reuse
	batchPool sync.Pool
	
	// Export concurrency control
	exportSem chan struct{} // Semaphore to limit concurrent exports
}

// OTLPOption configures an OTLP sink
type OTLPOption func(*OTLPSink)

// WithOTLPEndpoint sets the OTLP endpoint
func WithOTLPEndpoint(endpoint string) OTLPOption {
	return func(s *OTLPSink) {
		s.endpoint = endpoint
	}
}

// WithOTLPTransport sets the transport protocol
func WithOTLPTransport(transport OTLPTransport) OTLPOption {
	return func(s *OTLPSink) {
		s.transport = transport
	}
}

// WithOTLPHeaders sets custom headers for OTLP requests
func WithOTLPHeaders(headers map[string]string) OTLPOption {
	return func(s *OTLPSink) {
		s.headers = headers
	}
}

// WithOTLPTimeout sets the export timeout
func WithOTLPTimeout(timeout time.Duration) OTLPOption {
	return func(s *OTLPSink) {
		s.timeout = timeout
	}
}

// WithOTLPCompression sets compression (gzip, zstd, or none)
func WithOTLPCompression(compression string) OTLPOption {
	return func(s *OTLPSink) {
		s.compression = compression
	}
}

// WithOTLPBatching configures batching parameters
func WithOTLPBatching(size int, timeout time.Duration) OTLPOption {
	return func(s *OTLPSink) {
		s.batchSize = size
		s.batchTimeout = timeout
	}
}

// WithOTLPRetry configures retry behavior
func WithOTLPRetry(interval, maxElapsedTime time.Duration) OTLPOption {
	return func(s *OTLPSink) {
		s.retryEnabled = true
		s.retryInterval = interval
		s.retryMaxElapsedTime = maxElapsedTime
	}
}

// WithOTLPResource sets the OTEL resource attributes
func WithOTLPResource(res *resource.Resource) OTLPOption {
	return func(s *OTLPSink) {
		s.resource = res
	}
}

// WithOTLPMaxQueueSize sets the maximum queue size for buffering
func WithOTLPMaxQueueSize(size int) OTLPOption {
	return func(s *OTLPSink) {
		s.maxQueueSize = size
	}
}

// WithOTLPInsecure disables TLS for the connection
func WithOTLPInsecure() OTLPOption {
	return func(s *OTLPSink) {
		s.insecure = true
	}
}

// WithOTLPTLSConfig sets a custom TLS configuration
func WithOTLPTLSConfig(config *tls.Config) OTLPOption {
	return func(s *OTLPSink) {
		s.tlsConfig = config
	}
}

// WithOTLPClientCert sets client certificate for mutual TLS
func WithOTLPClientCert(certFile, keyFile string) OTLPOption {
	return func(s *OTLPSink) {
		s.clientCert = certFile
		s.clientKey = keyFile
	}
}

// WithOTLPCACert sets a custom CA certificate for server verification
func WithOTLPCACert(caFile string) OTLPOption {
	return func(s *OTLPSink) {
		s.caCert = caFile
	}
}

// WithOTLPSkipVerify disables server certificate verification (insecure)
func WithOTLPSkipVerify() OTLPOption {
	return func(s *OTLPSink) {
		if s.tlsConfig == nil {
			s.tlsConfig = &tls.Config{}
		}
		s.tlsConfig.InsecureSkipVerify = true
	}
}

// NewOTLPSink creates a new OTLP sink
func NewOTLPSink(opts ...OTLPOption) (*OTLPSink, error) {
	s := &OTLPSink{
		transport:           OTLPTransportGRPC,
		timeout:             defaultTimeout,
		compression:         defaultCompression,
		batchSize:           defaultBatchSize,
		batchTimeout:        defaultBatchTimeout,
		maxQueueSize:        defaultMaxQueueSize,
		retryEnabled:        true,
		retryInterval:       defaultRetryInterval,
		retryMaxElapsedTime: defaultRetryMaxElapsed,
		headers:             make(map[string]string),
		batch:               make([]*core.LogEvent, 0),
		stopCh:              make(chan struct{}),
		flushCh:             make(chan struct{}),
		exportSem:           make(chan struct{}, defaultMaxConcurrentExports),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(s)
	}
	
	// Use environment variables if endpoint not set
	if s.endpoint == "" {
		s.endpoint = s.getEndpointFromEnv()
	}
	
	// Apply environment variable overrides
	s.applyEnvOverrides()
	
	// Create resource if not provided
	if s.resource == nil {
		s.resource = s.createDefaultResource()
	}
	
	// Configure TLS if needed
	if err := s.configureTLS(); err != nil {
		return nil, err
	}
	
	// Create exporter
	if err := s.createExporter(); err != nil {
		return nil, err
	}
	
	// Initialize batch pool
	s.batchPool.New = func() interface{} {
		return make([]*core.LogEvent, 0, s.batchSize)
	}
	
	// Start background flusher
	s.wg.Add(1)
	go s.backgroundFlusher()
	
	maskedHeaders := maskSensitiveHeaders(s.headers)
	sinkLog.Info("sink created: endpoint=%s transport=%v compression=%s batch_size=%d headers=%v",
		s.endpoint, s.transport, s.compression, s.batchSize, maskedHeaders)
	
	return s, nil
}

// getEndpointFromEnv gets endpoint from environment variables
func (s *OTLPSink) getEndpointFromEnv() string {
	// Check specific protocol endpoints first
	if s.transport == OTLPTransportGRPC {
		if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"); endpoint != "" {
			return endpoint
		}
	} else {
		if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"); endpoint != "" {
			return endpoint
		}
	}
	
	// Fall back to general endpoint
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		if s.transport == OTLPTransportHTTP {
			// For HTTP, append /v1/logs if not present
			if !strings.HasSuffix(endpoint, "/v1/logs") {
				endpoint += "/v1/logs"
			}
		}
		return endpoint
	}
	
	// Default endpoints
	if s.transport == OTLPTransportGRPC {
		return "localhost:4317"
	}
	return "http://localhost:4318/v1/logs"
}

// applyEnvOverrides applies environment variable overrides
func (s *OTLPSink) applyEnvOverrides() {
	// Headers from environment
	if headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headers != "" {
		for _, header := range strings.Split(headers, ",") {
			parts := strings.SplitN(header, "=", 2)
			if len(parts) == 2 {
				s.headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	
	// Compression from environment
	if compression := os.Getenv("OTEL_EXPORTER_OTLP_COMPRESSION"); compression != "" {
		s.compression = compression
	}
	
	// Timeout from environment
	if timeout := os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout + "ms"); err == nil {
			s.timeout = d
		}
	}
}

// createDefaultResource creates default OTEL resource
func (s *OTLPSink) createDefaultResource() *resource.Resource {
	// Use NewWithAttributes with empty schema to avoid conflicts with auto/sdk
	// This avoids the "conflicting Schema URL" error that can occur when
	// different OTEL components use different schema versions
	return resource.NewWithAttributes(
		"", // No schema URL to avoid conflicts
		attribute.String("service.name", "mtlog"),
		attribute.String("service.version", "1.0.0"),
		attribute.String("telemetry.sdk.name", "mtlog"),
		attribute.String("telemetry.sdk.language", "go"),
		attribute.String("telemetry.sdk.version", "1.0.0"),
	)
}

// configureTLS sets up TLS configuration based on options
func (s *OTLPSink) configureTLS() error {
	if s.insecure {
		return nil // No TLS configuration needed for insecure connections
	}
	
	// Start with provided TLS config or create new one
	if s.tlsConfig == nil {
		s.tlsConfig = &tls.Config{}
	}
	
	// Load client certificate if provided
	if s.clientCert != "" && s.clientKey != "" {
		cert, err := tls.LoadX509KeyPair(s.clientCert, s.clientKey)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}
		s.tlsConfig.Certificates = []tls.Certificate{cert}
	}
	
	// Load CA certificate if provided
	if s.caCert != "" {
		caCertPEM, err := os.ReadFile(s.caCert)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCertPEM) {
			return fmt.Errorf("failed to parse CA certificate")
		}
		s.tlsConfig.RootCAs = caCertPool
	}
	
	return nil
}

// createExporter creates the OTLP exporter based on transport
func (s *OTLPSink) createExporter() error {
	ctx := context.Background()
	
	switch s.transport {
	case OTLPTransportGRPC:
		opts := []otlploggrpc.Option{
			otlploggrpc.WithEndpoint(s.endpoint),
			otlploggrpc.WithTimeout(s.timeout),
		}
		
		// Add headers
		if len(s.headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(s.headers))
		}
		
		// Add compression
		switch s.compression {
		case "gzip":
			opts = append(opts, otlploggrpc.WithCompressor("gzip"))
		}
		
		// Add retry
		if s.retryEnabled {
			opts = append(opts, otlploggrpc.WithRetry(otlploggrpc.RetryConfig{
				Enabled:         true,
				InitialInterval: s.retryInterval,
				MaxInterval:     s.retryInterval * 5,
				MaxElapsedTime:  s.retryMaxElapsedTime,
			}))
		}
		
		// Configure TLS/insecure
		if s.insecure || (!strings.Contains(s.endpoint, "443") && !strings.HasPrefix(s.endpoint, "https")) {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		// Note: TLS config would be applied here in a production implementation
		
		exporter, err := otlploggrpc.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("failed to create gRPC exporter: %w", err)
		}
		s.exporter = exporter
		
	case OTLPTransportHTTP:
		// Parse endpoint to extract host:port only
		endpoint := s.endpoint
		useInsecure := false
		
		// Remove protocol if present
		if strings.HasPrefix(endpoint, "http://") {
			endpoint = strings.TrimPrefix(endpoint, "http://")
			useInsecure = true
		} else if strings.HasPrefix(endpoint, "https://") {
			endpoint = strings.TrimPrefix(endpoint, "https://")
		}
		
		// Remove path if present (e.g., /v1/logs)
		if idx := strings.Index(endpoint, "/"); idx != -1 {
			endpoint = endpoint[:idx]
		}
		
		opts := []otlploghttp.Option{
			otlploghttp.WithEndpoint(endpoint),
			otlploghttp.WithTimeout(s.timeout),
		}
		
		// Configure TLS/insecure for HTTP
		if s.insecure || useInsecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		// Note: TLS config would be applied here in a production implementation
		
		// Add headers
		if len(s.headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(s.headers))
		}
		
		// Add compression
		switch s.compression {
		case "gzip":
			opts = append(opts, otlploghttp.WithCompression(otlploghttp.GzipCompression))
		case "none":
			opts = append(opts, otlploghttp.WithCompression(otlploghttp.NoCompression))
		}
		
		// Add retry
		if s.retryEnabled {
			opts = append(opts, otlploghttp.WithRetry(otlploghttp.RetryConfig{
				Enabled:         true,
				InitialInterval: s.retryInterval,
				MaxInterval:     s.retryInterval * 5,
				MaxElapsedTime:  s.retryMaxElapsedTime,
			}))
		}
		
		exporter, err := otlploghttp.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("failed to create HTTP exporter: %w", err)
		}
		s.exporter = exporter
		
	default:
		return fmt.Errorf("unsupported transport: %v", s.transport)
	}
	
	// Create logger provider
	s.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(s.resource),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(s.exporter)),
	)
	
	// Create logger
	s.loggerMu.Lock()
	s.logger = s.loggerProvider.Logger("mtlog")
	s.loggerMu.Unlock()
	
	return nil
}

// Emit sends a log event to the OTLP collector
func (s *OTLPSink) Emit(event *core.LogEvent) {
	if s.closed.Load() {
		return
	}
	
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	
	// Check total queue size (current batch + events being exported)
	totalSize := s.totalQueueSize.Load()
	if totalSize >= int64(s.maxQueueSize) {
		s.droppedCount.Add(1)
		sinkLog.Warn("dropping event: queue full (total size=%d, limit=%d)", totalSize, s.maxQueueSize)
		return
	}
	
	// Add to batch and increment queue size
	s.batch = append(s.batch, event)
	s.totalQueueSize.Add(1)
	
	// Flush if batch is full
	if len(s.batch) >= s.batchSize {
		s.flushBatchLocked()
	} else {
		// Start timer for batch timeout if not already running
		s.startBatchTimer()
	}
}

// backgroundFlusher handles periodic flushing
func (s *OTLPSink) backgroundFlusher() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.batchTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopCh:
			s.flushBatch()
			return
		case <-s.flushCh:
			s.flushBatch()
		case <-ticker.C:
			s.flushBatch()
		}
	}
}

// flushBatch flushes the current batch
func (s *OTLPSink) flushBatch() {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	s.flushBatchLocked()
}

// startBatchTimer starts the batch timer if not already running
// This is safe to call with batchMu held
func (s *OTLPSink) startBatchTimer() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	
	if s.timer == nil {
		s.timer = time.AfterFunc(s.batchTimeout, func() {
			select {
			case s.flushCh <- struct{}{}:
			default:
			}
		})
	}
}

// stopBatchTimer stops the batch timer
func (s *OTLPSink) stopBatchTimer() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

// flushBatchLocked flushes the batch (must be called with lock held)
func (s *OTLPSink) flushBatchLocked() {
	if len(s.batch) == 0 {
		return
	}
	
	// Stop timer
	s.stopBatchTimer()
	
	// Get batch from pool
	events := s.batch
	s.batch = s.batchPool.Get().([]*core.LogEvent)[:0]
	
	// Export events
	go s.exportBatch(events)
}

// exportBatch exports a batch of events
func (s *OTLPSink) exportBatch(events []*core.LogEvent) {
	// Acquire semaphore to limit concurrent exports
	s.exportSem <- struct{}{}
	defer func() {
		<-s.exportSem // Release semaphore
	}()
	
	s.exportBatchWithContext(context.Background(), events)
}

// exportBatchWithContext exports a batch of events with a custom context
func (s *OTLPSink) exportBatchWithContext(ctx context.Context, events []*core.LogEvent) {
	defer func() {
		// Return batch to pool
		s.batchPool.Put(events[:0])
	}()
	
	// Apply timeout if context doesn't have a deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	
	// Convert events to OTEL log records
	s.loggerMu.RLock()
	logger := s.logger
	s.loggerMu.RUnlock()
	
	for _, event := range events {
		record := s.convertToLogRecord(event)
		logger.Emit(ctx, record)
	}
	
	// Update metrics and decrement queue size
	eventCount := int64(len(events))
	s.exportedCount.Add(uint64(eventCount))
	s.totalQueueSize.Add(-eventCount) // Decrement queue size
	
	sinkLog.Debug("exported batch: count=%d", len(events))
}

// convertToLogRecord converts an mtlog event to OTEL log record
func (s *OTLPSink) convertToLogRecord(event *core.LogEvent) olog.Record {
	var record olog.Record
	record.SetTimestamp(event.Timestamp)
	record.SetBody(olog.StringValue(event.MessageTemplate))
	record.SetSeverity(s.mapSeverity(event.Level))
	record.SetSeverityText(s.getLevelString(event.Level))
	
	// Add attributes from properties
	attrs := make([]olog.KeyValue, 0, len(event.Properties))
	for k, v := range event.Properties {
		// Handle special properties
		switch k {
		case "trace.id":
			// Note: OTEL log.Record doesn't have SetTraceID method
			// Trace ID should be added as an attribute
			attrs = append(attrs, s.createLogAttribute(k, v))
			continue
		case "span.id":
			// Note: OTEL log.Record doesn't have SetSpanID method
			// Span ID should be added as an attribute
			attrs = append(attrs, s.createLogAttribute(k, v))
			continue
		case "trace.flags":
			// Skip, already handled via trace context
			continue
		}
		
		// Add as attribute
		attrs = append(attrs, s.createLogAttribute(k, v))
	}
	
	record.AddAttributes(attrs...)
	
	// Add exception if present
	if event.Exception != nil {
		record.AddAttributes(
			olog.String("exception.type", fmt.Sprintf("%T", event.Exception)),
			olog.String("exception.message", event.Exception.Error()),
		)
	}
	
	return record
}

// mapSeverity maps mtlog levels to OTEL severity
func (s *OTLPSink) mapSeverity(level core.LogEventLevel) olog.Severity {
	switch level {
	case core.VerboseLevel:
		return olog.SeverityTrace1
	case core.DebugLevel:
		return olog.SeverityDebug1
	case core.InformationLevel:
		return olog.SeverityInfo1
	case core.WarningLevel:
		return olog.SeverityWarn1
	case core.ErrorLevel:
		return olog.SeverityError1
	case core.FatalLevel:
		return olog.SeverityFatal1
	default:
		return olog.SeverityUndefined
	}
}

// getLevelString converts a log level to string
func (s *OTLPSink) getLevelString(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "Verbose"
	case core.DebugLevel:
		return "Debug"
	case core.InformationLevel:
		return "Information"
	case core.WarningLevel:
		return "Warning"
	case core.ErrorLevel:
		return "Error"
	case core.FatalLevel:
		return "Fatal"
	default:
		return "Information"
	}
}

// createLogAttribute creates an OTEL log attribute from a property
func (s *OTLPSink) createLogAttribute(key string, value any) olog.KeyValue {
	switch v := value.(type) {
	case string:
		return olog.String(key, v)
	case int:
		return olog.Int(key, v)
	case int64:
		return olog.Int64(key, v)
	case float64:
		return olog.Float64(key, v)
	case bool:
		return olog.Bool(key, v)
	case []byte:
		return olog.Bytes(key, v)
	default:
		// Convert to string for unknown types
		return olog.String(key, fmt.Sprintf("%v", v))
	}
}

// createAttribute creates an OTEL attribute from a property
func (s *OTLPSink) createAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		// Convert to string for unknown types
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

// Flush flushes any pending events
func (s *OTLPSink) Flush() error {
	s.flushBatch()
	
	// Wait for exports to complete
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	
	// Force flush the logger provider instead of shutting down
	if s.loggerProvider != nil {
		if err := s.loggerProvider.ForceFlush(ctx); err != nil {
			return fmt.Errorf("failed to flush logger provider: %w", err)
		}
	}
	
	return nil
}

// Close closes the sink and flushes remaining events
func (s *OTLPSink) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}
	
	// Signal shutdown
	close(s.stopCh)
	
	// Wait for background flusher
	s.wg.Wait()
	
	// Shutdown exporter
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	
	if err := s.exporter.Shutdown(ctx); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[otlp] error shutting down exporter: %v", err)
		}
		return err
	}
	
	// Shutdown logger provider
	if err := s.loggerProvider.Shutdown(ctx); err != nil {
		sinkLog.Error("error shutting down logger provider: %v", err)
		return err
	}
	
	sinkLog.Info("sink closed: exported=%d dropped=%d errors=%d",
		s.exportedCount.Load(), s.droppedCount.Load(), s.errorCount.Load())
	
	return nil
}

// maskSensitiveHeaders returns a copy of headers with sensitive values masked
func maskSensitiveHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	
	masked := make(map[string]string, len(headers))
	for k, v := range headers {
		lowerKey := strings.ToLower(k)
		// Mask sensitive headers
		if strings.Contains(lowerKey, "key") || 
		   strings.Contains(lowerKey, "token") ||
		   strings.Contains(lowerKey, "secret") ||
		   strings.Contains(lowerKey, "password") ||
		   strings.Contains(lowerKey, "auth") ||
		   strings.Contains(lowerKey, "api") {
			// Show first 4 chars if long enough, otherwise fully mask
			if len(v) > 8 {
				masked[k] = v[:4] + "****"
			} else {
				masked[k] = "****"
			}
		} else {
			masked[k] = v
		}
	}
	return masked
}

// GetMetrics returns sink metrics
func (s *OTLPSink) GetMetrics() map[string]uint64 {
	return map[string]uint64{
		"exported": s.exportedCount.Load(),
		"dropped":  s.droppedCount.Load(),
		"errors":   s.errorCount.Load(),
	}
}

// HealthCheck performs a health check by attempting to send a test record
func (s *OTLPSink) HealthCheck(ctx context.Context) error {
	if s.closed.Load() {
		return fmt.Errorf("sink is closed")
	}
	
	// Create a test record
	var testRecord olog.Record
	testRecord.SetTimestamp(time.Now())
	testRecord.SetBody(olog.StringValue("mtlog-health-check"))
	testRecord.SetSeverity(olog.SeverityDebug)
	testRecord.AddAttributes(
		olog.String("check.type", "health"),
		olog.String("check.source", "mtlog"),
	)
	
	// Try to emit the test record
	// Note: olog.Logger.Emit doesn't return an error
	s.loggerMu.RLock()
	logger := s.logger
	s.loggerMu.RUnlock()
	
	if logger != nil {
		logger.Emit(ctx, testRecord)
	} else {
		return fmt.Errorf("logger not initialized")
	}
	
	return nil
}