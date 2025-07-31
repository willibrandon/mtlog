package enrichers

import (
	"sync"
	"testing"

	"github.com/willibrandon/mtlog/core"
)

func TestSourceContextCaching(t *testing.T) {
	// Clear cache before test
	sourceContextCache.Lock()
	sourceContextCache.m = make(map[uintptr]*lruEntry)
	sourceContextCache.head = nil
	sourceContextCache.tail = nil
	sourceContextCache.size = 0
	sourceContextCache.Unlock()

	enricher := NewAutoSourceContextEnricher()
	factory := &mockPropertyFactory{}

	// First call - should detect and cache
	event1 := &core.LogEvent{
		Properties: make(map[string]any),
	}
	enricher.Enrich(event1, factory)

	ctx1, ok := event1.Properties["SourceContext"]
	if !ok {
		t.Fatal("SourceContext not added to event")
	}

	// Check cache was populated
	sourceContextCache.RLock()
	cacheSize := len(sourceContextCache.m)
	sourceContextCache.RUnlock()

	if cacheSize == 0 {
		t.Error("Expected cache to be populated after first call")
	}

	// Second call - should use cache
	event2 := &core.LogEvent{
		Properties: make(map[string]any),
	}
	enricher.Enrich(event2, factory)

	ctx2, ok := event2.Properties["SourceContext"]
	if !ok {
		t.Fatal("SourceContext not added to second event")
	}

	// Should get same context
	if ctx1 != ctx2 {
		t.Errorf("Expected same context from cache, got %v and %v", ctx1, ctx2)
	}
}

func TestSourceContextCacheConcurrency(t *testing.T) {
	// Clear cache before test
	sourceContextCache.Lock()
	sourceContextCache.m = make(map[uintptr]*lruEntry)
	sourceContextCache.head = nil
	sourceContextCache.tail = nil
	sourceContextCache.size = 0
	sourceContextCache.Unlock()

	enricher := NewAutoSourceContextEnricher()
	factory := &mockPropertyFactory{}

	// Run concurrent enrichments
	var wg sync.WaitGroup
	results := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := &core.LogEvent{
				Properties: make(map[string]any),
			}
			enricher.Enrich(event, factory)
			if ctx, ok := event.Properties["SourceContext"]; ok {
				results[idx] = ctx.(string)
			}
		}(i)
	}

	wg.Wait()

	// All results should be the same
	first := results[0]
	for i, ctx := range results {
		if ctx != first {
			t.Errorf("Concurrent access returned different contexts at index %d: %v vs %v", i, ctx, first)
		}
	}

	// Cache should have entries
	sourceContextCache.RLock()
	cacheSize := len(sourceContextCache.m)
	sourceContextCache.RUnlock()

	if cacheSize == 0 {
		t.Error("Expected cache to be populated after concurrent calls")
	}
}

func TestSourceContextExplicitOverridesCache(t *testing.T) {
	// Test that explicit source context doesn't use cache
	enricher := NewSourceContextEnricher("ExplicitContext")
	factory := &mockPropertyFactory{}

	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	enricher.Enrich(event, factory)

	ctx, ok := event.Properties["SourceContext"]
	if !ok {
		t.Fatal("SourceContext not added to event")
	}

	if ctx != "ExplicitContext" {
		t.Errorf("Expected explicit context 'ExplicitContext', got %v", ctx)
	}
}

func BenchmarkSourceContextWithCache(b *testing.B) {
	enricher := NewAutoSourceContextEnricher()
	factory := &mockPropertyFactory{}

	// Warm up cache
	warmupEvent := &core.LogEvent{
		Properties: make(map[string]any),
	}
	enricher.Enrich(warmupEvent, factory)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			event := &core.LogEvent{
				Properties: make(map[string]any),
			}
			enricher.Enrich(event, factory)
		}
	})
}

func BenchmarkSourceContextWithoutCache(b *testing.B) {
	// This benchmark simulates the cost without caching by clearing cache each time
	enricher := NewAutoSourceContextEnricher()
	factory := &mockPropertyFactory{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Clear cache to simulate no caching
			sourceContextCache.Lock()
			sourceContextCache.m = make(map[uintptr]*lruEntry)
			sourceContextCache.head = nil
			sourceContextCache.tail = nil
			sourceContextCache.size = 0
			sourceContextCache.Unlock()

			event := &core.LogEvent{
				Properties: make(map[string]any),
			}
			enricher.Enrich(event, factory)
		}
	})
}
