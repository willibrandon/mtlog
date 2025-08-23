package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	fmt.Println("=== mtlog Per-Message Sampling Examples ===")

	// Example 1: Sample every Nth message
	example1SampleEveryNth()

	// Example 2: Time-based sampling
	example2TimeBased()

	// Example 3: Rate-based sampling
	example3RateBased()

	// Example 4: First N occurrences
	example4FirstN()

	// Example 5: Group sampling
	example5GroupSampling()

	// Example 6: Conditional sampling
	example6ConditionalSampling()

	// Example 7: Exponential backoff
	example7ExponentialBackoff()

	// Example 8: Production scenario
	example8ProductionScenario()

	// Example 9: Advanced configuration examples
	fmt.Println()
	ExampleSamplingConfigBuilder()
}

// Example 1: Sample every Nth message
func example1SampleEveryNth() {
	fmt.Println("Example 1: Sample Every Nth Message")
	fmt.Println("------------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Sample every 3rd message
	sampledLogger := logger.Sample(3)

	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Processing item {ItemNumber}", i)
	}
	// Output: Messages 1, 4, 7, 10 are logged

	fmt.Println()
}

// Example 2: Time-based sampling
func example2TimeBased() {
	fmt.Println("Example 2: Time-Based Sampling")
	fmt.Println("-------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Sample at most once per 100ms
	sampledLogger := logger.SampleDuration(100 * time.Millisecond)

	// Rapid logging - only first and those 100ms+ apart are logged
	for i := 1; i <= 20; i++ {
		sampledLogger.Info("Rapid event {Number}", i)
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println()
}

// Example 3: Rate-based sampling
func example3RateBased() {
	fmt.Println("Example 3: Rate-Based Sampling (10%)")
	fmt.Println("-------------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Sample 10% of messages
	sampledLogger := logger.SampleRate(0.1)

	for i := 1; i <= 20; i++ {
		sampledLogger.Info("High volume event {EventId}", i)
	}
	// Approximately 2 messages logged (10% of 20)

	fmt.Println()
}

// Example 4: First N occurrences
func example4FirstN() {
	fmt.Println("Example 4: Log First N Occurrences")
	fmt.Println("-----------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Only log first 3 occurrences
	sampledLogger := logger.SampleFirst(3)

	for i := 1; i <= 10; i++ {
		sampledLogger.Warning("Initialization warning {Step}", i)
	}
	// Only warnings 1, 2, 3 are logged

	fmt.Println()
}

// Example 5: Group sampling
func example5GroupSampling() {
	fmt.Println("Example 5: Group Sampling (Shared Counters)")
	fmt.Println("--------------------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Multiple loggers sharing the same sampling group
	dbLogger := logger.SampleGroup("database", 5)
	cacheLogger := logger.SampleGroup("database", 5)

	// Both loggers share the same counter
	for i := 1; i <= 6; i++ {
		dbLogger.Info("Database query {QueryId}", i)
		cacheLogger.Info("Cache lookup {LookupId}", i)
	}
	// Total of 5 messages logged across both loggers (messages 1, 6, 11)

	fmt.Println()
}

// Example 6: Conditional sampling
func example6ConditionalSampling() {
	fmt.Println("Example 6: Conditional Sampling")
	fmt.Println("--------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Simulate load condition
	var highLoad atomic.Bool
	
	// Sample every 3rd message only when high load
	sampledLogger := logger.SampleWhen(func() bool {
		return highLoad.Load()
	}, 3)

	// Normal load - all messages filtered
	fmt.Println("Normal load (no sampling):")
	for i := 1; i <= 3; i++ {
		sampledLogger.Debug("Normal load debug {Number}", i)
	}

	// High load - sample every 3rd
	highLoad.Store(true)
	fmt.Println("High load (sample every 3rd):")
	for i := 4; i <= 10; i++ {
		sampledLogger.Debug("High load debug {Number}", i)
	}
	// Messages 4, 7, 10 are logged

	fmt.Println()
}

// Example 7: Exponential backoff
func example7ExponentialBackoff() {
	fmt.Println("Example 7: Exponential Backoff Sampling")
	fmt.Println("----------------------------------------")

	logger := mtlog.New(mtlog.WithConsole())
	
	// Log with exponential backoff (factor 2)
	// Logs at: 1st, 2nd, 4th, 8th, 16th... occurrence
	errorLogger := logger.SampleBackoff("connection-error", 2.0)

	for i := 1; i <= 20; i++ {
		errorLogger.Error("Connection failed, attempt {Attempt}", i)
	}
	// Messages 1, 2, 4, 8, 16 are logged

	fmt.Println()
}

// Example 8: Production scenario - API endpoint logging
func example8ProductionScenario() {
	fmt.Println("Example 8: Production API Endpoint Logging")
	fmt.Println("-------------------------------------------")

	baseLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Different sampling strategies for different endpoints
	
	// Health check endpoint - heavily rate limited
	healthLogger := baseLogger.
		ForContext("Endpoint", "/health").
		SampleDuration(10 * time.Second) // At most once per 10 seconds

	// High-traffic read endpoint - sample percentage
	readLogger := baseLogger.
		ForContext("Endpoint", "/api/users").
		SampleRate(0.01) // 1% of requests

	// Write endpoint - log first few then back off
	writeLogger := baseLogger.
		ForContext("Endpoint", "/api/users/create").
		SampleFirst(100) // First 100 requests

	// Error logging with backoff
	errorLogger := baseLogger.
		SampleBackoff("api-error", 2.0)

	// Simulate traffic
	var wg sync.WaitGroup
	
	// Health checks (1000 requests)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			healthLogger.Info("Health check {RequestId}", generateRequestId())
			time.Sleep(time.Millisecond)
		}
	}()

	// Read requests (100 requests)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			readLogger.Info("User fetch {UserId} {RequestId}", 
				rand.Intn(1000), generateRequestId())
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Write requests (50 requests)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			writeLogger.Info("User created {UserId} {RequestId}", 
				1000+i, generateRequestId())
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Errors (30 attempts)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			errorLogger.Error("Database connection error {Attempt}", i+1)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	wg.Wait()
	
	fmt.Println("\nProduction scenario complete!")
	fmt.Println("- Health checks: ~1-2 messages logged (time-based)")
	fmt.Println("- Read requests: ~1 message logged (1% sampling)")  
	fmt.Println("- Write requests: First 50 logged (within limit)")
	fmt.Println("- Errors: ~5 messages logged (exponential backoff)")
}

func generateRequestId() string {
	return fmt.Sprintf("req-%d", rand.Intn(1000000))
}