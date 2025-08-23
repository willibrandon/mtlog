package core

import (
	"fmt"
	"time"
)

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

// String returns a human-readable summary of sampling metrics.
// This is useful for periodic logging of sampling performance.
func (m SamplingMetrics) String() string {
	// Calculate cache hit rates
	var groupHitRate, backoffHitRate float64
	
	if m.GroupCacheHits+m.GroupCacheMisses > 0 {
		groupHitRate = float64(m.GroupCacheHits) * 100 / float64(m.GroupCacheHits+m.GroupCacheMisses)
	}
	
	if m.BackoffCacheHits+m.BackoffCacheMisses > 0 {
		backoffHitRate = float64(m.BackoffCacheHits) * 100 / float64(m.BackoffCacheHits+m.BackoffCacheMisses)
	}
	
	// Calculate sampling rate, avoid divide by zero
	total := m.TotalSampled + m.TotalSkipped
	if total == 0 {
		total = 1
	}
	samplingRate := float64(m.TotalSampled) * 100 / float64(total)
	
	return fmt.Sprintf(
		"Sampled=%d Skipped=%d (%.1f%% sampled) | "+
		"GroupCache[hits=%d misses=%d size=%d evict=%d hitRate=%.1f%%] | "+
		"BackoffCache[hits=%d misses=%d size=%d evict=%d hitRate=%.1f%%]",
		m.TotalSampled, 
		m.TotalSkipped, 
		samplingRate,
		m.GroupCacheHits, 
		m.GroupCacheMisses, 
		m.GroupCacheSize, 
		m.GroupCacheEvictions,
		groupHitRate,
		m.BackoffCacheHits,
		m.BackoffCacheMisses,
		m.BackoffCacheSize,
		m.BackoffCacheEvictions,
		backoffHitRate,
	)
}

// Format implements fmt.Formatter for custom formatting options.
// Supports:
//   %s - Default string representation
//   %v - Same as %s
//   %+v - Verbose format with field names
//   %#v - Go syntax representation
func (m SamplingMetrics) Format(f fmt.State, verb rune) {
	switch verb {
	case 's', 'v':
		if f.Flag('+') {
			// Verbose format with newlines for readability
			fmt.Fprintf(f, "SamplingMetrics{\n")
			fmt.Fprintf(f, "  Total: Sampled=%d Skipped=%d\n", m.TotalSampled, m.TotalSkipped)
			fmt.Fprintf(f, "  GroupCache: Hits=%d Misses=%d Size=%d Evictions=%d\n", 
				m.GroupCacheHits, m.GroupCacheMisses, m.GroupCacheSize, m.GroupCacheEvictions)
			fmt.Fprintf(f, "  BackoffCache: Hits=%d Misses=%d Size=%d Evictions=%d\n",
				m.BackoffCacheHits, m.BackoffCacheMisses, m.BackoffCacheSize, m.BackoffCacheEvictions)
			fmt.Fprintf(f, "  AdaptiveCache: Hits=%d Misses=%d Size=%d\n",
				m.AdaptiveCacheHits, m.AdaptiveCacheMisses, m.AdaptiveCacheSize)
			fmt.Fprint(f, "}")
		} else {
			fmt.Fprint(f, m.String())
		}
	default:
		// For %#v and other verbs, use Go syntax representation
		fmt.Fprintf(f, "core.SamplingMetrics{TotalSampled:%d, TotalSkipped:%d, GroupCacheHits:%d, GroupCacheMisses:%d, GroupCacheSize:%d, GroupCacheEvictions:%d, BackoffCacheHits:%d, BackoffCacheMisses:%d, BackoffCacheSize:%d, BackoffCacheEvictions:%d, AdaptiveCacheHits:%d, AdaptiveCacheMisses:%d, AdaptiveCacheSize:%d}",
			m.TotalSampled, m.TotalSkipped,
			m.GroupCacheHits, m.GroupCacheMisses, m.GroupCacheSize, m.GroupCacheEvictions,
			m.BackoffCacheHits, m.BackoffCacheMisses, m.BackoffCacheSize, m.BackoffCacheEvictions,
			m.AdaptiveCacheHits, m.AdaptiveCacheMisses, m.AdaptiveCacheSize)
	}
}

// PrometheusMetrics returns sampling metrics as a map suitable for Prometheus export.
// This makes it easy to integrate with monitoring systems.
func (m SamplingMetrics) PrometheusMetrics() map[string]float64 {
	metrics := make(map[string]float64)
	
	// Overall metrics
	metrics["mtlog_sampling_total_sampled"] = float64(m.TotalSampled)
	metrics["mtlog_sampling_total_skipped"] = float64(m.TotalSkipped)
	
	// Group cache metrics
	metrics["mtlog_sampling_group_cache_hits"] = float64(m.GroupCacheHits)
	metrics["mtlog_sampling_group_cache_misses"] = float64(m.GroupCacheMisses)
	metrics["mtlog_sampling_group_cache_size"] = float64(m.GroupCacheSize)
	metrics["mtlog_sampling_group_cache_evictions"] = float64(m.GroupCacheEvictions)
	
	// Backoff cache metrics
	metrics["mtlog_sampling_backoff_cache_hits"] = float64(m.BackoffCacheHits)
	metrics["mtlog_sampling_backoff_cache_misses"] = float64(m.BackoffCacheMisses)
	metrics["mtlog_sampling_backoff_cache_size"] = float64(m.BackoffCacheSize)
	metrics["mtlog_sampling_backoff_cache_evictions"] = float64(m.BackoffCacheEvictions)
	
	// Adaptive cache metrics
	metrics["mtlog_sampling_adaptive_cache_hits"] = float64(m.AdaptiveCacheHits)
	metrics["mtlog_sampling_adaptive_cache_misses"] = float64(m.AdaptiveCacheMisses)
	metrics["mtlog_sampling_adaptive_cache_size"] = float64(m.AdaptiveCacheSize)
	
	// Calculated metrics
	if m.TotalSampled+m.TotalSkipped > 0 {
		metrics["mtlog_sampling_rate"] = float64(m.TotalSampled) / float64(m.TotalSampled+m.TotalSkipped)
	}
	
	if m.GroupCacheHits+m.GroupCacheMisses > 0 {
		metrics["mtlog_sampling_group_cache_hit_rate"] = float64(m.GroupCacheHits) / float64(m.GroupCacheHits+m.GroupCacheMisses)
	}
	
	if m.BackoffCacheHits+m.BackoffCacheMisses > 0 {
		metrics["mtlog_sampling_backoff_cache_hit_rate"] = float64(m.BackoffCacheHits) / float64(m.BackoffCacheHits+m.BackoffCacheMisses)
	}
	
	if m.AdaptiveCacheHits+m.AdaptiveCacheMisses > 0 {
		metrics["mtlog_sampling_adaptive_cache_hit_rate"] = float64(m.AdaptiveCacheHits) / float64(m.AdaptiveCacheHits+m.AdaptiveCacheMisses)
	}
	
	return metrics
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