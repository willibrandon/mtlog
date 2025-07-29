package sinks_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestSeqSinkSelfLog(t *testing.T) {
	t.Run("http request failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create seq sink with invalid URL
		seqSink, err := sinks.NewSeqSink("http://invalid-host-that-does-not-exist:5341",
			sinks.WithSeqBatchSize(1), // Send immediately
			sinks.WithSeqRetry(0, 0), // Don't retry
		)
		if err != nil {
			t.Fatalf("failed to create seq sink: %v", err)
		}
		defer seqSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(seqSink))

		// Log something
		logger.Information("Test message")

		// Close to flush batch
		seqSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[seq] HTTP request failed") || 
		   !strings.Contains(output, "[seq] failed to send") {
			t.Errorf("expected HTTP request failure in selflog, got: %s", output)
		}
	})

	t.Run("http error response", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create test server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"error": "test error"}`)); err != nil {
				t.Logf("failed to write response: %v", err)
			}
		}))
		defer server.Close()

		// Create seq sink
		seqSink, err := sinks.NewSeqSink(server.URL,
			sinks.WithSeqBatchSize(1),
			sinks.WithSeqRetry(0, 0),
		)
		if err != nil {
			t.Fatalf("failed to create seq sink: %v", err)
		}
		defer seqSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(seqSink))

		// Log something
		logger.Information("Test message")

		// Close to flush batch
		seqSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[seq] failed to send") {
			t.Errorf("expected send failure in selflog, got: %s", output)
		}
	})

	t.Run("compression failure", func(t *testing.T) {
		// This is harder to test since gzip rarely fails
		// We could potentially test with a custom writer that fails
		t.Skip("Compression failures are difficult to simulate")
	})
}