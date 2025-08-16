package mtlog

import (
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// TestWithAllocations verifies that With() achieves 2 allocations
// (1 for logger struct, 1 for fields array)
// 
// Note: Unlike zap which pre-serializes to achieve 1 allocation,
// mtlog maintains structured properties for pipeline flexibility.
func TestWithAllocations(t *testing.T) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	t.Run("TwoFields", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_ = logger.With("key1", "value1", "key2", 42)
		})
		
		t.Logf("With(2 fields) allocations: %.1f", allocs)
		if allocs > 2 {
			t.Errorf("Expected 2 allocations, got %.1f", allocs)
		}
	})

	t.Run("FourFields", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_ = logger.With("key1", "value1", "key2", 42, "key3", true, "key4", 3.14)
		})
		
		t.Logf("With(4 fields) allocations: %.1f", allocs)
		if allocs > 2 {
			t.Errorf("Expected 2 allocations, got %.1f", allocs)
		}
	})

	t.Run("EightFields", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_ = logger.With(
				"f1", "v1", "f2", 2, "f3", true, "f4", 3.14,
				"f5", "v5", "f6", 6, "f7", false, "f8", 8.0,
			)
		})
		
		t.Logf("With(8 fields) allocations: %.1f", allocs)
		if allocs > 2 {
			t.Errorf("Expected 2 allocations, got %.1f", allocs)
		}
	})

	t.Run("ChainedWith", func(t *testing.T) {
		base := logger.With("service", "api")
		
		allocs := testing.AllocsPerRun(100, func() {
			_ = base.With("request_id", "abc-123")
		})
		
		t.Logf("Chained With() allocations: %.1f", allocs)
		// Chained operations need to merge fields, so may have more allocations
		if allocs > 4 {
			t.Errorf("Expected <=4 allocations, got %.1f", allocs)
		}
	})

	t.Run("WithOverride", func(t *testing.T) {
		base := logger.With("user_id", 123)
		
		allocs := testing.AllocsPerRun(100, func() {
			_ = base.With("user_id", 456) // Override
		})
		
		t.Logf("With() override allocations: %.1f", allocs)
		// Override operations need to scan existing fields
		if allocs > 4 {
			t.Errorf("Expected <=4 allocations, got %.1f", allocs)
		}
	})
}

// BenchmarkWithOnly benchmarks only the With() operation, not logging
func BenchmarkWithOnly(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("NoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With()
		}
		_ = l
	})

	b.Run("TwoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With("key1", "value1", "key2", 42)
		}
		_ = l
	})

	b.Run("FourFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
			)
		}
		_ = l
	})

	b.Run("EightFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
				"key5", "value5",
				"key6", 100,
				"key7", false,
				"key8", 2.71,
			)
		}
		_ = l
	})

	b.Run("SixteenFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With(
				"f1", "v1", "f2", 2, "f3", true, "f4", 4.0,
				"f5", "v5", "f6", 6, "f7", false, "f8", 8.0,
				"f9", "v9", "f10", 10, "f11", true, "f12", 12.0,
				"f13", "v13", "f14", 14, "f15", false, "f16", 16.0,
			)
		}
		_ = l
	})

	b.Run("ChainedWith", func(b *testing.B) {
		base := logger.With("service", "api", "version", "1.0")
		
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = base.With("request_id", i)
		}
		_ = l
	})
}

// BenchmarkWithComparison compares different With patterns
func BenchmarkWithComparison(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("ForContext_SingleField", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.ForContext("request_id", i)
		}
		_ = l
	})

	b.Run("With_SingleField", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var l core.Logger
		for i := 0; i < b.N; i++ {
			l = logger.With("request_id", i)
		}
		_ = l
	})

	b.Run("With_ThenLog", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With("request_id", i, "user_id", i*2).Info("test")
		}
	})

	b.Run("DirectLog_WithProperties", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("test {RequestId} {UserId}", i, i*2)
		}
	})
}