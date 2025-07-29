package sinks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestConsoleSinkSelfLog(t *testing.T) {
	t.Run("write error", func(t *testing.T) {
		// Setup selflog capture fresh for this test
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Test that selflog is working
		selflog.Printf("test selflog message")
		if got := selflogBuf.String(); !strings.Contains(got, "test selflog message") {
			t.Fatalf("selflog not working, got: %q", got)
		}
		selflogBuf.Reset()

		// Create a failing writer
		failWriter := &failingWriter{err: "broken pipe"}

		// Create console sink with failing writer
		sink := sinks.NewConsoleSink()
		sink.SetOutput(failWriter)

		// Create logger
		logger := mtlog.New(mtlog.WithSink(sink))

		// Log something
		logger.Information("Test message")

		// Force a sync if needed
		time.Sleep(10 * time.Millisecond)

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		t.Logf("selflog enabled: %v", selflog.IsEnabled())
		t.Logf("failWriter calls: %d", failWriter.calls)
		if !strings.Contains(output, "[console] write failed") {
			t.Errorf("expected write error in selflog, got: %s", output)
		}
		if !strings.Contains(output, "broken pipe") {
			t.Errorf("expected error details in selflog, got: %s", output)
		}
	})
}

// failingWriter always returns an error
type failingWriter struct {
	err string
	calls int
}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	f.calls++
	return 0, fmt.Errorf("%s", f.err)
}