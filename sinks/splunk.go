package sinks

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/selflog"
)

// SplunkSink writes log events to Splunk HTTP Event Collector (HEC)
type SplunkSink struct {
	url        string
	token      string
	client     *http.Client
	index      string
	source     string
	sourceType string
	host       string

	// Batching configuration
	batchSize    int
	batchTimeout time.Duration

	// Internal state
	batch   []*core.LogEvent
	batchMu sync.Mutex
	stopCh  chan struct{}
	flushCh chan struct{}
	wg      sync.WaitGroup
	closed  sync.Once
}

// SplunkOption configures a Splunk sink
type SplunkOption func(*SplunkSink)

// WithSplunkIndex sets the Splunk index
func WithSplunkIndex(index string) SplunkOption {
	return func(s *SplunkSink) {
		s.index = index
	}
}

// WithSplunkSource sets the source field
func WithSplunkSource(source string) SplunkOption {
	return func(s *SplunkSink) {
		s.source = source
	}
}

// WithSplunkSourceType sets the sourcetype field
func WithSplunkSourceType(sourceType string) SplunkOption {
	return func(s *SplunkSink) {
		s.sourceType = sourceType
	}
}

// WithSplunkHost sets the host field
func WithSplunkHost(host string) SplunkOption {
	return func(s *SplunkSink) {
		s.host = host
	}
}

// WithSplunkBatchSize sets the batch size
func WithSplunkBatchSize(size int) SplunkOption {
	return func(s *SplunkSink) {
		s.batchSize = size
	}
}

// WithSplunkBatchTimeout sets the batch timeout
func WithSplunkBatchTimeout(timeout time.Duration) SplunkOption {
	return func(s *SplunkSink) {
		s.batchTimeout = timeout
	}
}

// WithSplunkHTTPClient sets a custom HTTP client
func WithSplunkHTTPClient(client *http.Client) SplunkOption {
	return func(s *SplunkSink) {
		s.client = client
	}
}

// NewSplunkSink creates a new Splunk sink
func NewSplunkSink(url, token string, opts ...SplunkOption) (*SplunkSink, error) {
	if url == "" {
		return nil, fmt.Errorf("Splunk URL is required")
	}
	if token == "" {
		return nil, fmt.Errorf("Splunk HEC token is required")
	}

	// Ensure URL ends with the HEC endpoint
	if !strings.HasSuffix(url, "/services/collector/event") && !strings.HasSuffix(url, "/services/collector") {
		url = strings.TrimRight(url, "/") + "/services/collector/event"
	}

	// Create HTTP client with relaxed TLS config for self-signed certs
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // For test environments with self-signed certs
		},
	}

	sink := &SplunkSink{
		url:          url,
		token:        token,
		client:       &http.Client{Transport: transport, Timeout: 30 * time.Second},
		sourceType:   "mtlog",
		batchSize:    100,
		batchTimeout: 5 * time.Second,
		batch:        make([]*core.LogEvent, 0),
		stopCh:       make(chan struct{}),
		flushCh:      make(chan struct{}, 1),
	}

	// Apply options
	for _, opt := range opts {
		opt(sink)
	}

	// Start background worker
	sink.wg.Add(1)
	go sink.worker()

	return sink, nil
}

// Emit writes a log event to the sink
func (s *SplunkSink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}

	s.batchMu.Lock()
	s.batch = append(s.batch, event)
	shouldFlush := len(s.batch) >= s.batchSize
	s.batchMu.Unlock()

	if shouldFlush {
		select {
		case s.flushCh <- struct{}{}:
		default:
		}
	}
}

// Close closes the sink
func (s *SplunkSink) Close() error {
	var err error
	s.closed.Do(func() {
		close(s.stopCh)
		s.wg.Wait()
	})
	return err
}

// worker handles batching and sending
func (s *SplunkSink) worker() {
	defer s.wg.Done()

	timer := time.NewTimer(s.batchTimeout)
	defer timer.Stop()

	for {
		select {
		case <-s.stopCh:
			s.flush()
			return
		case <-timer.C:
			s.flush()
			timer.Reset(s.batchTimeout)
		case <-s.flushCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			s.flush()
			timer.Reset(s.batchTimeout)
		}
	}
}

// flush sends the current batch
func (s *SplunkSink) flush() {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	if len(s.batch) == 0 {
		return
	}

	// Send the batch directly without copying to avoid race conditions
	s.sendBatch(s.batch)
	s.batch = make([]*core.LogEvent, 0)
}

// sendBatch sends a batch of events to Splunk
func (s *SplunkSink) sendBatch(events []*core.LogEvent) {
	if len(events) == 0 {
		return
	}

	// Build request body
	var buf bytes.Buffer
	for _, event := range events {
		hecEvent := s.formatEvent(event)

		// Try JSON encoding with error handling
		if data, err := json.Marshal(hecEvent); err == nil {
			buf.Write(data)
			buf.WriteByte('\n')
		} else {
			if selflog.IsEnabled() {
				selflog.Printf("[splunk] failed to marshal HEC event: %v", err)
			}
			// Fallback: create a simplified event if JSON marshaling fails
			fallbackEvent := map[string]any{
				"time":  event.Timestamp.Unix(),
				"event": event.MessageTemplate,
				"fields": map[string]any{
					"level":           levelToString(event.Level),
					"messageTemplate": event.MessageTemplate,
				},
			}
			if fallbackData, fallbackErr := json.Marshal(fallbackEvent); fallbackErr == nil {
				buf.Write(fallbackData)
				buf.WriteByte('\n')
			}
		}
	}

	// Send with retry logic
	for i := 0; i < 3; i++ {
		if s.sendRequest(buf.Bytes()) {
			break
		}
		if i == 2 && selflog.IsEnabled() {
			selflog.Printf("[splunk] failed to send batch of %d events after 3 attempts", len(events))
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
}

// sendRequest sends the request to Splunk
func (s *SplunkSink) sendRequest(body []byte) bool {
	req, err := http.NewRequest("POST", s.url, bytes.NewReader(body))
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[splunk] failed to create request: %v (url=%s)", err, s.url)
		}
		return false
	}

	req.Header.Set("Authorization", "Splunk "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[splunk] HTTP request failed: %v (url=%s)", err, s.url)
		}
		return false
	}
	defer resp.Body.Close()

	// Splunk HEC returns 2xx for success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}

	// Log non-success status codes
	if selflog.IsEnabled() {
		selflog.Printf("[splunk] HTTP request failed with status %d (url=%s)", resp.StatusCode, s.url)
	}
	return false
}

// formatEvent formats a log event for Splunk HEC
func (s *SplunkSink) formatEvent(event *core.LogEvent) map[string]any {
	var message string

	// Safe template rendering with fallbacks
	func() {
		defer func() {
			if r := recover(); r != nil {
				message = event.MessageTemplate // Fallback on panic
			}
		}()

		tmpl, err := parser.Parse(event.MessageTemplate)
		if err != nil {
			message = event.MessageTemplate // Fallback on parse error
			return
		}

		message = tmpl.Render(event.Properties)
		if message == "" {
			message = event.MessageTemplate // Fallback on empty render
		}
	}()

	// For JSON sourcetype, send as structured JSON with fields at top level
	if s.sourceType == "json" || s.sourceType == "_json" {
		// Create a flat JSON event with all fields at the top level
		hecEvent := map[string]any{
			"time": event.Timestamp.Unix(),
		}

		// Add metadata
		if s.index != "" {
			hecEvent["index"] = s.index
		}
		if s.source != "" {
			hecEvent["source"] = s.source
		}
		hecEvent["sourcetype"] = "_json" // Force _json for automatic field extraction
		if s.host != "" {
			hecEvent["host"] = s.host
		}

		// Create structured event data with message and all properties at top level
		eventData := map[string]any{
			"message":         message,
			"messageTemplate": event.MessageTemplate,
			"level":           levelToString(event.Level),
			"@timestamp":      event.Timestamp.Format("2006-01-02T15:04:05.000Z"),
		}

		// Add all properties directly to event data with JSON safety
		for k, v := range event.Properties {
			if jsonSafeValue := s.makeJSONSafe(v); jsonSafeValue != nil {
				// Replace dots with underscores in field names for better Splunk compatibility
				fieldName := strings.Replace(k, ".", "_", -1)
				eventData[fieldName] = jsonSafeValue
			}
		}

		// Add error field if present
		if err, ok := event.Properties["Error"]; ok {
			if errStr, ok := err.(string); ok {
				eventData["error"] = errStr
			} else if errObj, ok := err.(error); ok {
				eventData["error"] = errObj.Error()
			}
		}

		hecEvent["event"] = eventData
		return hecEvent
	}

	// Standard HEC format for non-JSON sourcetypes
	hecEvent := map[string]any{
		"time":  event.Timestamp.Unix(),
		"event": message,
	}

	// Add optional metadata
	if s.index != "" {
		hecEvent["index"] = s.index
	}
	if s.source != "" {
		hecEvent["source"] = s.source
	}
	if s.sourceType != "" {
		hecEvent["sourcetype"] = s.sourceType
	}
	if s.host != "" {
		hecEvent["host"] = s.host
	}

	// Add fields (properties)
	fields := make(map[string]any)

	// Add standard fields
	fields["level"] = levelToString(event.Level)
	fields["messageTemplate"] = event.MessageTemplate

	// Add all properties with JSON safety
	for k, v := range event.Properties {
		// Ensure property is JSON-safe
		if jsonSafeValue := s.makeJSONSafe(v); jsonSafeValue != nil {
			fields[k] = jsonSafeValue
		}
	}

	// Add error field if present
	if err, ok := event.Properties["Error"]; ok {
		if errStr, ok := err.(string); ok {
			fields["error"] = errStr
		} else if errObj, ok := err.(error); ok {
			fields["error"] = errObj.Error()
		}
	}

	hecEvent["fields"] = fields

	return hecEvent
}

// makeJSONSafe ensures a value can be safely JSON marshaled and is Splunk HEC compatible
func (s *SplunkSink) makeJSONSafe(v any) any {
	if v == nil {
		return nil
	}

	// Check if it's a simple type that Splunk can handle as indexed fields
	switch v.(type) {
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return v // Primitive types are fine
	case []string, []int, []float64:
		return v // Simple arrays are fine
	default:
		// For complex objects (structs, complex maps, etc.), convert to string
		// This prevents Splunk HEC "Error in handling indexed fields" (code 15)
		return fmt.Sprintf("%+v", v)
	}
}
