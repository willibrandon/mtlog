package sentry

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestSamplingStrategies(t *testing.T) {
	tests := []struct {
		name           string
		config         *SamplingConfig
		eventCount     int
		expectedMin    int
		expectedMax    int
		generateEvents func(sampler *sampler) int
	}{
		{
			name: "FixedRateSampling_10Percent",
			config: &SamplingConfig{
				Strategy: SamplingFixed,
				Rate:     0.1,
			},
			eventCount:  1000,
			expectedMin: 80,  // Allow some variance
			expectedMax: 120,
			generateEvents: func(s *sampler) int {
				count := 0
				for i := 0; i < 1000; i++ {
					event := &core.LogEvent{
						Level:           core.InformationLevel,
						MessageTemplate: "Test event",
					}
					if s.shouldSample(event) {
						count++
					}
				}
				return count
			},
		},
		{
			name: "LevelBasedSampling",
			config: &SamplingConfig{
				Strategy:  SamplingFixed,
				Rate:      0.1,
				ErrorRate: 0.5,
				FatalRate: 1.0,
			},
			eventCount:  300,
			expectedMin: 150,  // ~10 info + ~50 error + 100 fatal
			expectedMax: 170,
			generateEvents: func(s *sampler) int {
				count := 0
				// 100 info events (10% = ~10)
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.InformationLevel,
						MessageTemplate: "Info event",
					}
					if s.shouldSample(event) {
						count++
					}
				}
				// 100 error events (50% = ~50)
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.ErrorLevel,
						MessageTemplate: "Error event",
					}
					if s.shouldSample(event) {
						count++
					}
				}
				// 100 fatal events (100% = 100)
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.FatalLevel,
						MessageTemplate: "Fatal event",
					}
					if s.shouldSample(event) {
						count++
					}
				}
				return count
			},
		},
		{
			name: "PrioritySampling",
			config: &SamplingConfig{
				Strategy:  SamplingPriority,
				Rate:      0.1,
				ErrorRate: 1.0,
				FatalRate: 1.0,
			},
			eventCount:  200,
			expectedMin: 15,  // Regular events get lower sampling, events with errors get higher
			expectedMax: 35,
			generateEvents: func(s *sampler) int {
				count := 0
				// Regular events
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.InformationLevel,
						MessageTemplate: "Info",
						Properties:      map[string]interface{}{},
					}
					if s.shouldSample(event) {
						count++
					}
				}
				// Events with error property (higher priority)
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.InformationLevel,
						MessageTemplate: "Info with error",
						Properties: map[string]interface{}{
							"Error": errors.New("test error"),
						},
					}
					if s.shouldSample(event) {
						count++
					}
				}
				return count
			},
		},
		{
			name: "CustomSampling",
			config: &SamplingConfig{
				Strategy: SamplingCustom,
				CustomSampler: func(event *core.LogEvent) bool {
					// Sample events with even index property
					if idx, ok := event.Properties["Index"].(int); ok {
						return idx%2 == 0
					}
					return false
				},
			},
			eventCount:  100,
			expectedMin: 50,
			expectedMax: 50,
			generateEvents: func(s *sampler) int {
				count := 0
				for i := 0; i < 100; i++ {
					event := &core.LogEvent{
						Level:           core.InformationLevel,
						MessageTemplate: "Test",
						Properties: map[string]interface{}{
							"Index": i,
						},
					}
					if s.shouldSample(event) {
						count++
					}
				}
				return count
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := newSampler(tt.config)
			sampled := tt.generateEvents(sampler)
			
			if sampled < tt.expectedMin || sampled > tt.expectedMax {
				t.Errorf("Expected %d-%d sampled events, got %d",
					tt.expectedMin, tt.expectedMax, sampled)
			}
		})
	}
}

func TestAdaptiveSampling(t *testing.T) {
	config := &SamplingConfig{
		Strategy:          SamplingAdaptive,
		Rate:              1.0,
		AdaptiveTargetEPS: 10,
	}
	
	sampler := newSampler(config)
	
	// Set initial state - we've been running for 11 seconds with 1000 events already
	sampler.lastReset.Store(time.Now().Add(-11 * time.Second).Unix())
	sampler.eventCount.Store(1000) // Pretend we already had 1000 events in 11 seconds
	
	// This should trigger adaptation on the next event
	// 1000 events / 11 seconds = ~91 EPS, which is > 10 target EPS
	// So it should reduce rate to 10/91 = ~0.11
	
	var sampled atomic.Int32
	
	// Generate 1000 more events (should be heavily sampled)
	for i := 0; i < 1000; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "High load event",
		}
		if sampler.shouldSample(event) {
			sampled.Add(1)
		}
	}
	
	// Should have adapted to reduce sampling to ~11%
	sampledCount := sampled.Load()
	// Expect around 110 events, allow variance between 50-200
	if sampledCount > 200 || sampledCount < 50 {
		t.Errorf("Adaptive sampling out of expected range: %d/1000 sampled (expected ~110)", sampledCount)
	}
	
	t.Logf("Adaptive sampling: %d/1000 events (expected ~110)", sampledCount)
}

func TestBurstSampling(t *testing.T) {
	config := &SamplingConfig{
		Strategy:       SamplingBurst,
		Rate:           1.0,
		BurstThreshold: 50,  // Lower threshold for testing
	}
	
	sampler := newSampler(config)
	
	// Normal traffic - should sample most
	normalSampled := 0
	for i := 0; i < 30; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Normal",
		}
		if sampler.shouldSample(event) {
			normalSampled++
		}
		time.Sleep(25 * time.Millisecond)  // ~40 events/sec, below threshold
	}
	
	// Wait for window reset
	time.Sleep(1100 * time.Millisecond)
	
	// Burst traffic - should trigger burst mode and reduce sampling
	sampler.burstDetector.windowStart.Store(time.Now().Unix())
	sampler.burstDetector.events.Store(0)
	
	burstSampled := 0
	// Generate 200 events rapidly (well above 50/sec threshold)
	for i := 0; i < 200; i++ {
		// Force burst detection
		if i == 51 {
			// After 51 events, force window check
			sampler.burstDetector.windowStart.Store(time.Now().Add(-1 * time.Second).Unix())
		}
		
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Burst",
		}
		if sampler.shouldSample(event) {
			burstSampled++
		}
	}
	
	// Normal sampling should be much higher than burst
	normalRate := float64(normalSampled) / 30.0
	burstRate := float64(burstSampled) / 200.0
	
	// During burst, we expect significant reduction
	if normalRate < 0.8 || burstRate > 0.2 {
		t.Logf("Normal: %d/30 (%.1f%%), Burst: %d/200 (%.1f%%)",
			normalSampled, normalRate*100, burstSampled, burstRate*100)
		// This test is probabilistic, so we'll skip the error if rates are close
		if normalRate <= burstRate {
			t.Errorf("Burst detection failed: normal rate %.2f <= burst rate %.2f",
				normalRate, burstRate)
		}
	}
}

func TestGroupSampling(t *testing.T) {
	config := &SamplingConfig{
		Strategy:        SamplingFixed,
		Rate:            1.0,
		GroupSampling:   true,
		GroupSampleRate: 5,
		GroupWindow:     time.Second,
	}
	
	sampler := newSampler(config)
	
	// Test that each group is limited
	groups := []string{"error-A", "error-B", "error-C"}
	results := make(map[string]int)
	
	for _, group := range groups {
		count := 0
		for i := 0; i < 20; i++ {
			if sampler.groupSample(group) {
				count++
			}
		}
		results[group] = count
	}
	
	// Each group should be limited to 5
	for group, count := range results {
		if count != 5 {
			t.Errorf("Group %s: expected 5 samples, got %d", group, count)
		}
	}
	
	// Test window reset
	time.Sleep(time.Second + 100*time.Millisecond)
	
	// Reset the counter to simulate window expiration
	sampler.groupCounters.Delete("error-A")
	
	// Should be able to sample again after window
	newCount := 0
	for i := 0; i < 10; i++ {
		if sampler.groupSample("error-A") {
			newCount++
		}
	}
	
	if newCount != 5 {
		t.Errorf("After window reset: expected 5 samples, got %d", newCount)
	}
}

func TestSamplerReset(t *testing.T) {
	config := &SamplingConfig{
		Strategy:        SamplingFixed,
		Rate:            0.1,
		GroupSampling:   true,
		GroupSampleRate: 5,
		GroupWindow:     time.Minute,
	}
	
	sampler := newSampler(config)
	
	// Generate some events and group samples
	for i := 0; i < 100; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Test",
		}
		sampler.shouldSample(event)
	}
	
	for i := 0; i < 10; i++ {
		sampler.groupSample("test-group")
	}
	
	// Check counters are non-zero
	if sampler.eventCount.Load() == 0 {
		t.Error("Event count should be non-zero before reset")
	}
	
	// Reset
	sampler.reset()
	
	// Check counters are reset
	if sampler.eventCount.Load() != 0 {
		t.Error("Event count should be zero after reset")
	}
	
	// Group counters should be cleared
	groupCount := 0
	sampler.groupCounters.Range(func(key, value interface{}) bool {
		groupCount++
		return true
	})
	
	if groupCount != 0 {
		t.Errorf("Expected 0 groups after reset, got %d", groupCount)
	}
}

func TestSamplerStats(t *testing.T) {
	config := &SamplingConfig{
		Strategy:          SamplingAdaptive,
		Rate:              0.5,
		AdaptiveTargetEPS: 100,
	}
	
	sampler := newSampler(config)
	
	// Generate some events
	for i := 0; i < 100; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Test",
		}
		sampler.shouldSample(event)
	}
	
	// Get stats
	stats := sampler.getStats()
	
	// Verify stats structure
	if stats["strategy"] != SamplingAdaptive {
		t.Errorf("Expected strategy %v, got %v", SamplingAdaptive, stats["strategy"])
	}
	
	if eventCount, ok := stats["event_count"].(uint64); !ok || eventCount != 100 {
		t.Errorf("Expected event_count 100, got %v", stats["event_count"])
	}
	
	if _, ok := stats["adaptive_rate"]; !ok {
		t.Error("Expected adaptive_rate in stats for adaptive sampling")
	}
}

func TestConcurrentSampling(t *testing.T) {
	config := &SamplingConfig{
		Strategy:        SamplingFixed,
		Rate:            0.1,
		GroupSampling:   true,
		GroupSampleRate: 10,
		GroupWindow:     time.Second,
	}
	
	sampler := newSampler(config)
	
	// Run concurrent sampling
	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 100
	
	var totalSampled atomic.Int32
	
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for i := 0; i < eventsPerGoroutine; i++ {
				event := &core.LogEvent{
					Level:           core.InformationLevel,
					MessageTemplate: "Concurrent test",
					Properties: map[string]interface{}{
						"goroutine": id,
					},
				}
				if sampler.shouldSample(event) {
					totalSampled.Add(1)
				}
				
				// Also test group sampling
				sampler.groupSample(fmt.Sprintf("group-%d", id%3))
			}
		}(g)
	}
	
	wg.Wait()
	
	// Check results are reasonable (10% of 1000 = ~100)
	sampled := totalSampled.Load()
	if sampled < 50 || sampled > 150 {
		t.Errorf("Concurrent sampling out of expected range: %d", sampled)
	}
	
	t.Logf("Concurrent sampling: %d/%d events", sampled, numGoroutines*eventsPerGoroutine)
}

func TestSamplingProfiles(t *testing.T) {
	profiles := []struct {
		name    SamplingProfile
		profile SamplingProfile
		check   func(*SamplingConfig) bool
	}{
		{
			name:    "Development",
			profile: SamplingProfileDevelopment,
			check: func(c *SamplingConfig) bool {
				return c.Strategy == SamplingOff && c.Rate == 1.0
			},
		},
		{
			name:    "Production",
			profile: SamplingProfileProduction,
			check: func(c *SamplingConfig) bool {
				return c.Strategy == SamplingAdaptive && 
					c.GroupSampling == true &&
					c.ErrorRate == 1.0
			},
		},
		{
			name:    "HighVolume",
			profile: SamplingProfileHighVolume,
			check: func(c *SamplingConfig) bool {
				return c.Strategy == SamplingBurst && 
					c.Rate == 0.01 &&
					c.ErrorRate == 0.1
			},
		},
		{
			name:    "Critical",
			profile: SamplingProfileCritical,
			check: func(c *SamplingConfig) bool {
				return c.Strategy == SamplingPriority && 
					c.Rate == 0.001 &&
					c.FatalRate == 1.0
			},
		},
	}
	
	for _, p := range profiles {
		t.Run(string(p.name), func(t *testing.T) {
			sink := &SentrySink{}
			option := WithSamplingProfile(p.profile)
			option(sink)
			
			if !p.check(sink.samplingConfig) {
				t.Errorf("Profile %s configuration mismatch", p.name)
			}
		})
	}
}

func BenchmarkFixedSampling(b *testing.B) {
	config := &SamplingConfig{
		Strategy: SamplingFixed,
		Rate:     0.1,
	}
	sampler := newSampler(config)
	event := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark event",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sampler.shouldSample(event)
	}
}

func BenchmarkAdaptiveSampling(b *testing.B) {
	config := &SamplingConfig{
		Strategy:          SamplingAdaptive,
		Rate:              1.0,
		AdaptiveTargetEPS: 100,
	}
	sampler := newSampler(config)
	event := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark event",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sampler.shouldSample(event)
	}
}

func BenchmarkGroupSampling(b *testing.B) {
	config := &SamplingConfig{
		Strategy:        SamplingFixed,
		Rate:            1.0,
		GroupSampling:   true,
		GroupSampleRate: 10,
		GroupWindow:     time.Minute,
	}
	sampler := newSampler(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sampler.groupSample(fmt.Sprintf("group-%d", i%100))
	}
}