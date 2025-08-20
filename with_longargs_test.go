package mtlog

import (
	"fmt"
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// TestWithLongArgumentList verifies that With() handles very long argument lists efficiently
func TestWithLongArgumentList(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := New(
		WithSink(memSink),
		WithMinimumLevel(core.VerboseLevel),
	)

	// Test with 100 key-value pairs (200 arguments)
	t.Run("100_pairs", func(t *testing.T) {
		args := make([]any, 200)
		for i := 0; i < 100; i++ {
			args[i*2] = fmt.Sprintf("key%d", i)
			args[i*2+1] = fmt.Sprintf("value%d", i)
		}

		// This should handle the large argument list without issues
		withLogger := logger.With(args...)
		withLogger.Info("test with 100 fields")

		// Verify the event was logged
		events := memSink.Events()
		// Specifically check for 1 event to validate that With() emits a single event,
		// even when handling a very long argument list. This ensures correct event emission behavior.
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}

		// Verify a sampling of properties
		event := events[0]
		if val, exists := event.Properties["key0"]; !exists || val != "value0" {
			t.Error("key0 not found or incorrect")
		}
		if val, exists := event.Properties["key50"]; !exists || val != "value50" {
			t.Error("key50 not found or incorrect")
		}
		if val, exists := event.Properties["key99"]; !exists || val != "value99" {
			t.Error("key99 not found or incorrect")
		}

		// Clear events for next test
		memSink.Clear()
	})

	// Test that it falls back to map for >64 fields
	t.Run("map_fallback", func(t *testing.T) {
		// Create 65 fields to trigger map fallback
		args := make([]any, 130) // 65 pairs
		for i := 0; i < 65; i++ {
			args[i*2] = fmt.Sprintf("field%d", i)
			args[i*2+1] = i
		}

		withLogger := logger.With(args...)
		withLogger.Info("test with 65 fields")

		// Verify the event was logged correctly
		events := memSink.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}

		// Verify all 65 fields are present
		event := events[0]
		for i := 0; i < 65; i++ {
			key := fmt.Sprintf("field%d", i)
			if val, exists := event.Properties[key]; !exists || val != i {
				t.Errorf("field%d not found or incorrect: got %v", i, val)
			}
		}
	})
}

// BenchmarkWithLongArgumentList measures performance with varying argument counts
func BenchmarkWithLongArgumentList(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	// Benchmark different sizes
	sizes := []int{10, 50, 100, 200}
	
	for _, size := range sizes {
		args := make([]any, size*2)
		for i := 0; i < size; i++ {
			args[i*2] = fmt.Sprintf("key%d", i)
			args[i*2+1] = i
		}

		b.Run(fmt.Sprintf("%d_pairs", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				_ = logger.With(args...)
			}
		})
	}

	// Special case: exactly 64 fields (boundary condition)
	b.Run("64_pairs_boundary", func(b *testing.B) {
		args := make([]any, 128)
		for i := 0; i < 64; i++ {
			args[i*2] = fmt.Sprintf("key%d", i)
			args[i*2+1] = i
		}

		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = logger.With(args...)
		}
	})

	// Special case: 65 fields (just over the boundary, triggers map)
	b.Run("65_pairs_map_fallback", func(b *testing.B) {
		args := make([]any, 130)
		for i := 0; i < 65; i++ {
			args[i*2] = fmt.Sprintf("key%d", i)
			args[i*2+1] = i
		}

		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = logger.With(args...)
		}
	})
}