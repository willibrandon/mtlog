package sinks

import (
	"bytes"
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

// ElasticsearchSink writes log events to Elasticsearch
type ElasticsearchSink struct {
	urls     []string
	index    string
	client   *http.Client
	apiKey   string
	username string
	password string

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
	closed  sync.Once

	// Options
	useDataStreams bool
	pipeline       string
}

// ElasticsearchOption configures an Elasticsearch sink
type ElasticsearchOption func(*ElasticsearchSink)

// WithElasticsearchURLs sets multiple Elasticsearch URLs for load balancing
func WithElasticsearchURLs(urls ...string) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.urls = urls
	}
}

// WithElasticsearchIndex sets the index name or data stream name
func WithElasticsearchIndex(index string) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.index = index
	}
}

// WithElasticsearchAPIKey sets the API key for authentication
func WithElasticsearchAPIKey(apiKey string) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.apiKey = apiKey
	}
}

// WithElasticsearchBasicAuth sets username and password for basic authentication
func WithElasticsearchBasicAuth(username, password string) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.username = username
		s.password = password
	}
}

// WithElasticsearchBatchSize sets the batch size
func WithElasticsearchBatchSize(size int) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.batchSize = size
	}
}

// WithElasticsearchBatchTimeout sets the batch timeout
func WithElasticsearchBatchTimeout(timeout time.Duration) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.batchTimeout = timeout
	}
}

// WithElasticsearchDataStreams enables data streams instead of indices
func WithElasticsearchDataStreams() ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.useDataStreams = true
	}
}

// WithElasticsearchPipeline sets the ingest pipeline to use
func WithElasticsearchPipeline(pipeline string) ElasticsearchOption {
	return func(s *ElasticsearchSink) {
		s.pipeline = pipeline
	}
}

// NewElasticsearchSink creates a new Elasticsearch sink
func NewElasticsearchSink(url string, opts ...ElasticsearchOption) (*ElasticsearchSink, error) {
	sink := &ElasticsearchSink{
		urls:         []string{url},
		index:        "logs",
		client:       &http.Client{Timeout: 30 * time.Second},
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
func (es *ElasticsearchSink) Emit(event *core.LogEvent) {
	es.batchMu.Lock()
	es.batch = append(es.batch, event)
	shouldFlush := len(es.batch) >= es.batchSize
	es.batchMu.Unlock()

	if shouldFlush {
		select {
		case es.flushCh <- struct{}{}:
		default:
		}
	}
}

// Close closes the sink
func (es *ElasticsearchSink) Close() error {
	var err error
	es.closed.Do(func() {
		close(es.stopCh)
		es.wg.Wait()
	})
	return err
}

// worker handles batching and sending
func (es *ElasticsearchSink) worker() {
	defer es.wg.Done()

	es.timer = time.NewTimer(es.batchTimeout)
	defer es.timer.Stop()

	for {
		select {
		case <-es.stopCh:
			es.flush()
			return
		case <-es.timer.C:
			es.flush()
			es.timer.Reset(es.batchTimeout)
		case <-es.flushCh:
			if !es.timer.Stop() {
				<-es.timer.C
			}
			es.flush()
			es.timer.Reset(es.batchTimeout)
		}
	}
}

// flush sends the current batch
func (es *ElasticsearchSink) flush() {
	es.batchMu.Lock()
	if len(es.batch) == 0 {
		es.batchMu.Unlock()
		return
	}
	batch := es.batch
	es.batch = make([]*core.LogEvent, 0)
	es.batchMu.Unlock()

	es.sendBatch(batch)
}

// sendBatch sends a batch of events to Elasticsearch
func (es *ElasticsearchSink) sendBatch(events []*core.LogEvent) {
	if len(events) == 0 {
		return
	}

	// Build bulk request body
	var buf bytes.Buffer
	for _, event := range events {
		// Write action line
		var actionType string
		if es.useDataStreams {
			actionType = "create"
		} else {
			actionType = "index"
		}

		action := map[string]any{
			actionType: map[string]any{
				"_index": es.getIndexName(event.Timestamp),
			},
		}
		if es.pipeline != "" {
			action[actionType].(map[string]any)["pipeline"] = es.pipeline
		}

		if err := json.NewEncoder(&buf).Encode(action); err != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[elasticsearch] failed to encode bulk action: %v", err)
			}
			continue
		}

		// Write document
		doc := es.formatEvent(event)
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[elasticsearch] failed to encode document: %v", err)
			}
			continue
		}
	}

	// Send bulk request with retry logic
	bulkData := buf.Bytes()

	for i := range 3 {
		if es.sendBulkRequest(bulkData) {
			break
		}
		if i == 2 && selflog.IsEnabled() {
			selflog.Printf("[elasticsearch] failed to send batch of %d events after 3 attempts", len(events))
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
}

// sendBulkRequest sends the bulk request to Elasticsearch
func (es *ElasticsearchSink) sendBulkRequest(body []byte) bool {
	// Try each URL in round-robin fashion
	for _, url := range es.urls {
		bulkURL := fmt.Sprintf("%s/_bulk", strings.TrimRight(url, "/"))

		req, err := http.NewRequest("POST", bulkURL, bytes.NewReader(body))
		if err != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[elasticsearch] failed to create bulk request: %v (url=%s)", err, bulkURL)
			}
			continue
		}

		req.Header.Set("Content-Type", "application/x-ndjson")

		// Set authentication
		if es.apiKey != "" {
			req.Header.Set("Authorization", "ApiKey "+es.apiKey)
		} else if es.username != "" && es.password != "" {
			req.SetBasicAuth(es.username, es.password)
		}

		resp, err := es.client.Do(req)
		if err != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[elasticsearch] HTTP request failed: %v (url=%s)", err, bulkURL)
			}
			continue
		}
		defer resp.Body.Close()

		// Check response
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Parse response to check for errors
			var result struct {
				Errors bool             `json:"errors"`
				Items  []map[string]any `json:"items"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				// Log any individual errors (in production, you'd handle these appropriately)
				if result.Errors {
					for _, item := range result.Items {
						// Check both index and create responses
						var errorInfo map[string]any

						if indexResp, ok := item["index"]; ok {
							if indexItem, ok := indexResp.(map[string]any); ok {
								if indexError, ok := indexItem["error"].(map[string]any); ok {
									errorInfo = indexError
								}
							}
						} else if createResp, ok := item["create"]; ok {
							if createItem, ok := createResp.(map[string]any); ok {
								if createError, ok := createItem["error"].(map[string]any); ok {
									errorInfo = createError
								}
							}
						}

						if errorInfo != nil {
							// Individual document error - could log or handle
							if selflog.IsEnabled() {
								errType := errorInfo["type"]
								errReason := errorInfo["reason"]
								selflog.Printf("[elasticsearch] bulk item error: type=%v, reason=%v", errType, errReason)
							}
							// In production, you might want to retry failed documents
							// For now, increment a counter or log the error
							continue // Skip to next item
						}
					}
				}
			}
			return true
		}

		// Read error response for debugging (in production, you might log this)
	}

	return false
}

// getIndexName returns the index name for the given timestamp
func (es *ElasticsearchSink) getIndexName(timestamp time.Time) string {
	if es.useDataStreams {
		// For data streams, just return the stream name
		return es.index
	}
	// For regular indices, append date
	return fmt.Sprintf("%s-%s", es.index, timestamp.Format("2006.01.02"))
}

// formatEvent formats a log event for Elasticsearch
func (es *ElasticsearchSink) formatEvent(event *core.LogEvent) map[string]any {
	// Parse template to render message
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		// Fallback to raw template
		tmpl = &parser.MessageTemplate{
			Raw:    event.MessageTemplate,
			Tokens: []parser.MessageTemplateToken{&parser.TextToken{Text: event.MessageTemplate}},
		}
	}

	// Render the message
	message := tmpl.Render(event.Properties)

	doc := map[string]any{
		"@timestamp":      event.Timestamp.Format(time.RFC3339Nano),
		"level":           levelToString(event.Level),
		"message":         message,
		"messageTemplate": event.MessageTemplate,
	}

	// Add all properties
	for k, v := range event.Properties {
		// Skip properties that would conflict with ES fields
		if k == "@timestamp" || k == "_index" || k == "_id" {
			continue
		}
		doc[k] = v
	}

	// Add ECS (Elastic Common Schema) compatible fields
	doc["log.level"] = strings.ToLower(levelToString(event.Level))
	doc["event.created"] = event.Timestamp.Format(time.RFC3339Nano)

	// Add error fields if present
	if err, ok := event.Properties["Error"]; ok {
		if errStr, ok := err.(string); ok {
			doc["error.message"] = errStr
		} else if errObj, ok := err.(error); ok {
			doc["error.message"] = errObj.Error()
		}
	}

	return doc
}

// levelToString converts log level to string
func levelToString(level core.LogEventLevel) string {
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
		return "Unknown"
	}
}
