// Package main demonstrates mtlog's context deadline awareness feature.
// This feature automatically detects and warns when operations approach
// context deadlines, helping catch timeout-related issues before they fail.
package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
)

func main() {
	fmt.Println("=== mtlog Context Deadline Awareness Demo ===")
	fmt.Println()

	// Example 1: Basic deadline awareness
	basicDeadlineExample()

	// Example 2: Percentage-based thresholds
	percentageThresholdExample()

	// Example 3: Custom handler for deadline events
	customHandlerExample()

	// Example 4: HTTP request handler with timeout
	httpHandlerExample()

	// Example 5: Deadline exceeded tracking
	deadlineExceededExample()
	
	// Example 6: Deadline statistics monitoring
	deadlineStatsExample()
	
	// Example 7: Common context patterns
	contextPatternsExample()
}

// basicDeadlineExample demonstrates basic deadline detection and warning.
func basicDeadlineExample() {
	fmt.Println("1. Basic Deadline Awareness")
	fmt.Println("----------------------------")

	// Create logger with 100ms deadline warning threshold
	// Using custom output template to show deadline properties
	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithContextDeadlineWarning(100*time.Millisecond),
	)

	// Create context with 500ms timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Log normally when deadline is far
	logger.InfoContext(ctx, "Starting operation")

	// Simulate work
	time.Sleep(350 * time.Millisecond)

	// This will trigger a warning as we're within 100ms of deadline
	logger.InfoContext(ctx, "Still processing...")

	// More work
	time.Sleep(100 * time.Millisecond)

	// Now approaching deadline even closer
	logger.InfoContext(ctx, "Almost done...")

	fmt.Println()
}

// percentageThresholdExample demonstrates percentage-based deadline warnings.
func percentageThresholdExample() {
	fmt.Println("2. Percentage-Based Threshold")
	fmt.Println("------------------------------")

	// Warn when 20% of time remains
	// Using the clean percentage-only API
	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithDeadlinePercentageOnly(0.2), // Warn at 20% remaining
	)

	// Create context with 1 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Prime the cache - log once at start to establish context start time
	logger.InfoContext(ctx, "Starting batch processing")

	// Simulate progress through the operation
	for i := 0; i < 10; i++ {
		progress := float64(i) / 10.0 * 100
		logger.InfoContext(ctx, "Processing batch {BatchNumber} ({Progress:F1}%)", i+1, progress)
		time.Sleep(90 * time.Millisecond)

		// After 80% of time (800ms), warnings will start
		if i >= 7 {
			logger.InfoContext(ctx, "Getting close to deadline!")
		}
	}

	fmt.Println()
}

// customHandlerExample demonstrates custom handling of deadline events.
func customHandlerExample() {
	fmt.Println("3. Custom Deadline Handler")
	fmt.Println("--------------------------")

	var deadlineWarnings int

	// Create logger with custom handler
	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithContextDeadlineWarning(100*time.Millisecond,
			enrichers.WithDeadlineCustomHandler(func(event *core.LogEvent, remaining time.Duration) {
				deadlineWarnings++
				// Add custom properties
				event.Properties["deadline_warning_count"] = deadlineWarnings
				event.Properties["action_required"] = "Consider cancelling non-critical operations"
				
				// Could also send metrics, alerts, etc.
				fmt.Printf("  [CUSTOM HANDLER] Deadline approaching: %v remaining (warning #%d)\n", 
					remaining, deadlineWarnings)
			}),
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Simulate operations that trigger multiple warnings
	for i := 0; i < 5; i++ {
		logger.InfoContext(ctx, "Operation {Step} in progress", i+1)
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("  Total deadline warnings: %d\n\n", deadlineWarnings)
}

// httpHandlerExample demonstrates deadline awareness in HTTP handlers.
func httpHandlerExample() {
	fmt.Println("4. HTTP Handler with Timeout")
	fmt.Println("-----------------------------")

	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond),
		mtlog.WithProperty("service", "api"),
	)

	// Simulate an HTTP handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Create context with timeout (normally from request context)
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()

		// Log request start
		logger.InfoContext(ctx, "Processing request {Method} {Path}", r.Method, r.URL.Path)

		// Simulate database query
		logger.InfoContext(ctx, "Querying database...")
		time.Sleep(100 * time.Millisecond)

		// Simulate processing
		logger.InfoContext(ctx, "Processing results...")
		time.Sleep(60 * time.Millisecond)

		// This should trigger warning as we're close to deadline
		logger.InfoContext(ctx, "Preparing response...")
		time.Sleep(30 * time.Millisecond)

		// Write response (if we have a writer)
		if w != nil {
			w.WriteHeader(http.StatusOK)
		}
		logger.InfoContext(ctx, "Request completed successfully")
	}

	// Simulate request
	req, _ := http.NewRequest("GET", "/api/users", nil)
	handler(nil, req)

	fmt.Println()
}

// deadlineExceededExample demonstrates handling of exceeded deadlines.
func deadlineExceededExample() {
	fmt.Println("5. Deadline Exceeded Tracking")
	fmt.Println("------------------------------")

	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond,
			enrichers.WithDeadlineMetrics(true), // Enable metrics
		),
	)

	// Create a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Log before deadline
	logger.InfoContext(ctx, "Starting time-sensitive operation")

	// Sleep past the deadline
	time.Sleep(150 * time.Millisecond)

	// These will be marked as deadline exceeded
	logger.ErrorContext(ctx, "Operation failed - took too long")
	logger.WarnContext(ctx, "Attempting cleanup after timeout")

	// Even Info level will be upgraded to Warning when deadline exceeded
	logger.InfoContext(ctx, "This info becomes a warning due to exceeded deadline")

	fmt.Println()
}

// deadlineStatsExample demonstrates monitoring deadline statistics.
func deadlineStatsExample() {
	fmt.Println("6. Deadline Statistics Monitoring")
	fmt.Println("---------------------------------")

	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message}"),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond,
			enrichers.WithDeadlineCacheSize(10),              // Small cache for demo
			enrichers.WithDeadlineCacheTTL(5*time.Second),   // Short TTL for demo
		),
	)

	// Create and log with multiple contexts
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		logger.InfoContext(ctx, "Context {Id} created", i)
		time.Sleep(60 * time.Millisecond)
		logger.InfoContext(ctx, "Context {Id} approaching deadline", i)
		cancel()
	}

	// Get and display statistics
	if statsInterface := logger.DeadlineStats(); statsInterface != nil {
		stats := statsInterface.(core.DeadlineStats)
		fmt.Printf("\nDeadline Statistics:\n")
		fmt.Printf("  Cache Size: %d/%d\n", stats.CacheSize, stats.CacheCapacity)
		fmt.Printf("  First Warnings Tracked: %d/%d\n", stats.FirstWarningCount, stats.FirstWarningCapacity)
		fmt.Printf("  Cache TTL: %v\n", stats.CacheTTL)
	} else {
		fmt.Println("Deadline awareness not configured")
	}

	fmt.Println()
}

// contextPatternsExample demonstrates common context deadline patterns.
func contextPatternsExample() {
	fmt.Println("7. Common Context Deadline Patterns")
	fmt.Println("-----------------------------------")

	logger := mtlog.New(
		mtlog.WithConsoleTemplate("[${Timestamp:15:04:05.000}] ${Level} ${Message} ${Properties}"),
		mtlog.WithContextDeadlineWarning(30*time.Millisecond),
	)

	// Pattern 1: Nested deadlines
	fmt.Println("\nPattern 1: Nested context deadlines")
	outerCtx, outerCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer outerCancel()

	logger.InfoContext(outerCtx, "Starting outer operation")

	// Inner operation with shorter deadline
	innerCtx, innerCancel := context.WithTimeout(outerCtx, 50*time.Millisecond)
	logger.InfoContext(innerCtx, "Starting inner operation")
	time.Sleep(30 * time.Millisecond)
	logger.InfoContext(innerCtx, "Inner operation checkpoint") // Should warn
	innerCancel()

	// Pattern 2: Deadline propagation in goroutines
	fmt.Println("\nPattern 2: Deadline propagation in goroutines")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.InfoContext(ctx, "Goroutine {Id} started", id)
			time.Sleep(80 * time.Millisecond)
			logger.InfoContext(ctx, "Goroutine {Id} near deadline", id) // All warn together
		}(i)
	}
	wg.Wait()

	// Pattern 3: Conditional deadline based on operation type
	fmt.Println("\nPattern 3: Conditional deadlines by operation type")
	processOperation := func(opType string, isSlowOperation bool) {
		var timeout time.Duration
		if isSlowOperation {
			timeout = 500*time.Millisecond
		} else {
			timeout = 100*time.Millisecond
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		logger.InfoContext(ctx, "Processing {OpType} operation", opType)
		time.Sleep(80 * time.Millisecond)
		logger.InfoContext(ctx, "{OpType} checkpoint", opType) // Fast ops warn, slow don't
	}

	processOperation("fast", false)
	processOperation("slow", true)

	// Pattern 4: Graceful degradation near deadline
	fmt.Println("\nPattern 4: Graceful degradation near deadline")
	degradeCtx, degradeCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer degradeCancel()

	// Full processing
	logger.InfoContext(degradeCtx, "Starting full processing")
	time.Sleep(50 * time.Millisecond)

	// Check if we have time for expensive operation
	if deadline, ok := degradeCtx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 80*time.Millisecond {
			logger.WarnContext(degradeCtx, "Skipping expensive operation, time remaining: {Remaining}", remaining)
			// Do simplified processing instead
			logger.InfoContext(degradeCtx, "Using cached results")
		} else {
			logger.InfoContext(degradeCtx, "Performing full computation")
		}
	}

	fmt.Println()
}