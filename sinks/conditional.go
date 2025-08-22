package sinks

import (
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// ConditionalSink routes events to a target sink based on a predicate.
// Events that don't match the predicate are discarded with zero overhead.
type ConditionalSink struct {
	predicate func(*core.LogEvent) bool
	target    core.LogEventSink
	name      string // Optional name for debugging
}

// NewConditionalSink creates a sink that only forwards events matching the predicate.
func NewConditionalSink(predicate func(*core.LogEvent) bool, target core.LogEventSink) *ConditionalSink {
	if predicate == nil {
		panic("predicate cannot be nil")
	}
	if target == nil {
		panic("target sink cannot be nil")
	}
	
	return &ConditionalSink{
		predicate: predicate,
		target:    target,
	}
}

// NewNamedConditionalSink creates a named conditional sink for better debugging.
func NewNamedConditionalSink(name string, predicate func(*core.LogEvent) bool, target core.LogEventSink) *ConditionalSink {
	sink := NewConditionalSink(predicate, target)
	sink.name = name
	return sink
}

// Emit forwards the event to the target sink if the predicate returns true.
func (s *ConditionalSink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}
	
	// Evaluate predicate
	shouldEmit := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				if selflog.IsEnabled() {
					name := s.name
					if name == "" {
						name = "unnamed"
					}
					selflog.Printf("[ConditionalSink:%s] predicate panic: %v", name, r)
				}
				// On panic, don't emit the event
				shouldEmit = false
			}
		}()
		shouldEmit = s.predicate(event)
	}()
	
	if shouldEmit {
		s.target.Emit(event)
	}
}

// Close closes the target sink.
func (s *ConditionalSink) Close() error {
	if closer, ok := s.target.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			if selflog.IsEnabled() {
				name := s.name
				if name == "" {
					name = "unnamed"
				}
				selflog.Printf("[ConditionalSink:%s] failed to close target sink: %v", name, err)
			}
			return err
		}
	}
	return nil
}

// Common predicates for convenience

// LevelPredicate creates a predicate that matches events at or above the specified level.
func LevelPredicate(minLevel core.LogEventLevel) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		return event.Level >= minLevel
	}
}

// PropertyPredicate creates a predicate that matches events containing a specific property.
func PropertyPredicate(propertyName string) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		_, exists := event.Properties[propertyName]
		return exists
	}
}

// PropertyValuePredicate creates a predicate that matches events with a specific property value.
func PropertyValuePredicate(propertyName string, expectedValue interface{}) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		value, exists := event.Properties[propertyName]
		return exists && value == expectedValue
	}
}

// AndPredicate combines multiple predicates with AND logic.
func AndPredicate(predicates ...func(*core.LogEvent) bool) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		for _, p := range predicates {
			if !p(event) {
				return false
			}
		}
		return true
	}
}

// OrPredicate combines multiple predicates with OR logic.
func OrPredicate(predicates ...func(*core.LogEvent) bool) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		for _, p := range predicates {
			if p(event) {
				return true
			}
		}
		return false
	}
}

// NotPredicate inverts a predicate.
func NotPredicate(predicate func(*core.LogEvent) bool) func(*core.LogEvent) bool {
	return func(event *core.LogEvent) bool {
		return !predicate(event)
	}
}