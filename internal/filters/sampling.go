package filters

import (
	"fmt"
	"hash/fnv"
	"sync/atomic"
	
	"github.com/willibrandon/mtlog/core"
)

// SamplingFilter filters log events based on sampling rules.
type SamplingFilter struct {
	rate    float32 // Sampling rate between 0.0 and 1.0
	counter uint32
}

// NewSamplingFilter creates a filter that samples events at the specified rate.
// Rate should be between 0.0 (no events) and 1.0 (all events).
func NewSamplingFilter(rate float32) *SamplingFilter {
	if rate < 0.0 {
		rate = 0.0
	} else if rate > 1.0 {
		rate = 1.0
	}
	
	return &SamplingFilter{
		rate: rate,
	}
}

// IsEnabled returns true for a percentage of events based on the sampling rate.
func (f *SamplingFilter) IsEnabled(event *core.LogEvent) bool {
	if f.rate <= 0.0 {
		return false
	}
	if f.rate >= 1.0 {
		return true
	}
	
	// Use atomic counter for thread-safe sampling
	count := atomic.AddUint32(&f.counter, 1)
	
	// Simple modulo-based sampling
	threshold := uint32(1.0 / f.rate)
	return count%threshold == 0
}

// HashSamplingFilter samples events based on a hash of a property value.
// This ensures consistent sampling for the same property values.
type HashSamplingFilter struct {
	propertyName string
	rate         float32
}

// NewHashSamplingFilter creates a filter that samples based on property value hash.
func NewHashSamplingFilter(propertyName string, rate float32) *HashSamplingFilter {
	if rate < 0.0 {
		rate = 0.0
	} else if rate > 1.0 {
		rate = 1.0
	}
	
	return &HashSamplingFilter{
		propertyName: propertyName,
		rate:         rate,
	}
}

// IsEnabled returns true if the property hash falls within the sampling range.
func (f *HashSamplingFilter) IsEnabled(event *core.LogEvent) bool {
	if f.rate <= 0.0 {
		return false
	}
	if f.rate >= 1.0 {
		return true
	}
	
	value, exists := event.Properties[f.propertyName]
	if !exists {
		return false
	}
	
	// Hash the property value
	h := fnv.New32a()
	h.Write([]byte(formatValue(value)))
	hash := h.Sum32()
	
	// Check if hash falls within sampling range
	threshold := uint32(float32(^uint32(0)) * f.rate)
	return hash <= threshold
}

// formatValue converts a value to string for hashing.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RateLimitFilter limits the number of events per time window.
type RateLimitFilter struct {
	maxEvents   int32
	windowSize  int64 // in nanoseconds
	windowStart int64
	counter     int32
}

// NewRateLimitFilter creates a filter that limits events to maxEvents per window.
func NewRateLimitFilter(maxEvents int, windowNanos int64) *RateLimitFilter {
	return &RateLimitFilter{
		maxEvents:  int32(maxEvents),
		windowSize: windowNanos,
	}
}

// IsEnabled returns true if the event is within the rate limit.
func (f *RateLimitFilter) IsEnabled(event *core.LogEvent) bool {
	now := event.Timestamp.UnixNano()
	
	// Check if we're in a new window
	windowStart := atomic.LoadInt64(&f.windowStart)
	if now >= windowStart+f.windowSize {
		// Try to start a new window
		if atomic.CompareAndSwapInt64(&f.windowStart, windowStart, now) {
			atomic.StoreInt32(&f.counter, 1)
			return true
		}
		// Someone else started the new window, re-read values
		windowStart = atomic.LoadInt64(&f.windowStart)
	}
	
	// Check if we're still in the current window
	if now < windowStart || now >= windowStart+f.windowSize {
		return false
	}
	
	// Increment counter and check limit
	count := atomic.AddInt32(&f.counter, 1)
	return count <= f.maxEvents
}