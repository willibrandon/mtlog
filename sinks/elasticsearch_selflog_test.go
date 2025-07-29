package sinks_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestElasticsearchSinkSelfLog(t *testing.T) {
	t.Run("http request failure", func(t *testing.T) {
		// Setup selflog capture with thread-safe writer
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create elasticsearch sink with invalid URL
		esSink, err := sinks.NewElasticsearchSink("http://invalid-host-that-does-not-exist:9200",
			sinks.WithElasticsearchIndex("logs"),
			sinks.WithElasticsearchBatchSize(1),
		)
		if err != nil {
			t.Fatalf("failed to create elasticsearch sink: %v", err)
		}
		defer esSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(esSink))

		// Log something
		logger.Information("Test message")

		// Close manually to flush (remove defer close)
		if err := esSink.Close(); err != nil {
			t.Logf("close error: %v", err)
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[elasticsearch] HTTP request failed") ||
		   !strings.Contains(output, "[elasticsearch] failed to send batch") {
			t.Errorf("expected HTTP request failure in selflog, got: %s", output)
		}
	})

	t.Run("bulk item errors", func(t *testing.T) {
		// Setup selflog capture with thread-safe writer
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create test server that returns bulk response with errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"took":   100,
				"errors": true,
				"items": []map[string]interface{}{
					{
						"index": map[string]interface{}{
							"_index":  "logs",
							"_id":     "1",
							"status":  400,
							"error": map[string]interface{}{
								"type":   "mapper_parsing_exception",
								"reason": "failed to parse field [count] of type [long]",
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Create elasticsearch sink
		esSink, err := sinks.NewElasticsearchSink(server.URL,
			sinks.WithElasticsearchIndex("logs"),
			sinks.WithElasticsearchBatchSize(1),
		)
		if err != nil {
			t.Fatalf("failed to create elasticsearch sink: %v", err)
		}

		// Create logger
		logger := mtlog.New(mtlog.WithSink(esSink))

		// Log something
		logger.Information("Test message with invalid count", "count", "not-a-number")

		// Wait for async processing
		time.Sleep(200 * time.Millisecond)

		// Close the sink to ensure all goroutines finish
		esSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[elasticsearch] bulk item error") {
			t.Errorf("expected bulk item error in selflog, got: %s", output)
		}
		if !strings.Contains(output, "mapper_parsing_exception") {
			t.Errorf("expected error details in selflog, got: %s", output)
		}
	})
}