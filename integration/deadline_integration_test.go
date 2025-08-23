// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/sinks"
)

// TestDeadlineAwareness_ConcurrentContexts tests deadline awareness with multiple concurrent contexts.
func TestDeadlineAwareness_ConcurrentContexts(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond),
	)

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine gets its own timeout
			timeout := time.Duration(100+id*10) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Log at different intervals
			for j := 0; j < 5; j++ {
				logger.InfoContext(ctx, "Goroutine {Id} iteration {Iteration}", id, j)
				time.Sleep(time.Duration(15+id) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	events := sink.Events()
	warningCount := 0
	for _, event := range events {
		if event.Level == core.WarningLevel {
			warningCount++
		}
		// Verify deadline properties are added when appropriate
		if approaching, ok := event.Properties["deadline.approaching"].(bool); ok && approaching {
			if _, hasRemaining := event.Properties["deadline.remaining_ms"]; !hasRemaining {
				t.Error("Missing deadline.remaining_ms when deadline.approaching is true")
			}
		}
	}

	if warningCount == 0 {
		t.Error("Expected some warnings for approaching deadlines")
	}

	t.Logf("Total events: %d, Warnings: %d", len(events), warningCount)
}

// TestDeadlineAwareness_SharedContext tests multiple goroutines sharing the same context.
func TestDeadlineAwareness_SharedContext(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(100*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	numGoroutines := 5

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Each goroutine logs at different times
			for j := 0; j < 10; j++ {
				logger.InfoContext(ctx, "Shared context: goroutine {Id} log {Log}", id, j)
				time.Sleep(25 * time.Millisecond)
				
				// Check if we're past the deadline
				if time.Since(start) > 300*time.Millisecond {
					return
				}
			}
		}(i)
	}

	wg.Wait()

	events := sink.Events()
	
	// Verify first warning is marked
	firstWarningFound := false
	for _, event := range events {
		if fw, ok := event.Properties["deadline.first_warning"].(bool); ok && fw {
			if firstWarningFound {
				t.Error("Multiple events marked as first warning")
			}
			firstWarningFound = true
		}
	}

	if !firstWarningFound {
		t.Error("No event marked as first warning")
	}

	// Check for deadline exceeded
	exceededFound := false
	for _, event := range events {
		if exceeded, ok := event.Properties["deadline.exceeded"].(bool); ok && exceeded {
			exceededFound = true
			if _, hasExceededBy := event.Properties["deadline.exceeded_by_ms"]; !hasExceededBy {
				t.Error("Missing deadline.exceeded_by_ms when deadline.exceeded is true")
			}
		}
	}

	t.Logf("Total events: %d, First warning found: %v, Exceeded found: %v", 
		len(events), firstWarningFound, exceededFound)
}

// TestDeadlineAwareness_HTTPHandler tests deadline awareness in HTTP handlers.
func TestDeadlineAwareness_HTTPHandler(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond),
		mtlog.WithProperty("service", "test-api"),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add timeout to request context
		ctx, cancel := context.WithTimeout(r.Context(), 150*time.Millisecond)
		defer cancel()

		// Simulate request processing
		logger.InfoContext(ctx, "Request started {Method} {Path}", r.Method, r.URL.Path)
		
		// Simulate database query
		time.Sleep(60 * time.Millisecond)
		logger.InfoContext(ctx, "Database query completed")
		
		// Simulate processing
		time.Sleep(50 * time.Millisecond)
		logger.InfoContext(ctx, "Processing completed")
		
		// Simulate response preparation (should trigger warning)
		time.Sleep(30 * time.Millisecond)
		logger.InfoContext(ctx, "Response prepared")
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make requests
	for i := 0; i < 3; i++ {
		resp, err := http.Get(server.URL + fmt.Sprintf("/api/endpoint%d", i))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}

	events := sink.Events()
	
	// Count warnings
	warningCount := 0
	for _, event := range events {
		if event.Level == core.WarningLevel {
			warningCount++
			// Log the warning details
			if remaining, ok := event.Properties["deadline.remaining_ms"]; ok {
				t.Logf("Warning with %v ms remaining", remaining)
			}
		}
	}

	t.Logf("Total events: %d, Warnings: %d", len(events), warningCount)

	if warningCount == 0 {
		t.Error("Expected warnings for HTTP requests approaching deadline")
	}
}

// TestDeadlineAwareness_ContextCancellation tests behavior with context cancellation vs deadline.
func TestDeadlineAwareness_ContextCancellation(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond),
	)

	// Test 1: Context cancelled before deadline
	ctx1, cancel1 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	
	logger.InfoContext(ctx1, "Before cancellation")
	cancel1() // Cancel immediately
	logger.InfoContext(ctx1, "After cancellation")
	
	// Test 2: Context deadline exceeded
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	
	logger.InfoContext(ctx2, "Before deadline")
	time.Sleep(100 * time.Millisecond)
	logger.InfoContext(ctx2, "After deadline")
	
	events := sink.Events()
	
	// Verify proper handling of cancelled vs exceeded
	cancelledFound := false
	exceededFound := false
	
	for i, event := range events {
		// Use MessageTemplate to identify which log this is
		if event.MessageTemplate == "After cancellation" {
			cancelledFound = true
			// Should not have deadline exceeded properties for cancelled context
			if _, hasExceeded := event.Properties["deadline.exceeded"]; hasExceeded {
				t.Error("Cancelled context should not have deadline.exceeded property")
			}
		}
		if event.MessageTemplate == "After deadline" {
			exceededFound = true
			// Should have deadline exceeded properties
			if _, hasExceeded := event.Properties["deadline.exceeded"]; !hasExceeded {
				t.Errorf("Event %d: Exceeded deadline should have deadline.exceeded property. Properties: %v", i, event.Properties)
			}
		}
	}
	
	if !cancelledFound {
		t.Error("Cancelled context log not found")
	}
	if !exceededFound {
		t.Error("Exceeded deadline log not found")
	}
}

// TestDeadlineAwareness_MemoryAndCacheCleanup tests cache cleanup and memory usage.
func TestDeadlineAwareness_MemoryAndCacheCleanup(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond,
			enrichers.WithDeadlineCacheSize(3),         // Very small cache to force evictions
			enrichers.WithDeadlineCacheTTL(100*time.Millisecond), // TTL for expiration test
		),
	)

	// Test Phase 1: Cache eviction due to size limit
	t.Log("Phase 1: Testing cache eviction")
	
	// Create contexts with staggered deadlines so we can test them at different times
	contexts1 := make([]context.Context, 6)
	cancels1 := make([]context.CancelFunc, 6)
	
	for i := 0; i < 6; i++ {
		// Each context has a 200ms deadline
		contexts1[i], cancels1[i] = context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancels1[i]()
	}

	// Log initial entries for all 6 contexts to prime the cache
	for i := 0; i < 6; i++ {
		logger.InfoContext(contexts1[i], "Context {Id} initial", i)
	}
	
	// Wait to approach deadline (need to be within 50ms)
	time.Sleep(155 * time.Millisecond)
	
	// Log warnings for all 6 contexts
	// With cache size 3, contexts 0-2 will be evicted when 3-5 are logged
	for i := 0; i < 6; i++ {
		logger.InfoContext(contexts1[i], "Context {Id} approaching", i)
	}
	
	// Now immediately log contexts 0-2 again
	// They were evicted from cache but should NOT generate new first warnings
	// because they're still the same context/operation
	for i := 0; i < 3; i++ {
		logger.InfoContext(contexts1[i], "Context {Id} after eviction", i)
	}
	
	// Test Phase 2: TTL expiration
	t.Log("Phase 2: Testing TTL expiration")
	
	// Create new contexts for TTL test
	contexts2 := make([]context.Context, 3)
	cancels2 := make([]context.CancelFunc, 3)
	
	for i := 0; i < 3; i++ {
		contexts2[i], cancels2[i] = context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancels2[i]()
	}
	
	// Log and trigger warnings
	for i := 0; i < 3; i++ {
		logger.InfoContext(contexts2[i], "TTL context {Id} initial", i)
	}
	
	time.Sleep(255 * time.Millisecond)  // Get within 45ms of deadline
	
	for i := 0; i < 3; i++ {
		logger.InfoContext(contexts2[i], "TTL context {Id} warning", i)
	}
	
	// Wait for TTL to expire (total 100ms since warnings)
	time.Sleep(105 * time.Millisecond)
	
	// Create new contexts and log - cache should be expired
	contexts3 := make([]context.Context, 2)
	cancels3 := make([]context.CancelFunc, 2)
	
	for i := 0; i < 2; i++ {
		contexts3[i], cancels3[i] = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancels3[i]()
	}
	
	// These should get first warnings as cache is expired
	for i := 0; i < 2; i++ {
		logger.InfoContext(contexts3[i], "After TTL context {Id} initial", i)
	}
	
	time.Sleep(55 * time.Millisecond)  // Get within 45ms of deadline (below 50ms threshold)
	
	for i := 0; i < 2; i++ {
		logger.InfoContext(contexts3[i], "After TTL context {Id} warning", i)
	}

	events := sink.Events()
	
	// Count first warnings
	firstWarningCount := 0
	warningCount := 0
	for _, event := range events {
		if event.Level == core.WarningLevel {
			warningCount++
		}
		if fw, ok := event.Properties["deadline.first_warning"].(bool); ok && fw {
			firstWarningCount++
		}
	}

	t.Logf("Total events: %d, Warnings: %d, First warnings: %d", len(events), warningCount, firstWarningCount)
	
	// Expected first warnings:
	// - Phase 1: 6 initial warnings (contexts1[0-5])
	// - Phase 1: 0 more after eviction (same contexts, no new first warnings)
	// - Phase 2: 3 warnings (contexts2[0-2])
	// - Phase 2: 2 warnings after TTL (contexts3[0-1])
	// Total expected: 11 first warnings
	//
	// "First warning" means FIRST - not "first since cache eviction"
	if firstWarningCount != 11 {
		t.Errorf("Expected 11 first warnings (6+3+2 unique contexts), got %d", firstWarningCount)
	}
}

// TestDeadlineAwareness_PercentageThreshold tests percentage-based threshold in real scenarios.
func TestDeadlineAwareness_PercentageThreshold(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithDeadlinePercentageThreshold(
			1*time.Millisecond, // Very small absolute threshold
			0.25,               // Warn at 25% remaining
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	// Prime the cache with the context at the start
	// This is crucial for percentage calculation to work correctly
	start := time.Now()
	logger.InfoContext(ctx, "Starting progress tracking")
	
	// Log at regular intervals
	for i := 0; i < 10; i++ {
		time.Sleep(40 * time.Millisecond)
		elapsed := time.Since(start)
		percentUsed := float64(elapsed) / float64(400*time.Millisecond) * 100
		logger.InfoContext(ctx, "Progress {Percent:F1}%", percentUsed)
		
	}

	events := sink.Events()
	
	// Find when warnings started (skip the first priming log)
	warningStartIndex := -1
	for i := 1; i < len(events); i++ {
		if events[i].Level == core.WarningLevel {
			warningStartIndex = i
			break
		}
	}

	if warningStartIndex == -1 {
		t.Fatal("No warnings found")
	}

	// Calculate percentage when warnings started
	warningEvent := events[warningStartIndex]
	if percent, ok := warningEvent.Properties["Percent"].(float64); ok {
		// Should start warning around 75-85% (when 25% remains)
		// Due to the 40ms sleep intervals, we can't hit exactly 75%
		if percent < 70 || percent > 85 {
			t.Errorf("Warnings started at unexpected percentage: %.1f%% (expected 75-85%%)", percent)
		}
		t.Logf("Warnings started at %.1f%% progress (25%% threshold means warnings when 75%% consumed)", percent)
	}
}

// TestDeadlineAwareness_Middleware tests integration with middleware patterns.
func TestDeadlineAwareness_Middleware(t *testing.T) {
	sink := sinks.NewMemorySink()
	baseLogger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithContextDeadlineWarning(30*time.Millisecond),
	)

	// Middleware that adds timeout to requests
	timeoutMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
			defer cancel()
			
			// Add request ID to context
			logger := baseLogger.ForContext("request_id", fmt.Sprintf("req-%d", time.Now().UnixNano()))
			logger.InfoContext(ctx, "Request started")
			
			// Pass modified context
			next.ServeHTTP(w, r.WithContext(ctx))
			
			logger.InfoContext(ctx, "Request completed")
		})
	}

	// Handler that simulates work
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := baseLogger.ForContext("handler", "main")
		
		// Simulate varying processing times
		for i := 0; i < 5; i++ {
			logger.InfoContext(r.Context(), "Processing step {Step}", i+1)
			time.Sleep(15 * time.Millisecond)
		}
		
		w.WriteHeader(http.StatusOK)
	})

	// Create handler chain
	finalHandler := timeoutMiddleware(handler)
	
	// Make test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	finalHandler.ServeHTTP(w, req)
	
	events := sink.Events()
	
	// Verify middleware and handler logs have consistent request_id
	requestIds := make(map[string]int)
	for _, event := range events {
		if id, ok := event.Properties["request_id"].(string); ok {
			requestIds[id]++
		}
	}
	
	if len(requestIds) != 1 {
		t.Errorf("Expected single request_id, got %d different IDs", len(requestIds))
	}
	
	// Count warnings
	warningCount := 0
	for _, event := range events {
		if event.Level == core.WarningLevel {
			warningCount++
		}
	}
	
	t.Logf("Total events: %d, Warnings: %d, Request IDs: %v", 
		len(events), warningCount, requestIds)
}