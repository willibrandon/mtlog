package filters

import (
	"github.com/willibrandon/mtlog/core"
)

// CompositeFilter combines multiple filters with AND logic.
type CompositeFilter struct {
	filters []core.LogEventFilter
}

// NewCompositeFilter creates a filter that requires all sub-filters to pass.
func NewCompositeFilter(filters ...core.LogEventFilter) *CompositeFilter {
	return &CompositeFilter{
		filters: filters,
	}
}

// IsEnabled returns true only if all sub-filters return true.
func (f *CompositeFilter) IsEnabled(event *core.LogEvent) bool {
	for _, filter := range f.filters {
		if !filter.IsEnabled(event) {
			return false
		}
	}
	return true
}

// Add adds a new filter to the composite.
func (f *CompositeFilter) Add(filter core.LogEventFilter) {
	f.filters = append(f.filters, filter)
}

// OrFilter combines multiple filters with OR logic.
type OrFilter struct {
	filters []core.LogEventFilter
}

// NewOrFilter creates a filter that passes if any sub-filter passes.
func NewOrFilter(filters ...core.LogEventFilter) *OrFilter {
	return &OrFilter{
		filters: filters,
	}
}

// IsEnabled returns true if any sub-filter returns true.
func (f *OrFilter) IsEnabled(event *core.LogEvent) bool {
	for _, filter := range f.filters {
		if filter.IsEnabled(event) {
			return true
		}
	}
	return false
}

// NotFilter inverts the result of another filter.
type NotFilter struct {
	inner core.LogEventFilter
}

// NewNotFilter creates a filter that inverts another filter's result.
func NewNotFilter(inner core.LogEventFilter) *NotFilter {
	return &NotFilter{
		inner: inner,
	}
}

// IsEnabled returns the inverse of the inner filter's result.
func (f *NotFilter) IsEnabled(event *core.LogEvent) bool {
	return !f.inner.IsEnabled(event)
}