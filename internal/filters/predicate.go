package filters

import (
	"github.com/willibrandon/mtlog/core"
)

// PredicateFilter filters log events based on a custom predicate function.
type PredicateFilter struct {
	predicate func(*core.LogEvent) bool
}

// NewPredicateFilter creates a filter that uses a custom predicate function.
func NewPredicateFilter(predicate func(*core.LogEvent) bool) *PredicateFilter {
	return &PredicateFilter{
		predicate: predicate,
	}
}

// IsEnabled returns the result of the predicate function.
func (f *PredicateFilter) IsEnabled(event *core.LogEvent) bool {
	if f.predicate == nil {
		return true
	}
	return f.predicate(event)
}

// ByExcluding creates a filter that excludes events matching the predicate.
func ByExcluding(predicate func(*core.LogEvent) bool) core.LogEventFilter {
	return NewPredicateFilter(func(event *core.LogEvent) bool {
		return !predicate(event)
	})
}

// ByIncluding creates a filter that includes only events matching the predicate.
func ByIncluding(predicate func(*core.LogEvent) bool) core.LogEventFilter {
	return NewPredicateFilter(predicate)
}

// When creates a conditional filter that applies when the condition is true.
func When(condition bool, filter core.LogEventFilter) core.LogEventFilter {
	if condition {
		return filter
	}
	// Return a filter that always passes
	return NewPredicateFilter(func(*core.LogEvent) bool { return true })
}