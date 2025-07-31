package mtlog

import (
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// simpleSink implements core.SimpleSink for testing.
type simpleSink struct {
	simpleMessages []string
	events         []core.LogEvent
}

func (s *simpleSink) EmitSimple(timestamp time.Time, level core.LogEventLevel, message string) {
	s.simpleMessages = append(s.simpleMessages, message)
}

func (s *simpleSink) Emit(event *core.LogEvent) {
	s.events = append(s.events, *event)
}

func (s *simpleSink) Close() error {
	return nil
}

func TestPipelineFastPath(t *testing.T) {
	sink := &simpleSink{}
	logger := New(WithSink(sink))

	// Log simple message - should use fast path
	logger.Information("Simple message")

	if len(sink.simpleMessages) != 1 {
		t.Errorf("Expected 1 simple message, got %d", len(sink.simpleMessages))
	}

	if len(sink.events) != 0 {
		t.Errorf("Expected 0 regular events (should use fast path), got %d", len(sink.events))
	}
}

func TestPipelineWithProperties(t *testing.T) {
	sink := &simpleSink{}
	logger := New(WithSink(sink))

	// Log with properties - should use regular path
	logger.Information("User {UserId} logged in", 123)

	if len(sink.simpleMessages) != 0 {
		t.Errorf("Expected 0 simple messages (should use regular path), got %d", len(sink.simpleMessages))
	}

	if len(sink.events) != 1 {
		t.Errorf("Expected 1 regular event, got %d", len(sink.events))
	}
}

func TestPipelineWithEnricher(t *testing.T) {
	sink := &simpleSink{}
	enricher := &testEnricher{property: "App", value: "TestApp"}
	logger := New(
		WithSink(sink),
		WithEnricher(enricher),
	)

	// Even simple messages should use regular path when enrichers present
	logger.Information("Simple message")

	if len(sink.simpleMessages) != 0 {
		t.Errorf("Expected 0 simple messages (enricher forces regular path), got %d", len(sink.simpleMessages))
	}

	if len(sink.events) != 1 {
		t.Errorf("Expected 1 regular event, got %d", len(sink.events))
	}

	// Check enriched property
	event := sink.events[0]
	if val, ok := event.Properties["App"].(string); !ok || val != "TestApp" {
		t.Errorf("Expected App='TestApp', got %v", event.Properties["App"])
	}
}

func TestPipelineWithGlobalProperties(t *testing.T) {
	sink := &simpleSink{}
	logger := New(
		WithSink(sink),
		WithProperty("Environment", "Test"),
		WithProperty("Version", "1.0"),
	)

	// Log with global properties
	logger.Information("Application started")

	if len(sink.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(sink.events))
	}

	event := sink.events[0]

	// Check global properties
	if env, ok := event.Properties["Environment"].(string); !ok || env != "Test" {
		t.Errorf("Expected Environment='Test', got %v", event.Properties["Environment"])
	}

	if ver, ok := event.Properties["Version"].(string); !ok || ver != "1.0" {
		t.Errorf("Expected Version='1.0', got %v", event.Properties["Version"])
	}
}

func TestPipelineOptions(t *testing.T) {
	sink := &memorySink{}

	// Test convenience options
	logger := New(
		Debug(), // Sets minimum level to Debug
		WithConsole(),
		WithMachineName(),
		WithTimestamp(),
		WithProcess(),
		WithSink(sink),
	)

	// Log at various levels
	logger.Verbose("Verbose")
	logger.Debug("Debug")
	logger.Information("Info")

	events := sink.GetEvents()

	// Should have 2 events (Debug and above)
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Check that enrichers added properties
	for _, event := range events {
		if _, ok := event.Properties["MachineName"]; !ok {
			t.Error("Expected MachineName property from enricher")
		}
		if _, ok := event.Properties["ProcessId"]; !ok {
			t.Error("Expected ProcessId property from enricher")
		}
		// Timestamp is set on the event itself, not as a property
		if event.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}
	}
}

func BenchmarkPipelineFastPath(b *testing.B) {
	sink := &simpleSink{}
	logger := New(WithSink(sink))

	for b.Loop() {
		logger.Information("Simple log message")
	}
}

func BenchmarkPipelineWithProperties(b *testing.B) {
	sink := &simpleSink{}
	logger := New(WithSink(sink))

	for b.Loop() {
		logger.Information("User {UserId} performed {Action}", 123, "login")
	}
}

func BenchmarkPipelineWithEnrichers(b *testing.B) {
	sink := &simpleSink{}
	logger := New(
		WithSink(sink),
		WithMachineName(),
		WithProcess(),
	)

	for b.Loop() {
		logger.Information("Simple log message")
	}
}
