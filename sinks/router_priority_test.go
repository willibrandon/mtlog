package sinks

import (
	"testing"
	
	"github.com/willibrandon/mtlog/core"
)

func TestRoutePriority(t *testing.T) {
	t.Run("routes execute in priority order", func(t *testing.T) {
		var executionOrder []string
		
		// Create routes with different priorities
		routes := []Route{
			{
				Name:     "low-priority",
				Priority: 200,
				Predicate: func(e *core.LogEvent) bool { 
					executionOrder = append(executionOrder, "low")
					return false // Don't actually match
				},
				Sink: NewMemorySink(),
			},
			{
				Name:     "high-priority",
				Priority: 10,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "high")
					return true // Match and stop (FirstMatch mode)
				},
				Sink: NewMemorySink(),
			},
			{
				Name:     "medium-priority",
				Priority: 100,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "medium")
					return false
				},
				Sink: NewMemorySink(),
			},
		}
		
		router := NewRouterSink(FirstMatch, routes...)
		router.Emit(&core.LogEvent{Level: core.InformationLevel})
		
		// Should execute in priority order: high (10), medium (100), low (200)
		// But stop at high since it matches
		if len(executionOrder) != 1 || executionOrder[0] != "high" {
			t.Errorf("Expected high priority route to execute first and stop, got %v", executionOrder)
		}
	})
	
	t.Run("AllMatch mode respects priority order", func(t *testing.T) {
		var executionOrder []string
		
		routes := []Route{
			{
				Name:     "third",
				Priority: 30,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "third")
					return true
				},
				Sink: NewMemorySink(),
			},
			{
				Name:     "first",
				Priority: 10,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "first")
					return true
				},
				Sink: NewMemorySink(),
			},
			{
				Name:     "second",
				Priority: 20,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "second")
					return true
				},
				Sink: NewMemorySink(),
			},
		}
		
		router := NewRouterSink(AllMatch, routes...)
		router.Emit(&core.LogEvent{Level: core.InformationLevel})
		
		// Should execute all in priority order
		expected := []string{"first", "second", "third"}
		if len(executionOrder) != 3 {
			t.Fatalf("Expected 3 executions, got %d", len(executionOrder))
		}
		for i, exp := range expected {
			if executionOrder[i] != exp {
				t.Errorf("Expected %s at position %d, got %s", exp, i, executionOrder[i])
			}
		}
	})
	
	t.Run("RouteBuilder WithPriority", func(t *testing.T) {
		route := NewRoute("test").
			WithPriority(50).
			When(LevelPredicate(core.ErrorLevel)).
			To(NewMemorySink())
		
		if route.Priority != 50 {
			t.Errorf("Expected priority 50, got %d", route.Priority)
		}
	})
}

func TestRouteGroups(t *testing.T) {
	t.Run("AddRouteGroup prefixes route names", func(t *testing.T) {
		router := NewRouterSink(FirstMatch)
		
		securityGroup := NewRouteGroup("security",
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "audit",
				Predicate: PropertyPredicate("Audit"),
				Sink:      NewMemorySink(),
			},
		)
		
		router.AddRouteGroup(securityGroup)
		
		// Test that routes are accessible with prefixed names
		errorEvent := &core.LogEvent{Level: core.ErrorLevel}
		matches := router.TestEvent(errorEvent)
		
		if len(matches) != 1 || matches[0] != "security.errors" {
			t.Errorf("Expected route 'security.errors', got %v", matches)
		}
		
		// Check stats use prefixed names
		stats := router.GetStats()
		if _, exists := stats.RouteHits["security.errors"]; !exists {
			t.Error("Expected 'security.errors' in stats")
		}
		if _, exists := stats.RouteHits["security.audit"]; !exists {
			t.Error("Expected 'security.audit' in stats")
		}
	})
	
	t.Run("RemoveRouteGroup removes all group routes", func(t *testing.T) {
		router := NewRouterSink(FirstMatch)
		
		// Add two groups
		group1 := NewRouteGroup("group1",
			Route{Name: "route1", Predicate: LevelPredicate(core.ErrorLevel), Sink: NewMemorySink()},
			Route{Name: "route2", Predicate: LevelPredicate(core.WarningLevel), Sink: NewMemorySink()},
		)
		group2 := NewRouteGroup("group2",
			Route{Name: "route1", Predicate: func(e *core.LogEvent) bool { return e.Level == core.InformationLevel }, Sink: NewMemorySink()},
		)
		
		router.AddRouteGroup(group1)
		router.AddRouteGroup(group2)
		
		// Remove group1
		removed := router.RemoveRouteGroup("group1")
		if removed != 2 {
			t.Errorf("Expected to remove 2 routes, removed %d", removed)
		}
		
		// Verify group1 routes are gone
		errorEvent := &core.LogEvent{Level: core.ErrorLevel}
		matches := router.TestEvent(errorEvent)
		if len(matches) != 0 {
			t.Errorf("Expected no matches for error event after removing group1, got %v", matches)
		}
		
		warningEvent := &core.LogEvent{Level: core.WarningLevel}
		matches = router.TestEvent(warningEvent)
		if len(matches) != 0 {
			t.Errorf("Expected no matches for warning event after removing group1, got %v", matches)
		}
		
		infoEvent := &core.LogEvent{Level: core.InformationLevel}
		matches = router.TestEvent(infoEvent)
		if len(matches) != 1 || matches[0] != "group2.route1" {
			t.Errorf("Expected group2.route1 to remain, got %v", matches)
		}
	})
	
	t.Run("route groups maintain priority order", func(t *testing.T) {
		var executionOrder []string
		
		router := NewRouterSink(AllMatch)
		
		highPriorityGroup := NewRouteGroup("high",
			Route{
				Name:     "route1",
				Priority: 10,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "high.route1")
					return true
				},
				Sink: NewMemorySink(),
			},
		)
		
		lowPriorityGroup := NewRouteGroup("low",
			Route{
				Name:     "route1",
				Priority: 100,
				Predicate: func(e *core.LogEvent) bool {
					executionOrder = append(executionOrder, "low.route1")
					return true
				},
				Sink: NewMemorySink(),
			},
		)
		
		// Add in reverse order to test sorting
		router.AddRouteGroup(lowPriorityGroup)
		router.AddRouteGroup(highPriorityGroup)
		
		router.Emit(&core.LogEvent{Level: core.InformationLevel})
		
		// Should execute in priority order
		if len(executionOrder) != 2 {
			t.Fatalf("Expected 2 executions, got %d", len(executionOrder))
		}
		if executionOrder[0] != "high.route1" {
			t.Errorf("Expected high priority group to execute first, got %s", executionOrder[0])
		}
		if executionOrder[1] != "low.route1" {
			t.Errorf("Expected low priority group to execute second, got %s", executionOrder[1])
		}
	})
}