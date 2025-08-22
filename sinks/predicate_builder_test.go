package sinks

import (
	"testing"
	
	"github.com/willibrandon/mtlog/core"
)

func TestPredicateBuilder(t *testing.T) {
	t.Run("simple level predicate", func(t *testing.T) {
		predicate := NewPredicateBuilder().
			Level(core.ErrorLevel).
			Build()
		
		errorEvent := &core.LogEvent{Level: core.ErrorLevel}
		infoEvent := &core.LogEvent{Level: core.InformationLevel}
		
		if !predicate(errorEvent) {
			t.Error("Should match error level")
		}
		if predicate(infoEvent) {
			t.Error("Should not match info level")
		}
	})
	
	t.Run("AND composition", func(t *testing.T) {
		predicate := NewPredicateBuilder().
			Level(core.ErrorLevel).
			And().Property("Alert").
			Build()
		
		// Both conditions met
		event1 := &core.LogEvent{
			Level:      core.ErrorLevel,
			Properties: map[string]any{"Alert": true},
		}
		if !predicate(event1) {
			t.Error("Should match when both conditions are met")
		}
		
		// Only level matches
		event2 := &core.LogEvent{
			Level:      core.ErrorLevel,
			Properties: map[string]any{},
		}
		if predicate(event2) {
			t.Error("Should not match when only one condition is met")
		}
	})
	
	t.Run("OR composition", func(t *testing.T) {
		predicate := NewPredicateBuilder().
			Level(core.ErrorLevel).
			Or().Property("Important").
			Build()
		
		// First condition matches
		event1 := &core.LogEvent{
			Level:      core.ErrorLevel,
			Properties: map[string]any{},
		}
		if !predicate(event1) {
			t.Error("Should match when first condition is met")
		}
		
		// Second condition matches
		event2 := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"Important": true},
		}
		if !predicate(event2) {
			t.Error("Should match when second condition is met")
		}
		
		// Neither matches
		event3 := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{},
		}
		if predicate(event3) {
			t.Error("Should not match when neither condition is met")
		}
	})
	
	t.Run("NOT modifier", func(t *testing.T) {
		predicate := NewPredicateBuilder().
			Not().Property("SkipLogging").
			Build()
		
		// Has SkipLogging
		event1 := &core.LogEvent{
			Properties: map[string]any{"SkipLogging": true},
		}
		if predicate(event1) {
			t.Error("Should not match when property exists")
		}
		
		// No SkipLogging
		event2 := &core.LogEvent{
			Properties: map[string]any{},
		}
		if !predicate(event2) {
			t.Error("Should match when property doesn't exist")
		}
	})
	
	t.Run("complex composition", func(t *testing.T) {
		// (Error AND Alert) OR (Fatal AND NOT Retry)
		predicate := NewPredicateBuilder().
			Level(core.ErrorLevel).
			And().Property("Alert").
			Or().Level(core.FatalLevel).
			And().Not().Property("Retry").
			Build()
		
		// Error with Alert - should match
		event1 := &core.LogEvent{
			Level:      core.ErrorLevel,
			Properties: map[string]any{"Alert": true},
		}
		if !predicate(event1) {
			t.Error("Should match error with alert")
		}
		
		// Fatal without Retry - should match
		event2 := &core.LogEvent{
			Level:      core.FatalLevel,
			Properties: map[string]any{},
		}
		if !predicate(event2) {
			t.Error("Should match fatal without retry")
		}
		
		// Fatal with Retry - should not match
		event3 := &core.LogEvent{
			Level:      core.FatalLevel,
			Properties: map[string]any{"Retry": true},
		}
		if predicate(event3) {
			t.Error("Should not match fatal with retry")
		}
	})
	
	t.Run("shortcut builders", func(t *testing.T) {
		errorPred := ErrorsOnly().Build()
		auditPred := AuditEvents().Build()
		criticalPred := CriticalAlerts().Build()
		prodPred := ProductionOnly().Build()
		
		errorEvent := &core.LogEvent{Level: core.ErrorLevel}
		if !errorPred(errorEvent) {
			t.Error("ErrorsOnly should match error events")
		}
		
		auditEvent := &core.LogEvent{Properties: map[string]any{"Audit": true}}
		if !auditPred(auditEvent) {
			t.Error("AuditEvents should match audit events")
		}
		
		criticalEvent := &core.LogEvent{
			Level:      core.ErrorLevel,
			Properties: map[string]any{"Critical": true},
		}
		if !criticalPred(criticalEvent) {
			t.Error("CriticalAlerts should match critical errors")
		}
		
		prodEvent := &core.LogEvent{
			Properties: map[string]any{"Environment": "production"},
		}
		if !prodPred(prodEvent) {
			t.Error("ProductionOnly should match production events")
		}
	})
}