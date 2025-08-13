package parser

import (
	"fmt"
	"runtime"
	"testing"
)

// TestCacheMemoryExhaustionProtection demonstrates that the cache is protected
// against the memory exhaustion vulnerability described in issue #39
func TestCacheMemoryExhaustionProtection(t *testing.T) {
	// Create a dedicated cache for this test to avoid interference
	cache := NewTemplateCache(WithMaxSize(100))
	defer cache.Close()
	
	// Get initial memory stats
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	// Simulate the attack scenario from issue #39
	// Try to create 1 million unique templates
	for i := 0; i < 1_000_000; i++ {
		template := fmt.Sprintf("User %d: {Action}", i)
		
		// Check cache first
		if cached, ok := cache.Get(template); ok {
			_ = cached
			continue
		}
		
		// Parse if not cached
		parsed, err := Parse(template)
		if err != nil {
			t.Fatalf("Parse failed at iteration %d: %v", i, err)
		}
		
		// Store in cache
		cache.Put(template, parsed)
	}
	
	// Force garbage collection
	runtime.GC()
	runtime.GC()
	
	// Get final memory stats
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	
	// Check cache stats
	stats := cache.Stats()
	
	// Cache size should be bounded at max size
	if stats.Size > 100 {
		t.Errorf("Cache size %d exceeds configured max of 100", stats.Size)
	}
	
	// Should have many evictions (approximately 999,900)
	if stats.Evictions < 999_000 {
		t.Errorf("Expected at least 999,000 evictions, got %d", stats.Evictions)
	}
	
	// Memory growth should be minimal (allowing for some overhead)
	memGrowth := m2.Alloc - m1.Alloc
	maxAllowedGrowth := uint64(10 * 1024 * 1024) // 10MB max growth
	
	if memGrowth > maxAllowedGrowth {
		t.Errorf("Memory grew by %d bytes, exceeding allowed %d bytes", memGrowth, maxAllowedGrowth)
	}
	
	t.Logf("Memory growth: %d bytes", memGrowth)
	t.Logf("Cache stats: Size=%d, Evictions=%d, Hits=%d, Misses=%d",
		stats.Size, stats.Evictions, stats.Hits, stats.Misses)
}

// TestCacheWithDynamicTemplates tests the specific scenario mentioned in the issue
func TestCacheWithDynamicTemplates(t *testing.T) {
	// Create a dedicated cache for this test
	cache := NewTemplateCache(WithMaxSize(50))
	defer cache.Close()
	
	// Simulate dynamic template generation in a loop
	for i := 0; i < 10_000; i++ {
		// Dynamic template that changes with each iteration
		dynamicTemplate := fmt.Sprintf("Iteration %d: User {UserId} performed {Action} at {Timestamp}", i)
		
		// Check cache first
		if cached, ok := cache.Get(dynamicTemplate); ok {
			_ = cached
			continue
		}
		
		// Parse if not cached
		parsed, err := Parse(dynamicTemplate)
		if err != nil {
			t.Fatalf("Failed to parse template at iteration %d: %v", i, err)
		}
		
		if parsed == nil {
			t.Fatal("Got nil template")
		}
		
		// Store in cache
		cache.Put(dynamicTemplate, parsed)
	}
	
	// Verify cache is still bounded
	stats := cache.Stats()
	if stats.Size > 50 {
		t.Errorf("Cache size %d exceeds max size 50", stats.Size)
	}
	
	// Should have significant evictions
	if stats.Evictions < 9900 {
		t.Errorf("Expected at least 9900 evictions, got %d", stats.Evictions)
	}
	
	t.Logf("After 10,000 dynamic templates - Size: %d, Evictions: %d", stats.Size, stats.Evictions)
}

// BenchmarkCacheUnderPressure simulates continuous pressure with unique templates
func BenchmarkCacheUnderPressure(b *testing.B) {
	ResetGlobalCacheForTesting()
	ConfigureGlobalCache(WithMaxSize(1000))
	defer func() {
		ClearCache()
		ResetGlobalCacheForTesting()
	}()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Each iteration uses a unique template
		template := fmt.Sprintf("Benchmark iteration %d: {Value}", i)
		_, err := ParseCached(template)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	// Report final stats
	stats := GetCacheStats()
	b.ReportMetric(float64(stats.Evictions), "evictions")
	b.ReportMetric(float64(stats.Size), "final_size")
}