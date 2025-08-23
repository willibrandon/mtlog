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