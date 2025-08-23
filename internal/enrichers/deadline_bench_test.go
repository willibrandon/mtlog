package enrichers

import (
	"context"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// BenchmarkDeadlineEnricher_NoContext benchmarks enricher with no context (zero-cost path).
func BenchmarkDeadlineEnricher_NoContext(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_NoDeadline benchmarks enricher with context but no deadline.
func BenchmarkDeadlineEnricher_NoDeadline(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx := context.Background()
	factory := &mockPropertyFactory{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_FarFromDeadline benchmarks enricher when deadline is far.
func BenchmarkDeadlineEnricher_FarFromDeadline(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Hour)
	defer cancel()
	factory := &mockPropertyFactory{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_ApproachingDeadline benchmarks enricher when deadline is approaching.
func BenchmarkDeadlineEnricher_ApproachingDeadline(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	factory := &mockPropertyFactory{}

	// Pre-log first warning to avoid that overhead in benchmark
	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	enricher.Enrich(event, factory)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_CacheHit benchmarks cache hit performance.
func BenchmarkDeadlineEnricher_CacheHit(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Hour)
	defer cancel()
	factory := &mockPropertyFactory{}

	// Prime the cache
	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	enricher.Enrich(event, factory)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_CacheMiss benchmarks cache miss performance.
func BenchmarkDeadlineEnricher_CacheMiss(b *testing.B) {
	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineCacheSize(1)) // Very small cache to force misses
	factory := &mockPropertyFactory{}

	contexts := make([]context.Context, 100)
	cancels := make([]context.CancelFunc, 100)
	for i := range contexts {
		contexts[i], cancels[i] = context.WithTimeout(context.Background(), 10*time.Hour)
		defer cancels[i]()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := contexts[i%len(contexts)]
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		enricher.Enrich(event, factory)
	}
}

// BenchmarkDeadlineEnricher_Concurrent benchmarks concurrent access to the enricher.
func BenchmarkDeadlineEnricher_Concurrent(b *testing.B) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Hour)
	defer cancel()
	factory := &mockPropertyFactory{}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			event := &core.LogEvent{
				Level:      core.InformationLevel,
				Properties: map[string]any{"__context__": ctx},
			}
			enricher.Enrich(event, factory)
		}
	})
}

// BenchmarkDeadlineCache_GetPut benchmarks cache get/put operations.
func BenchmarkDeadlineCache_GetPut(b *testing.B) {
	cache := newDeadlineLRUCache(1000, 5*time.Minute)
	ctx := context.Background()
	info := &deadlineInfo{
		hasDeadline: true,
		deadline:    time.Now().Add(time.Hour),
		lastCheck:   time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.put(ctx, info)
		_ = cache.get(ctx)
	}
}

// BenchmarkDeadlineCache_ConcurrentAccess benchmarks concurrent cache access.
func BenchmarkDeadlineCache_ConcurrentAccess(b *testing.B) {
	cache := newDeadlineLRUCache(1000, 5*time.Minute)
	contexts := make([]context.Context, 100)
	for i := range contexts {
		contexts[i] = context.Background()
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx := contexts[i%len(contexts)]
			i++
			
			info := &deadlineInfo{
				hasDeadline: true,
				deadline:    time.Now().Add(time.Hour),
				lastCheck:   time.Now(),
			}
			cache.put(ctx, info)
			_ = cache.get(ctx)
		}
	})
}