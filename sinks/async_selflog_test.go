package sinks_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

// panickingSink is a sink that panics when Emit is called
type panickingSink struct{}

func (p *panickingSink) Emit(event *core.LogEvent) {
	panic("test panic")
}

func (p *panickingSink) Close() error {
	return nil
}

func TestAsyncSinkSelfLog(t *testing.T) {
	t.Run("worker panic recovery", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create async sink with panicking inner sink
		innerSink := &panickingSink{}
		asyncSink := sinks.NewAsyncSink(innerSink, sinks.AsyncOptions{
			BufferSize:       10,
			OverflowStrategy: sinks.OverflowDrop,
			ShutdownTimeout:  1 * time.Second,
		})

		// Create logger
		logger := mtlog.New(mtlog.WithSink(asyncSink))

		// Log something - should trigger panic in worker
		logger.Information("Test message")

		// Wait for async processing
		time.Sleep(100 * time.Millisecond)

		// Close the sink to ensure all goroutines finish
		asyncSink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[async] wrapped sink panic") || !strings.Contains(output, "test panic") {
			t.Errorf("expected wrapped sink panic in selflog, got: %s", output)
		}
	})

	t.Run("buffer overflow drop", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create a slow sink that blocks
		blockingSink := &slowSink{delay: 100 * time.Millisecond}

		// Create async sink with small buffer
		asyncSink := sinks.NewAsyncSink(blockingSink, sinks.AsyncOptions{
			BufferSize:       2,
			OverflowStrategy: sinks.OverflowDrop,
			ShutdownTimeout:  1 * time.Second,
		})
		defer asyncSink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(asyncSink))

		// Log many messages quickly to trigger overflow
		for i := range 10 {
			logger.Information("Test message {Number}", i)
		}

		// Wait for async processing
		time.Sleep(200 * time.Millisecond)

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[async] buffer full, dropped") {
			t.Errorf("expected buffer overflow message in selflog, got: %s", output)
		}
	})
}

// slowSink is a sink that delays on each emit
type slowSink struct {
	delay time.Duration
}

func (s *slowSink) Emit(event *core.LogEvent) {
	time.Sleep(s.delay)
}

func (s *slowSink) Close() error {
	return nil
}
