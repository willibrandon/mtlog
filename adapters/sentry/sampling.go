package sentry

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// SamplingStrategy defines different sampling strategies for Sentry events
type SamplingStrategy string

const (
	// SamplingOff disables sampling - all events are sent
	SamplingOff SamplingStrategy = "off"
	
	// SamplingFixed uses a fixed rate for all events
	SamplingFixed SamplingStrategy = "fixed"
	
	// SamplingAdaptive adjusts rate based on volume
	SamplingAdaptive SamplingStrategy = "adaptive"
	
	// SamplingPriority samples based on event severity
	SamplingPriority SamplingStrategy = "priority"
	
	// SamplingBurst handles burst scenarios with backoff
	SamplingBurst SamplingStrategy = "burst"
	
	// SamplingCustom uses a custom sampling function
	SamplingCustom SamplingStrategy = "custom"
)

// SamplingConfig configures sampling behavior for the Sentry sink
type SamplingConfig struct {
	// Strategy defines the sampling approach
	Strategy SamplingStrategy
	
	// Rate is the base sampling rate (0.0 to 1.0)
	Rate float32
	
	// ErrorRate is the sampling rate for errors (defaults to 1.0)
	ErrorRate float32
	
	// FatalRate is the sampling rate for fatal events (defaults to 1.0)
	FatalRate float32
	
	// AdaptiveTargetEPS is the target events per second for adaptive sampling
	AdaptiveTargetEPS uint64
	
	// BurstThreshold is the events/sec threshold to trigger burst mode
	BurstThreshold uint64
	
	// CustomSampler is a function for custom sampling logic
	CustomSampler func(event *core.LogEvent) bool
	
	// GroupSampling enables sampling by error fingerprint/group
	GroupSampling bool
	
	// GroupSampleRate is events per group per time window
	GroupSampleRate uint64
	
	// GroupWindow is the time window for group sampling
	GroupWindow time.Duration
}

// DefaultSamplingConfig returns a sensible default sampling configuration
func DefaultSamplingConfig() *SamplingConfig {
	return &SamplingConfig{
		Strategy:          SamplingOff,
		Rate:              1.0,
		ErrorRate:         1.0,
		FatalRate:         1.0,
		AdaptiveTargetEPS: 100,
		BurstThreshold:    1000,
		GroupSampling:     false,
		GroupSampleRate:   10,
		GroupWindow:       time.Minute,
	}
}

// sampler manages sampling decisions for the Sentry sink
type sampler struct {
	config         *SamplingConfig
	eventCount     atomic.Uint64
	lastReset      atomic.Int64
	adaptiveRate   atomic.Uint32 // Stored as uint32 (rate * 10000)
	groupCounters  sync.Map      // map[string]*groupCounter
	burstDetector  *burstDetector
}

// groupCounter tracks sampling for a specific error group
type groupCounter struct {
	count      atomic.Uint64
	windowStart atomic.Int64
}

// burstDetector identifies and handles traffic bursts
type burstDetector struct {
	window       time.Duration
	threshold    uint64
	events       atomic.Uint64
	windowStart  atomic.Int64
	inBurst      atomic.Bool
	backoffUntil atomic.Int64
}

// newSampler creates a new sampler with the given configuration
func newSampler(config *SamplingConfig) *sampler {
	if config == nil {
		config = DefaultSamplingConfig()
	}
	
	s := &sampler{
		config: config,
	}
	
	// Initialize adaptive rate
	if config.Strategy == SamplingAdaptive {
		s.adaptiveRate.Store(uint32(config.Rate * 10000))
	}
	
	// Initialize burst detector
	if config.Strategy == SamplingBurst {
		s.burstDetector = &burstDetector{
			window:    time.Second,
			threshold: config.BurstThreshold,
		}
		s.burstDetector.windowStart.Store(time.Now().Unix())
	}
	
	s.lastReset.Store(time.Now().Unix())
	
	return s
}

// shouldSample determines if an event should be sampled
func (s *sampler) shouldSample(event *core.LogEvent) bool {
	// Always sample if sampling is off
	if s.config.Strategy == SamplingOff {
		return true
	}
	
	// Apply level-based sampling first
	rate := s.getLevelRate(event.Level)
	
	// Apply strategy-specific sampling
	switch s.config.Strategy {
	case SamplingFixed:
		return s.fixedSample(rate)
		
	case SamplingAdaptive:
		return s.adaptiveSample(event, rate)
		
	case SamplingPriority:
		return s.prioritySample(event, rate)
		
	case SamplingBurst:
		return s.burstSample(event, rate)
		
	case SamplingCustom:
		if s.config.CustomSampler != nil {
			return s.config.CustomSampler(event)
		}
		return s.fixedSample(rate)
		
	default:
		return s.fixedSample(rate)
	}
}

// getLevelRate returns the sampling rate for a given level
func (s *sampler) getLevelRate(level core.LogEventLevel) float32 {
	switch level {
	case core.FatalLevel:
		return s.config.FatalRate
	case core.ErrorLevel:
		return s.config.ErrorRate
	default:
		return s.config.Rate
	}
}

// fixedSample implements fixed-rate sampling
func (s *sampler) fixedSample(rate float32) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}
	
	// Simple sampling based on counter
	count := s.eventCount.Add(1)
	threshold := uint64(1.0 / rate)
	return (count % threshold) == 0
}

// adaptiveSample adjusts sampling rate based on traffic volume
func (s *sampler) adaptiveSample(event *core.LogEvent, baseRate float32) bool {
	// Increment event counter
	s.eventCount.Add(1)
	
	now := time.Now().Unix()
	lastReset := s.lastReset.Load()
	
	// Calculate current event rate
	if now > lastReset {
		elapsed := now - lastReset
		if elapsed >= 10 { // Adjust every 10 seconds
			count := s.eventCount.Load()
			currentEPS := count / uint64(elapsed)
			
			// Adjust sampling rate
			if currentEPS > s.config.AdaptiveTargetEPS {
				// Reduce sampling rate
				newRate := float32(s.config.AdaptiveTargetEPS) / float32(currentEPS)
				if newRate < 0.01 {
					newRate = 0.01 // Minimum 1% sampling
				}
				s.adaptiveRate.Store(uint32(newRate * 10000))
			} else {
				// Increase sampling rate towards base rate
				currentRate := float32(s.adaptiveRate.Load()) / 10000
				newRate := currentRate + (baseRate-currentRate)*0.1 // Gradual increase
				s.adaptiveRate.Store(uint32(newRate * 10000))
			}
			
			s.lastReset.Store(now)
			s.eventCount.Store(0) // Reset counter after adjustment
		}
	}
	
	// Use current adaptive rate
	adaptiveRate := float32(s.adaptiveRate.Load()) / 10000
	
	// Use random sampling instead of counter-based for adaptive
	return rand.Float32() < adaptiveRate
}

// prioritySample samples based on event importance
func (s *sampler) prioritySample(event *core.LogEvent, rate float32) bool {
	// Always sample fatal events
	if event.Level == core.FatalLevel {
		return true
	}
	
	// Check for important properties that increase sampling priority
	priority := rate
	
	// Increase priority for events with errors
	if _, hasError := event.Properties["Error"]; hasError {
		priority = min(priority*2, 1.0)
	}
	
	// Increase priority for events with user context
	if _, hasUser := event.Properties["UserId"]; hasUser {
		priority = min(priority*1.5, 1.0)
	}
	
	// Increase priority for events with stack traces
	if event.Exception != nil {
		priority = min(priority*3, 1.0)
	}
	
	return s.fixedSample(priority)
}

// burstSample handles burst scenarios with exponential backoff
func (s *sampler) burstSample(event *core.LogEvent, rate float32) bool {
	if s.burstDetector == nil {
		return s.fixedSample(rate)
	}
	
	now := time.Now().Unix()
	
	// Check if we're in backoff period
	if now < s.burstDetector.backoffUntil.Load() {
		// During backoff, sample at 10% rate
		return s.fixedSample(0.1)
	}
	
	// Update burst detection window
	windowStart := s.burstDetector.windowStart.Load()
	if now > windowStart {
		elapsed := now - windowStart
		if elapsed >= 1 { // Check every second
			count := s.burstDetector.events.Swap(1) // Reset to 1 for current event
			eventsPerSec := count / uint64(elapsed)
			
			if eventsPerSec > s.burstDetector.threshold {
				// Enter burst mode with backoff
				s.burstDetector.inBurst.Store(true)
				s.burstDetector.backoffUntil.Store(now + 10) // 10 second backoff
				return s.fixedSample(0.05) // 5% sampling during burst
			} else {
				s.burstDetector.inBurst.Store(false)
			}
			
			s.burstDetector.windowStart.Store(now)
		}
	} else {
		s.burstDetector.events.Add(1)
	}
	
	// Check if we're currently in burst mode
	if s.burstDetector.inBurst.Load() {
		return s.fixedSample(rate * 0.1) // Reduce sampling by 90% during burst
	}
	
	return s.fixedSample(rate)
}

// groupSample implements per-error-group sampling
func (s *sampler) groupSample(fingerprint string) bool {
	if !s.config.GroupSampling {
		return true
	}
	
	now := time.Now()
	windowStart := now.Add(-s.config.GroupWindow).Unix()
	
	// Get or create group counter
	value, _ := s.groupCounters.LoadOrStore(fingerprint, &groupCounter{})
	counter := value.(*groupCounter)
	
	// Check if we need to reset the window
	if counter.windowStart.Load() < windowStart {
		counter.count.Store(0)
		counter.windowStart.Store(now.Unix())
	}
	
	// Check if we've exceeded the group rate
	count := counter.count.Add(1)
	return count <= s.config.GroupSampleRate
}

// reset resets all sampling counters
func (s *sampler) reset() {
	s.eventCount.Store(0)
	s.lastReset.Store(time.Now().Unix())
	s.groupCounters.Range(func(key, value interface{}) bool {
		s.groupCounters.Delete(key)
		return true
	})
	
	if s.burstDetector != nil {
		s.burstDetector.events.Store(0)
		s.burstDetector.windowStart.Store(time.Now().Unix())
		s.burstDetector.inBurst.Store(false)
		s.burstDetector.backoffUntil.Store(0)
	}
}

// getStats returns sampling statistics
func (s *sampler) getStats() map[string]interface{} {
	stats := map[string]interface{}{
		"strategy":    s.config.Strategy,
		"event_count": s.eventCount.Load(),
	}
	
	if s.config.Strategy == SamplingAdaptive {
		stats["adaptive_rate"] = float32(s.adaptiveRate.Load()) / 10000
	}
	
	if s.burstDetector != nil {
		stats["in_burst"] = s.burstDetector.inBurst.Load()
		stats["burst_events"] = s.burstDetector.events.Load()
	}
	
	// Count active groups
	groupCount := 0
	s.groupCounters.Range(func(key, value interface{}) bool {
		groupCount++
		return true
	})
	stats["active_groups"] = groupCount
	
	return stats
}

// min returns the minimum of two float32 values
func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// Helper functions for creating sampling configurations

// WithSampling creates a basic fixed-rate sampling configuration
func WithSampling(rate float32) Option {
	return WithSamplingConfig(&SamplingConfig{
		Strategy:  SamplingFixed,
		Rate:      rate,
		ErrorRate: 1.0, // Always sample errors by default
		FatalRate: 1.0, // Always sample fatal events
	})
}

// WithAdaptiveSampling creates an adaptive sampling configuration
func WithAdaptiveSampling(targetEPS uint64) Option {
	return WithSamplingConfig(&SamplingConfig{
		Strategy:          SamplingAdaptive,
		Rate:              1.0, // Start at full rate
		ErrorRate:         1.0,
		FatalRate:         1.0,
		AdaptiveTargetEPS: targetEPS,
	})
}

// WithPrioritySampling creates a priority-based sampling configuration
func WithPrioritySampling(baseRate float32) Option {
	return WithSamplingConfig(&SamplingConfig{
		Strategy:  SamplingPriority,
		Rate:      baseRate,
		ErrorRate: 1.0,
		FatalRate: 1.0,
	})
}

// WithBurstSampling creates a burst-aware sampling configuration
func WithBurstSampling(threshold uint64) Option {
	return WithSamplingConfig(&SamplingConfig{
		Strategy:       SamplingBurst,
		Rate:           1.0,
		ErrorRate:      1.0,
		FatalRate:      1.0,
		BurstThreshold: threshold,
	})
}

// WithGroupSampling enables per-error-group sampling
func WithGroupSampling(eventsPerGroup uint64, window time.Duration) Option {
	return func(s *SentrySink) {
		if s.samplingConfig == nil {
			s.samplingConfig = DefaultSamplingConfig()
		}
		s.samplingConfig.GroupSampling = true
		s.samplingConfig.GroupSampleRate = eventsPerGroup
		s.samplingConfig.GroupWindow = window
		s.sampler = newSampler(s.samplingConfig)
	}
}

// WithCustomSampling uses a custom sampling function
func WithCustomSampling(sampler func(event *core.LogEvent) bool) Option {
	return WithSamplingConfig(&SamplingConfig{
		Strategy:      SamplingCustom,
		CustomSampler: sampler,
	})
}

// WithSamplingConfig applies a complete sampling configuration
func WithSamplingConfig(config *SamplingConfig) Option {
	return func(s *SentrySink) {
		s.samplingConfig = config
		s.sampler = newSampler(config)
	}
}

// SamplingProfile represents a predefined sampling configuration
type SamplingProfile string

const (
	// SamplingProfileDevelopment - verbose sampling for development
	SamplingProfileDevelopment SamplingProfile = "development"
	
	// SamplingProfileProduction - balanced sampling for production
	SamplingProfileProduction SamplingProfile = "production"
	
	// SamplingProfileHighVolume - aggressive sampling for high-volume apps
	SamplingProfileHighVolume SamplingProfile = "high-volume"
	
	// SamplingProfileCritical - minimal sampling, only critical events
	SamplingProfileCritical SamplingProfile = "critical"
)

// WithSamplingProfile applies a predefined sampling profile
func WithSamplingProfile(profile SamplingProfile) Option {
	var config *SamplingConfig
	
	switch profile {
	case SamplingProfileDevelopment:
		config = &SamplingConfig{
			Strategy:  SamplingOff, // No sampling in dev
			Rate:      1.0,
			ErrorRate: 1.0,
			FatalRate: 1.0,
		}
		
	case SamplingProfileProduction:
		config = &SamplingConfig{
			Strategy:          SamplingAdaptive,
			Rate:              0.1,      // 10% base rate
			ErrorRate:         1.0,      // All errors
			FatalRate:         1.0,      // All fatals
			AdaptiveTargetEPS: 100,     // Target 100 events/sec
			GroupSampling:     true,
			GroupSampleRate:   10,       // 10 per error group per minute
			GroupWindow:       time.Minute,
		}
		
	case SamplingProfileHighVolume:
		config = &SamplingConfig{
			Strategy:          SamplingBurst,
			Rate:              0.01,     // 1% base rate
			ErrorRate:         0.1,      // 10% of errors
			FatalRate:         1.0,      // All fatals
			BurstThreshold:    1000,     // Burst mode above 1000 eps
			GroupSampling:     true,
			GroupSampleRate:   5,        // 5 per error group per minute
			GroupWindow:       time.Minute,
		}
		
	case SamplingProfileCritical:
		config = &SamplingConfig{
			Strategy:  SamplingPriority,
			Rate:      0.001,    // 0.1% base rate
			ErrorRate: 0.01,     // 1% of errors
			FatalRate: 1.0,      // All fatals
		}
		
	default:
		config = DefaultSamplingConfig()
	}
	
	return WithSamplingConfig(config)
}

// Example of using mtlog's sampling with Sentry:
//
// logger := mtlog.New(
//     mtlog.WithSink(sentrySink),
// )
//
// // Use mtlog's sampling for fine-grained control
// logger.SampleRate(0.1).Error("High volume error", err)
// logger.SampleFirst(10).Warning("Repetitive warning")
// logger.SampleDuration(time.Minute).Info("Rate limited info")
// logger.SampleBackoff("api-error", 2.0).Error("API error with backoff")
//
// // Or use Sentry's built-in sampling
// sentrySink, _ := sentry.NewSentrySink(dsn,
//     sentry.WithAdaptiveSampling(100), // Target 100 events/sec
// )