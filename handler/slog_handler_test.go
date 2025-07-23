package handler_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/handler"
	"github.com/willibrandon/mtlog/sinks"
)

func TestSlogHandler(t *testing.T) {
	// Create memory sink to capture output
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
	)

	// Create slog logger with our handler
	slogger := slog.New(handler.NewSlogHandler(logger))

	// Test basic logging at different levels
	slogger.Debug("debug message", "key", "value")
	slogger.Info("info message", "count", 42)
	slogger.Warn("warning message", "error", "something went wrong")
	slogger.Error("error message", "fatal", false)

	// Verify events were captured
	events := memSink.Events()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Verify levels
	expectedLevels := []core.LogEventLevel{
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
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
	count := events[1].Properties["count"]
	if countInt, ok := count.(int); ok {
		if countInt != 42 {
			t.Errorf("expected property 'count' to be 42, got %v", countInt)
		}
	} else if countInt64, ok := count.(int64); ok {
		if countInt64 != 42 {
			t.Errorf("expected property 'count' to be 42, got %v", countInt64)
		}
	} else {
		t.Errorf("expected property 'count' to be int or int64, got %T: %v", count, count)
	}
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))

	// Create slog logger with attributes
	slogger := slog.New(handler.NewSlogHandler(logger).WithAttrs([]slog.Attr{
		slog.String("service", "test-service"),
		slog.Int("version", 1),
	}))

	slogger.Info("test message")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify pre-set attributes are included
	if events[0].Properties["service"] != "test-service" {
		t.Errorf("expected service='test-service', got %v", events[0].Properties["service"])
	}
	version := events[0].Properties["version"]
	if versionInt, ok := version.(int); ok {
		if versionInt != 1 {
			t.Errorf("expected version=1, got %v", versionInt)
		}
	} else if versionInt64, ok := version.(int64); ok {
		if versionInt64 != 1 {
			t.Errorf("expected version=1, got %v", versionInt64)
		}
	} else {
		t.Errorf("expected property 'version' to be int or int64, got %T: %v", version, version)
	}
}

func TestSlogHandlerWithGroup(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))

	// Create slog logger with group
	slogger := slog.New(handler.NewSlogHandler(logger).WithGroup("request"))

	slogger.Info("test message", "id", "123", "method", "GET")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify grouped attributes
	if events[0].Properties["request.id"] != "123" {
		t.Errorf("expected request.id='123', got %v", events[0].Properties["request.id"])
	}
	if events[0].Properties["request.method"] != "GET" {
		t.Errorf("expected request.method='GET', got %v", events[0].Properties["request.method"])
	}
}

func TestSlogHandlerEnabled(t *testing.T) {
	logger := mtlog.New(mtlog.WithMinimumLevel(core.WarningLevel))
	h := handler.NewSlogHandler(logger)

	tests := []struct {
		level   slog.Level
		enabled bool
	}{
		{slog.LevelDebug, false},
		{slog.LevelInfo, false},
		{slog.LevelWarn, true},
		{slog.LevelError, true},
	}

	for _, tt := range tests {
		if h.Enabled(context.Background(), tt.level) != tt.enabled {
			t.Errorf("level %v: expected enabled=%v", tt.level, tt.enabled)
		}
	}
}

func TestNewSlogLogger(t *testing.T) {
	// Test the convenience function
	memSink := sinks.NewMemorySink()
	slogger := mtlog.NewSlogLogger(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	slogger.Info("test message", "key", "value")

	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Properties["key"] != "value" {
		t.Errorf("expected key='value', got %v", events[0].Properties["key"])
	}
}