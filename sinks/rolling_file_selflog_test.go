package sinks_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
	"github.com/willibrandon/mtlog/internal/formatters"
)

func TestRollingFileSinkSelfLog(t *testing.T) {
	t.Run("roll failure - read only directory", func(t *testing.T) {
		// Skip on Windows as directory permissions work differently
		if os.PathSeparator == '\\' {
			t.Skip("Skipping directory permission test on Windows")
		}
		
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		// Create rolling file sink with small size to trigger rolling
		sink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
			FilePath:    logFile,
			MaxFileSize: 200, // Small size to trigger rolling
			Formatter:   formatters.NewCLEFFormatter(),
		})
		if err != nil {
			t.Fatalf("failed to create rolling file sink: %v", err)
		}
		defer sink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(sink))

		// Write enough to fill the file
		for i := 0; i < 10; i++ {
			logger.Information("This is a test message to fill the log file {Number}", i)
		}

		// Force close to flush
		sink.Close()

		// Reopen the sink
		sink, err = sinks.NewRollingFileSink(sinks.RollingFileOptions{
			FilePath:    logFile,
			MaxFileSize: 200,
			Formatter:   formatters.NewCLEFFormatter(),
		})
		if err != nil {
			t.Fatalf("failed to recreate rolling file sink: %v", err)
		}

		// Make directory read-only to prevent rolling
		if err := os.Chmod(tmpDir, 0555); err != nil {
			t.Skipf("cannot make directory read-only: %v", err)
		}
		defer func() {
			if err := os.Chmod(tmpDir, 0755); err != nil {
				t.Logf("failed to restore permissions: %v", err)
			}
		}()

		// Create new logger with reopened sink
		logger = mtlog.New(mtlog.WithSink(sink))

		// Write more to trigger failed roll
		for i := 0; i < 20; i++ {
			logger.Information("This should trigger a roll that fails due to read-only directory {Number}", i)
		}

		// Force close to ensure all writes are attempted
		sink.Close()

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		// Check for any rolling-related failure message
		if !strings.Contains(output, "[rolling] failed to") {
			t.Errorf("expected rolling error in selflog, got: %s", output)
		}
	})

	t.Run("format error", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		// Create a custom formatter that always fails
		failingFormatter := &failingLogEventFormatter{}

		// Create rolling file sink
		sink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
			FilePath:  logFile,
			Formatter: failingFormatter,
		})
		if err != nil {
			t.Fatalf("failed to create rolling file sink: %v", err)
		}
		defer sink.Close()

		// Create logger
		logger := mtlog.New(mtlog.WithSink(sink))

		// Log something - formatting should fail
		logger.Information("Test message")

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[rolling] failed to format event") {
			t.Errorf("expected format error in selflog, got: %s", output)
		}
	})
}

// failingLogEventFormatter always returns an error
type failingLogEventFormatter struct{}

func (f *failingLogEventFormatter) Format(event *core.LogEvent) ([]byte, error) {
	return nil, fmt.Errorf("format error")
}