package otel

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// SamplingStrategy defines how events should be sampled
type SamplingStrategy interface {
	// ShouldSample determines if an event should be sampled
	ShouldSample(event *core.LogEvent) bool
}

// RateSampler samples events at a fixed rate
type RateSampler struct {
	rate float64
	rng  *rand.Rand
}

// NewRateSampler creates a sampler that samples a percentage of events
// rate should be between 0.0 (no sampling) and 1.0 (all events)
func NewRateSampler(rate float64) *RateSampler {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &RateSampler{
		rate: rate,
		rng:  rand.New(rand.NewSource(cryptoSeed())),
	}
}

// ShouldSample returns true for approximately rate% of events
func (s *RateSampler) ShouldSample(event *core.LogEvent) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0 {
		return false
	}
	return s.rng.Float64() < s.rate
}

// CounterSampler samples every Nth event
type CounterSampler struct {
	n       uint64
	counter atomic.Uint64
}

// NewCounterSampler creates a sampler that samples every nth event
func NewCounterSampler(n uint64) *CounterSampler {
	if n == 0 {
		n = 1
	}
	return &CounterSampler{n: n}
}

// ShouldSample returns true for every nth event
func (s *CounterSampler) ShouldSample(event *core.LogEvent) bool {
	count := s.counter.Add(1)
	return count%s.n == 0
}

// LevelSampler samples based on log level
type LevelSampler struct {
	minLevel core.LogEventLevel
}

// NewLevelSampler creates a sampler that only samples events at or above minLevel
func NewLevelSampler(minLevel core.LogEventLevel) *LevelSampler {
	return &LevelSampler{minLevel: minLevel}
}

// ShouldSample returns true for events at or above the minimum level
func (s *LevelSampler) ShouldSample(event *core.LogEvent) bool {
	return event.Level >= s.minLevel
}

// AdaptiveSampler adjusts sampling rate based on volume
type AdaptiveSampler struct {
	targetRate     uint64 // Target events per second
	windowSize     time.Duration
	lastWindowTime time.Time
	windowCount    atomic.Uint64
	currentRate    atomic.Uint64 // Current sampling rate as percentage * 100
	rng            *rand.Rand
}

// NewAdaptiveSampler creates a sampler that adapts to maintain a target rate
func NewAdaptiveSampler(targetEventsPerSecond uint64) *AdaptiveSampler {
	return &AdaptiveSampler{
		targetRate:     targetEventsPerSecond,
		windowSize:     time.Second,
		lastWindowTime: time.Now(),
		currentRate:    atomic.Uint64{},
		rng:            rand.New(rand.NewSource(cryptoSeed())),
	}
}

// ShouldSample adapts sampling rate to maintain target throughput
func (s *AdaptiveSampler) ShouldSample(event *core.LogEvent) bool {
	// Increment window count
	count := s.windowCount.Add(1)
	
	// Check if we need to adjust the rate
	now := time.Now()
	if now.Sub(s.lastWindowTime) >= s.windowSize {
		// Calculate actual rate
		actualRate := float64(count) / s.windowSize.Seconds()
		
		// Adjust sampling rate
		if actualRate > float64(s.targetRate) {
			// Reduce sampling
			newRate := float64(s.targetRate) / actualRate * 100
			s.currentRate.Store(uint64(newRate))
		} else {
			// Increase sampling (max 100%)
			s.currentRate.Store(100)
		}
		
		// Reset window
		s.windowCount.Store(0)
		s.lastWindowTime = now
	}
	
	// Apply current sampling rate
	rate := s.currentRate.Load()
	if rate >= 100 {
		return true
	}
	if rate == 0 {
		return false
	}
	
	// Use random sampling for proper rate distribution
	return uint64(s.rng.Intn(100)) < rate
}

// CompositeSampler combines multiple sampling strategies
type CompositeSampler struct {
	samplers []SamplingStrategy
	mode     CompositeMode
}

// CompositeMode defines how multiple samplers are combined
type CompositeMode int

const (
	// AllMode requires all samplers to agree
	AllMode CompositeMode = iota
	// AnyMode requires any sampler to agree
	AnyMode
)

// NewCompositeSampler creates a sampler that combines multiple strategies
func NewCompositeSampler(mode CompositeMode, samplers ...SamplingStrategy) *CompositeSampler {
	return &CompositeSampler{
		samplers: samplers,
		mode:     mode,
	}
}

// ShouldSample applies all sampling strategies based on mode
func (s *CompositeSampler) ShouldSample(event *core.LogEvent) bool {
	if len(s.samplers) == 0 {
		return true
	}
	
	switch s.mode {
	case AllMode:
		for _, sampler := range s.samplers {
			if !sampler.ShouldSample(event) {
				return false
			}
		}
		return true
	case AnyMode:
		for _, sampler := range s.samplers {
			if sampler.ShouldSample(event) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// SamplingSink wraps another sink with sampling
type SamplingSink struct {
	inner    core.LogEventSink
	sampler  SamplingStrategy
	sampled  atomic.Uint64
	dropped  atomic.Uint64
}

// NewSamplingSink creates a sink that samples events before forwarding
func NewSamplingSink(inner core.LogEventSink, sampler SamplingStrategy) *SamplingSink {
	return &SamplingSink{
		inner:   inner,
		sampler: sampler,
	}
}

// Emit samples the event and forwards if selected
func (s *SamplingSink) Emit(event *core.LogEvent) {
	if s.sampler.ShouldSample(event) {
		s.sampled.Add(1)
		s.inner.Emit(event)
	} else {
		s.dropped.Add(1)
	}
}

// GetStats returns sampling statistics
func (s *SamplingSink) GetStats() (sampled, dropped uint64) {
	return s.sampled.Load(), s.dropped.Load()
}

// Close closes the sampling sink
func (s *SamplingSink) Close() error {
	// No cleanup needed for sampling sink itself
	return nil
}

// WithOTLPSampling adds sampling to OTLP sink
func WithOTLPSampling(sampler SamplingStrategy) OTLPOption {
	return func(s *OTLPSink) {
		s.sampler = sampler
	}
}

// cryptoSeed generates a cryptographically secure random seed
func cryptoSeed() int64 {
	var seed int64
	// Try to use crypto/rand for better randomness
	var buf [8]byte
	if _, err := crypto_rand.Read(buf[:]); err == nil {
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
	} else {
		// Fall back to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	return seed
}