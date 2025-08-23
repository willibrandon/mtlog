package main

import (
	"fmt"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/selflog"
)

func main() {
	// Enable selflog to see debug output
	selflog.Enable(os.Stdout)
	defer selflog.Disable()
	
	fmt.Println("=== Sampling Debug Example ===")
	
	// Example 1: Debug counter-based sampling
	debugCounterSampling()
	
	// Example 2: Debug rate-based sampling
	debugRateSampling()
	
	// Example 3: Debug group sampling
	debugGroupSampling()
	
	// Example 4: Debug backoff sampling
	debugBackoffSampling()
	
	// Example 5: Check debug status
	checkDebugStatus()
}

func debugCounterSampling() {
	fmt.Println("--- Counter-Based Sampling Debug ---")
	
	// Enable sampling debug
	mtlog.EnableSamplingDebug()
	defer mtlog.DisableSamplingDebug()
	
	// Create logger with counter sampling
	logger := mtlog.New(mtlog.WithConsole()).Sample(3) // Every 3rd message
	
	// Log messages - debug output will show decisions
	for i := 1; i <= 6; i++ {
		logger.Info("Counter message {Number}", i)
	}
	
	// Show stats
	sampled, skipped := logger.GetSamplingStats()
	fmt.Printf("Stats: Sampled=%d, Skipped=%d\n\n", sampled, skipped)
}

func debugRateSampling() {
	fmt.Println("--- Rate-Based Sampling Debug ---")
	
	// Enable sampling debug
	mtlog.EnableSamplingDebug()
	defer mtlog.DisableSamplingDebug()
	
	// Create logger with rate sampling
	logger := mtlog.New(mtlog.WithConsole()).SampleRate(0.5) // 50% sampling
	
	// Log messages - debug output will show decisions
	for i := 1; i <= 10; i++ {
		logger.Info("Rate message {Number}", i)
	}
	
	// Show stats
	sampled, skipped := logger.GetSamplingStats()
	fmt.Printf("Stats: Sampled=%d, Skipped=%d (approximately 50%%)\n\n", sampled, skipped)
}

func debugGroupSampling() {
	fmt.Println("--- Group Sampling Debug ---")
	
	// Enable sampling debug
	mtlog.EnableSamplingDebug()
	defer mtlog.DisableSamplingDebug()
	
	logger := mtlog.New(mtlog.WithConsole())
	
	// Create multiple group samplers
	apiLogger := logger.SampleGroup("api-endpoint", 2)
	dbLogger := logger.SampleGroup("database-query", 3)
	
	// Log to different groups
	for i := 1; i <= 4; i++ {
		apiLogger.Info("API request {RequestID}", i)
		dbLogger.Info("DB query {QueryID}", i)
	}
	
	fmt.Println()
}

func debugBackoffSampling() {
	fmt.Println("--- Backoff Sampling Debug ---")
	
	// Enable sampling debug
	mtlog.EnableSamplingDebug()
	defer mtlog.DisableSamplingDebug()
	
	logger := mtlog.New(mtlog.WithConsole())
	
	// Simulate errors with backoff
	for i := 1; i <= 8; i++ {
		// Use backoff sampling for error messages
		logger.SampleBackoff("error-type-1", 2.0).
			Error("Error occurred: attempt {Attempt}", i)
		
		// Small delay to show time-based behavior
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Println()
}

func checkDebugStatus() {
	fmt.Println("--- Debug Status Check ---")
	
	// Check initial status
	fmt.Printf("Debug enabled initially: %v\n", mtlog.IsSamplingDebugEnabled())
	
	// Enable and check
	mtlog.EnableSamplingDebug()
	fmt.Printf("Debug enabled after enable: %v\n", mtlog.IsSamplingDebugEnabled())
	
	// Disable and check
	mtlog.DisableSamplingDebug()
	fmt.Printf("Debug enabled after disable: %v\n", mtlog.IsSamplingDebugEnabled())
	
	// Demonstrate conditional debug
	logger := mtlog.New(mtlog.WithConsole()).Sample(2)
	
	// Log without debug
	fmt.Println("\nLogging without debug:")
	for i := 1; i <= 3; i++ {
		logger.Info("No debug message {N}", i)
	}
	
	// Log with debug
	fmt.Println("\nLogging with debug:")
	mtlog.EnableSamplingDebug()
	for i := 4; i <= 6; i++ {
		logger.Info("With debug message {N}", i)
	}
	mtlog.DisableSamplingDebug()
	
	// Get final stats
	sampled, skipped := logger.GetSamplingStats()
	fmt.Printf("\nFinal stats: Sampled=%d, Skipped=%d\n", sampled, skipped)
}