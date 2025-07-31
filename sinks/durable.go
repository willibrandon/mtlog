package sinks

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// DurableOptions configures the durable buffer sink.
type DurableOptions struct {
	// BufferPath is the directory where buffer files are stored.
	BufferPath string

	// MaxBufferSize is the maximum size in bytes of the buffer file.
	MaxBufferSize int64

	// MaxBufferFiles is the maximum number of buffer files to keep.
	MaxBufferFiles int

	// RetryInterval is how often to attempt delivery to the wrapped sink.
	RetryInterval time.Duration

	// BatchSize is the number of events to process in each retry batch.
	BatchSize int

	// OnError is called when an error occurs in the background worker.
	OnError func(error)

	// ShutdownTimeout is the maximum time to wait during shutdown.
	ShutdownTimeout time.Duration

	// FlushInterval is how often to flush the buffer file to disk.
	FlushInterval time.Duration

	// ChannelBufferSize is the size of the internal event channel.
	// Defaults to 10000 if not specified.
	ChannelBufferSize int
}

// DurableSink wraps another sink to provide persistent buffering when the sink fails.
type DurableSink struct {
	wrapped core.LogEventSink
	options DurableOptions

	// File management
	bufferFile  *os.File
	encoder     *json.Encoder
	currentSize atomic.Int64
	fileIndex   atomic.Int32
	fileMu      sync.Mutex

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Event channels
	events       chan *core.LogEvent
	flushTrigger chan struct{}

	// State tracking
	sinkHealthy atomic.Bool
	lastCheck   atomic.Int64

	// Metrics
	buffered  atomic.Uint64
	delivered atomic.Uint64
	dropped   atomic.Uint64
	retries   atomic.Uint64
}

// BufferedLogEvent represents a persisted log event with metadata.
type BufferedLogEvent struct {
	Event     *core.LogEvent `json:"event"`
	Timestamp int64          `json:"timestamp"`
	Sequence  uint64         `json:"sequence"`
}

// NewDurableSink creates a new durable buffer sink.
func NewDurableSink(wrapped core.LogEventSink, options DurableOptions) (*DurableSink, error) {
	// Apply defaults
	if options.BufferPath == "" {
		options.BufferPath = filepath.Join(os.TempDir(), "mtlog-buffer")
	}
	if options.MaxBufferSize <= 0 {
		options.MaxBufferSize = 100 * 1024 * 1024 // 100MB
	}
	if options.MaxBufferFiles <= 0 {
		options.MaxBufferFiles = 10
	}
	if options.RetryInterval <= 0 {
		options.RetryInterval = 30 * time.Second
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 30 * time.Second
	}
	if options.FlushInterval <= 0 {
		options.FlushInterval = 5 * time.Second
	}
	if options.ChannelBufferSize <= 0 {
		options.ChannelBufferSize = 10000
	}
	if options.OnError == nil {
		options.OnError = func(err error) {
			fmt.Printf("DurableSink error: %v\n", err)
		}
	}

	// Ensure buffer directory exists
	if err := os.MkdirAll(options.BufferPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create buffer directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sink := &DurableSink{
		wrapped:      wrapped,
		options:      options,
		ctx:          ctx,
		cancel:       cancel,
		events:       make(chan *core.LogEvent, options.ChannelBufferSize),
		flushTrigger: make(chan struct{}, 1),
	}

	// Initialize sink health state
	sink.sinkHealthy.Store(true)
	sink.lastCheck.Store(time.Now().UnixNano())

	// Initialize buffer file
	if err := sink.initBufferFile(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize buffer file: %w", err)
	}

	// Start background workers
	sink.wg.Add(3)
	go sink.eventWorker()
	go sink.retryWorker()
	go sink.flushWorker()

	// Recover any existing buffered events on startup
	go sink.recoverBufferedEvents()

	return sink, nil
}

// Emit sends a log event through the durable buffer.
func (ds *DurableSink) Emit(event *core.LogEvent) {
	select {
	case ds.events <- event:
		// Event queued successfully
	default:
		// Channel full, drop event and increment counter
		ds.dropped.Add(1)
		if ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("event dropped due to full buffer"))
		}
	}
}

// Close shuts down the durable sink and flushes remaining events.
func (ds *DurableSink) Close() error {
	// Signal shutdown
	ds.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		ds.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-time.After(ds.options.ShutdownTimeout):
		// Forced shutdown
		if ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("shutdown timeout exceeded"))
		}
	}

	// Close buffer file properly
	ds.fileMu.Lock()
	if ds.bufferFile != nil {
		if err := ds.bufferFile.Sync(); err != nil && ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to sync buffer file during close: %w", err))
		}
		if err := ds.bufferFile.Close(); err != nil && ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to close buffer file: %w", err))
		}
		ds.bufferFile = nil
	}
	ds.fileMu.Unlock()

	// Close wrapped sink
	return ds.wrapped.Close()
}

// eventWorker handles incoming events and attempts direct delivery or buffering.
func (ds *DurableSink) eventWorker() {
	defer ds.wg.Done()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case event := <-ds.events:
			ds.processEvent(event)
		}
	}
}

// processEvent attempts to deliver an event or buffer it on failure.
func (ds *DurableSink) processEvent(event *core.LogEvent) {
	// Try direct delivery first if sink is healthy
	if ds.sinkHealthy.Load() {
		if ds.tryDeliverEvent(event) {
			ds.delivered.Add(1)
			return
		}

		// Sink failed, mark as unhealthy
		ds.sinkHealthy.Store(false)
	}

	// Buffer the event
	if err := ds.bufferEvent(event); err != nil {
		ds.dropped.Add(1)
		if ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to buffer event: %w", err))
		}
	} else {
		ds.buffered.Add(1)
	}
}

// tryDeliverEvent attempts to deliver an event to the wrapped sink.
func (ds *DurableSink) tryDeliverEvent(event *core.LogEvent) bool {
	defer func() {
		if r := recover(); r != nil {
			// Sink panicked, consider it unhealthy
			if selflog.IsEnabled() {
				selflog.Printf("[durable] wrapped sink panicked: %v", r)
			}
			ds.sinkHealthy.Store(false)
		}
	}()

	// For HTTP-based sinks that use batching (Seq, Elasticsearch, Splunk),
	// test connectivity first since Emit() always succeeds
	if seqSink, ok := ds.wrapped.(*SeqSink); ok {
		if !ds.testSeqConnectivity(seqSink) {
			return false
		}
	} else if esSink, ok := ds.wrapped.(*ElasticsearchSink); ok {
		if !ds.testElasticsearchConnectivity(esSink) {
			return false
		}
	} else if splunkSink, ok := ds.wrapped.(*SplunkSink); ok {
		if !ds.testSplunkConnectivity(splunkSink) {
			return false
		}
	}

	// Use a timeout to prevent hanging
	done := make(chan struct{})
	var success bool

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Sink panicked, mark as unhealthy
				if selflog.IsEnabled() {
					selflog.Printf("[durable] wrapped sink panicked: %v", r)
				}
				ds.sinkHealthy.Store(false)
				if ds.options.OnError != nil {
					ds.options.OnError(fmt.Errorf("sink panicked: %v", r))
				}
			}
			close(done)
		}()

		ds.wrapped.Emit(event)
		success = true
	}()

	select {
	case <-done:
		return success
	case <-time.After(5 * time.Second):
		// Delivery timeout
		return false
	}
}

// testSeqConnectivity tests if we can connect to the Seq server
func (ds *DurableSink) testSeqConnectivity(sink *SeqSink) bool {
	// Simple HTTP HEAD request to test connectivity
	client := &http.Client{Timeout: 2 * time.Second}

	// Extract the server URL from the sink (we need to access the private field)
	// For now, let's try a simple approach - attempt to reach the health endpoint
	serverURL := ds.extractSeqURL(sink)
	if serverURL == "" {
		return false
	}

	resp, err := client.Get(serverURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Any response (even 404) means the server is reachable
	return resp.StatusCode < 500
}

// testElasticsearchConnectivity tests Elasticsearch connectivity
func (ds *DurableSink) testElasticsearchConnectivity(sink *ElasticsearchSink) bool {
	// Similar connectivity test for Elasticsearch
	client := &http.Client{Timeout: 2 * time.Second}

	esURL := ds.extractElasticsearchURL(sink)
	if esURL == "" {
		return false
	}

	resp, err := client.Get(esURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// testSplunkConnectivity tests Splunk HEC connectivity
func (ds *DurableSink) testSplunkConnectivity(sink *SplunkSink) bool {
	// Test Splunk HEC health endpoint
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	splunkURL := ds.extractSplunkURL(sink)
	if splunkURL == "" {
		return false
	}

	// Test HEC health endpoint
	healthURL := strings.Replace(splunkURL, "/services/collector/event", "/services/collector/health", 1)
	if !strings.Contains(healthURL, "/services/collector/health") {
		healthURL = strings.TrimRight(splunkURL, "/") + "/services/collector/health"
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// Helper functions to extract URLs from sinks
// These use reflection since the fields are private
func (ds *DurableSink) extractSeqURL(sink *SeqSink) string {
	// Use reflection to get the serverURL field
	v := reflect.ValueOf(sink).Elem()
	field := v.FieldByName("serverURL")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func (ds *DurableSink) extractElasticsearchURL(sink *ElasticsearchSink) string {
	// Use reflection to get the url field
	v := reflect.ValueOf(sink).Elem()
	field := v.FieldByName("url")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func (ds *DurableSink) extractSplunkURL(sink *SplunkSink) string {
	// Use reflection to get the url field
	v := reflect.ValueOf(sink).Elem()
	field := v.FieldByName("url")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

// bufferEvent persists an event to the buffer file.
func (ds *DurableSink) bufferEvent(event *core.LogEvent) error {
	ds.fileMu.Lock()
	defer ds.fileMu.Unlock()

	// Check if we need to rotate the buffer file
	if ds.currentSize.Load() >= ds.options.MaxBufferSize {
		if err := ds.rotateBufferFile(); err != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[durable] failed to rotate buffer file: %v", err)
			}
			return fmt.Errorf("failed to rotate buffer file: %w", err)
		}
	}

	bufferedEvent := &BufferedLogEvent{
		Event:     event,
		Timestamp: time.Now().UnixNano(),
		Sequence:  ds.buffered.Load(),
	}

	if err := ds.encoder.Encode(bufferedEvent); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[durable] failed to encode event: %v", err)
		}
		return fmt.Errorf("failed to encode event: %w", err)
	}

	// Update size (approximate)
	ds.currentSize.Add(200) // Rough estimate

	// Trigger flush
	select {
	case ds.flushTrigger <- struct{}{}:
	default:
	}

	return nil
}

// retryWorker periodically attempts to deliver buffered events.
func (ds *DurableSink) retryWorker() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.options.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.attemptRetry()
		}
	}
}

// attemptRetry tries to deliver buffered events.
func (ds *DurableSink) attemptRetry() {
	// Test sink health
	testEvent := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "health check",
		Properties:      make(map[string]any),
	}

	if !ds.tryDeliverEvent(testEvent) {
		return // Sink still unhealthy
	}

	// Sink is healthy, process buffered events
	ds.sinkHealthy.Store(true)
	ds.retries.Add(1)

	// Process buffer files
	bufferFiles, err := ds.getBufferFiles()
	if err != nil {
		if ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to get buffer files: %w", err))
		}
		return
	}

	// Get current buffer file name to exclude from retry processing
	ds.fileMu.Lock()
	currentBufferFile := ""
	if ds.bufferFile != nil {
		currentBufferFile = ds.getBufferFileName(int(ds.fileIndex.Load()))
	}
	ds.fileMu.Unlock()

	for _, filename := range bufferFiles {
		// Skip the currently open buffer file
		if filename == currentBufferFile {
			continue
		}

		if err := ds.processBufferFile(filename); err != nil {
			if ds.options.OnError != nil {
				ds.options.OnError(fmt.Errorf("failed to process buffer file %s: %w", filename, err))
			}
			break // Stop processing on error
		}
	}
}

// flushWorker periodically flushes the buffer file.
func (ds *DurableSink) flushWorker() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.options.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			ds.flushBufferFile()
			return
		case <-ticker.C:
			ds.flushBufferFile()
		case <-ds.flushTrigger:
			ds.flushBufferFile()
		}
	}
}

// flushBufferFile forces a flush of the buffer file to disk.
func (ds *DurableSink) flushBufferFile() {
	ds.fileMu.Lock()
	defer ds.fileMu.Unlock()

	if ds.bufferFile != nil {
		if err := ds.bufferFile.Sync(); err != nil && ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to sync buffer file: %w", err))
		}
	}
}

// initBufferFile creates the initial buffer file.
func (ds *DurableSink) initBufferFile() error {
	// Find the next available buffer file index
	nextIndex := ds.getNextBufferFileIndex()
	ds.fileIndex.Store(int32(nextIndex))

	filename := ds.getBufferFileName(nextIndex)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	ds.bufferFile = file
	ds.encoder = json.NewEncoder(file)

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		return err
	}
	ds.currentSize.Store(info.Size())

	return nil
}

// getNextBufferFileIndex finds the next available buffer file index.
func (ds *DurableSink) getNextBufferFileIndex() int {
	files, err := ds.getBufferFiles()
	if err != nil || len(files) == 0 {
		return 0
	}

	// Find the highest existing index and increment
	maxIndex := -1
	for _, file := range files {
		base := filepath.Base(file)
		var index int
		if n, err := fmt.Sscanf(base, "buffer-%06d.jsonl", &index); n == 1 && err == nil {
			if index > maxIndex {
				maxIndex = index
			}
		}
	}

	return maxIndex + 1
}

// rotateBufferFile creates a new buffer file.
func (ds *DurableSink) rotateBufferFile() error {
	// Close current file
	if ds.bufferFile != nil {
		ds.bufferFile.Close()
	}

	// Increment file index
	newIndex := ds.fileIndex.Add(1)

	// Create new file
	filename := ds.getBufferFileName(int(newIndex))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	ds.bufferFile = file
	ds.encoder = json.NewEncoder(file)
	ds.currentSize.Store(0)

	// Clean up old files
	ds.cleanupOldBufferFiles()

	return nil
}

// getBufferFileName generates a buffer file name.
func (ds *DurableSink) getBufferFileName(index int) string {
	return filepath.Join(ds.options.BufferPath, fmt.Sprintf("buffer-%06d.jsonl", index))
}

// getBufferFiles returns all buffer files sorted by creation time.
func (ds *DurableSink) getBufferFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(ds.options.BufferPath, "buffer-*.jsonl"))
	if err != nil {
		return nil, err
	}
	return files, nil
}

// processBufferFile processes events from a buffer file.
func (ds *DurableSink) processBufferFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[durable] failed to open buffer file: %v (path=%s)", err, filename)
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	batch := make([]*core.LogEvent, 0, ds.options.BatchSize)

	for scanner.Scan() {
		var bufferedEvent BufferedLogEvent
		if err := json.Unmarshal(scanner.Bytes(), &bufferedEvent); err != nil {
			// Log malformed events sparingly
			if selflog.IsEnabled() {
				selflog.Printf("[durable] skipping malformed event in buffer: %v", err)
			}
			continue // Skip malformed events
		}

		batch = append(batch, bufferedEvent.Event)

		if len(batch) >= ds.options.BatchSize {
			if !ds.deliverBatch(batch) {
				if selflog.IsEnabled() {
					selflog.Printf("[durable] batch delivery failed for %d events", len(batch))
				}
				return fmt.Errorf("batch delivery failed")
			}
			batch = batch[:0]
		}
	}

	// Deliver remaining events
	if len(batch) > 0 {
		if !ds.deliverBatch(batch) {
			if selflog.IsEnabled() {
				selflog.Printf("[durable] final batch delivery failed for %d events", len(batch))
			}
			return fmt.Errorf("final batch delivery failed")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// File is closed by defer above

	// Don't delete the file immediately to avoid Windows file locking issues
	// The file will be cleaned up during the next cleanup cycle
	return nil
}

// deliverBatch attempts to deliver a batch of events.
func (ds *DurableSink) deliverBatch(events []*core.LogEvent) bool {
	for _, event := range events {
		if !ds.tryDeliverEvent(event) {
			ds.sinkHealthy.Store(false)
			return false
		}
		ds.delivered.Add(1)
	}
	return true
}

// cleanupOldBufferFiles removes buffer files exceeding the limit.
func (ds *DurableSink) cleanupOldBufferFiles() {
	files, err := ds.getBufferFiles()
	if err != nil {
		return
	}

	// Also clean up processed files
	processedFiles, _ := filepath.Glob(filepath.Join(ds.options.BufferPath, "buffer-*.jsonl.processed"))
	for _, processedFile := range processedFiles {
		os.Remove(processedFile) // Best effort, ignore errors
	}

	if len(files) <= ds.options.MaxBufferFiles {
		return
	}

	// Remove oldest files
	filesToRemove := len(files) - ds.options.MaxBufferFiles
	for i := range filesToRemove {
		if err := os.Remove(files[i]); err != nil && ds.options.OnError != nil {
			ds.options.OnError(fmt.Errorf("failed to delete old buffer file %s: %w", files[i], err))
		}
	}
}

// recoverBufferedEvents processes existing buffer files on startup.
func (ds *DurableSink) recoverBufferedEvents() {
	time.Sleep(1 * time.Second) // Allow sink to stabilize

	// Clean up old files first
	ds.cleanupOldBufferFiles()

	files, err := ds.getBufferFiles()
	if err != nil || len(files) == 0 {
		return
	}

	// Get current buffer file name to exclude from recovery
	ds.fileMu.Lock()
	currentBufferFile := ""
	if ds.bufferFile != nil {
		currentBufferFile = ds.getBufferFileName(int(ds.fileIndex.Load()))
	}
	ds.fileMu.Unlock()

	// Process existing buffer files (excluding current one)
	for _, filename := range files {
		select {
		case <-ds.ctx.Done():
			return
		default:
			// Skip the currently open buffer file
			if filename == currentBufferFile {
				continue
			}

			if err := ds.processBufferFile(filename); err != nil {
				if ds.options.OnError != nil {
					ds.options.OnError(fmt.Errorf("recovery failed for %s: %w", filename, err))
				}
				break
			}
		}
	}
}

// GetMetrics returns current metrics for monitoring.
func (ds *DurableSink) GetMetrics() map[string]uint64 {
	return map[string]uint64{
		"buffered":  ds.buffered.Load(),
		"delivered": ds.delivered.Load(),
		"dropped":   ds.dropped.Load(),
		"retries":   ds.retries.Load(),
	}
}

// IsHealthy returns true if the wrapped sink is currently healthy.
func (ds *DurableSink) IsHealthy() bool {
	return ds.sinkHealthy.Load()
}
