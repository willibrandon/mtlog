package mtlog

import (
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// BenchmarkWithPerformance validates that With() meets our performance targets
func BenchmarkWithPerformance(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("With_2Fields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			l := logger.With("key1", "value1", "key2", 42)
			_ = l
		}
		
		// Verify we achieve 2 allocations (logger + fields)
		allocsPerOp := testing.AllocsPerRun(100, func() {
			_ = logger.With("key1", "value1", "key2", 42)
		})
		
		if allocsPerOp > 2 {
			b.Errorf("Performance regression: expected 2 allocs, got %.2f allocs/op", allocsPerOp)
		}
	})

	b.Run("With_8Fields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			l := logger.With(
				"field1", "value1",
				"field2", 42,
				"field3", true,
				"field4", 3.14,
				"field5", "value5",
				"field6", 100,
				"field7", false,
				"field8", 2.71,
			)
			_ = l
		}
		
		// Still should be 2 allocations
		allocsPerOp := testing.AllocsPerRun(100, func() {
			_ = logger.With(
				"f1", 1, "f2", 2, "f3", 3, "f4", 4,
				"f5", 5, "f6", 6, "f7", 7, "f8", 8,
			)
		})
		
		if allocsPerOp > 2 {
			b.Errorf("Performance regression: expected 2 allocs, got %.2f allocs/op", allocsPerOp)
		}
	})

	b.Run("ChainedWith", func(b *testing.B) {
		base := logger.With("service", "api", "version", "1.0")
		
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			l := base.With("request_id", i, "user_id", i*2)
			_ = l
		}
		
		// Chained operations may need to merge fields
		allocsPerOp := testing.AllocsPerRun(100, func() {
			_ = base.With("request_id", 1, "user_id", 2)
		})
		
		if allocsPerOp > 3 {
			b.Errorf("Performance regression: expected <=3 allocs for chained With, got %.2f", allocsPerOp)
		}
	})
}

// BenchmarkWithRealWorld tests realistic usage patterns
func BenchmarkWithRealWorld(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("HTTPRequest", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			logger.With(
				"method", "GET",
				"path", "/api/users",
				"status", 200,
				"duration_ms", 42,
				"user_id", 123,
				"trace_id", "abc-xyz",
			).Info("Request handled")
		}
	})

	b.Run("BaseLogger_Reuse", func(b *testing.B) {
		base := logger.With("service", "api", "env", "prod")
		
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			base.With("request_id", i).Info("Request {RequestId} handled", i)
		}
	})
}