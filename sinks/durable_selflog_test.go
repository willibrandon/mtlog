package sinks_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestDurableSinkSelfLog(t *testing.T) {
	t.Run("wrapped sink panic", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir for buffer
		bufferPath := t.TempDir()

		// Create durable sink with panicking inner sink
		innerSink := &panickingSink{}
		durableSink, err := sinks.NewDurableSink(innerSink, sinks.DurableOptions{
			BufferPath:      bufferPath,
			MaxBufferSize:   1024 * 1024,
			RetryInterval:   50 * time.Millisecond, // Shorter retry for faster test
			BatchSize:       10,
			ShutdownTimeout: 1 * time.Second,
			OnError: func(err error) {
				// Don't print to stdout during tests
				t.Logf("OnError called: %v", err)
			},
		})
		if err != nil {
			t.Fatalf("failed to create durable sink: %v", err)
		}

		// Create logger
		logger := mtlog.New(mtlog.WithSink(durableSink))

		// Log something - the first attempt should panic and buffer the event
		logger.Information("Test message")

		// Wait for the event to be processed and panic to occur
		time.Sleep(200 * time.Millisecond)
		
		// Check sink health
		t.Logf("sink healthy: %v", durableSink.IsHealthy())

		// Close the sink to stop background workers
		durableSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[durable] wrapped sink panicked") {
			// The wrapped sink might be a batch sink that doesn't emit immediately
			// Try to force a retry by waiting for the retry interval
			time.Sleep(200 * time.Millisecond)
			output = selflogBuf.String()
			t.Logf("selflog output after retry wait: %q", output)
		}
		if !strings.Contains(output, "[durable] wrapped sink panicked") {
			t.Errorf("expected wrapped sink panic in selflog, got: %s", output)
		}
	})

	t.Run("batch delivery failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir for buffer
		bufferPath := t.TempDir()
		
		// Write a pre-existing buffer file with events
		bufferFile := filepath.Join(bufferPath, "buffer-000001.jsonl")
		bufferContent := `{"event":{"Timestamp":"2025-01-01T00:00:00Z","Level":2,"MessageTemplate":"Test","Properties":{}},"timestamp":1704067200000000000,"sequence":0}
{"event":{"Timestamp":"2025-01-01T00:00:01Z","Level":2,"MessageTemplate":"Test2","Properties":{}},"timestamp":1704067201000000000,"sequence":1}
`
		if err := os.WriteFile(bufferFile, []byte(bufferContent), 0644); err != nil {
			t.Fatalf("failed to write buffer file: %v", err)
		}
		
		// Create a sink that panics to simulate batch delivery failure
		innerSink := &panickingSink{}
		
		// Create durable sink
		durableSink, err := sinks.NewDurableSink(innerSink, sinks.DurableOptions{
			BufferPath:      bufferPath,
			MaxBufferSize:   1024 * 1024,
			RetryInterval:   50 * time.Millisecond, // Short retry interval
			BatchSize:       10,
			ShutdownTimeout: 1 * time.Second,
			OnError: func(err error) {
				// Don't print to stdout during tests
				t.Logf("OnError: %v", err)
			},
		})
		if err != nil {
			t.Fatalf("failed to create durable sink: %v", err)
		}

		// Wait for retry worker to process the buffer file
		time.Sleep(300 * time.Millisecond)

		// Close the sink to stop background workers
		durableSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		// Should see batch delivery failure or wrapped sink panic
		if !strings.Contains(output, "[durable] batch delivery failed") && 
		   !strings.Contains(output, "[durable] wrapped sink panicked") &&
		   !strings.Contains(output, "[durable] final batch delivery failed") {
			t.Errorf("expected batch delivery failure in selflog, got: %s", output)
		}
	})

	t.Run("malformed buffer event", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir for buffer
		bufferPath := t.TempDir()
		
		// Write a malformed buffer file with correct naming format
		bufferFile := filepath.Join(bufferPath, "buffer-000000.jsonl")
		if err := os.WriteFile(bufferFile, []byte("invalid json\n{\"bad\": json}\n"), 0644); err != nil {
			t.Fatalf("failed to write buffer file: %v", err)
		}

		// Create a failing sink to ensure events stay buffered
		innerSink := &failingSink{}
		
		// Create durable sink
		durableSink, err := sinks.NewDurableSink(innerSink, sinks.DurableOptions{
			BufferPath:      bufferPath,
			MaxBufferSize:   1024 * 1024,
			RetryInterval:   100 * time.Millisecond,
			BatchSize:       10,
			ShutdownTimeout: 1 * time.Second,
			OnError: func(err error) {
				// Don't print to stdout during tests
			},
		})
		if err != nil {
			t.Fatalf("failed to create durable sink: %v", err)
		}

		// Wait for retry worker to process the malformed file
		time.Sleep(200 * time.Millisecond)

		// Close the sink to stop background workers
		durableSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[durable] skipping malformed event") {
			t.Errorf("expected malformed event message in selflog, got: %s", output)
		}
	})
}

// failingSink always returns false from Emit
type failingSink struct{}

func (f *failingSink) Emit(event *core.LogEvent) {
	// Do nothing - simulates a sink that can't deliver
}

func (f *failingSink) Close() error {
	return nil
}

