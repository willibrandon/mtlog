package handler_test

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/handler"
	"github.com/willibrandon/mtlog/sinks"
)

func TestLogrSink(t *testing.T) {
	// Create memory sink to capture output
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
	)

	// Create logr logger with our sink
	logrLogger := logr.New(handler.NewLogrSink(logger))

	// Test basic logging at different V-levels
	logrLogger.V(0).Info("info message", "key", "value")
	logrLogger.V(1).Info("debug message", "count", 42)
	logrLogger.V(2).Info("verbose message", "enabled", true)

	// Test error logging
	err := errors.New("test error")
	logrLogger.Error(err, "error occurred", "operation", "test")

	// Verify events were captured
	events := memSink.Events()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Verify levels
	expectedLevels := []core.LogEventLevel{
		core.InformationLevel,
		core.DebugLevel,
		core.VerboseLevel,
		core.ErrorLevel,
	}

	for i, event := range events {
		if event.Level != expectedLevels[i] {
			t.Errorf("event %d: expected level %v, got %v", i, expectedLevels[i], event.Level)
		}
	}

	// Verify properties
	if events[0].Properties["key"] != "value" {
		t.Errorf("expected property 'key' to be 'value', got %v", events[0].Properties["key"])
	}
	if events[3].Properties["operation"] != "test" {
		t.Errorf("expected property 'operation' to be 'test', got %v", events[3].Properties["operation"])
	}

	// Verify error is included
	if events[3].Properties["error"] == nil {
		t.Error("expected error property in error event")
	}
}

func TestLogrSinkWithValues(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))

	// Create logr logger with persistent values
	logrLogger := logr.New(handler.NewLogrSink(logger)).WithValues(
		"service", "test-service",
		"version", "1.0.0",
	)

	logrLogger.Info("test message", "request_id", "123")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify all values are included
	expectedProps := map[string]interface{}{
		"service":    "test-service",
		"version":    "1.0.0",
		"request_id": "123",
	}

	for key, expected := range expectedProps {
		if actual, ok := events[0].Properties[key]; !ok || actual != expected {
			t.Errorf("expected %s=%v, got %v", key, expected, actual)
		}
	}
}

func TestLogrSinkWithName(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))

	// Create logr logger with name hierarchy
	logrLogger := logr.New(handler.NewLogrSink(logger)).
		WithName("controller").
		WithName("reconciler")

	logrLogger.Info("reconciling", "resource", "pod/test")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify logger name is included
	if events[0].Properties["logger"] != "controller.reconciler" {
		t.Errorf("expected logger='controller.reconciler', got %v", events[0].Properties["logger"])
	}
}

func TestLogrSinkEnabled(t *testing.T) {
	logger := mtlog.New(mtlog.WithMinimumLevel(core.InformationLevel))
	logrLogger := logr.New(handler.NewLogrSink(logger))

	tests := []struct {
		vLevel  int
		enabled bool
	}{
		{0, true},  // Info level
		{1, false}, // Debug level (filtered out)
		{2, false}, // Verbose level (filtered out)
	}

	for _, tt := range tests {
		if logrLogger.V(tt.vLevel).Enabled() != tt.enabled {
			t.Errorf("V(%d).Enabled(): expected %v", tt.vLevel, tt.enabled)
		}
	}
}

func TestLogrSinkOddKeyValues(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))
	logrLogger := logr.New(handler.NewLogrSink(logger))

	// Test with odd number of key/values
	logrLogger.Info("test", "key1", "value1", "key2")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify first key/value pair
	if events[0].Properties["key1"] != "value1" {
		t.Errorf("expected key1='value1', got %v", events[0].Properties["key1"])
	}

	// Verify odd key gets nil value
	if _, hasKey2 := events[0].Properties["key2"]; !hasKey2 {
		t.Error("expected key2 to be present with nil value")
	}
}

func TestNewLogrLogger(t *testing.T) {
	// Test the convenience function
	memSink := sinks.NewMemorySink()
	logrLogger := mtlog.NewLogrLogger(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	logrLogger.Info("test message", "key", "value")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Properties["key"] != "value" {
		t.Errorf("expected key='value', got %v", events[0].Properties["key"])
	}
}