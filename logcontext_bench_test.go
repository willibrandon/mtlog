package mtlog_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/sinks"
)

func BenchmarkLogContext(b *testing.B) {
	// Use memory sink for benchmarking - events are stored but we don't check them
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	b.Run("NoContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.Information("Test message")
		}
	})

	b.Run("SingleProperty", func(b *testing.B) {
		ctx := context.Background()
		ctx = mtlog.PushProperty(ctx, "UserId", 123)
		log := logger.WithContext(ctx)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			log.Information("Test message")
		}
	})

	b.Run("MultipleProperties", func(b *testing.B) {
		ctx := context.Background()
		ctx = mtlog.PushProperty(ctx, "UserId", 123)
		ctx = mtlog.PushProperty(ctx, "TenantId", "acme")
		ctx = mtlog.PushProperty(ctx, "RequestId", "req-456")
		ctx = mtlog.PushProperty(ctx, "SessionId", "sess-789")
		log := logger.WithContext(ctx)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			log.Information("Test message")
		}
	})

	b.Run("PushPropertyOverhead", func(b *testing.B) {
		ctx := context.Background()

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = mtlog.PushProperty(ctx, "Key", "Value")
		}
	})

	b.Run("NestedContext", func(b *testing.B) {
		ctx := context.Background()
		ctx = mtlog.PushProperty(ctx, "Level1", "value1")

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			childCtx := mtlog.PushProperty(ctx, "Level2", "value2")
			log := logger.WithContext(childCtx)
			log.Information("Nested context test")
		}
	})

	b.Run("DeeplyNestedContext", func(b *testing.B) {
		// Test performance with deeply nested contexts (10 levels)
		ctx := context.Background()

		// Create 10 levels of nested context
		for i := range 10 {
			ctx = mtlog.PushProperty(ctx, fmt.Sprintf("Level%d", i), fmt.Sprintf("value%d", i))
		}

		log := logger.WithContext(ctx)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			log.Information("Deeply nested context test")
		}
	})

	b.Run("DeeplyNestedContextCreation", func(b *testing.B) {
		// Benchmark the cost of creating deeply nested contexts
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			ctx := context.Background()
			for j := range 10 {
				ctx = mtlog.PushProperty(ctx, fmt.Sprintf("Level%d", j), fmt.Sprintf("value%d", j))
			}
		}
	})
}
