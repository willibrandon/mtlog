package sinks

import (
	"github.com/willibrandon/mtlog/core"
)

// PredicateBuilder provides a fluent API for building complex predicates.
type PredicateBuilder struct {
	predicates []func(*core.LogEvent) bool
	operators  []logicalOp
	negateNext bool
}

type logicalOp int

const (
	opAnd logicalOp = iota
	opOr
)

// NewPredicateBuilder creates a new predicate builder.
func NewPredicateBuilder() *PredicateBuilder {
	return &PredicateBuilder{
		predicates: make([]func(*core.LogEvent) bool, 0),
		operators:  make([]logicalOp, 0),
	}
}

// Level adds a level-based predicate.
func (b *PredicateBuilder) Level(minLevel core.LogEventLevel) *PredicateBuilder {
	predicate := LevelPredicate(minLevel)
	if b.negateNext {
		predicate = NotPredicate(predicate)
		b.negateNext = false
	}
	b.predicates = append(b.predicates, predicate)
	return b
}

// Property adds a property existence predicate.
func (b *PredicateBuilder) Property(name string) *PredicateBuilder {
	predicate := PropertyPredicate(name)
	if b.negateNext {
		predicate = NotPredicate(predicate)
		b.negateNext = false
	}
	b.predicates = append(b.predicates, predicate)
	return b
}

// PropertyValue adds a property value predicate.
func (b *PredicateBuilder) PropertyValue(name string, value interface{}) *PredicateBuilder {
	predicate := PropertyValuePredicate(name, value)
	if b.negateNext {
		predicate = NotPredicate(predicate)
		b.negateNext = false
	}
	b.predicates = append(b.predicates, predicate)
	return b
}

// Custom adds a custom predicate function.
func (b *PredicateBuilder) Custom(predicate func(*core.LogEvent) bool) *PredicateBuilder {
	if b.negateNext {
		predicate = NotPredicate(predicate)
		b.negateNext = false
	}
	b.predicates = append(b.predicates, predicate)
	return b
}

// And adds an AND operator for the next predicate.
func (b *PredicateBuilder) And() *PredicateBuilder {
	if len(b.predicates) > len(b.operators) {
		b.operators = append(b.operators, opAnd)
	}
	return b
}

// Or adds an OR operator for the next predicate.
func (b *PredicateBuilder) Or() *PredicateBuilder {
	if len(b.predicates) > len(b.operators) {
		b.operators = append(b.operators, opOr)
	}
	return b
}

// Not negates the next predicate.
func (b *PredicateBuilder) Not() *PredicateBuilder {
	b.negateNext = true
	return b
}

// Build creates the final predicate function.
// The builder uses left-to-right evaluation with AND having higher precedence than OR.
func (b *PredicateBuilder) Build() func(*core.LogEvent) bool {
	if len(b.predicates) == 0 {
		return func(*core.LogEvent) bool { return true }
	}
	
	if len(b.predicates) == 1 {
		return b.predicates[0]
	}
	
	// Build groups of AND predicates, then combine with OR
	var groups []func(*core.LogEvent) bool
	currentGroup := []func(*core.LogEvent) bool{b.predicates[0]}
	
	for i := 0; i < len(b.operators); i++ {
		if b.operators[i] == opAnd {
			currentGroup = append(currentGroup, b.predicates[i+1])
		} else { // opOr
			// Finish current AND group
			if len(currentGroup) > 0 {
				groups = append(groups, AndPredicate(currentGroup...))
			}
			// Start new group
			currentGroup = []func(*core.LogEvent) bool{b.predicates[i+1]}
		}
	}
	
	// Add final group
	if len(currentGroup) > 0 {
		groups = append(groups, AndPredicate(currentGroup...))
	}
	
	// Combine groups with OR
	if len(groups) == 1 {
		return groups[0]
	}
	return OrPredicate(groups...)
}

// Common predicate builder shortcuts

// ErrorsOnly creates a builder for error-level events.
func ErrorsOnly() *PredicateBuilder {
	return NewPredicateBuilder().Level(core.ErrorLevel)
}

// AuditEvents creates a builder for audit events.
func AuditEvents() *PredicateBuilder {
	return NewPredicateBuilder().Property("Audit")
}

// MetricEvents creates a builder for metric events.
func MetricEvents() *PredicateBuilder {
	return NewPredicateBuilder().Property("Metric")
}

// CriticalAlerts creates a builder for critical alerts.
func CriticalAlerts() *PredicateBuilder {
	return NewPredicateBuilder().
		Level(core.ErrorLevel).
		And().Property("Critical")
}

// ProductionOnly creates a builder for production environment events.
func ProductionOnly() *PredicateBuilder {
	return NewPredicateBuilder().PropertyValue("Environment", "production")
}