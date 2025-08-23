package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	fmt.Println("=== Advanced Per-Message Sampling Features ===")
	fmt.Println()

	// Example 1: Cache Warmup for High-Traffic Applications
	example1CacheWarmup()

	// Example 2: Sampling Statistics and Summaries
	example2SamplingStatistics()

	// Example 3: Memory-Limited Sampling
	example3MemoryLimitedSampling()
}

// Example 1: Cache Warmup for High-Traffic Applications
func example1CacheWarmup() {
	fmt.Println("Example 1: Cache Warmup")
	fmt.Println("-----------------------")

	// Pre-warm caches at application startup to avoid cold-start allocation spikes
	commonGroups := []string{"database", "cache", "api", "auth", "metrics"}
	mtlog.WarmupSamplingGroups(commonGroups)
	fmt.Printf("✓ Pre-warmed %d group names\n", len(commonGroups))

	commonErrors := []string{"connection-error", "timeout-error", "rate-limit", "auth-failed"}
	mtlog.WarmupSamplingBackoff(commonErrors, 2.0)
	fmt.Printf("✓ Pre-warmed %d error keys\n", len(commonErrors))

	// Now use the pre-warmed keys - no allocation spike
	logger := mtlog.New(mtlog.WithConsole())
	
	dbLogger := logger.SampleGroup("database", 10)
	dbLogger.Information("Database connection established")
	
	errorLogger := logger.SampleBackoff("connection-error", 2.0)
	errorLogger.Error("Initial connection attempt failed")

	fmt.Println()
}

// Example 2: Sampling Statistics and Summaries  
func example2SamplingStatistics() {
	fmt.Println("Example 2: Sampling Statistics & Summaries")
	fmt.Println("------------------------------------------")

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Create a sampled logger with periodic summary emission
	sampledLogger := logger.
		Sample(5).                                      // Every 5th message
		EnableSamplingSummary(2 * time.Second)         // Emit summary every 2 seconds

	// Simulate high-frequency logging
	fmt.Println("Logging 50 messages with sampling...")
	for i := 1; i <= 50; i++ {
		sampledLogger.Information("High-frequency event {EventId}", i)
		time.Sleep(10 * time.Millisecond)
	}

	// Check statistics
	sampled, skipped := sampledLogger.GetSamplingStats()
	fmt.Printf("\nStatistics: %d sampled, %d skipped (%.1f%% sampling rate)\n",
		sampled, skipped, float64(sampled)*100/float64(sampled+skipped))

	// Wait for summary to be emitted
	fmt.Println("\nWaiting for automatic summary emission...")
	time.Sleep(2100 * time.Millisecond)

	fmt.Println()
}

// Example 3: Memory-Limited Sampling
func example3MemoryLimitedSampling() {
	fmt.Println("Example 3: Memory-Limited Sampling")
	fmt.Println("-----------------------------------")

	// Configure memory limits to prevent unbounded growth
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSamplingMemoryLimit(1000), // Limit to 1000 cached keys
	)

	fmt.Println("✓ Configured sampling cache limit to 1000 keys")

	// Simulate dynamic key generation (e.g., user IDs, session IDs)
	for userId := 1; userId <= 10; userId++ {
		userLogger := logger.SampleGroup(fmt.Sprintf("user-%d", userId), 3)
		for event := 1; event <= 5; event++ {
			userLogger.Information("User {UserId} event {EventNum}", userId, event)
		}
	}

	fmt.Println("✓ Dynamic sampling groups with LRU eviction")
	fmt.Println()

	// Show how per-filter random sources prevent contention
	fmt.Println("Using per-filter random sources (no global rand contention):")
	
	// Each logger gets its own random source
	logger1 := logger.SampleRate(0.3)
	logger2 := logger.SampleRate(0.3)
	logger3 := logger.SampleRate(0.3)

	for i := 1; i <= 10; i++ {
		logger1.Information("Service A event {Num}", i)
		logger2.Information("Service B event {Num}", i) 
		logger3.Information("Service C event {Num}", i)
	}

	fmt.Println("✓ Each sampler uses independent random source")
	fmt.Println()

	fmt.Println("All advanced features demonstrated successfully!")
}