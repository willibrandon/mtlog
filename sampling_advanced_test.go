package mtlog

import (
	"testing"
	"time"

	"github.com/willibrandon/mtlog/sinks"
)

// TestSamplingSummary tests the dedicated summary event emission
func TestSamplingSummary(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create a sampled logger with summary enabled
	sampledLogger := logger.Sample(2).EnableSamplingSummary(100 * time.Millisecond)
	
	// Log some messages
	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Message {Number}", i)
	}
	
	// Wait for summary to be emitted
	time.Sleep(150 * time.Millisecond)
	
	events := sink.Events()
	
	// Should have 5 sampled messages (every 2nd) plus at least 1 summary
	if len(events) < 6 {
		t.Errorf("Expected at least 6 events (5 sampled + 1 summary), got %d", len(events))
	}
	
	// Check for summary event
	foundSummary := false
	for _, event := range events {
		if event.MessageTemplate == "Sampling summary for last {Period}: {Sampled} messages logged, {Skipped} messages skipped" {
			foundSummary = true
			break
		}
	}
	
	if !foundSummary {
		t.Error("Expected to find a sampling summary event")
	}
}

// TestGetSamplingStats tests retrieving sampling statistics
func TestGetSamplingStats(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create a sampled logger
	sampledLogger := logger.Sample(3)
	
	// Log some messages
	for i := 1; i <= 10; i++ {
		sampledLogger.Info("Message {Number}", i)
	}
	
	// Get statistics
	sampled, skipped := sampledLogger.GetSamplingStats()
	
	// Should have sampled 4 messages (1, 4, 7, 10) and skipped 6
	if sampled != 4 {
		t.Errorf("Expected 4 sampled messages, got %d", sampled)
	}
	
	if skipped != 6 {
		t.Errorf("Expected 6 skipped messages, got %d", skipped)
	}
}

// TestCacheWarmup tests the cache warmup functionality
func TestCacheWarmup(t *testing.T) {
	// Warmup common group names
	commonGroups := []string{"database", "cache", "api", "auth"}
	WarmupSamplingGroups(commonGroups)
	
	// Warmup common backoff keys
	commonErrors := []string{"connection-error", "timeout-error", "rate-limit"}
	WarmupSamplingBackoff(commonErrors, 2.0)
	
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Use a pre-warmed group - should work immediately without allocation
	dbLogger := logger.SampleGroup("database", 2)
	dbLogger.Info("Database query")
	
	// Use a pre-warmed backoff key
	errorLogger := logger.SampleBackoff("connection-error", 2.0)
	errorLogger.Error("Connection failed")
	
	events := sink.Events()
	if len(events) != 2 {
		t.Errorf("Expected 2 events after warmup, got %d", len(events))
	}
}

// TestPerFilterRandomSource tests that each filter has its own random source
func TestPerFilterRandomSource(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Create multiple rate-sampled loggers
	logger1 := logger.SampleRate(0.5)
	logger2 := logger.SampleRate(0.5)
	
	// Each should have independent random behavior
	// This is hard to test deterministically, but we can ensure they work
	for i := 0; i < 100; i++ {
		logger1.Info("Logger 1 message {Number}", i)
		logger2.Info("Logger 2 message {Number}", i)
	}
	
	events := sink.Events()
	// With 50% sampling of 200 messages, we expect around 100
	// Allow wide variance for random sampling
	if len(events) < 50 || len(events) > 150 {
		t.Errorf("Expected around 100 events (50-150 range), got %d", len(events))
	}
}