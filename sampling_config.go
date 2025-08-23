package mtlog

import (
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/filters"
)

// SamplingConfigBuilder provides a fluent interface for building complex sampling configurations.
type SamplingConfigBuilder struct {
	filters []core.LogEventFilter
}

// Sampling creates a new sampling configuration builder.
func Sampling() *SamplingConfigBuilder {
	return &SamplingConfigBuilder{
		filters: make([]core.LogEventFilter, 0),
	}
}

// Every samples every Nth message.
func (s *SamplingConfigBuilder) Every(n uint64) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewCounterSamplingFilter(n))
	return s
}

// Rate samples a percentage of messages (0.0 to 1.0).
func (s *SamplingConfigBuilder) Rate(rate float32) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewRateSamplingFilter(rate))
	return s
}

// Duration samples at most once per duration.
func (s *SamplingConfigBuilder) Duration(d time.Duration) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewDurationSamplingFilter(d))
	return s
}

// First logs only the first N occurrences.
func (s *SamplingConfigBuilder) First(n uint64) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewFirstNSamplingFilter(n))
	return s
}

// Group samples within a named group.
func (s *SamplingConfigBuilder) Group(name string, n uint64) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewGroupSamplingFilter(name, n, globalSamplingGroupManager))
	return s
}

// When samples conditionally based on a predicate.
func (s *SamplingConfigBuilder) When(predicate func() bool, n uint64) *SamplingConfigBuilder {
	s.filters = append(s.filters, filters.NewConditionalSamplingFilter(predicate, n))
	return s
}

// Backoff samples with exponential backoff.
func (s *SamplingConfigBuilder) Backoff(key string, factor float64) *SamplingConfigBuilder {
	// Validate factor - must be > 1.0 for exponential backoff to work
	if factor <= 1.0 {
		factor = filters.DefaultBackoffFactor
	}
	s.filters = append(s.filters, filters.NewBackoffSamplingFilter(key, factor, globalBackoffState))
	return s
}

// Build returns an Option that applies all the configured sampling filters in a pipeline.
//
// Pipeline Mode (Build):
// Each filter processes the output of the previous filter sequentially.
//
//     Event → Every(2) → Rate(0.5) → First(10) → Output
//             ↓ 50%      ↓ 25%       ↓ 10 max
//
// In this example:
// - Every(2): Passes 50% of events (every 2nd message)
// - Rate(0.5): Processes only the events that passed Every(2), passes 50% of those (25% total)
// - First(10): Processes only events that passed both previous filters, passes first 10
//
func (s *SamplingConfigBuilder) Build() Option {
	return func(c *config) {
		// Add all sampling filters to the configuration
		for _, filter := range s.filters {
			c.filters = append(c.filters, filter)
		}
	}
}

// AsOption is an alias for Build for convenience.
func (s *SamplingConfigBuilder) AsOption() Option {
	return s.Build()
}

// CombineAND creates a composite filter that requires all conditions to pass.
//
// Composite AND Mode:
// All filters evaluate the same event independently. ALL must approve for the event to pass.
//
//     Event → [Every(2)]   ⎤
//           → [Rate(0.5)]  ⎬ → AND → Output (only if ALL approve)
//           → [First(10)]  ⎦
//
// In this example, an event passes only if:
// - Every(2) approves (even-numbered events), AND
// - Rate(0.5) approves (50% random chance), AND  
// - First(10) approves (within first 10 evaluations)
//
func (s *SamplingConfigBuilder) CombineAND() Option {
	return WithFilter(&compositeSamplingFilter{
		filters: s.filters,
		mode:    compositeAND,
	})
}

// CombineOR creates a composite filter that passes if any condition passes.
//
// Composite OR Mode:
// All filters evaluate the same event independently. ANY can approve for the event to pass.
//
//     Event → [Every(2)]   ⎤
//           → [Rate(0.5)]  ⎬ → OR → Output (if ANY approve)
//           → [First(10)]  ⎦
//
// In this example, an event passes if:
// - Every(2) approves (even-numbered events), OR
// - Rate(0.5) approves (50% random chance), OR
// - First(10) approves (within first 10 evaluations)
//
func (s *SamplingConfigBuilder) CombineOR() Option {
	return WithFilter(&compositeSamplingFilter{
		filters: s.filters,
		mode:    compositeOR,
	})
}

// compositeSamplingFilter combines multiple filters with AND/OR logic.
type compositeSamplingFilter struct {
	filters []core.LogEventFilter
	mode    compositeMode
}

type compositeMode int

const (
	compositeAND compositeMode = iota
	compositeOR
)

// IsEnabled implements core.LogEventFilter.
func (c *compositeSamplingFilter) IsEnabled(event *core.LogEvent) bool {
	if len(c.filters) == 0 {
		return true
	}
	
	switch c.mode {
	case compositeAND:
		// All filters must pass
		for _, filter := range c.filters {
			if filter != nil && !filter.IsEnabled(event) {
				return false
			}
		}
		return true
		
	case compositeOR:
		// Any filter can pass
		for _, filter := range c.filters {
			if filter != nil && filter.IsEnabled(event) {
				return true
			}
		}
		return false
		
	default:
		return true
	}
}

// WithSamplingPolicy creates an Option from a custom SamplingPolicy.
func WithSamplingPolicy(policy core.SamplingPolicy) Option {
	return WithFilter(&samplingPolicyFilter{policy: policy})
}

// samplingPolicyFilter adapts a SamplingPolicy to a LogEventFilter.
type samplingPolicyFilter struct {
	policy core.SamplingPolicy
}

// IsEnabled implements core.LogEventFilter.
func (f *samplingPolicyFilter) IsEnabled(event *core.LogEvent) bool {
	return f.policy.ShouldSample(event)
}