package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// ExampleSamplingConfigBuilder demonstrates the new fluent sampling configuration API
func ExampleSamplingConfigBuilder() {
	fmt.Println("=== Sampling Configuration Builder Examples ===")
	fmt.Println()

	// Example 1: Pipeline-style sampling (Build)
	example1PipelineSampling()

	// Example 2: Composite AND sampling
	example2CompositeANDSampling()

	// Example 3: Composite OR sampling  
	example3CompositeORSampling()

	// Example 4: Custom SamplingPolicy
	example4CustomSamplingPolicy()

	// Example 5: Complex scenarios
	example5ComplexScenarios()
}

// Example 1: Pipeline-style sampling (filters in sequence)
func example1PipelineSampling() {
	fmt.Println("Example 1: Pipeline-Style Sampling (Build)")
	fmt.Println("-------------------------------------------")

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.Sampling().
			Every(3).     // First: sample every 3rd message
			Rate(0.8).    // Then: 80% of those that pass
			First(5).     // Finally: only first 5 that make it through
			Build(),      // Apply as sequential pipeline
	)

	fmt.Println("Logging 20 messages through pipeline:")
	for i := 1; i <= 20; i++ {
		logger.Info("Pipeline message {Number}", i)
	}

	fmt.Println("↳ Pipeline flow: Every(3) → Rate(0.8) → First(5)")
	fmt.Println("↳ Each filter sees only what the previous filter passed")
	fmt.Println()
}

// Example 2: Composite AND sampling (all conditions must match)
func example2CompositeANDSampling() {
	fmt.Println("Example 2: Composite AND Sampling")
	fmt.Println("----------------------------------")

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.Sampling().
			Every(2).       // Condition 1: must be every 2nd message
			First(10).      // Condition 2: must be within first 10 evaluations
			CombineAND(),   // Both conditions must be true
	)

	fmt.Println("Logging 20 messages with AND logic:")
	for i := 1; i <= 20; i++ {
		logger.Info("AND message {Number}", i)
	}

	fmt.Println("↳ AND logic: Both Every(2) AND First(10) must approve each message")
	fmt.Println("↳ First(10) only counts calls it receives (from odd numbers)")
	fmt.Println("↳ Result: All odd numbers (1,3,5,7,9,11,13,15,17,19)")
	fmt.Println()
}

// Example 3: Composite OR sampling (any condition can match)
func example3CompositeORSampling() {
	fmt.Println("Example 3: Composite OR Sampling")
	fmt.Println("---------------------------------")

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.Sampling().
			Every(5).      // Condition 1: every 5th message
			First(3).      // Condition 2: first 3 messages
			CombineOR(),   // Either condition can be true
	)

	fmt.Println("Logging 20 messages with OR logic:")
	for i := 1; i <= 20; i++ {
		logger.Info("OR message {Number}", i)
	}

	fmt.Println("↳ OR logic: Either Every(5) OR First(3) can approve")
	fmt.Println("↳ Result: Messages 1,2,3 (first 3) + 5,10,15,20 (every 5th)")
	fmt.Println()
}

// Example 4: Custom SamplingPolicy
func example4CustomSamplingPolicy() {
	fmt.Println("Example 4: Custom SamplingPolicy")
	fmt.Println("---------------------------------")

	// Create a custom policy that samples based on user ID
	userPolicy := &UserBasedSamplingPolicy{
		sampleRates: map[string]float32{
			"admin":     1.0,   // Always log admin users
			"premium":   0.5,   // 50% for premium users  
			"basic":     0.1,   // 10% for basic users
		},
		defaultRate: 0.01, // 1% for unknown users
	}

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSamplingPolicy(userPolicy),
	)

	// Simulate different user types
	users := []struct {
		id   string
		tier string
	}{
		{"user1", "admin"}, {"user2", "premium"}, {"user3", "basic"},
		{"user4", "basic"}, {"user5", "premium"}, {"user6", "unknown"},
	}

	fmt.Println("Logging with user-based sampling:")
	for i, user := range users {
		// Add user tier as property for the policy to use
		event := logger.ForContext("UserTier", user.tier)
		event.Info("User {UserId} performed action {ActionId}", user.id, i+1)
	}

	stats := userPolicy.Stats()
	fmt.Printf("↳ Policy stats: %d sampled, %d skipped\n", stats.Sampled, stats.Skipped)
	fmt.Println()
}

// Example 5: Complex real-world scenarios
func example5ComplexScenarios() {
	fmt.Println("Example 5: Complex Real-World Scenarios")
	fmt.Println("----------------------------------------")

	// Scenario A: Error logging with escalation
	// Log all errors initially, then back off exponentially
	errorLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithProperty("Component", "ErrorHandler"),
		mtlog.Sampling().
			First(10).                    // Always log first 10 errors
			Backoff("system-error", 2.0). // Then exponential backoff
			CombineOR(),                  // Either condition allows logging
	).EnableSamplingSummary(5 * time.Second) // Summary every 5s

	// Scenario B: High-frequency debug with burst allowance  
	// Allow first 50 debug messages, then 1% sampling
	debugLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithProperty("Component", "DebugTracer"),
		mtlog.Sampling().
			First(50).      // Burst allowance
			Rate(0.01).     // Then heavy sampling
			CombineOR(),    // Either allows through
	)

	// Scenario C: Performance monitoring
	// Sample every 100th perf metric OR first 20
	perfLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithProperty("Component", "PerfMonitor"),
		mtlog.Sampling().
			Every(100).     // Normal sampling
			First(20).      // Initial burst
			CombineOR(),    // Either condition allows logging
	)

	fmt.Println("Simulating complex logging scenarios...")

	// Simulate error burst
	fmt.Println("Error burst:")
	for i := 1; i <= 20; i++ {
		errorLogger.Error("System error occurred {ErrorId}", i)
	}

	// Simulate debug flood  
	fmt.Println("\nDebug flood:")
	for i := 1; i <= 100; i++ {
		debugLogger.Debug("Debug trace point {TraceId}", i)
	}

	// Simulate performance metrics
	fmt.Println("\nPerformance metrics:")
	for i := 1; i <= 200; i++ {
		perfLogger.Info("Performance metric {MetricId} = {Value}ms", i, 150+i%50)
	}

	fmt.Println("\n↳ Each scenario uses different sampling strategies")
	fmt.Println("↳ Complex policies can be built with simple building blocks")
	fmt.Println()
}

// UserBasedSamplingPolicy implements a custom sampling strategy based on user tiers
type UserBasedSamplingPolicy struct {
	sampleRates map[string]float32
	defaultRate float32
	sampled     uint64
	skipped     uint64
}

func (p *UserBasedSamplingPolicy) ShouldSample(event *core.LogEvent) bool {
	// Get user tier from event properties
	var tier string
	if userTier, exists := event.Properties["UserTier"]; exists {
		if tierStr, ok := userTier.(string); ok {
			tier = tierStr
		}
	}

	// Get sampling rate for this tier
	rate, exists := p.sampleRates[tier]
	if !exists {
		rate = p.defaultRate
	}

	// Always sample admin users
	if rate >= 1.0 {
		p.sampled++
		return true
	}

	// Skip if rate is zero
	if rate <= 0.0 {
		p.skipped++
		return false
	}

	// Pseudo-random sampling based on event hash
	// In real implementation, you might use proper random sampling
	hash := p.hashEvent(event)
	threshold := uint32(float32(^uint32(0)) * rate)

	if hash <= threshold {
		p.sampled++
		return true
	}

	p.skipped++
	return false
}

func (p *UserBasedSamplingPolicy) Reset() {
	p.sampled = 0
	p.skipped = 0
}

func (p *UserBasedSamplingPolicy) Stats() core.SamplingStats {
	return core.SamplingStats{
		Sampled: p.sampled,
		Skipped: p.skipped,
	}
}

// Simple hash function for demonstration
func (p *UserBasedSamplingPolicy) hashEvent(event *core.LogEvent) uint32 {
	hash := uint32(2166136261) // FNV-1a offset basis
	for key, value := range event.Properties {
		for _, b := range []byte(key + fmt.Sprintf("%v", value)) {
			hash ^= uint32(b)
			hash *= 16777619 // FNV-1a prime
		}
	}
	return hash
}