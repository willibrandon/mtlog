package filters

import (
	"regexp"
	"strings"

	"github.com/willibrandon/mtlog/core"
)

// ExpressionFilter filters log events based on property values.
type ExpressionFilter struct {
	propertyName string
	matcher      func(any) bool
}

// NewExpressionFilter creates a filter that matches based on a property value.
func NewExpressionFilter(propertyName string, matcher func(any) bool) *ExpressionFilter {
	return &ExpressionFilter{
		propertyName: propertyName,
		matcher:      matcher,
	}
}

// IsEnabled returns true if the property matches the expression.
func (f *ExpressionFilter) IsEnabled(event *core.LogEvent) bool {
	value, exists := event.Properties[f.propertyName]
	if !exists {
		return false
	}
	return f.matcher(value)
}

// MatchProperty creates a filter that matches when a property equals a specific value.
func MatchProperty(propertyName string, expectedValue any) core.LogEventFilter {
	return NewExpressionFilter(propertyName, func(value any) bool {
		return value == expectedValue
	})
}

// MatchPropertyRegex creates a filter that matches when a property matches a regex pattern.
func MatchPropertyRegex(propertyName string, pattern string) core.LogEventFilter {
	re := regexp.MustCompile(pattern)
	return NewExpressionFilter(propertyName, func(value any) bool {
		str, ok := value.(string)
		if !ok {
			return false
		}
		return re.MatchString(str)
	})
}

// MatchPropertyContains creates a filter that matches when a property contains a substring.
func MatchPropertyContains(propertyName string, substring string) core.LogEventFilter {
	return NewExpressionFilter(propertyName, func(value any) bool {
		str, ok := value.(string)
		if !ok {
			return false
		}
		return strings.Contains(str, substring)
	})
}

// MatchPropertyExists creates a filter that matches when a property exists.
func MatchPropertyExists(propertyName string) core.LogEventFilter {
	return NewPredicateFilter(func(event *core.LogEvent) bool {
		_, exists := event.Properties[propertyName]
		return exists
	})
}

// MatchPropertyAbsent creates a filter that matches when a property does not exist.
func MatchPropertyAbsent(propertyName string) core.LogEventFilter {
	return NewPredicateFilter(func(event *core.LogEvent) bool {
		_, exists := event.Properties[propertyName]
		return !exists
	})
}

// MatchAnyProperty creates a filter that matches when any of the properties match.
func MatchAnyProperty(matchers ...core.LogEventFilter) core.LogEventFilter {
	return NewPredicateFilter(func(event *core.LogEvent) bool {
		for _, matcher := range matchers {
			if matcher.IsEnabled(event) {
				return true
			}
		}
		return false
	})
}

// MatchAllProperties creates a filter that matches when all properties match.
func MatchAllProperties(matchers ...core.LogEventFilter) core.LogEventFilter {
	return NewPredicateFilter(func(event *core.LogEvent) bool {
		for _, matcher := range matchers {
			if !matcher.IsEnabled(event) {
				return false
			}
		}
		return true
	})
}
