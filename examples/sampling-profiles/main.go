package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	fmt.Println("=== Sampling Profiles Example ===")
	
	// Example 1: Discover available profiles
	discoverProfiles()
	
	// Example 2: Use different profiles
	useProfiles()
	
	// Example 3: Custom profile registration
	customProfileExample()
	
	// Example 4: Profile comparison
	compareProfiles()
}

func discoverProfiles() {
	fmt.Println("--- Available Sampling Profiles ---")
	
	// Get all available profiles with descriptions
	profiles := mtlog.GetAvailableProfileDescriptions()
	
	// Display profiles
	fmt.Println("Built-in profiles:")
	for name, description := range profiles {
		fmt.Printf("  • %s: %s\n", name, description)
	}
	fmt.Println()
}

func useProfiles() {
	fmt.Println("--- Using Different Profiles ---")
	
	baseLogger := mtlog.New(mtlog.WithConsole())
	
	// High Traffic API Profile
	fmt.Println("\n1. HighTrafficAPI Profile (aggressive sampling):")
	apiLogger := baseLogger.SampleProfile("HighTrafficAPI")
	
	// Simulate high-volume API requests
	for i := 0; i < 1000; i++ {
		apiLogger.Info("API request {RequestID}", i)
	}
	sampled, skipped := apiLogger.GetSamplingStats()
	fmt.Printf("   Results: Sampled=%d, Skipped=%d (%.2f%% sampled)\n", 
		sampled, skipped, float64(sampled)*100/float64(sampled+skipped))
	
	// Background Worker Profile
	fmt.Println("\n2. BackgroundWorker Profile (moderate sampling):")
	workerLogger := baseLogger.SampleProfile("BackgroundWorker")
	
	for i := 0; i < 100; i++ {
		workerLogger.Info("Processing job {JobID}", i)
	}
	sampled, skipped = workerLogger.GetSamplingStats()
	fmt.Printf("   Results: Sampled=%d, Skipped=%d (%.2f%% sampled)\n",
		sampled, skipped, float64(sampled)*100/float64(sampled+skipped))
	
	// Debug Verbose Profile
	fmt.Println("\n3. DebugVerbose Profile (minimal sampling):")
	debugLogger := baseLogger.SampleProfile("DebugVerbose")
	
	for i := 0; i < 200; i++ {
		debugLogger.Debug("Debug info {Index}", i)
	}
	sampled, skipped = debugLogger.GetSamplingStats()
	fmt.Printf("   Results: Sampled=%d, Skipped=%d\n", sampled, skipped)
	
	// Critical Alerts Profile
	fmt.Println("\n4. CriticalAlerts Profile (no sampling):")
	alertLogger := baseLogger.SampleProfile("CriticalAlerts")
	
	for i := 0; i < 10; i++ {
		alertLogger.Fatal("Critical alert {AlertID}", i)
	}
	sampled, skipped = alertLogger.GetSamplingStats()
	fmt.Printf("   Results: Sampled=%d, Skipped=%d (all messages logged)\n", sampled, skipped)
}

func customProfileExample() {
	fmt.Println("\n--- Custom Profile Registration ---")
	
	// Register a custom profile using AddCustomProfile
	// For simplicity, we'll just use a counter-based filter
	err := mtlog.AddCustomProfile("CustomMetrics",
		"Custom profile for metrics collection (every 5th message)",
		func() core.LogEventFilter {
			// Use the filters package to create a proper filter
			return &customFilter{n: 5}
		})
	
	if err != nil {
		fmt.Printf("Failed to register custom profile: %v\n", err)
		return
	}
	
	// Verify it was registered
	profiles := mtlog.GetAvailableProfileDescriptions()
	if desc, exists := profiles["CustomMetrics"]; exists {
		fmt.Printf("Custom profile registered: %s\n", desc)
	}
	
	// Use the custom profile
	baseLogger := mtlog.New(mtlog.WithConsole())
	customLogger := baseLogger.SampleProfile("CustomMetrics")
	
	// Test the custom profile
	start := time.Now()
	for i := 0; i < 50; i++ {
		customLogger.Info("Metric {Value}", i)
		time.Sleep(10 * time.Millisecond)
	}
	duration := time.Since(start)
	
	sampled, skipped := customLogger.GetSamplingStats()
	fmt.Printf("Custom profile results: Sampled=%d, Skipped=%d over %.1fs\n",
		sampled, skipped, duration.Seconds())
}

func compareProfiles() {
	fmt.Println("\n--- Profile Comparison ---")
	
	// Test message volume
	const messageCount = 1000
	
	profiles := []string{
		"HighTrafficAPI",
		"BackgroundWorker", 
		"HealthChecks",
		"ProductionErrors",
	}
	
	fmt.Printf("Testing with %d messages:\n", messageCount)
	fmt.Println("Profile            | Sampled | Skipped | Rate")
	fmt.Println("-------------------|---------|---------|--------")
	
	baseLogger := mtlog.New(mtlog.WithConsole())
	
	for _, profile := range profiles {
		logger := baseLogger.SampleProfile(profile)
		
		// Simulate logging
		for i := 0; i < messageCount; i++ {
			switch profile {
			case "ProductionErrors":
				// Use error level for ProductionErrors profile
				logger.Error("Test error {N}", i)
			default:
				logger.Info("Test message {N}", i)
			}
		}
		
		sampled, skipped := logger.GetSamplingStats()
		rate := float64(sampled) * 100 / float64(messageCount)
		fmt.Printf("%-18s | %7d | %7d | %6.2f%%\n", profile, sampled, skipped, rate)
	}
	
	fmt.Println("\nNote: Actual rates may vary due to randomization and time-based factors")
}

// Helper function to simulate different log levels
func simulateTraffic(logger core.Logger, profile string) {
	switch profile {
	case "HighTrafficAPI":
		// Simulate API endpoint logs
		for i := 0; i < 100; i++ {
			logger.Info("GET /api/users {RequestID}", i)
			if i%10 == 0 {
				logger.Warning("Slow response {RequestID}", i)
			}
		}
	case "BackgroundWorker":
		// Simulate background job logs
		for i := 0; i < 50; i++ {
			logger.Info("Processing job {JobID}", i)
			logger.Debug("Job details {JobID}", i)
		}
	case "ProductionErrors":
		// Simulate error scenarios
		for i := 0; i < 20; i++ {
			logger.Error("Database connection failed {Attempt}", i)
			time.Sleep(50 * time.Millisecond) // Simulate backoff
		}
	}
}

// customFilter is a simple counter-based filter for demonstration
type customFilter struct {
	n       uint64
	counter uint64
}

func (f *customFilter) IsEnabled(event *core.LogEvent) bool {
	f.counter++
	return f.counter%f.n == 0
}