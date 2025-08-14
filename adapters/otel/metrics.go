package otel

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// metricsHandler serves Prometheus metrics from a custom registry
type metricsHandler struct {
	registry *prometheus.Registry
	path     string
}

// ServeHTTP implements http.Handler
func (h *metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Default path is /metrics
	path := h.path
	if path == "" {
		path = "/metrics"
	}
	
	// Only serve metrics on the configured path
	if r.URL.Path != path {
		http.NotFound(w, r)
		return
	}
	
	// Use promhttp with our custom registry
	promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// MetricsExporter exports mtlog metrics to Prometheus
type MetricsExporter struct {
	meter          metric.Meter
	provider       *sdkmetric.MeterProvider
	exporter       *otelprom.Exporter
	server         *http.Server
	registry       *prometheus.Registry
	
	// Metrics
	logCounter     metric.Int64Counter
	errorCounter   metric.Int64Counter
	droppedCounter metric.Int64Counter
	latencyHist    metric.Float64Histogram
	
	// Internal state
	mu             sync.RWMutex
	levelCounters  map[core.LogEventLevel]metric.Int64Counter
	closed         bool
}

// MetricsOption configures the metrics exporter
type MetricsOption func(*MetricsExporter)

// WithMetricsPort sets the HTTP port for Prometheus scraping
func WithMetricsPort(port int) MetricsOption {
	return func(e *MetricsExporter) {
		e.server.Addr = fmt.Sprintf(":%d", port)
	}
}

// WithMetricsPath sets the HTTP path for metrics endpoint
func WithMetricsPath(path string) MetricsOption {
	return func(e *MetricsExporter) {
		// Set the path on the handler
		if handler, ok := e.server.Handler.(*metricsHandler); ok {
			handler.path = path
		}
	}
}

// NewMetricsExporter creates a new Prometheus metrics exporter.
// Note: This function does not set the global meter provider to avoid
// conflicts with user applications. If you need global metrics, call
// otel.SetMeterProvider(exporter.provider) after creation.
func NewMetricsExporter(opts ...MetricsOption) (*MetricsExporter, error) {
	// Create a custom Prometheus registry
	registry := prometheus.NewRegistry()
	
	// Create Prometheus exporter with the custom registry
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	
	// Create resource
	res := resource.NewWithAttributes(
		"",
		attribute.String("service.name", "mtlog"),
		attribute.String("telemetry.sdk.name", "mtlog"),
		attribute.String("telemetry.sdk.language", "go"),
	)
	
	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)
	
	// Create meter
	meter := provider.Meter("mtlog")
	
	e := &MetricsExporter{
		meter:         meter,
		provider:      provider,
		exporter:      exporter,
		registry:      registry,
		levelCounters: make(map[core.LogEventLevel]metric.Int64Counter),
		server: &http.Server{
			Addr:         ":9090",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
	
	// Store the registry for use in HTTP handlers
	e.server.Handler = &metricsHandler{registry: registry}
	
	// Apply options
	for _, opt := range opts {
		opt(e)
	}
	
	// Create metrics
	if err := e.createMetrics(); err != nil {
		metricsLog.Error("failed to create metrics: %v", err)
		return nil, err
	}
	
	// Handler is already set above, no need to override
	
	// Start HTTP server
	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			metricsLog.Error("HTTP server error: %v", err)
		}
	}()
	
	return e, nil
}

// SetAsGlobal sets this exporter's meter provider as the global OTEL meter provider.
// This should only be called if your application doesn't already have a global
// meter provider configured.
func (e *MetricsExporter) SetAsGlobal() {
	otel.SetMeterProvider(e.provider)
}

// createMetrics initializes all metrics
func (e *MetricsExporter) createMetrics() error {
	var err error
	
	// Total log events counter
	e.logCounter, err = e.meter.Int64Counter(
		"mtlog_events_total",
		metric.WithDescription("Total number of log events"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create log counter: %w", err)
	}
	
	// Error events counter
	e.errorCounter, err = e.meter.Int64Counter(
		"mtlog_errors_total",
		metric.WithDescription("Total number of error events"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error counter: %w", err)
	}
	
	// Dropped events counter
	e.droppedCounter, err = e.meter.Int64Counter(
		"mtlog_dropped_total",
		metric.WithDescription("Total number of dropped events"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create dropped counter: %w", err)
	}
	
	// Processing latency histogram
	e.latencyHist, err = e.meter.Float64Histogram(
		"mtlog_latency_milliseconds",
		metric.WithDescription("Log event processing latency"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create latency histogram: %w", err)
	}
	
	// Create per-level counters
	levels := []core.LogEventLevel{
		core.VerboseLevel,
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}
	
	for _, level := range levels {
		levelStr := e.levelToString(level)
		counter, err := e.meter.Int64Counter(
			fmt.Sprintf("mtlog_events_%s_total", levelStr),
			metric.WithDescription(fmt.Sprintf("Total number of %s events", levelStr)),
			metric.WithUnit("1"),
		)
		if err != nil {
			return fmt.Errorf("failed to create %s counter: %w", levelStr, err)
		}
		e.levelCounters[level] = counter
	}
	
	return nil
}

// RecordEvent records metrics for a log event
func (e *MetricsExporter) RecordEvent(event *core.LogEvent, latencyMs float64) {
	if e.closed {
		return
	}
	
	ctx := context.Background()
	
	// Common attributes
	attrs := []attribute.KeyValue{
		attribute.String("level", e.levelToString(event.Level)),
	}
	
	// Add trace ID if present
	if traceID, ok := event.Properties["trace.id"].(string); ok && traceID != "" {
		attrs = append(attrs, attribute.String("trace_id", traceID))
	}
	
	// Total counter
	e.logCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	
	// Level-specific counter
	e.mu.RLock()
	if counter, ok := e.levelCounters[event.Level]; ok {
		counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	e.mu.RUnlock()
	
	// Error counter
	if event.Level >= core.ErrorLevel {
		e.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	
	// Latency histogram
	if latencyMs > 0 {
		e.latencyHist.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
	}
}

// RecordDropped records a dropped event
func (e *MetricsExporter) RecordDropped(reason string) {
	if e.closed {
		return
	}
	
	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("reason", reason),
	}
	
	e.droppedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// Start starts the metrics HTTP server
func (e *MetricsExporter) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.closed {
		return fmt.Errorf("exporter is closed")
	}
	
	// Start HTTP server in background
	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			metricsLog.Error("HTTP server error: %v", err)
		}
	}()
	
	metricsLog.Info("metrics server started on %s", e.server.Addr)
	return nil
}

// Stop stops the metrics HTTP server
func (e *MetricsExporter) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.closed {
		return nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := e.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}
	
	e.closed = true
	return nil
}

// levelToString converts log level to string for metrics
func (e *MetricsExporter) levelToString(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "verbose"
	case core.DebugLevel:
		return "debug"
	case core.InformationLevel:
		return "info"
	case core.WarningLevel:
		return "warning"
	case core.ErrorLevel:
		return "error"
	case core.FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

// Close shuts down the metrics exporter
func (e *MetricsExporter) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	e.mu.Unlock()
	
	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := e.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}
	
	// Shutdown meter provider
	if err := e.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}
	
	return nil
}

// MetricsSink is a sink that records metrics for all events
type MetricsSink struct {
	inner    core.LogEventSink
	exporter *MetricsExporter
}

// NewMetricsSink creates a sink that records metrics
func NewMetricsSink(inner core.LogEventSink, exporter *MetricsExporter) *MetricsSink {
	return &MetricsSink{
		inner:    inner,
		exporter: exporter,
	}
}

// Emit records metrics and forwards the event
func (s *MetricsSink) Emit(event *core.LogEvent) {
	start := time.Now()
	
	// Forward to inner sink
	s.inner.Emit(event)
	
	// Record metrics
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	s.exporter.RecordEvent(event, latency)
}

// Close closes the metrics sink
func (s *MetricsSink) Close() error {
	// No cleanup needed for metrics sink itself
	return nil
}

// WithPrometheusMetrics adds Prometheus metrics export
func WithPrometheusMetrics(port int) OTLPOption {
	return func(s *OTLPSink) {
		// Create metrics exporter
		exporter, err := NewMetricsExporter(
			WithMetricsPort(port),
			WithMetricsPath("/metrics"),
		)
		if err != nil {
			metricsLog.Error("failed to create metrics exporter for OTLP sink: %v", err)
			return
		}
		
		// Store metrics exporter and start the server
		s.metricsExporter = exporter
		go exporter.Start()
	}
}