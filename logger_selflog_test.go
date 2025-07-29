package mtlog_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestLoggerSelfLog(t *testing.T) {
	t.Run("template parse error", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create logger with memory sink
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))

		// Log with invalid template (unclosed property)
		logger.Information("User {Name logged in")

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[parser] template validation error") {
			t.Errorf("expected template validation error in selflog, got: %s", output)
		}
		if !strings.Contains(output, "unclosed property") {
			t.Errorf("expected 'unclosed property' in error message, got: %s", output)
		}
		if !strings.Contains(output, `template="User {Name logged in"`) {
			t.Errorf("expected template in error message, got: %s", output)
		}
	})

	t.Run("multiple parse errors", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create logger with memory sink
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))

		// Log multiple messages with parse errors
		logger.Information("Unclosed {property")
		logger.Warning("Another {unclosed")
		logger.Error("Missing close {brace")

		// Check selflog output contains all errors
		output := selflogBuf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 parse errors, got %d: %s", len(lines), output)
		}

		// Each line should contain parse error
		for i, line := range lines {
			if !strings.Contains(line, "[parser] template validation error") {
				t.Errorf("line %d missing validation error: %s", i+1, line)
			}
		}
	})

	t.Run("valid templates produce no selflog", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create logger with memory sink
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))

		// Log with valid templates
		logger.Information("Simple message")
		logger.Information("User {Name} logged in", "Alice")
		logger.Information("Complex {A} {B:F2} {C,-10}", "test", 3.14, "left")

		// Check no selflog output
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output for valid templates, got: %s", output)
		}
	})

	t.Run("empty property name", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create logger with memory sink
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))

		// Log with empty property name
		logger.Information("Test {} message")

		// Check selflog output
		output := selflogBuf.String()
		if !strings.Contains(output, "[parser] template validation error") {
			t.Errorf("expected template validation error for empty property, got: %s", output)
		}
	})
}