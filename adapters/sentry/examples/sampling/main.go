package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/sentry"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	fmt.Println("=== Sentry Sampling Examples ===")
	fmt.Println()

	// Create different Sentry sinks with various sampling strategies
	examples := []struct {
		name string
		sink func() (*sentry.SentrySink, error)
		demo func(logger core.Logger)
	}{
		{
			name: "1. Fixed Rate Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.InformationLevel),
					sentry.WithSampling(0.1), // 10% sampling rate
				)
			},
			demo: demoFixedSampling,
		},
		{
			name: "2. Adaptive Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.InformationLevel),
					sentry.WithAdaptiveSampling(100), // Target 100 events/sec
				)
			},
			demo: demoAdaptiveSampling,
		},
		{
			name: "3. Priority-Based Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.InformationLevel),
					sentry.WithPrioritySampling(0.05), // 5% base rate
				)
			},
			demo: demoPrioritySampling,
		},
		{
			name: "4. Burst Detection Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.InformationLevel),
					sentry.WithBurstSampling(100), // Burst threshold: 100 events/sec
				)
			},
			demo: demoBurstSampling,
		},
		{
			name: "5. Group-Based Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.ErrorLevel),
					sentry.WithGroupSampling(5, time.Minute), // 5 events per error group per minute
				)
			},
			demo: demoGroupSampling,
		},
		{
			name: "6. Profile-Based Sampling",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithSamplingProfile(sentry.SamplingProfileProduction),
				)
			},
			demo: demoProfileSampling,
		},
		{
			name: "7. mtlog Sampling Integration",
			sink: func() (*sentry.SentrySink, error) {
				// Sentry sink without its own sampling
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithMinLevel(core.InformationLevel),
				)
			},
			demo: demoMtlogSampling,
		},
		{
			name: "8. Custom Sampling Logic",
			sink: func() (*sentry.SentrySink, error) {
				return sentry.NewSentrySink("https://your-key@sentry.io/project",
					sentry.WithEnvironment("production"),
					sentry.WithCustomSampling(func(event *core.LogEvent) bool {
						// Custom logic: sample based on user ID
						if userId, ok := event.Properties["UserId"].(int); ok {
							// Sample 50% of events from premium users (ID < 1000)
							if userId < 1000 {
								return rand.Float32() < 0.5
							}
							// Sample 1% of events from regular users
							return rand.Float32() < 0.01
						}
						// Default: 10% sampling
						return rand.Float32() < 0.1
					}),
				)
			},
			demo: demoCustomSampling,
		},
	}

	// Run each example
	for _, example := range examples {
		fmt.Printf("\n%s\n", example.name)
		fmt.Println(strings.Repeat("-", 40))
		
		sink, err := example.sink()
		if err != nil {
			log.Printf("Failed to create sink: %v", err)
			continue
		}
		defer sink.Close()
		
		logger := mtlog.New(
			mtlog.WithConsole(),
			mtlog.WithSink(sink),
		)
		
		example.demo(logger)
		
		// Show metrics if available
		metrics := sink.Metrics()
		fmt.Printf("\nMetrics:\n")
		fmt.Printf("  Events Sent: %d\n", metrics.EventsSent)
		fmt.Printf("  Events Dropped: %d\n", metrics.EventsDropped)
		if metrics.EventsSent > 0 || metrics.EventsDropped > 0 {
			total := float64(metrics.EventsSent + metrics.EventsDropped)
			if total > 0 {
				samplingRate := float64(metrics.EventsSent) / total * 100
				fmt.Printf("  Effective Sampling: %.1f%%\n", samplingRate)
			}
		}
		
		time.Sleep(500 * time.Millisecond)
	}
}

func demoFixedSampling(logger core.Logger) {
	fmt.Println("Generating 100 events with 10% fixed sampling...")
	
	for i := 0; i < 100; i++ {
		logger.Information("Event {Number} with fixed sampling", i)
	}
}

func demoAdaptiveSampling(logger core.Logger) {
	fmt.Println("Generating variable load to demonstrate adaptive sampling...")
	
	// Low load
	fmt.Println("Low load (10 events/sec)...")
	for i := 0; i < 10; i++ {
		logger.Information("Low load event {Number}", i)
		time.Sleep(100 * time.Millisecond)
	}
	
	// High load burst
	fmt.Println("High load burst (1000 events)...")
	for i := 0; i < 1000; i++ {
		logger.Information("High load event {Number}", i)
	}
	
	// Return to low load
	fmt.Println("Return to low load...")
	for i := 0; i < 10; i++ {
		logger.Information("Normal load event {Number}", i)
		time.Sleep(100 * time.Millisecond)
	}
}

func demoPrioritySampling(logger core.Logger) {
	fmt.Println("Generating events with different priorities...")
	
	// Low priority events (5% sampling)
	for i := 0; i < 50; i++ {
		logger.Information("Regular event {Number}", i)
	}
	
	// High priority events with errors (increased sampling)
	for i := 0; i < 20; i++ {
		err := errors.New("database connection failed")
		logger.Error("Critical error {Number}: {Error}", i, err)
	}
	
	// Events with user context (medium priority)
	for i := 0; i < 30; i++ {
		logger.ForContext("UserId", 12345).
			Warning("User action {Number} failed", i)
	}
}

func demoBurstSampling(logger core.Logger) {
	fmt.Println("Simulating traffic burst scenario...")
	
	// Normal traffic
	fmt.Println("Normal traffic (10 events)...")
	for i := 0; i < 10; i++ {
		logger.Information("Normal traffic {Number}", i)
		time.Sleep(50 * time.Millisecond)
	}
	
	// Sudden burst
	fmt.Println("Traffic burst (500 events rapidly)...")
	for i := 0; i < 500; i++ {
		logger.Warning("Burst event {Number}", i)
	}
	
	// Recovery period
	fmt.Println("Recovery period...")
	time.Sleep(2 * time.Second)
	
	// Return to normal
	fmt.Println("Post-burst normal traffic...")
	for i := 0; i < 10; i++ {
		logger.Information("Recovery event {Number}", i)
		time.Sleep(50 * time.Millisecond)
	}
}

func demoGroupSampling(logger core.Logger) {
	fmt.Println("Generating repetitive errors to test group sampling...")
	
	errorTypes := []struct {
		template string
		err      error
	}{
		{"Database connection failed", errors.New("connection timeout")},
		{"API rate limit exceeded", errors.New("429 too many requests")},
		{"Invalid user input", errors.New("validation failed")},
	}
	
	// Generate 20 instances of each error type
	for _, errorType := range errorTypes {
		fmt.Printf("Generating 20 instances of: %s\n", errorType.template)
		for i := 0; i < 20; i++ {
			logger.Error(errorType.template+": {Error}", errorType.err)
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	fmt.Println("With group sampling, only 5 per error type should be sent per minute")
}

func demoProfileSampling(logger core.Logger) {
	fmt.Println("Using production profile (adaptive + group sampling)...")
	
	// Simulate production workload
	for i := 0; i < 100; i++ {
		switch rand.Intn(10) {
		case 0:
			logger.Fatal("Critical system failure {Number}", i)
		case 1, 2:
			logger.Error("Application error {Number}", i)
		case 3, 4, 5:
			logger.Warning("Warning condition {Number}", i)
		default:
			logger.Information("Normal operation {Number}", i)
		}
		
		if i%20 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func demoMtlogSampling(logger core.Logger) {
	fmt.Println("Using mtlog's built-in sampling methods...")
	
	// Sample every 10th message
	fmt.Println("\nEvery 10th message:")
	for i := 0; i < 30; i++ {
		logger.Sample(10).Information("Sampled event {Number}", i)
	}
	
	// Sample once per second
	fmt.Println("\nOnce per second:")
	for i := 0; i < 10; i++ {
		logger.SampleDuration(time.Second).Warning("Rate limited warning {Number}", i)
		time.Sleep(200 * time.Millisecond)
	}
	
	// Sample 20% of messages
	fmt.Println("\n20% sampling rate:")
	for i := 0; i < 50; i++ {
		logger.SampleRate(0.2).Information("Percentage sampled {Number}", i)
	}
	
	// Sample first 5 occurrences only
	fmt.Println("\nFirst 5 only:")
	for i := 0; i < 20; i++ {
		logger.SampleFirst(5).Error("Limited error {Number}", i)
	}
	
	// Sample with exponential backoff
	fmt.Println("\nExponential backoff:")
	for i := 0; i < 20; i++ {
		logger.SampleBackoff("api-error", 2.0).Error("API error {Attempt}", i)
		time.Sleep(100 * time.Millisecond)
	}
	
	// Conditional sampling
	fmt.Println("\nConditional sampling (only even numbers):")
	for i := 0; i < 20; i++ {
		logger.SampleWhen(func() bool { return i%2 == 0 }, 1).
			Information("Conditional event {Number}", i)
	}
}

func demoCustomSampling(logger core.Logger) {
	fmt.Println("Using custom sampling based on user type...")
	
	users := []struct {
		id   int
		name string
		tier string
	}{
		{100, "Alice", "premium"},
		{200, "Bob", "premium"},
		{1001, "Charlie", "regular"},
		{1002, "Diana", "regular"},
		{1003, "Eve", "regular"},
	}
	
	// Generate events for different users
	for _, user := range users {
		fmt.Printf("\nGenerating 20 events for %s (tier: %s, id: %d)\n", 
			user.name, user.tier, user.id)
		for i := 0; i < 20; i++ {
			logger.ForContext("UserId", user.id).
				ForContext("UserName", user.name).
				Information("User action {Action} performed", 
					fmt.Sprintf("action-%d", i))
		}
	}
	
	fmt.Println("\nPremium users (ID < 1000) get 50% sampling")
	fmt.Println("Regular users get 1% sampling")
}

