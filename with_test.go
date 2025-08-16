package mtlog

import (
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestWith(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected map[string]any
	}{
		{
			name:     "empty args",
			args:     []any{},
			expected: map[string]any{},
		},
		{
			name:     "single pair",
			args:     []any{"key1", "value1"},
			expected: map[string]any{"key1": "value1"},
		},
		{
			name:     "multiple pairs",
			args:     []any{"key1", "value1", "key2", 42, "key3", true},
			expected: map[string]any{"key1": "value1", "key2": 42, "key3": true},
		},
		{
			name:     "odd number of args",
			args:     []any{"key1", "value1", "key2"},
			expected: map[string]any{"key1": "value1"},
		},
		{
			name:     "non-string key",
			args:     []any{123, "value1", "key2", "value2"},
			expected: map[string]any{"key2": "value2"},
		},
		{
			name:     "nil values",
			args:     []any{"key1", nil, "key2", "value2"},
			expected: map[string]any{"key1": nil, "key2": "value2"},
		},
		{
			name:     "complex values",
			args:     []any{"struct", struct{ Name string }{Name: "test"}, "slice", []int{1, 2, 3}},
			expected: map[string]any{"struct": struct{ Name string }{Name: "test"}, "slice": []int{1, 2, 3}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a memory sink to capture events
			memSink := sinks.NewMemorySink()
			logger := New(
				WithSink(memSink),
				WithMinimumLevel(core.VerboseLevel),
			)

			// Create logger with With() method
			withLogger := logger.With(tt.args...)

			// Log a message
			withLogger.Info("test message")

			// Get captured events
			events := memSink.Events()
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			// Verify properties
			event := events[0]
			for key, expectedValue := range tt.expected {
				actualValue, exists := event.Properties[key]
				if !exists {
					t.Errorf("expected property %s not found", key)
					continue
				}
				
				// Compare values
				if !compareValues(actualValue, expectedValue) {
					t.Errorf("property %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Check for unexpected properties
			for key := range event.Properties {
				if _, expected := tt.expected[key]; !expected {
					t.Errorf("unexpected property %s found", key)
				}
			}
		})
	}
}

func TestWithChaining(t *testing.T) {
	// Create a memory sink to capture events
	memSink := sinks.NewMemorySink()
	logger := New(
		WithSink(memSink),
		WithMinimumLevel(core.VerboseLevel),
	)

	// Chain multiple With() calls
	logger.
		With("service", "auth").
		With("version", "1.0").
		With("environment", "production").
		Info("service started")

	// Get captured events
	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify all properties are present
	event := events[0]
	expected := map[string]any{
		"service":     "auth",
		"version":     "1.0",
		"environment": "production",
	}

	for key, expectedValue := range expected {
		actualValue, exists := event.Properties[key]
		if !exists {
			t.Errorf("expected property %s not found", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("property %s: expected %v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestWithOverride(t *testing.T) {
	// Create a memory sink to capture events
	memSink := sinks.NewMemorySink()
	logger := New(
		WithSink(memSink),
		WithMinimumLevel(core.VerboseLevel),
	)

	// Test that later With() calls override earlier ones
	logger.
		With("user_id", 123).
		With("user_id", 456). // Override
		Info("user action")

	// Get captured events
	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify the property was overridden
	event := events[0]
	userID, exists := event.Properties["user_id"]
	if !exists {
		t.Error("expected property user_id not found")
	} else if userID != 456 {
		t.Errorf("expected user_id to be 456, got %v", userID)
	}
}

func TestWithAndForContext(t *testing.T) {
	// Create a memory sink to capture events
	memSink := sinks.NewMemorySink()
	logger := New(
		WithSink(memSink),
		WithMinimumLevel(core.VerboseLevel),
	)

	// Combine With() and ForContext()
	logger.
		With("service", "api").
		ForContext("request_id", "abc-123").
		With("user_id", 789).
		Info("request processed")

	// Get captured events
	events := memSink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Verify all properties are present
	event := events[0]
	expected := map[string]any{
		"service":    "api",
		"request_id": "abc-123",
		"user_id":    789,
	}

	for key, expectedValue := range expected {
		actualValue, exists := event.Properties[key]
		if !exists {
			t.Errorf("expected property %s not found", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("property %s: expected %v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestWithThreadSafety(t *testing.T) {
	// Create a memory sink to capture events
	memSink := sinks.NewMemorySink()
	logger := New(
		WithSink(memSink),
		WithMinimumLevel(core.VerboseLevel),
	)

	// Create a logger with With()
	baseLogger := logger.With("base", "value")

	// Run concurrent operations
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			baseLogger.With("goroutine", 1, "iteration", i).Info("message from goroutine 1")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			baseLogger.With("goroutine", 2, "iteration", i).Info("message from goroutine 2")
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Get captured events
	events := memSink.Events()
	if len(events) != 200 {
		t.Fatalf("expected 200 events, got %d", len(events))
	}

	// Verify all events have the base property
	for _, event := range events {
		if base, exists := event.Properties["base"]; !exists || base != "value" {
			t.Error("base property missing or incorrect")
		}
	}
}

// compareValues compares two values, handling special cases like slices and structs
func compareValues(a, b any) bool {
	// Simple equality check for most types
	// For slices and structs, this will use Go's default comparison
	// which works for our test cases
	switch va := a.(type) {
	case []int:
		vb, ok := b.([]int)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if va[i] != vb[i] {
				return false
			}
		}
		return true
	case struct{ Name string }:
		vb, ok := b.(struct{ Name string })
		if !ok {
			return false
		}
		return va.Name == vb.Name
	default:
		return a == b
	}
}