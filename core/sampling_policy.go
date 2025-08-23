package core

import "time"

// SamplingPolicy defines the interface for custom sampling strategies.
// Implementations should be thread-safe as they may be called concurrently.
type SamplingPolicy interface {
	// ShouldSample determines if an event should be logged.
	ShouldSample(event *LogEvent) bool
	
	// Reset resets any internal state of the sampling policy.
	Reset()
	
	// Stats returns current sampling statistics.
	Stats() SamplingStats
}

// SamplingStats contains statistics about sampling decisions.
type SamplingStats struct {
	Sampled uint64 // Number of events that were sampled (logged)
	Skipped uint64 // Number of events that were skipped
}

// SamplingMetrics provides detailed metrics about sampling cache performance.
// This helps operators tune cache limits and understand sampling behavior.
type SamplingMetrics struct {
	// Group sampling cache metrics
	GroupCacheHits     uint64 // Number of cache hits for group sampling
	GroupCacheMisses   uint64 // Number of cache misses for group sampling
	GroupCacheSize     int    // Current size of group cache
	GroupCacheEvictions uint64 // Number of evictions from group cache
	
	// Backoff sampling cache metrics
	BackoffCacheHits   uint64 // Number of cache hits for backoff sampling
	BackoffCacheMisses uint64 // Number of cache misses for backoff sampling
	BackoffCacheSize   int    // Current size of backoff cache
	BackoffCacheEvictions uint64 // Number of evictions from backoff cache
	
	// Adaptive sampling metrics
	AdaptiveCacheHits  uint64 // Number of cache hits for adaptive sampling
	AdaptiveCacheMisses uint64 // Number of cache misses for adaptive sampling
	AdaptiveCacheSize  int    // Current size of adaptive cache
	
	// Overall sampling decisions
	TotalSampled uint64 // Total events sampled across all strategies
	TotalSkipped uint64 // Total events skipped across all strategies
}

// CompositeSamplingPolicy combines multiple policies with AND/OR logic.
type CompositeSamplingPolicy interface {
	SamplingPolicy
	
	// Add adds a policy to the composite.
	Add(policy SamplingPolicy)
	
	// Remove removes a policy from the composite.
	Remove(policy SamplingPolicy)
}

// SamplingPolicyFunc is a function adapter for SamplingPolicy.
type SamplingPolicyFunc func(event *LogEvent) bool

// ShouldSample calls the function.
func (f SamplingPolicyFunc) ShouldSample(event *LogEvent) bool {
	return f(event)
}

// Reset is a no-op for function-based policies.
func (f SamplingPolicyFunc) Reset() {}

// Stats returns zero stats for function-based policies.
func (f SamplingPolicyFunc) Stats() SamplingStats {
	return SamplingStats{}
}

// TimeBasedSamplingPolicy samples events based on time windows.
type TimeBasedSamplingPolicy interface {
	SamplingPolicy
	
	// SetWindow sets the time window for sampling.
	SetWindow(duration time.Duration)
	
	// GetWindow returns the current time window.
	GetWindow() time.Duration
}

// AdaptiveSamplingPolicy adjusts sampling rate based on load.
type AdaptiveSamplingPolicy interface {
	SamplingPolicy
	
	// SetTargetRate sets the target events per second.
	SetTargetRate(eventsPerSecond float64)
	
	// GetCurrentRate returns the current sampling rate.
	GetCurrentRate() float64
}