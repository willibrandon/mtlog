package main

import (
	"fmt"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/filters"
)

func main() {
	// Example 1: Level filtering (already built into pipeline)
	fmt.Println("=== Example 1: Level Filtering ===")
	log1 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)
	
	log1.Debug("This debug message won't be shown")
	log1.Information("This info message will be shown")
	log1.Warning("This warning message will be shown")
	
	// Example 2: Property-based filtering
	fmt.Println("\n=== Example 2: Property Filtering ===")
	log2 := mtlog.New(
		mtlog.WithConsole(),
		// Exclude health check endpoints
		mtlog.WithExcludeFilter(func(event *core.LogEvent) bool {
			path, ok := event.Properties["Path"].(string)
			return ok && path == "/health"
		}),
	)
	
	log2.Information("Request to {Path}", "/api/users")
	log2.Information("Request to {Path}", "/health") // This will be filtered out
	log2.Information("Request to {Path}", "/api/orders")
	
	// Example 3: Sampling for high-volume logs
	fmt.Println("\n=== Example 3: Sampling (10% of events) ===")
	log3 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSampling(0.1), // Only 10% of events
	)
	
	for i := 0; i < 20; i++ {
		log3.Information("High-volume event {Number}", i)
	}
	
	// Example 4: Hash-based sampling (consistent per user)
	fmt.Println("\n=== Example 4: Hash-based Sampling ===")
	log4 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithHashSampling("UserId", 0.5), // 50% of users
	)
	
	// Same user will always get same sampling decision
	for i := 0; i < 5; i++ {
		log4.Information("User {UserId} action {Action}", "user123", i)
	}
	
	// Different users might get different decisions
	for i := 0; i < 5; i++ {
		log4.Information("User {UserId} action {Action}", fmt.Sprintf("user%d", i), "login")
	}
	
	// Example 5: Rate limiting
	fmt.Println("\n=== Example 5: Rate Limiting ===")
	log5 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithRateLimit(5, int64(time.Second)), // Max 5 events per second
	)
	
	// Burst of events - only first 5 should pass
	for i := 0; i < 10; i++ {
		log5.Warning("Rapid event {Number}", i)
	}
	
	// Wait and try again
	time.Sleep(1100 * time.Millisecond)
	log5.Information("This should pass after waiting")
	
	// Example 6: Complex filtering with multiple conditions
	fmt.Println("\n=== Example 6: Complex Filtering ===")
	log6 := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithProperty("Environment", "Production"),
		// Only log errors in production, or any level with Priority=High
		mtlog.WithFilter(filters.NewOrFilter(
			filters.NewCompositeFilter(
				filters.MatchProperty("Environment", "Production"),
				filters.NewLevelFilter(core.ErrorLevel),
			),
			filters.MatchProperty("Priority", "High"),
		)),
	)
	
	log6.Information("Regular info message") // Filtered out
	log6.Error("Production error") // Passes (Production + Error level)
	log6.Information("High priority info", ) // Need to add priority
	log6.ForContext("Priority", "High").Information("High priority info") // Passes (Priority=High)
	
	// Example 7: Filtering sensitive data
	fmt.Println("\n=== Example 7: Security Filtering ===")
	log7 := mtlog.New(
		mtlog.WithConsoleProperties(),
		// Don't log events that contain passwords
		mtlog.WithFilter(filters.ByExcluding(func(event *core.LogEvent) bool {
			for key := range event.Properties {
				if key == "Password" || key == "Token" || key == "Secret" {
					return true
				}
			}
			return false
		})),
	)
	
	log7.Information("User {UserId} logged in", "user123") // OK
	log7.Information("Auth with {Password}", "secret123") // Filtered out
	log7.Information("Using token {Token}", "abc123") // Filtered out
	
	// Example 8: Development vs Production filtering
	fmt.Println("\n=== Example 8: Environment-based Filtering ===")
	isDevelopment := false // Change to true to see verbose logs
	
	log8 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithFilter(filters.When(
			!isDevelopment,
			filters.NewLevelFilter(core.InformationLevel), // Production: Info and above
		)),
	)
	
	log8.Verbose("Verbose development details")
	log8.Debug("Debug information")
	log8.Information("Normal operation info")
	log8.Error("Error occurred")
}