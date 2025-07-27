package sinks

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/formatters"
)

// SeqSink writes log events to Seq
type SeqSink struct {
	serverURL string
	apiKey    string
	client    *http.Client
	formatter *formatters.CLEFFormatter

	// Batching configuration
	batchSize    int
	batchTimeout time.Duration

	// Internal state
	batch   []*core.LogEvent
	batchMu sync.Mutex
	timer   *time.Timer
	stopCh  chan struct{}
	flushCh chan struct{}
	wg      sync.WaitGroup

	// Options
	useCompression bool
	retryCount     int
	retryDelay     time.Duration
}

// SeqOption configures a Seq sink
type SeqOption func(*SeqSink)

// WithSeqAPIKey sets the API key for authentication
func WithSeqAPIKey(apiKey string) SeqOption {
	return func(s *SeqSink) {
		s.apiKey = apiKey
	}
}

// WithSeqBatchSize sets the batch size
func WithSeqBatchSize(size int) SeqOption {
	return func(s *SeqSink) {
		s.batchSize = size
	}
}

// WithSeqBatchTimeout sets the batch timeout
func WithSeqBatchTimeout(timeout time.Duration) SeqOption {
	return func(s *SeqSink) {
		s.batchTimeout = timeout
	}
}

// WithSeqCompression enables gzip compression
func WithSeqCompression(enabled bool) SeqOption {
	return func(s *SeqSink) {
		s.useCompression = enabled
	}
}

// WithSeqRetry configures retry behavior
func WithSeqRetry(count int, delay time.Duration) SeqOption {
	return func(s *SeqSink) {
		s.retryCount = count
		s.retryDelay = delay
	}
}

// WithSeqHTTPClient sets a custom HTTP client
func WithSeqHTTPClient(client *http.Client) SeqOption {
	return func(s *SeqSink) {
		s.client = client
	}
}

// NewSeqSink creates a new Seq sink
func NewSeqSink(serverURL string, opts ...SeqOption) (*SeqSink, error) {
	s := &SeqSink{
		serverURL:      serverURL,
		client:         &http.Client{Timeout: 30 * time.Second},
		formatter:      formatters.NewCLEFFormatter(),
		batchSize:      100,
		batchTimeout:   5 * time.Second,
		batch:          make([]*core.LogEvent, 0),
		stopCh:         make(chan struct{}),
		flushCh:        make(chan struct{}, 1),
		useCompression: false, // Disabled by default as Seq may not support it
		retryCount:     3,
		retryDelay:     time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Start background flusher
	s.wg.Add(1)
	go s.backgroundFlusher()

	return s, nil
}

// Emit adds an event to the batch
func (s *SeqSink) Emit(event *core.LogEvent) {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	s.batch = append(s.batch, event)

	// Check if we should flush
	if len(s.batch) >= s.batchSize {
		select {
		case s.flushCh <- struct{}{}:
		default:
			// Flush already signaled
		}
	}

	// Reset timer
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.batchTimeout, func() {
		select {
		case s.flushCh <- struct{}{}:
		default:
		}
	})
}

// backgroundFlusher handles periodic flushing
func (s *SeqSink) backgroundFlusher() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			// Final flush before stopping
			s.flush()
			return
		case <-s.flushCh:
			s.flush()
		}
	}
}

// flush sends the current batch to Seq
func (s *SeqSink) flush() error {
	s.batchMu.Lock()
	if len(s.batch) == 0 {
		s.batchMu.Unlock()
		return nil
	}

	// Copy and clear batch
	events := make([]*core.LogEvent, len(s.batch))
	copy(events, s.batch)
	s.batch = s.batch[:0]
	s.batchMu.Unlock()

	// Format events
	payload, err := formatters.FormatForSeqIngestion(events)
	if err != nil {
		return fmt.Errorf("failed to format events: %w", err)
	}

	// Send with retries
	for attempt := 0; attempt <= s.retryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(s.retryDelay * time.Duration(attempt))
		}

		if err := s.sendToSeq(payload); err == nil {
			return nil
		} else if attempt == s.retryCount {
			return fmt.Errorf("failed to send to Seq after %d attempts: %w", s.retryCount+1, err)
		}
	}

	return nil
}

// sendToSeq sends the payload to Seq
func (s *SeqSink) sendToSeq(payload []byte) error {
	url := s.serverURL + "/api/events/raw"

	var body io.Reader = bytes.NewReader(payload)
	contentType := "application/vnd.serilog.clef"

	// Apply compression if enabled
	if s.useCompression {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(payload); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		body = &buf
		contentType = "application/vnd.serilog.clef+gzip"
	}

	// Create request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	if s.apiKey != "" {
		req.Header.Set("X-Seq-ApiKey", s.apiKey)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Read error response
	bodyBytes, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("seq error (status %d): %s", resp.StatusCode, errResp.Error)
	}

	return fmt.Errorf("seq returned status %d: %s", resp.StatusCode, string(bodyBytes))
}

// Close flushes remaining events and stops the sink
func (s *SeqSink) Close() error {
	// Stop timer
	if s.timer != nil {
		s.timer.Stop()
	}

	// Signal stop
	close(s.stopCh)

	// Wait for background flusher
	s.wg.Wait()

	return nil
}

// SeqHealthCheck checks if Seq is healthy
func (s *SeqSink) SeqHealthCheck() error {
	resp, err := s.client.Get(s.serverURL + "/api/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("seq health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetMinimumLevel queries Seq for the current minimum level by sending a test event
// and reading the MinimumLevelAccepted from the response
func (s *SeqSink) GetMinimumLevel() (core.LogEventLevel, error) {
	// Send a minimal test event to get the MinimumLevelAccepted response
	testEvent := `{"@t":"` + time.Now().Format(time.RFC3339Nano) + `","@mt":"Level probe","@l":"Information"}`
	
	url := s.serverURL + "/api/events/raw"
	if s.apiKey != "" {
		url += "?apiKey=" + s.apiKey
	}

	resp, err := s.client.Post(url, "application/vnd.serilog.clef", strings.NewReader(testEvent))
	if err != nil {
		return core.InformationLevel, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return core.InformationLevel, fmt.Errorf("failed to get minimum level: status %d", resp.StatusCode)
	}

	// Check for MinimumLevelAccepted header
	if minLevel := resp.Header.Get("X-Seq-MinimumLevelAccepted"); minLevel != "" {
		return parseSeqLevel(minLevel), nil
	}

	// If no minimum level header, return default
	return core.InformationLevel, nil
}

// parseSeqLevel converts Seq level string to LogEventLevel
func parseSeqLevel(level string) core.LogEventLevel {
	switch level {
	case "Verbose":
		return core.VerboseLevel
	case "Debug":
		return core.DebugLevel
	case "Information":
		return core.InformationLevel
	case "Warning":
		return core.WarningLevel
	case "Error":
		return core.ErrorLevel
	case "Fatal":
		return core.FatalLevel
	default:
		return core.InformationLevel
	}
}
