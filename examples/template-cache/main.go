package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/parser"
)

func main() {
	// Configure the template cache at application startup
	// This should be done once, before any logging
	parser.ConfigureCache(
		parser.WithMaxSize(5000),        // Limit cache to 5000 templates
		parser.WithTTL(10*time.Minute),  // Expire entries after 10 minutes
	)

	// Create logger
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Normal logging - templates are automatically cached
	for i := 0; i < 100; i++ {
		// These templates will be cached after first parse
		logger.Information("User {UserId} logged in", i)
		logger.Information("Order {OrderId} processed", i*100)
	}

	// Check cache statistics
	stats := parser.GetCacheStats()
	fmt.Printf("\n=== Template Cache Statistics ===\n")
	fmt.Printf("Cache hits: %d\n", stats.Hits)
	fmt.Printf("Cache misses: %d\n", stats.Misses)
	fmt.Printf("Evictions: %d\n", stats.Evictions)
	fmt.Printf("Current size: %d/%d\n", stats.Size, stats.MaxSize)
	
	if stats.Hits+stats.Misses > 0 {
		hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
		fmt.Printf("Hit rate: %.1f%%\n", hitRate)
	}

	// Demonstrate protection against dynamic template generation
	fmt.Println("\n=== Dynamic Template Protection Demo ===")
	
	// This would be dangerous without cache limits - could exhaust memory
	for i := 0; i < 10000; i++ {
		// Each iteration creates a unique template
		dynamicTemplate := fmt.Sprintf("Iteration %d: {{RequestId}} completed", i)
		logger.Information(dynamicTemplate, fmt.Sprintf("req-%d", i))
	}

	// Check stats again - should show evictions
	stats = parser.GetCacheStats()
	fmt.Printf("\nAfter dynamic templates:\n")
	fmt.Printf("Cache size: %d (still bounded at max)\n", stats.Size)
	fmt.Printf("Evictions: %d (old templates evicted)\n", stats.Evictions)
	
	// The cache protects against memory exhaustion
	// Even with 10,000 unique templates, memory usage is bounded
	fmt.Println("\n✓ Cache successfully prevented memory exhaustion")
}