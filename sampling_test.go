package mtlog

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/filters"
	"github.com/willibrandon/mtlog/sinks"
)

// TestSampleEveryNth tests the Sample method for every nth message sampling
func TestSampleEveryNth(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample every 3rd message
	sampledLogger := logger.Sample(3)

	// Log 10 messages
	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Message {Number}", i)
	}

	// Should have logged messages 1, 4, 7, 10 (every 3rd starting from 1)
	events := sink.Events()
	expectedCount := 4
	if len(events) != expectedCount {
		t.Errorf("Expected %d events, got %d", expectedCount, len(events))
	}

	// Verify the logged message numbers
	expectedNumbers := []int{1, 4, 7, 10}
	for i, event := range events {
		if num, ok := event.Properties["Number"].(int); ok {
			if num != expectedNumbers[i] {
				t.Errorf("Expected message %d, got %d", expectedNumbers[i], num)
			}
		}
	}
}

// TestSampleRate tests the SampleRate method for percentage-based sampling
func TestSampleRate(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample 20% of messages (1 in 5)
	sampledLogger := logger.SampleRate(0.2)

	// Log 100 messages
	for i := 1; i <= 100; i++ {
		sampledLogger.Info("Message {Number}", i)
	}

	events := sink.Events()
	// With 20% sampling of 100 messages, we expect around 20 messages
	// Allow wider variance due to true random sampling
	if len(events) < 10 || len(events) > 30 {
		t.Errorf("Expected around 20 events (10-30 range for random sampling), got %d", len(events))
	}
}

// TestSampleDuration tests the SampleDuration method for time-based sampling
func TestSampleDuration(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample at most once per 100ms
	sampledLogger := logger.SampleDuration(100 * time.Millisecond)

	// Log messages rapidly
	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Message {Number}", i)
		time.Sleep(10 * time.Millisecond)
	}

	events := sink.Events()
	// Should have logged first message and maybe 1 more after 100ms
	if len(events) < 1 || len(events) > 2 {
		t.Errorf("Expected 1-2 events with 100ms duration sampling, got %d", len(events))
	}
}

// TestSampleFirst tests the SampleFirst method for logging only first N occurrences
func TestSampleFirst(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Log only first 5 messages
	sampledLogger := logger.SampleFirst(5)

	// Try to log 10 messages
	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Message {Number}", i)
	}

	events := sink.Events()
	if len(events) != 5 {
		t.Errorf("Expected exactly 5 events, got %d", len(events))
	}

	// Verify it's the first 5 messages
	for i, event := range events {
		if num, ok := event.Properties["Number"].(int); ok {
			if num != i+1 {
				t.Errorf("Expected message %d, got %d", i+1, num)
			}
		}
	}
}

// TestSampleGroup tests the SampleGroup method for grouped sampling
func TestSampleGroup(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Create two loggers sampling in the same group
	logger1 := logger.SampleGroup("test-group", 3)
	logger2 := logger.SampleGroup("test-group", 3)

	// Log from both loggers - they should share the counter
	for i := 1; i <= 5; i++ {
		logger1.Info("Logger1 message {Number}", i)
		logger2.Info("Logger2 message {Number}", i)
	}

	events := sink.Events()
	// With shared counter and sampling every 3rd:
	// Messages 1, 4, 7, 10 should be logged (out of 10 total)
	expectedCount := 4
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with shared group counter, got %d", expectedCount, len(events))
	}
}

// TestSampleWhen tests the SampleWhen method for conditional sampling
func TestSampleWhen(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Track a condition
	var highLoad atomic.Bool
	highLoad.Store(false)

	// Sample every 2nd message when high load
	sampledLogger := logger.SampleWhen(func() bool {
		return highLoad.Load()
	}, 2)

	// Log with condition false
	for i := 1; i <= 3; i++ {
		sampledLogger.Info("Low load message {Number}", i)
	}

	// Enable condition
	highLoad.Store(true)

	// Log with condition true
	for i := 4; i <= 8; i++ {
		sampledLogger.Info("High load message {Number}", i)
	}

	events := sink.Events()
	// Should have 0 from low load, and messages 4, 6, 8 from high load
	expectedCount := 3
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with conditional sampling, got %d", expectedCount, len(events))
	}
}

// TestSampleBackoff tests the SampleBackoff method for exponential backoff
func TestSampleBackoff(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample with exponential backoff (factor 2)
	sampledLogger := logger.SampleBackoff("error-key", 2.0)

	// Log 20 messages
	for i := 1; i <= 20; i++ {
		sampledLogger.Error("Error {Number}", i)
	}

	events := sink.Events()
	// With factor 2.0, should log messages at positions: 1, 2, 4, 8, 16
	expectedCount := 5
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with exponential backoff, got %d", expectedCount, len(events))
	}

	// Verify the logged positions
	expectedNumbers := []int{1, 2, 4, 8, 16}
	for i, event := range events {
		if num, ok := event.Properties["Number"].(int); ok {
			if num != expectedNumbers[i] {
				t.Errorf("Expected message at position %d, got %d", expectedNumbers[i], num)
			}
		}
	}
}

// TestResetSampling tests the ResetSampling method
func TestResetSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample every 3rd message
	sampledLogger := logger.Sample(3)

	// Log 3 messages
	for i := 1; i <= 3; i++ {
		sampledLogger.Info("First batch {Number}", i)
	}

	// Reset the sampling counter
	sampledLogger.ResetSampling()

	// Log 3 more messages
	for i := 4; i <= 6; i++ {
		sampledLogger.Info("Second batch {Number}", i)
	}

	events := sink.Events()
	// Should have messages 1 from first batch and 4 from second batch (after reset)
	expectedCount := 2
	if len(events) != expectedCount {
		t.Errorf("Expected %d events after reset, got %d", expectedCount, len(events))
	}
}

// TestResetSamplingGroup tests the ResetSamplingGroup method
func TestResetSamplingGroup(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Create logger with group sampling
	sampledLogger := logger.SampleGroup("reset-test", 3)

	// Log 3 messages
	for i := 1; i <= 3; i++ {
		sampledLogger.Info("First batch {Number}", i)
	}

	// Reset the group counter
	sampledLogger.ResetSamplingGroup("reset-test")

	// Log 3 more messages
	for i := 4; i <= 6; i++ {
		sampledLogger.Info("Second batch {Number}", i)
	}

	events := sink.Events()
	// Should have messages 1 from first batch and 4 from second batch (after reset)
	expectedCount := 2
	if len(events) != expectedCount {
		t.Errorf("Expected %d events after group reset, got %d", expectedCount, len(events))
	}
}

// TestSamplingWithLevels tests that sampling works with different log levels
func TestSamplingWithLevels(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(
		WithSink(sink),
		WithMinimumLevel(core.InformationLevel),
	)

	// Sample every 2nd message
	sampledLogger := logger.Sample(2)

	// Log at different levels
	sampledLogger.Debug("Debug 1")     // Below minimum level
	sampledLogger.Info("Info 1")       // Sampled (1st)
	sampledLogger.Info("Info 2")       // Not sampled
	sampledLogger.Warning("Warning 1") // Sampled (3rd)
	sampledLogger.Error("Error 1")     // Not sampled (4th)
	sampledLogger.Error("Error 2")     // Sampled (5th)

	events := sink.Events()
	// Debug is filtered by level, others follow sampling pattern
	expectedCount := 3 // Info 1, Warning 1, Error 2
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with level filtering and sampling, got %d", expectedCount, len(events))
	}
}

// TestConcurrentSampling tests thread safety of sampling
func TestConcurrentSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Sample every 10th message
	sampledLogger := logger.Sample(10)

	// Log concurrently from multiple goroutines
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < messagesPerGoroutine; i++ {
				sampledLogger.Info("Message from goroutine {GoroutineID} iteration {Iteration}", 
					goroutineID, i)
			}
		}(g)
	}

	wg.Wait()

	events := sink.Events()
	totalMessages := numGoroutines * messagesPerGoroutine
	expectedCount := totalMessages / 10
	// Allow some variance for concurrent execution
	if len(events) < expectedCount-1 || len(events) > expectedCount+1 {
		t.Errorf("Expected around %d events with concurrent sampling, got %d", 
			expectedCount, len(events))
	}
}

// TestChainedSampling tests chaining multiple sampling methods
func TestChainedSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Create two separate sampling loggers from the base logger
	sampled1 := logger.Sample(2)      // Every 2nd
	sampled2 := logger.Sample(3)      // Every 3rd (independent counter)

	// Log 10 messages with each logger
	for i := 1; i <= 10; i++ {
		sampled1.Info("Logger1 message {Number}", i)
		sampled2.Info("Logger2 message {Number}", i)
	}

	events := sink.Events()
	// sampled1 logs every 2nd: 1, 3, 5, 7, 9 (5 messages)
	// sampled2 logs every 3rd: 1, 4, 7, 10 (4 messages)
	// Total: 9 messages
	expectedCount := 9
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with two sampling loggers, got %d", expectedCount, len(events))
	}

	// Test that chaining adds filters cumulatively (both filters apply)
	sink2 := sinks.NewMemorySink()
	logger2 := New(WithSink(sink2))
	
	// Chain sampling - both filters apply cumulatively
	chainedLogger := logger2.Sample(2).Sample(3)
	
	// Log 20 messages to see the pattern
	for i := 1; i <= 20; i++ {
		chainedLogger.Info("Chained message {Number}", i)
	}
	
	events2 := sink2.Events()
	// With both filters applied cumulatively:
	// First filter passes every 2nd: 1, 3, 5, 7, 9, 11, 13, 15, 17, 19
	// Second filter passes every 3rd: 1, 7, 13, 19
	expectedChainedCount := 4
	if len(events2) != expectedChainedCount {
		t.Errorf("Expected %d events with chained sampling, got %d", 
			expectedChainedCount, len(events2))
	}
	
	// Verify the specific message numbers
	expectedChainedNumbers := []int{1, 7, 13, 19}
	for i, event := range events2 {
		if num, ok := event.Properties["Number"].(int); ok {
			if num != expectedChainedNumbers[i] {
				t.Errorf("Expected chained message %d, got %d", expectedChainedNumbers[i], num)
			}
		}
	}
}

// TestWithDefaultSampling tests the WithDefaultSampling configuration option
func TestWithDefaultSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(
		WithSink(sink),
		WithDefaultSampling(3), // Sample every 3rd message by default
	)

	// Log 10 messages
	for i := 1; i <= 10; i++ {
		logger.Info("Message {Number}", i)
	}

	events := sink.Events()
	// Should sample every 3rd: 1, 4, 7, 10
	expectedCount := 4
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with default sampling, got %d", expectedCount, len(events))
	}
}

// TestSampleRateEdgeCases tests edge cases for rate sampling
func TestSampleRateEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		rate          float32
		messageCount  int
		expectedCount int
	}{
		{"Rate 0.0", 0.0, 10, 0},
		{"Rate 1.0", 1.0, 10, 10},
		{"Rate -0.5 (clamped to 0)", -0.5, 10, 0},
		{"Rate 1.5 (clamped to 1)", 1.5, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := sinks.NewMemorySink()
			logger := New(WithSink(sink))
			sampledLogger := logger.SampleRate(tt.rate)

			for i := 1; i <= tt.messageCount; i++ {
				sampledLogger.Info("Message {Number}", i)
			}

			events := sink.Events()
			if len(events) != tt.expectedCount {
				t.Errorf("Expected %d events with rate %f, got %d", 
					tt.expectedCount, tt.rate, len(events))
			}
		})
	}
}

// TestSampleFirstZero tests SampleFirst with n=0
func TestSampleFirstZero(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Log zero messages
	sampledLogger := logger.SampleFirst(0)

	// Try to log messages
	for i := 1; i <= 5; i++ {
		sampledLogger.Info("Message {Number}", i)
	}

	events := sink.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 events with SampleFirst(0), got %d", len(events))
	}
}

// TestConcurrentGroupSampling tests concurrent access to shared group counters
func TestConcurrentGroupSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	const numGoroutines = 20
	const messagesPerGoroutine = 100
	const samplingFactor = 10
	const groupName = "concurrent-test"

	var wg sync.WaitGroup

	// Launch multiple goroutines, each with their own logger sharing the same group
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			// Each goroutine gets its own logger but sharing the same group
			groupLogger := logger.SampleGroup(groupName, samplingFactor)
			
			for i := 0; i < messagesPerGoroutine; i++ {
				groupLogger.Info("Message from goroutine {GoroutineID} iteration {Iteration}", 
					goroutineID, i)
			}
		}(g)
	}

	wg.Wait()

	events := sink.Events()
	totalMessages := numGoroutines * messagesPerGoroutine
	expectedCount := totalMessages / samplingFactor
	
	// Allow variance due to the atomic nature of concurrent operations
	// In concurrent scenarios, the exact count may vary slightly
	tolerance := 2
	if len(events) < expectedCount-tolerance || len(events) > expectedCount+tolerance {
		t.Errorf("Expected approximately %d events (±%d), got %d from %d total messages", 
			expectedCount, tolerance, len(events), totalMessages)
	}

	// Verify that we got messages from multiple goroutines
	goroutineMap := make(map[int]bool)
	for _, event := range events {
		if gid, ok := event.Properties["GoroutineID"].(int); ok {
			goroutineMap[gid] = true
		}
	}

	// Should have messages from multiple goroutines (though not necessarily all)
	if len(goroutineMap) < 2 {
		t.Errorf("Expected messages from multiple goroutines, got from %d goroutines", len(goroutineMap))
	}
}

// TestConcurrentGroupCounterStress stress tests the group counter under extreme load
func TestConcurrentGroupCounterStress(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	const numGoroutines = 50
	const messagesPerGoroutine = 1000
	const samplingFactor = 100
	const groupName = "stress-test"

	var wg sync.WaitGroup

	// Stress test with high concurrency
	start := time.Now()
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			groupLogger := logger.SampleGroup(groupName, samplingFactor)
			
			for i := 0; i < messagesPerGoroutine; i++ {
				groupLogger.Info("Stress message {GoroutineID}-{Iteration}", goroutineID, i)
			}
		}(g)
	}

	wg.Wait()
	duration := time.Since(start)

	events := sink.Events()
	totalMessages := numGoroutines * messagesPerGoroutine
	expectedCount := totalMessages / samplingFactor

	t.Logf("Processed %d messages in %v with %d goroutines", totalMessages, duration, numGoroutines)
	t.Logf("Got %d sampled events (expected ~%d)", len(events), expectedCount)

	// Ensure we got a reasonable number of events (within 20% tolerance for high concurrency)
	tolerance := expectedCount / 5 // 20% tolerance
	if len(events) < expectedCount-tolerance || len(events) > expectedCount+tolerance {
		t.Errorf("Expected approximately %d events (±%d), got %d", 
			expectedCount, tolerance, len(events))
	}
}

// TestGroupSamplingCacheMetrics tests the cache metrics functionality
func TestGroupSamplingCacheMetrics(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Access group manager metrics (this requires accessing internals)
	// We'll create multiple groups to test cache hits/misses
	groups := []string{"group1", "group2", "group3", "group1", "group2"} // group1 and group2 accessed twice
	
	for _, groupName := range groups {
		groupLogger := logger.SampleGroup(groupName, 2)
		groupLogger.Info("Test message for {Group}", groupName)
	}

	// Note: The cache metrics are internal to the SamplingGroupManager
	// In a real scenario, you'd expose these metrics through the public API
	// For now, this test verifies that the functionality works without error
	events := sink.Events()
	
	// Should have logged some messages (exact count depends on sampling)
	if len(events) == 0 {
		t.Error("Expected at least some events to be logged")
	}
}

// TestSampleBackoffLargeFactors tests backoff behavior with large multiplication factors
func TestSampleBackoffLargeFactors(t *testing.T) {
	testCases := []struct {
		name   string
		factor float64
		logs   int
		expectedMinLogs int
		expectedMaxLogs int
	}{
		{"Small Factor", 2.0, 100, 7, 10}, // 1, 2, 4, 8, 16, 32, 64 = 7 logs
		{"Medium Factor", 5.0, 100, 3, 5},  // 1, 5, 25 = 3 logs  
		{"Large Factor", 10.0, 1000, 4, 6}, // 1, 10, 100, 1000 = 4 logs
		{"Very Large Factor", 100.0, 10000, 3, 4}, // 1, 100, 10000 = 3 logs
		{"Invalid Factor", 0.5, 100, 7, 10}, // Should default to 2.0 due to validation
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink := sinks.NewMemorySink()
			logger := New(WithSink(sink))
			
			backoffLogger := logger.SampleBackoff("test-key-"+tc.name, tc.factor)
			
			// Log the specified number of messages
			for i := 1; i <= tc.logs; i++ {
				backoffLogger.Error("Error {Count} with factor {Factor}", i, tc.factor)
			}
			
			events := sink.Events()
			if len(events) < tc.expectedMinLogs || len(events) > tc.expectedMaxLogs {
				t.Errorf("Factor %.1f with %d logs: expected %d-%d events, got %d", 
					tc.factor, tc.logs, tc.expectedMinLogs, tc.expectedMaxLogs, len(events))
			}
			
			// Verify the exponential pattern by checking that early messages were logged
			if len(events) > 0 {
				// First message should always be logged
				if firstCount, ok := events[0].Properties["Count"].(int); ok {
					if firstCount != 1 {
						t.Errorf("Expected first logged message to be Count=1, got %d", firstCount)
					}
				}
			}
		})
	}
}

// TestSampleBackoffKeyIsolation tests that different keys maintain separate backoff counters
func TestSampleBackoffKeyIsolation(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create backoff loggers with different keys but same factor
	logger1 := logger.SampleBackoff("error-type-1", 2.0)
	logger2 := logger.SampleBackoff("error-type-2", 2.0)
	
	// Each should log their first message independently
	logger1.Error("Error type 1 - attempt 1")
	logger2.Error("Error type 2 - attempt 1")
	logger1.Error("Error type 1 - attempt 2") // Should be logged (2nd for this key)
	logger2.Error("Error type 2 - attempt 2") // Should be logged (2nd for this key)
	logger1.Error("Error type 1 - attempt 3") // Should be skipped
	logger2.Error("Error type 2 - attempt 3") // Should be skipped
	logger1.Error("Error type 1 - attempt 4") // Should be logged (4th for this key)
	logger2.Error("Error type 2 - attempt 4") // Should be logged (4th for this key)
	
	events := sink.Events()
	expectedCount := 6 // Messages 1,2,4 for each key
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with isolated keys, got %d", expectedCount, len(events))
	}
}

// TestLRUEvictionGroupSampling tests that LRU eviction works correctly in group sampling
func TestLRUEvictionGroupSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	// Create logger with limited capacity for testing eviction
	// Note: We need to test this with the internal cache, which has default capacity
	// For this test, we'll create many groups to potentially trigger eviction
	logger := New(WithSink(sink))
	
	const numGroups = 50 // Try to create more groups than might fit in cache
	const messagesPerGroup = 5
	
	// Create many different groups
	for i := 0; i < numGroups; i++ {
		groupName := fmt.Sprintf("group-%d", i)
		groupLogger := logger.SampleGroup(groupName, 2) // Sample every 2nd message
		
		for j := 1; j <= messagesPerGroup; j++ {
			groupLogger.Info("Message {MessageNumber} for group {GroupName}", j, groupName)
		}
	}
	
	// Verify that sampling still works correctly even with many groups
	events := sink.Events()
	
	// Each group should have sampled 3 out of 5 messages (1st, 3rd, 5th)
	// So total should be approximately numGroups * 3
	expectedMin := numGroups * 2 // Allow some tolerance
	expectedMax := numGroups * 4
	
	if len(events) < expectedMin || len(events) > expectedMax {
		t.Errorf("Expected %d-%d events with %d groups, got %d", 
			expectedMin, expectedMax, numGroups, len(events))
	}
	
	// Verify we got messages from different groups
	groupsFound := make(map[string]bool)
	for _, event := range events {
		if groupName, ok := event.Properties["GroupName"].(string); ok {
			groupsFound[groupName] = true
		}
	}
	
	// Should have messages from multiple groups
	if len(groupsFound) < 10 {
		t.Errorf("Expected messages from many groups, got from %d groups", len(groupsFound))
	}
}

// TestLRUEvictionBackoffSampling tests LRU eviction with backoff sampling
func TestLRUEvictionBackoffSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	const numKeys = 100 // Create many backoff keys
	const attemptsPerKey = 10
	
	// Create many different backoff keys to test cache behavior
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("error-type-%d", i)
		backoffLogger := logger.SampleBackoff(key, 2.0)
		
		for j := 1; j <= attemptsPerKey; j++ {
			backoffLogger.Error("Error {Attempt} for {ErrorType}", j, key)
		}
	}
	
	events := sink.Events()
	
	// With factor 2.0 and 10 attempts per key, each key should log approximately:
	// Messages 1, 2, 4, 8 = 4 messages per key
	expectedMin := numKeys * 3 // Allow some tolerance
	expectedMax := numKeys * 5
	
	if len(events) < expectedMin || len(events) > expectedMax {
		t.Errorf("Expected %d-%d events with %d backoff keys, got %d", 
			expectedMin, expectedMax, numKeys, len(events))
	}
	
	// Verify we got messages for different error types
	errorTypesFound := make(map[string]bool)
	for _, event := range events {
		if errorType, ok := event.Properties["ErrorType"].(string); ok {
			errorTypesFound[errorType] = true
		}
	}
	
	// Should have messages from multiple error types
	if len(errorTypesFound) < 20 {
		t.Errorf("Expected messages from many error types, got from %d types", len(errorTypesFound))
	}
}

// TestMemoryUsageHighKeyCardinality tests memory behavior under high key cardinality
func TestMemoryUsageHighKeyCardinality(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Test with many unique group names (high cardinality)
	const numUniqueGroups = 1000
	const messagesPerGroup = 2
	
	initialEvents := len(sink.Events())
	
	// Generate many unique group names with timestamp to ensure uniqueness
	baseTime := time.Now().UnixNano()
	for i := 0; i < numUniqueGroups; i++ {
		groupName := fmt.Sprintf("unique-group-%d-%d", baseTime, i)
		groupLogger := logger.SampleGroup(groupName, 1) // Log every message
		
		for j := 0; j < messagesPerGroup; j++ {
			groupLogger.Info("High cardinality test message {Index}", j)
		}
	}
	
	events := sink.Events()
	newEvents := len(events) - initialEvents
	expectedEvents := numUniqueGroups * messagesPerGroup
	
	if newEvents != expectedEvents {
		t.Errorf("Expected %d events with high cardinality groups, got %d", 
			expectedEvents, newEvents)
	}
	
	// Test with many unique backoff keys
	const numUniqueKeys = 500
	baseTime2 := time.Now().UnixNano()
	
	initialEvents2 := len(sink.Events())
	
	for i := 0; i < numUniqueKeys; i++ {
		key := fmt.Sprintf("unique-error-%d-%d", baseTime2, i)
		backoffLogger := logger.SampleBackoff(key, 3.0)
		
		// Log just enough to get the first message (which should always be logged)
		backoffLogger.Error("High cardinality backoff test {KeyID}", i)
	}
	
	events2 := sink.Events()
	newEvents2 := len(events2) - initialEvents2
	expectedEvents2 := numUniqueKeys // Each key should log its first message
	
	if newEvents2 != expectedEvents2 {
		t.Errorf("Expected %d events with high cardinality backoff keys, got %d", 
			expectedEvents2, newEvents2)
	}
	
	t.Logf("Successfully handled %d unique groups and %d unique backoff keys", 
		numUniqueGroups, numUniqueKeys)
}

// TestSampleProfile tests the predefined sampling profiles
func TestSampleProfile(t *testing.T) {
	testCases := []struct {
		profileName string
		shouldWork  bool
	}{
		{"HighTrafficAPI", true},
		{"BackgroundWorker", true},
		{"DevelopmentDebug", true},
		{"NonExistentProfile", false}, // Should return unchanged logger
	}

	for _, tc := range testCases {
		t.Run(tc.profileName, func(t *testing.T) {
			sink := sinks.NewMemorySink()
			logger := New(WithSink(sink))
			
			profileLogger := logger.SampleProfile(tc.profileName)
			
			if tc.shouldWork {
				// Should get a different logger instance (with sampling applied)
				if profileLogger == logger {
					t.Errorf("Expected different logger instance for profile %s", tc.profileName)
				}
			} else {
				// Should get the same logger instance (unchanged)
				if profileLogger != logger {
					t.Errorf("Expected same logger instance for non-existent profile %s", tc.profileName)
				}
			}
			
			// Try logging some messages
			for i := 1; i <= 10; i++ {
				profileLogger.Info("Test message {Number}", i)
			}
			
			events := sink.Events()
			// For valid profiles, we expect some sampling (likely fewer than 10 events)
			// For invalid profiles, no sampling should be applied (expect all 10 events)
			if tc.shouldWork {
				t.Logf("Profile %s: logged %d out of 10 events", tc.profileName, len(events))
			} else {
				if len(events) != 10 {
					t.Errorf("Expected all 10 events for invalid profile, got %d", len(events))
				}
			}
		})
	}
}

// TestAdaptiveSampling tests the adaptive sampling functionality
func TestAdaptiveSampling(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create adaptive logger targeting 5 events per second
	adaptiveLogger := logger.SampleAdaptive(5)
	
	// Log a burst of messages
	start := time.Now()
	for i := 1; i <= 50; i++ {
		adaptiveLogger.Info("Adaptive message {Number}", i)
		time.Sleep(1 * time.Millisecond) // Small delay to spread over time
	}
	duration := time.Since(start)
	
	events := sink.Events()
	
	t.Logf("Adaptive sampling: logged %d out of 50 events in %v", len(events), duration)
	
	// Should have sampled some messages (not necessarily exactly 5 due to adaptation)
	if len(events) == 0 {
		t.Error("Expected at least some events with adaptive sampling")
	}
	
	// Should not log all messages (unless rate adapted to 100%)
	if len(events) == 50 {
		t.Logf("Adaptive sampling logged all events (rate may have adapted to 100%%)")
	}
}

// BenchmarkAdaptiveSamplingBasic benchmarks basic adaptive sampling performance
func BenchmarkAdaptiveSamplingBasic(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptive(1000) // Target 1000 events/sec
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		adaptiveLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkAdaptiveSamplingConcurrent benchmarks concurrent adaptive sampling
func BenchmarkAdaptiveSamplingConcurrent(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptive(1000) // Target 1000 events/sec
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			adaptiveLogger.Info("Concurrent benchmark message {Number}", i)
			i++
		}
	})
}

// BenchmarkAdaptiveSamplingHighFrequency benchmarks high-frequency adaptive sampling
func BenchmarkAdaptiveSamplingHighFrequency(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptive(10000) // Target 10,000 events/sec
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		adaptiveLogger.Info("High frequency message {Number}", i)
	}
}

// BenchmarkAdaptiveSamplingWithOptions benchmarks adaptive sampling with custom options
func BenchmarkAdaptiveSamplingWithOptions(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptiveWithOptions(
		1000,            // Target 1000 events/sec
		0.01,            // Min rate 1%
		1.0,             // Max rate 100%
		500*time.Millisecond, // Adjust every 500ms
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		adaptiveLogger.Info("Configured adaptive message {Number}", i)
	}
}

// BenchmarkAdaptiveSamplingWithHysteresis benchmarks adaptive sampling with hysteresis and aggressiveness control
func BenchmarkAdaptiveSamplingWithHysteresis(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptiveWithHysteresis(
		1000,            // Target 1000 events/sec
		0.001,           // Min rate 0.1%
		1.0,             // Max rate 100%
		500*time.Millisecond, // Adjust every 500ms
		0.1,             // Hysteresis threshold 10%
		0.5,             // Moderate aggressiveness
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		adaptiveLogger.Info("Hysteresis adaptive message {Number}", i)
	}
}

// BenchmarkAdaptiveSamplingWithDampening benchmarks adaptive sampling with dampening for extreme load variations
func BenchmarkAdaptiveSamplingWithDampening(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	adaptiveLogger := logger.SampleAdaptiveWithDampening(
		1000,            // Target 1000 events/sec
		0.001,           // Min rate 0.1%
		1.0,             // Max rate 100%
		500*time.Millisecond, // Adjust every 500ms
		0.15,            // Hysteresis threshold 15%
		0.7,             // Moderate-high aggressiveness
		0.4,             // Moderate dampening
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		adaptiveLogger.Info("Dampening adaptive message {Number}", i)
	}
}

// BenchmarkAdaptiveSamplingVsRegularSampling compares adaptive vs regular sampling
func BenchmarkAdaptiveSamplingVsRegularSampling(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	b.Run("Regular Rate Sampling", func(b *testing.B) {
		rateSampledLogger := logger.SampleRate(0.1) // 10% sampling
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			rateSampledLogger.Info("Regular rate message {Number}", i)
		}
	})
	
	b.Run("Adaptive Sampling", func(b *testing.B) {
		adaptiveLogger := logger.SampleAdaptive(1000) // Target 1000 events/sec
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			adaptiveLogger.Info("Adaptive message {Number}", i)
		}
	})
}

// BenchmarkProfileSwitching benchmarks the performance of switching between sampling profiles
func BenchmarkProfileSwitching(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	profiles := []string{
		"HighTrafficAPI",
		"BackgroundWorker", 
		"DevelopmentDebug",
		"PerformanceMetrics",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Cycle through profiles
		profileName := profiles[i%len(profiles)]
		profileLogger := logger.SampleProfile(profileName)
		profileLogger.Info("Profile switching message {Profile} {Number}", profileName, i)
	}
}

// BenchmarkProfileCreation benchmarks the creation of profile-based loggers
func BenchmarkProfileCreation(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Create a new profile logger each time
		profileLogger := logger.SampleProfile("HighTrafficAPI")
		profileLogger.Info("Profile creation message {Number}", i)
	}
}

// BenchmarkConcurrentProfileUsage benchmarks concurrent usage of different sampling profiles
func BenchmarkConcurrentProfileUsage(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	profiles := []string{
		"HighTrafficAPI",
		"BackgroundWorker",
		"DevelopmentDebug", 
		"PerformanceMetrics",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			profileName := profiles[i%len(profiles)]
			profileLogger := logger.SampleProfile(profileName)
			profileLogger.Info("Concurrent profile message {Profile} {Number}", profileName, i)
			i++
		}
	})
}

// BenchmarkAdaptiveSamplingStatsRetrieval benchmarks stats retrieval performance
func BenchmarkAdaptiveSamplingStatsRetrieval(b *testing.B) {
	// For benchmarking stats retrieval, we'll create a direct filter instance
	// since accessing the internal logger structure isn't possible
	adaptiveFilter := NewAdaptiveSamplingFilter(1000)
	
	// Generate some activity for realistic stats
	for i := 0; i < 100; i++ {
		adaptiveFilter.IsEnabled(&core.LogEvent{
			Timestamp: time.Now(),
			MessageTemplate: "Test message",
		})
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = adaptiveFilter.GetStats()
	}
}

// TestProfileSwitchingPerformance tests the performance of switching between profiles
func TestProfileSwitchingPerformance(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	profiles := GetAvailableProfiles()
	if len(profiles) == 0 {
		t.Skip("No profiles available for switching test")
	}
	
	const iterations = 10000
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		profileName := profiles[i%len(profiles)]
		profileLogger := logger.SampleProfile(profileName)
		profileLogger.Info("Profile switching test {Profile} {Number}", profileName, i)
	}
	
	duration := time.Since(start)
	t.Logf("Profile switching: %d operations in %v (%v per operation)", 
		iterations, duration, duration/iterations)
	
	// Should complete reasonably quickly (less than 1ms per operation)
	avgPerOp := duration / iterations
	if avgPerOp > time.Millisecond {
		t.Errorf("Profile switching too slow: %v per operation", avgPerOp)
	}
}

// TestProfileRegistryThreadSafety tests thread safety of the profile registry
func TestProfileRegistryThreadSafety(t *testing.T) {
	// Save the current frozen state and restore it after test
	wasFrozen := IsProfileRegistryFrozen()
	
	if wasFrozen {
		// Can't test thread safety if already frozen - skip this test
		t.Skip("Profile registry is already frozen, can't test thread safety")
	}
	
	defer func() {
		if wasFrozen {
			FreezeProfiles()
		}
	}()
	
	const numGoroutines = 10
	const numOperations = 100
	
	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numOperations)
	
	// Test concurrent profile registration
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < numOperations; i++ {
				profileName := fmt.Sprintf("TestProfile_%d_%d", goroutineID, i)
				err := AddCustomProfile(profileName, "Test profile", func() core.LogEventFilter {
					return NewAdaptiveSamplingFilter(100)
				})
				if err != nil {
					errorChan <- fmt.Errorf("goroutine %d operation %d: %w", goroutineID, i, err)
					return
				}
			}
		}(g)
	}
	
	wg.Wait()
	close(errorChan)
	
	// Check for any errors
	for err := range errorChan {
		t.Errorf("Thread safety error: %v", err)
	}
	
	// Verify we can read profiles concurrently
	profiles := GetAvailableProfiles()
	expectedMinimum := numGoroutines * numOperations
	if len(profiles) < expectedMinimum {
		t.Errorf("Expected at least %d profiles, got %d", expectedMinimum, len(profiles))
	}
}

// TestProfileFreezing tests the profile registry freezing mechanism
func TestProfileFreezing(t *testing.T) {
	// Reset the registry to ensure clean state
	resetProfileRegistryForTesting()
	
	// This test can only run if the registry isn't already frozen
	if IsProfileRegistryFrozen() {
		t.Skip("Profile registry is already frozen")
	}
	
	// Add a test profile
	testProfileName := "TestFreezeProfile"
	err := AddCustomProfile(testProfileName, "Test freeze profile", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(50)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile before freezing: %v", err)
	}
	
	// Freeze the registry
	FreezeProfiles()
	
	if !IsProfileRegistryFrozen() {
		t.Error("Expected registry to be frozen after calling FreezeProfiles()")
	}
	
	// Try to add another profile - should fail
	err = AddCustomProfile("ShouldFailProfile", "This should fail", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(25)
	})
	if err == nil {
		t.Error("Expected error when adding profile to frozen registry")
	}
	
	// Should still be able to read profiles
	profiles := GetAvailableProfiles()
	found := false
	for _, profile := range profiles {
		if profile == testProfileName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find test profile '%s' in available profiles", testProfileName)
	}
	
	// Should still be able to use existing profiles
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	profileLogger := logger.SampleProfile(testProfileName)
	
	// Try logging multiple times to account for sampling probability
	for i := 0; i < 10; i++ {
		profileLogger.Info("Test message after freezing #%d", i)
	}
	
	events := sink.Events()
	if len(events) == 0 {
		t.Error("Expected to be able to use profiles after freezing")
	}
}

// TestBulkProfileRegistration tests registering multiple profiles at once
func TestBulkProfileRegistration(t *testing.T) {
	if IsProfileRegistryFrozen() {
		t.Skip("Profile registry is already frozen")
	}
	
	customProfiles := map[string]SamplingProfile{
		"BulkTest1": {
			name:        "Bulk Test 1",
			description: "First bulk test profile",
			factory: func() core.LogEventFilter {
				return NewAdaptiveSamplingFilter(100)
			},
		},
		"BulkTest2": {
			name:        "Bulk Test 2", 
			description: "Second bulk test profile",
			factory: func() core.LogEventFilter {
				return NewAdaptiveSamplingFilter(200)
			},
		},
		"BulkTest3": {
			name:        "Bulk Test 3",
			description: "Third bulk test profile",
			factory: func() core.LogEventFilter {
				return NewAdaptiveSamplingFilter(300)
			},
		},
	}
	
	err := RegisterCustomProfiles(customProfiles)
	if err != nil {
		t.Fatalf("Failed to register custom profiles: %v", err)
	}
	
	// Verify all profiles were registered
	availableProfiles := GetAvailableProfiles()
	for profileName := range customProfiles {
		found := false
		for _, available := range availableProfiles {
			if available == profileName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find bulk registered profile '%s'", profileName)
		}
	}
	
	// Test that they can be used
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	for profileName := range customProfiles {
		profileLogger := logger.SampleProfile(profileName)
		profileLogger.Info("Testing bulk profile {Profile}", profileName)
	}
	
	events := sink.Events()
	if len(events) == 0 {
		t.Error("Expected events from bulk registered profiles")
	}
}

// TestAdaptiveSamplingWithDampening tests the dampening factor functionality
func TestAdaptiveSamplingWithDampening(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create adaptive logger with dampening
	adaptiveLogger := logger.SampleAdaptiveWithDampening(
		10,                   // Target 10 events per second
		0.01,                 // Min rate 1%
		1.0,                  // Max rate 100%
		100*time.Millisecond, // Fast adjustment for testing
		0.2,                  // Hysteresis threshold 20%
		0.8,                  // High aggressiveness
		0.3,                  // Strong dampening factor
	)
	
	// Log initial burst to trigger rate adjustments
	for i := 1; i <= 100; i++ {
		adaptiveLogger.Info("Dampening test message {Number}", i)
		if i%20 == 0 {
			time.Sleep(150 * time.Millisecond) // Allow adjustment cycles
		}
	}
	
	events := sink.Events()
	if len(events) == 0 {
		t.Error("Expected some events to be logged with dampening")
	}
	
	// The exact count isn't deterministic due to timing, but should be reasonable
	if len(events) > 90 {
		t.Error("Dampening factor should reduce the number of logged events significantly")
	}
}

// TestAdaptiveSamplingWithHysteresis tests the hysteresis functionality for stability
func TestAdaptiveSamplingWithHysteresis(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create adaptive logger with hysteresis
	adaptiveLogger := logger.SampleAdaptiveWithHysteresis(
		5,                    // Target 5 events per second
		0.1,                  // Min rate 10%
		1.0,                  // Max rate 100%
		50*time.Millisecond,  // Fast adjustment for testing
		0.15,                 // Hysteresis threshold 15%
		0.5,                  // Moderate aggressiveness
	)
	
	// Log messages to trigger adjustments
	start := time.Now()
	messageCount := 0
	
	// Log for about 500ms
	for time.Since(start) < 500*time.Millisecond {
		adaptiveLogger.Info("Hysteresis test message {Number}", messageCount)
		messageCount++
		time.Sleep(10 * time.Millisecond)
	}
	
	events := sink.Events()
	if len(events) == 0 {
		t.Error("Expected some events to be logged with hysteresis")
	}
}

// TestAdaptiveSamplingOscillationDetection tests oscillation detection and dampening
func TestAdaptiveSamplingOscillationDetection(t *testing.T) {
	// Create a filter directly to test internal oscillation detection
	filter := NewAdaptiveSamplingFilterWithDampening(
		50,                   // Target 50 events per second
		0.01,                 // Min rate 1%
		1.0,                  // Max rate 100%
		50*time.Millisecond,  // Fast adjustment interval
		0.1,                  // Low hysteresis threshold
		0.9,                  // High aggressiveness (more likely to oscillate)
		0.2,                  // Strong dampening when oscillation detected
	)
	
	// Simulate oscillating conditions by rapidly changing event frequency
	testEvent := &core.LogEvent{
		Timestamp:       time.Now(),
		MessageTemplate: "Oscillation test",
		Level:           core.InformationLevel,
	}
	
	acceptedCount := 0
	totalTests := 1000
	
	for i := 0; i < totalTests; i++ {
		testEvent.Timestamp = time.Now()
		if filter.IsEnabled(testEvent) {
			acceptedCount++
		}
		
		// Create varying load patterns that could cause oscillation
		if i%100 == 0 && i > 0 {
			time.Sleep(60 * time.Millisecond) // Allow adjustment
		}
	}
	
	// Should have sampled some events but dampening should prevent excessive sampling
	if acceptedCount == 0 {
		t.Error("Expected some events to be accepted with dampening")
	}
	
	if acceptedCount > int(float64(totalTests)*0.8) {
		t.Error("Dampening should prevent excessive event acceptance during oscillations")
	}
}

// TestAdaptiveSamplingAdjustmentHistory tests the adjustment history tracking
func TestAdaptiveSamplingAdjustmentHistory(t *testing.T) {
	filter := NewAdaptiveSamplingFilterWithDampening(
		20,                   // Target 20 events per second
		0.05,                 // Min rate 5%
		1.0,                  // Max rate 100%
		25*time.Millisecond,  // Very fast adjustment for testing
		0.05,                 // Low hysteresis
		0.7,                  // Moderate aggressiveness
		0.4,                  // Moderate dampening
	)
	
	testEvent := &core.LogEvent{
		Timestamp:       time.Now(),
		MessageTemplate: "History test",
		Level:           core.InformationLevel,
	}
	
	// Generate events at varying rates to build adjustment history
	phases := []struct {
		duration time.Duration
		sleep    time.Duration
	}{
		{100 * time.Millisecond, 5 * time.Millisecond},   // High rate
		{100 * time.Millisecond, 20 * time.Millisecond},  // Medium rate
		{100 * time.Millisecond, 2 * time.Millisecond},   // Very high rate
	}
	
	totalAccepted := 0
	
	for _, phase := range phases {
		phaseStart := time.Now()
		for time.Since(phaseStart) < phase.duration {
			testEvent.Timestamp = time.Now()
			if filter.IsEnabled(testEvent) {
				totalAccepted++
			}
			time.Sleep(phase.sleep)
		}
		
		// Allow adjustment cycles between phases
		time.Sleep(50 * time.Millisecond)
	}
	
	if totalAccepted == 0 {
		t.Error("Expected some events to be accepted during history building")
	}
	
	// Verify the filter is still responsive
	currentRate := filter.GetCurrentRate()
	if currentRate <= 0 || currentRate > 1 {
		t.Errorf("Current rate should be between 0 and 1, got %f", currentRate)
	}
}

// TestDampeningFactorBounds tests dampening factor validation
func TestDampeningFactorBounds(t *testing.T) {
	testCases := []struct {
		name            string
		dampeningFactor float64
		expectValid     bool
	}{
		{"Valid low dampening", 0.1, true},
		{"Valid moderate dampening", 0.5, true},
		{"Valid high dampening", 0.9, true},
		{"Valid maximum dampening", 1.0, true},
		{"Invalid zero dampening", 0.0, false},
		{"Invalid negative dampening", -0.5, false},
		{"Invalid excessive dampening", 1.5, false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewAdaptiveSamplingFilterWithDampening(
				100,                  // Target events per second
				0.01,                 // Min rate
				1.0,                  // Max rate
				1*time.Second,        // Adjustment interval
				0.1,                  // Hysteresis threshold
				0.5,                  // Aggressiveness
				tc.dampeningFactor,   // Test dampening factor
			)
			
			// Filter should be created successfully regardless of input
			// Invalid values should be clamped to valid ranges
			if filter == nil {
				t.Error("Filter should be created even with invalid dampening factor")
			}
			
			// Test that the filter is functional
			testEvent := &core.LogEvent{
				Timestamp:       time.Now(),
				MessageTemplate: "Bounds test",
				Level:           core.InformationLevel,
			}
			
			// Should not panic
			enabled := filter.IsEnabled(testEvent)
			_ = enabled // Use the result
		})
	}
}

// TestProfileVersioning tests the profile versioning system
func TestProfileVersioning(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	// Test adding a versioned profile
	err := AddCustomProfileWithVersion(
		"VersionedProfile",
		"Test versioned profile",
		"2.0",
		false,
		"",
		func() core.LogEventFilter {
			return NewAdaptiveSamplingFilter(100)
		},
	)
	if err != nil {
		t.Fatalf("Failed to add versioned profile: %v", err)
	}
	
	// Test retrieving profile version
	version, exists := GetProfileVersion("VersionedProfile")
	if !exists {
		t.Error("Expected versioned profile to exist")
	}
	if version != "2.0" {
		t.Errorf("Expected version '2.0', got '%s'", version)
	}
	
	// Test retrieving specific version
	profile, found := GetProfileWithVersion("VersionedProfile", "2.0")
	if !found {
		t.Error("Expected to find specific version of profile")
	}
	if profile.version != "2.0" {
		t.Errorf("Expected profile version '2.0', got '%s'", profile.version)
	}
	
	// Test retrieving non-existent version
	_, found = GetProfileWithVersion("VersionedProfile", "1.0")
	if found {
		t.Error("Should not find non-existent version")
	}
	
	// Test getting all versions
	versions := GetProfileVersions("VersionedProfile")
	if len(versions) != 1 {
		t.Errorf("Expected 1 version, got %d", len(versions))
	}
	if versions[0] != "2.0" {
		t.Errorf("Expected version '2.0', got '%s'", versions[0])
	}
}

// TestProfileVersioningWithMultipleVersions tests multiple versions of the same profile
func TestProfileVersioningWithMultipleVersions(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	profileName := "MultiVersionProfile"
	
	// Add multiple versions of the same profile
	versions := []string{"1.0", "1.1", "2.0"}
	for _, version := range versions {
		var replacement string
		if version == "1.0" {
			replacement = "2.0" // Only deprecated version has replacement
		}
		
		err := AddCustomProfileWithVersion(
			profileName,
			fmt.Sprintf("Multi-version profile v%s", version),
			version,
			version == "1.0", // Mark 1.0 as deprecated
			replacement,     // Replacement only for deprecated versions
			func() core.LogEventFilter {
				return NewAdaptiveSamplingFilter(100)
			},
		)
		if err != nil {
			t.Fatalf("Failed to add version %s: %v", version, err)
		}
	}
	
	// Test getting all versions
	allVersions := GetProfileVersions(profileName)
	if len(allVersions) != len(versions) {
		t.Errorf("Expected %d versions, got %d", len(versions), len(allVersions))
	}
	
	// Test deprecation status - the current profile is the last one added (2.0), which should not be deprecated
	isDeprecated, replacement := IsProfileDeprecated(profileName)
	if isDeprecated {
		t.Error("Expected current profile (2.0) to not be deprecated")
	}
	if replacement != "" {
		t.Errorf("Expected no replacement for non-deprecated profile, got '%s'", replacement)
	}
	
	// Test retrieving specific versions
	for _, version := range versions {
		profile, found := GetProfileWithVersion(profileName, version)
		if !found {
			t.Errorf("Expected to find version %s", version)
		}
		if profile.version != version {
			t.Errorf("Expected profile version %s, got %s", version, profile.version)
		}
		if version == "1.0" && !profile.deprecated {
			t.Error("Version 1.0 should be marked as deprecated")
		}
		if version != "1.0" && profile.deprecated {
			t.Errorf("Version %s should not be deprecated", version)
		}
	}
}

// TestSampleProfileWithVersion tests using specific versions of profiles in loggers
func TestSampleProfileWithVersion(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	profileName := "TestVersionedSampling"
	
	// Add two versions with different behaviors
	err := AddCustomProfileWithVersion(
		profileName,
		"Version 1.0 - Sample every 5th message",
		"1.0",
		true,
		"2.0",
		func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(5) // Every 5th message
		},
	)
	if err != nil {
		t.Fatalf("Failed to add version 1.0: %v", err)
	}
	
	err = AddCustomProfileWithVersion(
		profileName,
		"Version 2.0 - Sample every 3rd message",
		"2.0",
		false,
		"",
		func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(3) // Every 3rd message
		},
	)
	if err != nil {
		t.Fatalf("Failed to add version 2.0: %v", err)
	}
	
	// Test using specific version
	versionedLogger := logger.SampleProfileWithVersion(profileName, "1.0")
	
	// Log 15 messages with version 1.0 (should sample every 5th: messages 5, 10, 15)
	for i := 1; i <= 15; i++ {
		versionedLogger.Info("Versioned message {Number}", i)
	}
	
	events := sink.Events()
	expectedCount := 3 // Messages 5, 10, 15
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with version 1.0, got %d", expectedCount, len(events))
	}
	
	// Clear events and test version 2.0
	sink.Clear()
	
	versionedLogger2 := logger.SampleProfileWithVersion(profileName, "2.0")
	
	// Log 12 messages with version 2.0 (should sample every 3rd: messages 3, 6, 9, 12)
	for i := 1; i <= 12; i++ {
		versionedLogger2.Info("Versioned message v2 {Number}", i)
	}
	
	events = sink.Events()
	expectedCount = 4 // Messages 3, 6, 9, 12
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with version 2.0, got %d", expectedCount, len(events))
	}
}

// TestProfileVersioningFallback tests fallback behavior when version doesn't exist
func TestProfileVersioningFallback(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	profileName := "FallbackTestProfile"
	
	// Add only version 1.0
	err := AddCustomProfileWithVersion(
		profileName,
		"Fallback test profile",
		"1.0",
		false,
		"",
		func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(2) // Every 2nd message
		},
	)
	if err != nil {
		t.Fatalf("Failed to add fallback profile: %v", err)
	}
	
	// Try to use non-existent version 2.0 - should fall back to current version (1.0)
	versionedLogger := logger.SampleProfileWithVersion(profileName, "2.0")
	
	// Log messages to test fallback
	for i := 1; i <= 6; i++ {
		versionedLogger.Info("Fallback test message {Number}", i)
	}
	
	events := sink.Events()
	expectedCount := 3 // Messages 2, 4, 6 (every 2nd with version 1.0)
	if len(events) != expectedCount {
		t.Errorf("Expected %d events with fallback, got %d", expectedCount, len(events))
	}
	
	// Try with completely non-existent profile - should return unchanged logger
	sink.Clear()
	nonExistentLogger := logger.SampleProfileWithVersion("NonExistentProfile", "1.0")
	
	// Should log all messages since no sampling is applied
	for i := 1; i <= 3; i++ {
		nonExistentLogger.Info("Non-existent profile message {Number}", i)
	}
	
	events = sink.Events()
	if len(events) != 3 {
		t.Errorf("Expected 3 events with non-existent profile, got %d", len(events))
	}
}

// TestProfileVersioningBackwardCompatibility tests backward compatibility of versioning
func TestProfileVersioningBackwardCompatibility(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	profileName := "BackwardCompatProfile"
	
	// Add profile using old method (should default to version 1.0)
	err := AddCustomProfile(profileName, "Backward compatible profile", func() core.LogEventFilter {
		return filters.NewCounterSamplingFilter(4)
	})
	if err != nil {
		t.Fatalf("Failed to add backward compatible profile: %v", err)
	}
	
	// Verify it has default version
	version, exists := GetProfileVersion(profileName)
	if !exists {
		t.Error("Expected backward compatible profile to exist")
	}
	if version != "1.0" {
		t.Errorf("Expected default version '1.0', got '%s'", version)
	}
	
	// Verify it's accessible through versioned API
	profile, found := GetProfileWithVersion(profileName, "1.0")
	if !found {
		t.Error("Expected to find backward compatible profile with version 1.0")
	}
	if profile.deprecated {
		t.Error("Backward compatible profile should not be deprecated by default")
	}
}

// TestProfileVersioningWithFreeze tests versioning behavior with frozen registry
func TestProfileVersioningWithFreeze(t *testing.T) {
	// Reset registry state for clean test
	resetProfileRegistryForTesting()
	
	profileName := "FreezeVersionTest"
	
	// Add a versioned profile before freezing
	err := AddCustomProfileWithVersion(
		profileName,
		"Profile before freeze",
		"1.0",
		false,
		"",
		func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(3)
		},
	)
	if err != nil {
		t.Fatalf("Failed to add profile before freeze: %v", err)
	}
	
	// Freeze the registry
	FreezeProfiles()
	
	// Should still be able to read versioned data
	version, exists := GetProfileVersion(profileName)
	if !exists || version != "1.0" {
		t.Error("Should still be able to read version after freeze")
	}
	
	versions := GetProfileVersions(profileName)
	if len(versions) != 1 || versions[0] != "1.0" {
		t.Error("Should still be able to read versions after freeze")
	}
	
	// Should not be able to add new versions
	err = AddCustomProfileWithVersion(
		profileName,
		"Profile after freeze",
		"2.0",
		false,
		"",
		func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(2)
		},
	)
	if err == nil {
		t.Error("Should not be able to add profiles after freeze")
	}
}

// TestAutoMigrationDeny tests migration policy with deny consent
func TestAutoMigrationDeny(t *testing.T) {
	resetProfileRegistryForTesting()
	
	// Set migration policy to deny
	err := SetMigrationPolicy(MigrationPolicy{
		Consent:            MigrationDeny,
		PreferStable:       true,
		MaxVersionDistance: 2,
	})
	if err != nil {
		t.Fatalf("Failed to set migration policy: %v", err)
	}
	
	// Add a profile with version 1.0
	profileName := "TestMigrationDeny"
	err = AddCustomProfileWithVersion(profileName, "Test migration deny", "1.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(50)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile: %v", err)
	}
	
	// Add version 2.0
	err = AddCustomProfileWithVersion(profileName, "Test migration deny v2", "2.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(100)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile v2: %v", err)
	}
	
	// Try to get a non-existent version (1.5) - should fail with deny policy
	_, _, found := GetProfileWithMigration(profileName, "1.5")
	if found {
		t.Error("Expected migration to be denied for non-existent version")
	}
}

// TestAutoMigrationPrompt tests migration policy with prompt consent
func TestAutoMigrationPrompt(t *testing.T) {
	resetProfileRegistryForTesting()
	
	// Set migration policy to prompt
	err := SetMigrationPolicy(MigrationPolicy{
		Consent:            MigrationPrompt,
		PreferStable:       true,
		MaxVersionDistance: 2,
	})
	if err != nil {
		t.Fatalf("Failed to set migration policy: %v", err)
	}
	
	// Add a profile with version 1.0
	profileName := "TestMigrationPrompt"
	err = AddCustomProfileWithVersion(profileName, "Test migration prompt", "1.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(50)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile: %v", err)
	}
	
	// Add version 2.0
	err = AddCustomProfileWithVersion(profileName, "Test migration prompt v2", "2.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(100)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile v2: %v", err)
	}
	
	// Try to get a non-existent version (1.5) - should migrate to best available
	profile, actualVersion, found := GetProfileWithMigration(profileName, "1.5")
	if !found {
		t.Error("Expected migration to succeed with prompt policy")
	}
	
	// Should migrate to version 2.0 (latest)
	if actualVersion != "2.0" {
		t.Errorf("Expected migration to version 2.0, got %s", actualVersion)
	}
	
	if profile.name != "Test migration prompt v2" {
		t.Errorf("Expected migrated profile name to be 'Test migration prompt v2', got %s", profile.name)
	}
}

// TestAutoMigrationAuto tests migration policy with automatic consent
func TestAutoMigrationAuto(t *testing.T) {
	resetProfileRegistryForTesting()
	
	// Set migration policy to auto
	err := SetMigrationPolicy(MigrationPolicy{
		Consent:            MigrationAuto,
		PreferStable:       true,
		MaxVersionDistance: 0, // Only allow migration within same major version
	})
	if err != nil {
		t.Fatalf("Failed to set migration policy: %v", err)
	}
	
	// Add profiles with different versions
	profileName := "TestMigrationAuto"
	
	// Version 1.0
	err = AddCustomProfileWithVersion(profileName, "Test migration auto v1.0", "1.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(50)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile v1.0: %v", err)
	}
	
	// Version 1.5
	err = AddCustomProfileWithVersion(profileName, "Test migration auto v1.5", "1.5", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(75)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile v1.5: %v", err)
	}
	
	// Version 2.0 (should be excluded due to MaxVersionDistance = 0)
	err = AddCustomProfileWithVersion(profileName, "Test migration auto v2.0", "2.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(100)
	})
	if err != nil {
		t.Fatalf("Failed to add test profile v2.0: %v", err)
	}
	
	// Try to get version 1.2 - should auto-migrate to 1.5 (within distance constraint)
	profile, actualVersion, found := GetProfileWithMigration(profileName, "1.2")
	if !found {
		t.Error("Expected auto-migration to succeed")
	}
	
	// Should migrate to version 1.5 (latest within constraint)
	if actualVersion != "1.5" {
		t.Errorf("Expected auto-migration to version 1.5, got %s", actualVersion)
	}
	
	if profile.name != "Test migration auto v1.5" {
		t.Errorf("Expected migrated profile name to be 'Test migration auto v1.5', got %s", profile.name)
	}
}

// TestMigrationWithDeprecatedProfiles tests migration behavior with deprecated profiles
func TestMigrationWithDeprecatedProfiles(t *testing.T) {
	resetProfileRegistryForTesting()
	
	// Set migration policy to prefer stable
	err := SetMigrationPolicy(MigrationPolicy{
		Consent:            MigrationPrompt,
		PreferStable:       true,
		MaxVersionDistance: 0, // No distance limit
	})
	if err != nil {
		t.Fatalf("Failed to set migration policy: %v", err)
	}
	
	// Add profiles with different deprecation status
	profileName := "TestMigrationDeprecated"
	
	// Version 1.0 - deprecated
	err = AddCustomProfileWithVersion(profileName, "Test deprecated v1.0", "1.0", true, "2.0", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(50)
	})
	if err != nil {
		t.Fatalf("Failed to add deprecated profile v1.0: %v", err)
	}
	
	// Version 1.5 - deprecated
	err = AddCustomProfileWithVersion(profileName, "Test deprecated v1.5", "1.5", true, "2.0", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(75)
	})
	if err != nil {
		t.Fatalf("Failed to add deprecated profile v1.5: %v", err)
	}
	
	// Version 2.0 - stable
	err = AddCustomProfileWithVersion(profileName, "Test stable v2.0", "2.0", false, "", func() core.LogEventFilter {
		return NewAdaptiveSamplingFilter(100)
	})
	if err != nil {
		t.Fatalf("Failed to add stable profile v2.0: %v", err)
	}
	
	// Try to get non-existent version - should migrate to stable 2.0, skipping deprecated ones
	profile, actualVersion, found := GetProfileWithMigration(profileName, "1.3")
	if !found {
		t.Error("Expected migration to succeed")
	}
	
	// Should migrate to version 2.0 (stable, not deprecated)
	if actualVersion != "2.0" {
		t.Errorf("Expected migration to stable version 2.0, got %s", actualVersion)
	}
	
	if profile.name != "Test stable v2.0" {
		t.Errorf("Expected migrated profile name to be 'Test stable v2.0', got %s", profile.name)
	}
	
	if profile.deprecated {
		t.Error("Expected migrated profile to not be deprecated")
	}
}

// TestVersionComparison tests the version comparison utility functions
func TestVersionComparison(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0", "2.0", -1},
		{"2.0", "1.0", 1},
		{"1.0", "1.0", 0},
		{"1.2", "1.1", 1},
		{"1.0.1", "1.0.2", -1},
		{"2.1.3", "2.1.3", 0},
		{"10.0", "2.0", 1},
		{"1.10", "1.2", 1},
	}
	
	for _, test := range tests {
		result := compareVersions(test.v1, test.v2)
		if result != test.expected {
			t.Errorf("compareVersions(%s, %s) = %d, expected %d", test.v1, test.v2, result, test.expected)
		}
	}
}

// TestVersionDistance tests the version distance calculation
func TestVersionDistance(t *testing.T) {
	tests := []struct {
		from     string
		to       string
		expected int
	}{
		{"1.0", "2.0", 1},
		{"2.0", "1.0", 1},
		{"1.0", "3.0", 2},
		{"1.5", "1.8", 0}, // Same major version
		{"1.0", "1.0", 0},
		{"5.0", "2.0", 3},
	}
	
	for _, test := range tests {
		result := getVersionDistance(test.from, test.to)
		if result != test.expected {
			t.Errorf("getVersionDistance(%s, %s) = %d, expected %d", test.from, test.to, result, test.expected)
		}
	}
}

// TestMigrationPolicyGetSet tests getting and setting migration policies
func TestMigrationPolicyGetSet(t *testing.T) {
	resetProfileRegistryForTesting()
	
	// Test default policy
	defaultPolicy := GetMigrationPolicy()
	if defaultPolicy.Consent != MigrationPrompt {
		t.Errorf("Expected default consent to be MigrationPrompt, got %v", defaultPolicy.Consent)
	}
	
	// Test setting new policy
	newPolicy := MigrationPolicy{
		Consent:            MigrationAuto,
		PreferStable:       false,
		MaxVersionDistance: 5,
	}
	
	err := SetMigrationPolicy(newPolicy)
	if err != nil {
		t.Fatalf("Failed to set migration policy: %v", err)
	}
	
	// Verify it was set
	currentPolicy := GetMigrationPolicy()
	if currentPolicy.Consent != MigrationAuto {
		t.Errorf("Expected consent to be MigrationAuto, got %v", currentPolicy.Consent)
	}
	if currentPolicy.PreferStable != false {
		t.Errorf("Expected PreferStable to be false, got %v", currentPolicy.PreferStable)
	}
	if currentPolicy.MaxVersionDistance != 5 {
		t.Errorf("Expected MaxVersionDistance to be 5, got %v", currentPolicy.MaxVersionDistance)
	}
}

// TestDampeningPresets tests all available dampening presets
func TestDampeningPresets(t *testing.T) {
	presets := GetAvailableDampeningPresets()
	if len(presets) != 5 {
		t.Errorf("Expected 5 dampening presets, got %d", len(presets))
	}
	
	// Test each preset has required fields
	expectedNames := []string{"Conservative", "Moderate", "Aggressive", "Ultra Stable", "Responsive"}
	for i, preset := range presets {
		if preset.Name != expectedNames[i] {
			t.Errorf("Expected preset %d name to be '%s', got '%s'", i, expectedNames[i], preset.Name)
		}
		
		if preset.Description == "" {
			t.Errorf("Preset %s should have a description", preset.Name)
		}
		
		if preset.HysteresisThreshold <= 0 || preset.HysteresisThreshold > 1 {
			t.Errorf("Preset %s has invalid hysteresis threshold: %f", preset.Name, preset.HysteresisThreshold)
		}
		
		if preset.Aggressiveness <= 0 || preset.Aggressiveness > 1 {
			t.Errorf("Preset %s has invalid aggressiveness: %f", preset.Name, preset.Aggressiveness)
		}
		
		if preset.DampeningFactor <= 0 || preset.DampeningFactor > 1 {
			t.Errorf("Preset %s has invalid dampening factor: %f", preset.Name, preset.DampeningFactor)
		}
		
		if preset.AdjustmentInterval <= 0 {
			t.Errorf("Preset %s has invalid adjustment interval: %v", preset.Name, preset.AdjustmentInterval)
		}
	}
}

// TestDampeningPresetFilters tests creating filters with different presets
func TestDampeningPresetFilters(t *testing.T) {
	targetEvents := uint64(100)
	
	// Test each preset
	presets := []DampeningPreset{
		DampeningConservative,
		DampeningModerate,
		DampeningAggressive,
		DampeningUltraStable,
		DampeningResponsive,
	}
	
	for _, preset := range presets {
		// Test with defaults
		filter1 := NewAdaptiveSamplingFilterPresetDefaults(targetEvents, preset)
		if filter1 == nil {
			t.Errorf("Failed to create filter with preset %v", preset)
			continue
		}
		
		config := GetDampeningConfig(preset)
		stats := filter1.GetStats()
		
		// Verify the filter was configured with preset values
		if stats.TargetEventsPerSecond != targetEvents {
			t.Errorf("Preset %v: expected target %d, got %d", preset, targetEvents, stats.TargetEventsPerSecond)
		}
		
		if stats.HysteresisThreshold != config.HysteresisThreshold {
			t.Errorf("Preset %v: expected hysteresis %f, got %f", preset, config.HysteresisThreshold, stats.HysteresisThreshold)
		}
		
		if stats.Aggressiveness != config.Aggressiveness {
			t.Errorf("Preset %v: expected aggressiveness %f, got %f", preset, config.Aggressiveness, stats.Aggressiveness)
		}
		
		// Test with custom rate limits
		filter2 := NewAdaptiveSamplingFilterWithPreset(targetEvents, preset, 0.01, 0.8)
		if filter2 == nil {
			t.Errorf("Failed to create filter with preset %v and custom rates", preset)
		}
	}
}

// TestLoggerPresetMethods tests logger methods for dampening presets
func TestLoggerPresetMethods(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Test SampleAdaptiveWithPreset
	presetLogger := logger.SampleAdaptiveWithPreset(50, DampeningModerate)
	if presetLogger == nil {
		t.Error("SampleAdaptiveWithPreset returned nil logger")
	}
	
	// Test SampleAdaptiveWithPresetCustom
	customLogger := logger.SampleAdaptiveWithPresetCustom(75, DampeningAggressive, 0.05, 0.9)
	if customLogger == nil {
		t.Error("SampleAdaptiveWithPresetCustom returned nil logger")
	}
	
	// Test that the loggers work - log multiple times to account for sampling probability
	for i := 0; i < 10; i++ {
		presetLogger.Info("Test message with preset %d", i)
		customLogger.Info("Test message with custom preset %d", i)
	}
	
	events := sink.Events()
	if len(events) == 0 {
		t.Error("Expected at least some events to be logged with preset filters")
	}
}

// TestDampeningPresetCharacteristics tests the characteristics of different presets
func TestDampeningPresetCharacteristics(t *testing.T) {
	conservative := GetDampeningConfig(DampeningConservative)
	aggressive := GetDampeningConfig(DampeningAggressive)
	ultraStable := GetDampeningConfig(DampeningUltraStable)
	responsive := GetDampeningConfig(DampeningResponsive)
	
	// Conservative should be more conservative than aggressive
	if conservative.Aggressiveness >= aggressive.Aggressiveness {
		t.Error("Conservative preset should be less aggressive than aggressive preset")
	}
	
	if conservative.HysteresisThreshold <= aggressive.HysteresisThreshold {
		t.Error("Conservative preset should have higher hysteresis threshold than aggressive preset")
	}
	
	if conservative.DampeningFactor >= aggressive.DampeningFactor {
		t.Error("Conservative preset should have more dampening (lower factor) than aggressive preset")
	}
	
	// Ultra stable should be the most stable
	if ultraStable.Aggressiveness >= conservative.Aggressiveness {
		t.Error("Ultra stable should be less aggressive than conservative")
	}
	
	if ultraStable.HysteresisThreshold <= conservative.HysteresisThreshold {
		t.Error("Ultra stable should have higher hysteresis threshold than conservative")
	}
	
	// Responsive should be the most responsive
	if responsive.Aggressiveness <= aggressive.Aggressiveness {
		t.Error("Responsive preset should be more aggressive than aggressive preset")
	}
	
	if responsive.HysteresisThreshold >= aggressive.HysteresisThreshold {
		t.Error("Responsive preset should have lower hysteresis threshold than aggressive preset")
	}
	
	if responsive.AdjustmentInterval >= aggressive.AdjustmentInterval {
		t.Error("Responsive preset should have shorter adjustment interval than aggressive preset")
	}
}

// TestDampeningPresetBehavior tests the actual behavior differences between presets
func TestDampeningPresetBehavior(t *testing.T) {
	targetEvents := uint64(100)
	
	// Create filters with different presets
	conservative := NewAdaptiveSamplingFilterPresetDefaults(targetEvents, DampeningConservative)
	aggressive := NewAdaptiveSamplingFilterPresetDefaults(targetEvents, DampeningAggressive)
	
	// Create mock events for testing
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		MessageTemplate: "Test message",
		Level:          core.InformationLevel,
	}
	
	// Test initial rates
	conservativeRate := conservative.GetCurrentRate()
	aggressiveRate := aggressive.GetCurrentRate()
	
	// Both should start around 50% (middle rate)
	if conservativeRate < 0.4 || conservativeRate > 0.6 {
		t.Errorf("Conservative filter should start around 50%%, got %f", conservativeRate)
	}
	
	if aggressiveRate < 0.4 || aggressiveRate > 0.6 {
		t.Errorf("Aggressive filter should start around 50%%, got %f", aggressiveRate)
	}
	
	// Simulate some events to trigger rate adjustments
	// Note: This is a basic test - full behavior testing would require more complex scenarios
	for i := 0; i < 10; i++ {
		conservative.IsEnabled(event)
		aggressive.IsEnabled(event)
	}
	
	// After processing events, both filters should still be functional
	newConservativeRate := conservative.GetCurrentRate()
	newAggressiveRate := aggressive.GetCurrentRate()
	
	if newConservativeRate < 0 || newConservativeRate > 1 {
		t.Errorf("Conservative rate out of bounds: %f", newConservativeRate)
	}
	
	if newAggressiveRate < 0 || newAggressiveRate > 1 {
		t.Errorf("Aggressive rate out of bounds: %f", newAggressiveRate)
	}
}