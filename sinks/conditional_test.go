package sinks

import (
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

func TestConditionalSink(t *testing.T) {
	t.Run("filters events based on predicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool {
				return e.Level >= core.ErrorLevel
			},
			memory,
		)
		
		// Emit various levels
		conditional.Emit(&core.LogEvent{
			Level:           core.DebugLevel,
			MessageTemplate: "Debug message",
			Properties:      make(map[string]any),
		})
		
		conditional.Emit(&core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Info message",
			Properties:      make(map[string]any),
		})
		
		conditional.Emit(&core.LogEvent{
			Level:           core.ErrorLevel,
			MessageTemplate: "Error message",
			Properties:      make(map[string]any),
		})
		
		conditional.Emit(&core.LogEvent{
			Level:           core.FatalLevel,
			MessageTemplate: "Fatal message",
			Properties:      make(map[string]any),
		})
		
		// Only error and fatal should be captured
		events := memory.Events()
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}
		
		if events[0].MessageTemplate != "Error message" {
			t.Errorf("Expected first event to be error, got %s", events[0].MessageTemplate)
		}
		
		if events[1].MessageTemplate != "Fatal message" {
			t.Errorf("Expected second event to be fatal, got %s", events[1].MessageTemplate)
		}
	})
	
	t.Run("handles nil event", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool { return true },
			memory,
		)
		
		// Should not panic
		conditional.Emit(nil)
		
		if len(memory.Events()) != 0 {
			t.Error("Expected no events for nil input")
		}
	})
	
	t.Run("handles predicate panic", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewNamedConditionalSink(
			"test-sink",
			func(e *core.LogEvent) bool {
				panic("predicate error")
			},
			memory,
		)
		
		// Should not panic, event should not be emitted
		conditional.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate: "Test",
			Properties: make(map[string]any),
		})
		
		if len(memory.Events()) != 0 {
			t.Error("Expected no events when predicate panics")
		}
	})
	
	t.Run("PropertyPredicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			PropertyPredicate("UserId"),
			memory,
		)
		
		// Event without UserId
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "No user",
			Properties: map[string]any{},
		})
		
		// Event with UserId
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "With user",
			Properties: map[string]any{"UserId": 123},
		})
		
		events := memory.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		if events[0].MessageTemplate != "With user" {
			t.Error("Expected event with UserId")
		}
	})
	
	t.Run("PropertyValuePredicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			PropertyValuePredicate("Environment", "production"),
			memory,
		)
		
		// Wrong value
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Dev env",
			Properties: map[string]any{"Environment": "development"},
		})
		
		// Correct value
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Prod env",
			Properties: map[string]any{"Environment": "production"},
		})
		
		// Property missing
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "No env",
			Properties: map[string]any{},
		})
		
		events := memory.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		if events[0].MessageTemplate != "Prod env" {
			t.Error("Expected production environment event")
		}
	})
	
	t.Run("AndPredicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			AndPredicate(
				LevelPredicate(core.ErrorLevel),
				PropertyPredicate("Alert"),
			),
			memory,
		)
		
		// Only error level
		conditional.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate:    "Error without alert",
			Properties: map[string]any{},
		})
		
		// Only alert property
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Info with alert",
			Properties: map[string]any{"Alert": true},
		})
		
		// Both conditions met
		conditional.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate:    "Error with alert",
			Properties: map[string]any{"Alert": true},
		})
		
		events := memory.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		if events[0].MessageTemplate != "Error with alert" {
			t.Error("Expected event matching both conditions")
		}
	})
	
	t.Run("OrPredicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			OrPredicate(
				LevelPredicate(core.ErrorLevel),
				PropertyPredicate("Important"),
			),
			memory,
		)
		
		// Error level
		conditional.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate:    "Error",
			Properties: map[string]any{},
		})
		
		// Important property
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Important info",
			Properties: map[string]any{"Important": true},
		})
		
		// Neither condition
		conditional.Emit(&core.LogEvent{
			Level:      core.DebugLevel,
			MessageTemplate:    "Debug",
			Properties: map[string]any{},
		})
		
		events := memory.Events()
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}
	})
	
	t.Run("NotPredicate", func(t *testing.T) {
		memory := NewMemorySink()
		conditional := NewConditionalSink(
			NotPredicate(PropertyPredicate("SkipLogging")),
			memory,
		)
		
		// Should be logged
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Normal event",
			Properties: map[string]any{},
		})
		
		// Should be skipped
		conditional.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Skip this",
			Properties: map[string]any{"SkipLogging": true},
		})
		
		events := memory.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		if events[0].MessageTemplate != "Normal event" {
			t.Error("Expected normal event to be logged")
		}
	})
	
	t.Run("Close propagates to target sink", func(t *testing.T) {
		// Use a mock closeable sink
		mockSink := &mockCloseableSink{}
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool { return true },
			mockSink,
		)
		
		err := conditional.Close()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !mockSink.closed {
			t.Error("Expected target sink to be closed")
		}
	})
}

func TestConditionalSinkPerformance(t *testing.T) {
	t.Run("zero overhead when predicate returns false", func(t *testing.T) {
		// Create a sink that would be expensive if called
		expensiveSink := &mockExpensiveSink{}
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool { return false }, // Always false
			expensiveSink,
		)
		
		start := time.Now()
		for i := 0; i < 10000; i++ {
			conditional.Emit(&core.LogEvent{
				Level:      core.DebugLevel,
				MessageTemplate: "Test",
				Properties: make(map[string]any),
			})
		}
		elapsed := time.Since(start)
		
		if expensiveSink.callCount > 0 {
			t.Error("Expensive sink should not have been called")
		}
		
		// Should be very fast when predicate returns false
		if elapsed > 10*time.Millisecond {
			t.Errorf("Expected near-zero overhead, took %v", elapsed)
		}
	})
}

// Mock sinks for testing

type mockCloseableSink struct {
	closed bool
	events []*core.LogEvent
}

func (m *mockCloseableSink) Emit(event *core.LogEvent) {
	m.events = append(m.events, event)
}

func (m *mockCloseableSink) Close() error {
	m.closed = true
	return nil
}

type mockExpensiveSink struct {
	callCount int
}

func (m *mockExpensiveSink) Emit(event *core.LogEvent) {
	m.callCount++
	// Simulate expensive operation
	time.Sleep(10 * time.Millisecond)
}

func (m *mockExpensiveSink) Close() error {
	return nil
}

func BenchmarkConditionalSink(b *testing.B) {
	b.Run("predicate returns true", func(b *testing.B) {
		sink := NewMemorySink()
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool { return e.Level >= core.ErrorLevel },
			sink,
		)
		
		event := &core.LogEvent{
			Level:           core.ErrorLevel,
			MessageTemplate: "Test",
			Properties:      make(map[string]any),
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			conditional.Emit(event)
		}
	})
	
	b.Run("predicate returns false", func(b *testing.B) {
		sink := NewMemorySink()
		conditional := NewConditionalSink(
			func(e *core.LogEvent) bool { return e.Level >= core.ErrorLevel },
			sink,
		)
		
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Test",
			Properties:      make(map[string]any),
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			conditional.Emit(event)
		}
	})
	
	b.Run("complex predicate", func(b *testing.B) {
		sink := NewMemorySink()
		conditional := NewConditionalSink(
			AndPredicate(
				LevelPredicate(core.ErrorLevel),
				PropertyPredicate("Alert"),
				PropertyValuePredicate("Environment", "production"),
			),
			sink,
		)
		
		event := &core.LogEvent{
			Level:           core.ErrorLevel,
			MessageTemplate: "Test",
			Properties: map[string]any{
				"Alert":       true,
				"Environment": "production",
			},
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			conditional.Emit(event)
		}
	})
}