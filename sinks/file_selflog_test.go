package sinks_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestFileSinkSelfLog(t *testing.T) {
	t.Run("close errors", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create temp dir
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		// Create file sink
		sink, err := sinks.NewFileSink(logFile)
		if err != nil {
			t.Fatalf("failed to create file sink: %v", err)
		}

		// Write something
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test message",
			Properties:      map[string]interface{}{},
		}
		sink.Emit(event)

		// Close once
		sink.Close()
		
		// Try to emit after close - should be ignored due to isOpen check
		sink.Emit(event)
		
		// Close again - should be safe due to isOpen check
		err = sink.Close()
		if err != nil {
			t.Errorf("second close returned error: %v", err)
		}

		// Check selflog output - we don't expect any errors for double close
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
	})

	t.Run("directory creation error", func(t *testing.T) {
		// Skip on Windows as directory permissions work differently
		if os.PathSeparator == '\\' {
			t.Skip("Skipping directory permission test on Windows")
		}
		
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Try to create file in non-existent nested directory under read-only parent
		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.Mkdir(readOnlyDir, 0755); err != nil {
			t.Fatal(err)
		}
		
		// Make parent directory read-only
		if err := os.Chmod(readOnlyDir, 0444); err != nil {
			t.Skipf("cannot make directory read-only: %v", err)
		}
		defer func() {
			if err := os.Chmod(readOnlyDir, 0755); err != nil {
				t.Logf("failed to restore permissions: %v", err)
			}
		}()

		logFile := filepath.Join(readOnlyDir, "subdir", "test.log")

		// Try to create file sink - should fail on directory creation
		_, err := sinks.NewFileSink(logFile)
		if err == nil {
			t.Fatal("expected error creating file sink in read-only directory")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[file] failed to create directory") && 
		   !strings.Contains(output, "[file] failed to open file") {
			t.Errorf("expected directory or file open error in selflog, got: %s", output)
		}
	})
}