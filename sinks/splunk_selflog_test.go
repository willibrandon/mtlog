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

func TestSplunkSinkSelfLog(t *testing.T) {
	t.Run("http request failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create splunk sink with invalid URL
		splunkSink, err := sinks.NewSplunkSink("http://invalid-host-that-does-not-exist:8088/services/collector", "test-token",
			sinks.WithSplunkBatchSize(1),
		)
		if err != nil {
			t.Fatalf("failed to create splunk sink: %v", err)
		}
		defer splunkSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(splunkSink))

		// Log something
		logger.Information("Test message")

		// Close to flush batch
		splunkSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[splunk] HTTP request failed") ||
		   !strings.Contains(output, "[splunk] failed to send batch") {
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
			// Check auth header
			if r.Header.Get("Authorization") != "Splunk test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(`{"text":"Invalid event data","code":6}`)); err != nil {
				t.Logf("failed to write response: %v", err)
			}
		}))
		defer server.Close()

		// Create splunk sink
		splunkSink, err := sinks.NewSplunkSink(server.URL+"/services/collector", "test-token",
			sinks.WithSplunkBatchSize(1),
		)
		if err != nil {
			t.Fatalf("failed to create splunk sink: %v", err)
		}
		defer splunkSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(splunkSink))

		// Log something
		logger.Information("Test message")

		// Close to flush batch
		splunkSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[splunk] HTTP request failed with status 400") &&
		   !strings.Contains(output, "[splunk] failed to send batch") {
			t.Errorf("expected HTTP error response in selflog, got: %s", output)
		}
	})
}