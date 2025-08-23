package mtlog

import (
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// TestSamplingConfigBuilder tests the fluent sampling configuration builder
func TestSamplingConfigBuilder(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	// Build a logger with multiple sampling strategies
	// When using Build(), filters are added separately and work in sequence (AND logic)
	logger := New(
		WithSink(sink),
		Sampling().
			Every(3).
			Build(),
	)
	
	// Log 30 messages
	for i := 1; i <= 30; i++ {
		logger.Info("Message {Number}", i)
	}
	
	events := sink.Events()
	
	// Should have 10 messages (every 3rd)
	// Messages: 1, 4, 7, 10, 13, 16, 19, 22, 25, 28
	if len(events) != 10 {
		t.Errorf("Expected 10 events (every 3rd), got %d", len(events))
	}
}

// TestSamplingConfigCombineAND tests AND combination of sampling filters
func TestSamplingConfigCombineAND(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	logger := New(
		WithSink(sink),
		Sampling().
			Every(2).      // Every 2nd message
			First(10).     // First 10 messages
			CombineAND(),  // Both conditions must be true
	)
	
	// Log 20 messages
	for i := 1; i <= 20; i++ {
		logger.Info("Message {Number}", i)
	}
	
	events := sink.Events()
	
	// With AND logic in a composite filter:
	// - Every(2) passes: 1, 3, 5, 7, 9, 11, 13, 15, 17, 19 (odd numbers)
	// - First(10) only gets called for events that pass Every(2) (i.e., odd numbers)
	// - First(10) counts the calls it receives: 1st call (event 1), 2nd call (event 3), etc.
	// - Since there are exactly 10 odd numbers in range 1-20, First(10) passes all of them
	// Result: 1,3,5,7,9,11,13,15,17,19 (all odd numbers)
	//
	// This behavior differs from separate pipeline filters where First(10) would
	// see all events (1-10) and only pass events 1,3,5,7,9 (odd numbers within first 10).
	expectedEvents := []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19}
	if len(events) != len(expectedEvents) {
		t.Errorf("Expected %d events (all odd numbers), got %d", len(expectedEvents), len(events))
		for i, e := range events {
			if num, ok := e.Properties["Number"].(int); ok {
				t.Logf("Event %d: Number=%d", i+1, num)
			}
		}
	} else {
		// Verify we got the correct events
		for i, e := range events {
			if num, ok := e.Properties["Number"].(int); ok {
				if i < len(expectedEvents) && num != expectedEvents[i] {
					t.Errorf("Expected event %d to be %d, got %d", i+1, expectedEvents[i], num)
				}
			}
		}
	}
}

// TestSamplingConfigCombineOR tests OR combination of sampling filters
func TestSamplingConfigCombineOR(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	logger := New(
		WithSink(sink),
		Sampling().
			Every(5).     // Every 5th message
			First(3).     // First 3 messages
			CombineOR(),  // Either condition can be true
	)
	
	// Log 20 messages
	for i := 1; i <= 20; i++ {
		logger.Info("Message {Number}", i)
	}
	
	events := sink.Events()
	
	// Should log messages: 1, 2, 3 (first 3) and 5, 10, 15, 20 (every 5th)
	// Total: 7 messages
	if len(events) != 7 {
		t.Errorf("Expected 7 events (every 5th OR first 3), got %d", len(events))
	}
}

// TestSamplingConfigComplex tests a complex sampling configuration
func TestSamplingConfigComplex(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	// Complex sampling with multiple strategies
	logger := New(
		WithSink(sink),
		Sampling().
			Rate(0.8).                    // 80% sampling (higher rate for reliability)
			Duration(50*time.Millisecond). // At most once per 50ms
			Group("test", 2).             // Every 2nd in group
			Build(),
	)
	
	// Log many messages to ensure some pass through
	for i := 1; i <= 20; i++ {
		logger.Info("Message {Number}", i)
		if i%3 == 0 {
			time.Sleep(60 * time.Millisecond) // Ensure some pass duration filter
		}
	}
	
	events := sink.Events()
	
	// Should have some events (exact count varies due to rate sampling)
	if len(events) == 0 {
		t.Error("Expected some events with complex sampling, got none")
	}
	
	if len(events) > 15 {
		t.Errorf("Expected at most 15 events, got %d", len(events))
	}
}

// TestWithSamplingPolicy tests custom sampling policy
func TestWithSamplingPolicy(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	// Create a custom policy that only logs even-numbered events
	evenPolicy := &customPolicy{
		shouldSampleFunc: func(event *core.LogEvent) bool {
			// Check if the event has a Number property that's even
			if props, ok := event.Properties["Number"].(int); ok {
				return props%2 == 0
			}
			return false
		},
	}
	
	logger := New(
		WithSink(sink),
		WithSamplingPolicy(evenPolicy),
	)
	
	// Log 10 messages
	for i := 1; i <= 10; i++ {
		logger.Info("Message {Number}", i)
	}
	
	events := sink.Events()
	
	// Should have 5 events (even numbers: 2, 4, 6, 8, 10)
	if len(events) != 5 {
		t.Errorf("Expected 5 even-numbered events, got %d", len(events))
	}
	
	// Verify all are even
	for _, event := range events {
		if num, ok := event.Properties["Number"].(int); ok {
			if num%2 != 0 {
				t.Errorf("Expected only even numbers, got %d", num)
			}
		}
	}
	
	// Check stats
	stats := evenPolicy.Stats()
	if stats.Sampled != 5 {
		t.Errorf("Expected 5 sampled in stats, got %d", stats.Sampled)
	}
	if stats.Skipped != 5 {
		t.Errorf("Expected 5 skipped in stats, got %d", stats.Skipped)
	}
}

// customPolicy is a test implementation of SamplingPolicy
type customPolicy struct {
	shouldSampleFunc func(event *core.LogEvent) bool
	sampled          uint64
	skipped          uint64
}

func (p *customPolicy) ShouldSample(event *core.LogEvent) bool {
	if p.shouldSampleFunc != nil && p.shouldSampleFunc(event) {
		p.sampled++
		return true
	}
	p.skipped++
	return false
}

func (p *customPolicy) Reset() {
	p.sampled = 0
	p.skipped = 0
}

func (p *customPolicy) Stats() core.SamplingStats {
	return core.SamplingStats{
		Sampled: p.sampled,
		Skipped: p.skipped,
	}
}

// TestSamplingAsOption tests using the sampling builder as an option
func TestSamplingAsOption(t *testing.T) {
	sink := sinks.NewMemorySink()
	
	// Create sampling config separately
	samplingConfig := Sampling().
		Every(2).
		First(6).
		AsOption()
	
	logger := New(
		WithSink(sink),
		samplingConfig,
	)
	
	// Log 10 messages
	for i := 1; i <= 10; i++ {
		logger.Info("Message {Number}", i)
	}
	
	events := sink.Events()
	
	// With pipeline behavior (AsOption() uses Build()):
	// - Every(2) passes: 1, 3, 5, 7, 9 (odd numbers from 1-10)
	// - First(6) sees those 5 events and passes all of them (since 5 < 6)
	// Result: 5 events (1, 3, 5, 7, 9)
	if len(events) != 5 {
		t.Errorf("Expected 5 events (all odd numbers 1-10), got %d", len(events))
		for i, e := range events {
			if num, ok := e.Properties["Number"].(int); ok {
				t.Logf("Event %d: Number=%d", i+1, num)
			}
		}
	}
}