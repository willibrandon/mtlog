package parser

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLRUCacheBasicOperations(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(3))
	defer cache.Close()

	// Test Put and Get
	tmpl1 := &MessageTemplate{Raw: "template1"}
	cache.Put("key1", tmpl1)

	got, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if got.Raw != "template1" {
		t.Errorf("expected template1, got %s", got.Raw)
	}

	// Test miss
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Fatal("expected miss for nonexistent key")
	}

	// Test statistics
	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestLRUCacheEviction(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(3))
	defer cache.Close()

	// Fill cache to capacity
	for i := 1; i <= 3; i++ {
		cache.Put(fmt.Sprintf("key%d", i), &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
	}

	// Access key1 and key2 to make them more recently used
	cache.Get("key1")
	cache.Get("key2")

	// Add key4, should evict key3 (least recently used)
	cache.Put("key4", &MessageTemplate{Raw: "template4"})

	// key3 should be evicted
	_, ok := cache.Get("key3")
	if ok {
		t.Error("expected key3 to be evicted")
	}

	// key1, key2, key4 should still be present
	for _, key := range []string{"key1", "key2", "key4"} {
		if _, ok := cache.Get(key); !ok {
			t.Errorf("expected %s to be present", key)
		}
	}

	stats := cache.Stats()
	if stats.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestLRUCacheTTL(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(10), WithTTL(100*time.Millisecond))
	defer cache.Close()

	cache.Put("key1", &MessageTemplate{Raw: "template1"})

	// Should be present initially
	if _, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to be present initially")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be expired")
	}

	stats := cache.Stats()
	if stats.Expirations != 1 {
		t.Errorf("expected 1 expiration, got %d", stats.Expirations)
	}
}

func TestLRUCacheConcurrency(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(1000))
	defer cache.Close()

	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j%10)
				tmpl := &MessageTemplate{Raw: fmt.Sprintf("template-%d-%d", id, j)}

				// Mix of operations
				switch j % 3 {
				case 0:
					cache.Put(key, tmpl)
				case 1:
					cache.Get(key)
				case 2:
					cache.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()

	// Cache should still be functional
	cache.Put("final", &MessageTemplate{Raw: "final"})
	if got, ok := cache.Get("final"); !ok || got.Raw != "final" {
		t.Error("cache not functional after concurrent operations")
	}
}

func TestLRUCacheShardDistribution(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(100))
	defer cache.Close()

	// Add many entries
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Put(key, &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
	}

	// Check that entries are distributed across shards
	shardSizes := make([]int, len(cache.shards))
	for i, shard := range cache.shards {
		shard.mu.Lock()
		shardSizes[i] = len(shard.entries)
		shard.mu.Unlock()
	}

	// At least some shards should have entries
	hasEntries := false
	for _, size := range shardSizes {
		if size > 0 {
			hasEntries = true
			break
		}
	}

	if !hasEntries {
		t.Error("no shards have entries")
	}

	stats := cache.Stats()
	if stats.Size > 100 {
		t.Errorf("cache size %d exceeds max size 100", stats.Size)
	}
}

func TestLRUCacheCapacityDistribution(t *testing.T) {
	testCases := []struct {
		maxSize        int
		expectedShards int
	}{
		{10, 1},     // Small cache uses 1 shard
		{100, 1},    // Still small enough for 1 shard
		{500, 2},    // Medium cache
		{2000, 8},   // Larger cache
		{10000, 32}, // Large cache
		{20000, 64}, // Very large cache hits max shards
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("maxSize=%d", tc.maxSize), func(t *testing.T) {
			cache := NewTemplateCache(WithMaxSize(tc.maxSize))
			defer cache.Close()

			// Calculate total capacity
			totalCapacity := 0
			for _, shard := range cache.shards {
				totalCapacity += shard.maxSize
			}

			if totalCapacity != tc.maxSize {
				t.Errorf("total capacity %d != configured max size %d", totalCapacity, tc.maxSize)
			}

			actualShards := len(cache.shards)
			// Check that shard count is reasonable (power of 2)
			if actualShards&(actualShards-1) != 0 {
				t.Errorf("shard count %d is not a power of 2", actualShards)
			}
		})
	}
}

func TestLRUCacheClear(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(10))
	defer cache.Close()

	// Add some entries
	for i := 0; i < 5; i++ {
		cache.Put(fmt.Sprintf("key%d", i), &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
	}

	// Verify entries exist
	stats := cache.Stats()
	if stats.Size != 5 {
		t.Errorf("expected size 5, got %d", stats.Size)
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.Size)
	}

	// Stats should be reset
	if stats.Hits != 0 || stats.Misses != 0 || stats.Evictions != 0 || stats.Expirations != 0 {
		t.Error("expected all stats to be reset after clear")
	}

	// Cache should still be functional
	cache.Put("new", &MessageTemplate{Raw: "new"})
	if _, ok := cache.Get("new"); !ok {
		t.Error("cache not functional after clear")
	}
}

func TestLRUCacheDelete(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(10))
	defer cache.Close()

	cache.Put("key1", &MessageTemplate{Raw: "template1"})
	cache.Put("key2", &MessageTemplate{Raw: "template2"})

	// Delete existing key
	if !cache.Delete("key1") {
		t.Error("expected Delete to return true for existing key")
	}

	// Verify it's gone
	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be deleted")
	}

	// Delete non-existent key
	if cache.Delete("nonexistent") {
		t.Error("expected Delete to return false for non-existent key")
	}

	// key2 should still exist
	if _, ok := cache.Get("key2"); !ok {
		t.Error("expected key2 to still exist")
	}
}

func TestGlobalCacheConfiguration(t *testing.T) {
	// Reset for testing
	ResetGlobalCacheForTesting()
	defer ResetGlobalCacheForTesting()

	// Configure global cache
	ConfigureGlobalCache(WithMaxSize(50), WithTTL(time.Second))

	// Get the configured cache
	cache := GetGlobalCache()
	if cache == nil {
		t.Fatal("expected global cache to be initialized")
	}

	// Use through ParseCached
	tmpl, err := ParseCached("User {UserId} logged in")
	if err != nil {
		t.Fatalf("ParseCached failed: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected non-nil template")
	}

	// Should be cached
	tmpl2, err := ParseCached("User {UserId} logged in")
	if err != nil {
		t.Fatalf("ParseCached failed: %v", err)
	}
	if tmpl != tmpl2 {
		t.Error("expected same template instance from cache")
	}

	stats := GetCacheStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}

	// Clear for other tests
	ClearCache()
}

func TestGlobalCacheReconfigurationPanic(t *testing.T) {
	// Reset for testing
	ResetGlobalCacheForTesting()
	defer ResetGlobalCacheForTesting()

	// Configure global cache once
	ConfigureGlobalCache(WithMaxSize(50))

	// Verify that reconfiguration panics
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on reconfiguration")
		} else if msg, ok := r.(string); ok {
			if !strings.Contains(msg, "already configured") {
				t.Errorf("unexpected panic message: %s", msg)
			}
		}
	}()

	// This should panic
	ConfigureGlobalCache(WithMaxSize(100))
}

func TestLRUCacheCleanup(t *testing.T) {
	cache := NewTemplateCache(WithMaxSize(10), WithTTL(50*time.Millisecond))

	// Add entries
	for i := 0; i < 5; i++ {
		cache.Put(fmt.Sprintf("key%d", i), &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
	}

	// Wait for cleanup to run
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup manually (normally runs periodically)
	cache.cleanupExpired()

	// All entries should be expired
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after cleanup, got %d", stats.Size)
	}
	if stats.Expirations != 5 {
		t.Errorf("expected 5 expirations, got %d", stats.Expirations)
	}

	// Close should work without issues
	cache.Close()
	cache.Close() // Second close should be no-op
}

// Benchmark tests
func BenchmarkLRUCacheGet(b *testing.B) {
	cache := NewTemplateCache(WithMaxSize(10000))
	defer cache.Close()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Put(fmt.Sprintf("key%d", i), &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%1000)
			cache.Get(key)
			i++
		}
	})
}

func BenchmarkLRUCachePut(b *testing.B) {
	cache := NewTemplateCache(WithMaxSize(10000))
	defer cache.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			cache.Put(key, &MessageTemplate{Raw: fmt.Sprintf("template%d", i)})
			i++
		}
	})
}

func BenchmarkParseCached(b *testing.B) {
	// Use a mix of templates that will hit and miss
	templates := []string{
		"User {UserId} logged in",
		"Order {OrderId} processed",
		"Payment {PaymentId} received",
		"Error processing {RequestId}: {Error}",
		"Cache hit ratio: {Ratio:P2}",
	}

	ResetGlobalCacheForTesting()
	ConfigureGlobalCache(WithMaxSize(100))
	b.Cleanup(func() {
		stats := GetCacheStats()
		if stats.Hits+stats.Misses > 0 {
			hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		}
		ResetGlobalCacheForTesting()
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			template := templates[i%len(templates)]
			_, err := ParseCached(template)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func TestLRUCacheMemoryBounded(t *testing.T) {
	const maxSize = 100
	const numEntries = 10000
	cache := NewTemplateCache(WithMaxSize(maxSize))
	defer cache.Close()

	// Try to add way more entries than the max size
	for i := range numEntries {
		key := fmt.Sprintf("dynamic-key-%d", i)
		cache.Put(key, &MessageTemplate{Raw: fmt.Sprintf("template-%d", i)})
	}

	// Cache size should never exceed max
	stats := cache.Stats()
	if stats.Size > maxSize {
		t.Errorf("cache size %d exceeds max size %d", stats.Size, maxSize)
	}

	// Should have many evictions (at least 90% of the overflow)
	minExpectedEvictions := int((numEntries - maxSize) * 9 / 10)
	if stats.Evictions < uint64(minExpectedEvictions) {
		t.Errorf("expected at least %d evictions, got %d", minExpectedEvictions, stats.Evictions)
	}

	// Force GC and check memory is reasonable
	runtime.GC()
	runtime.GC() // Run twice to ensure finalizers run
}
