package sinks

import (
	"testing"
	
	"github.com/willibrandon/mtlog/core"
)

func TestRouterStats(t *testing.T) {
	t.Run("tracks route hits", func(t *testing.T) {
		memorySink := NewMemorySink()
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      memorySink,
			},
			Route{
				Name:      "warnings",
				Predicate: LevelPredicate(core.WarningLevel),
				Sink:      memorySink,
			},
		)
		
		// Emit some events
		router.Emit(&core.LogEvent{Level: core.ErrorLevel})
		router.Emit(&core.LogEvent{Level: core.ErrorLevel})
		router.Emit(&core.LogEvent{Level: core.WarningLevel})
		router.Emit(&core.LogEvent{Level: core.InformationLevel}) // No match
		
		stats := router.GetStats()
		
		if stats.TotalEvents != 4 {
			t.Errorf("Expected 4 total events, got %d", stats.TotalEvents)
		}
		if stats.RouteHits["errors"] != 2 {
			t.Errorf("Expected 2 error hits, got %d", stats.RouteHits["errors"])
		}
		if stats.RouteHits["warnings"] != 1 {
			t.Errorf("Expected 1 warning hit, got %d", stats.RouteHits["warnings"])
		}
		if stats.DroppedEvents != 1 {
			t.Errorf("Expected 1 dropped event, got %d", stats.DroppedEvents)
		}
	})
	
	t.Run("tracks default sink hits", func(t *testing.T) {
		defaultSink := NewMemorySink()
		router := NewRouterSinkWithDefault(FirstMatch, defaultSink,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
		)
		
		router.Emit(&core.LogEvent{Level: core.ErrorLevel})
		router.Emit(&core.LogEvent{Level: core.InformationLevel}) // Goes to default
		router.Emit(&core.LogEvent{Level: core.DebugLevel})       // Goes to default
		
		stats := router.GetStats()
		
		if stats.DefaultHits != 2 {
			t.Errorf("Expected 2 default hits, got %d", stats.DefaultHits)
		}
		if stats.RouteHits["errors"] != 1 {
			t.Errorf("Expected 1 error hit, got %d", stats.RouteHits["errors"])
		}
	})
	
	t.Run("tracks AllMatch mode correctly", func(t *testing.T) {
		router := NewRouterSink(AllMatch,
			Route{
				Name:      "all",
				Predicate: func(e *core.LogEvent) bool { return true },
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
		)
		
		router.Emit(&core.LogEvent{Level: core.ErrorLevel})
		router.Emit(&core.LogEvent{Level: core.InformationLevel})
		
		stats := router.GetStats()
		
		// Error event should hit both routes
		if stats.RouteHits["all"] != 2 {
			t.Errorf("Expected 2 hits for 'all' route, got %d", stats.RouteHits["all"])
		}
		if stats.RouteHits["errors"] != 1 {
			t.Errorf("Expected 1 hit for 'errors' route, got %d", stats.RouteHits["errors"])
		}
	})
	
	t.Run("reset stats", func(t *testing.T) {
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "test",
				Predicate: func(e *core.LogEvent) bool { return true },
				Sink:      NewMemorySink(),
			},
		)
		
		router.Emit(&core.LogEvent{})
		router.Emit(&core.LogEvent{})
		
		stats := router.GetStats()
		if stats.TotalEvents != 2 {
			t.Errorf("Expected 2 events before reset, got %d", stats.TotalEvents)
		}
		
		router.ResetStats()
		
		stats = router.GetStats()
		if stats.TotalEvents != 0 {
			t.Errorf("Expected 0 events after reset, got %d", stats.TotalEvents)
		}
		if stats.RouteHits["test"] != 0 {
			t.Errorf("Expected 0 route hits after reset, got %d", stats.RouteHits["test"])
		}
	})
}

func TestRouterTestEvent(t *testing.T) {
	t.Run("FirstMatch mode", func(t *testing.T) {
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "all-errors", // Would also match but FirstMatch stops
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "warnings",
				Predicate: LevelPredicate(core.WarningLevel),
				Sink:      NewMemorySink(),
			},
		)
		
		errorEvent := &core.LogEvent{Level: core.ErrorLevel}
		matches := router.TestEvent(errorEvent)
		
		if len(matches) != 1 {
			t.Errorf("Expected 1 match in FirstMatch mode, got %d", len(matches))
		}
		if matches[0] != "errors" {
			t.Errorf("Expected 'errors' route, got %s", matches[0])
		}
	})
	
	t.Run("AllMatch mode", func(t *testing.T) {
		router := NewRouterSink(AllMatch,
			Route{
				Name:      "audit",
				Predicate: PropertyPredicate("Audit"),
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
			Route{
				Name:      "critical",
				Predicate: PropertyPredicate("Critical"),
				Sink:      NewMemorySink(),
			},
		)
		
		event := &core.LogEvent{
			Level: core.ErrorLevel,
			Properties: map[string]any{
				"Audit":    true,
				"Critical": true,
			},
		}
		
		matches := router.TestEvent(event)
		
		if len(matches) != 3 {
			t.Errorf("Expected 3 matches in AllMatch mode, got %d", len(matches))
		}
		
		// Check all expected routes are present
		expectedRoutes := map[string]bool{
			"audit":    false,
			"errors":   false,
			"critical": false,
		}
		for _, match := range matches {
			expectedRoutes[match] = true
		}
		for route, found := range expectedRoutes {
			if !found {
				t.Errorf("Expected route %s was not matched", route)
			}
		}
	})
	
	t.Run("default sink match", func(t *testing.T) {
		router := NewRouterSinkWithDefault(FirstMatch, NewMemorySink(),
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
		)
		
		infoEvent := &core.LogEvent{Level: core.InformationLevel}
		matches := router.TestEvent(infoEvent)
		
		if len(matches) != 1 {
			t.Errorf("Expected 1 match (default), got %d", len(matches))
		}
		if matches[0] != "<default>" {
			t.Errorf("Expected '<default>', got %s", matches[0])
		}
	})
	
	t.Run("no matches", func(t *testing.T) {
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      NewMemorySink(),
			},
		)
		
		infoEvent := &core.LogEvent{Level: core.InformationLevel}
		matches := router.TestEvent(infoEvent)
		
		if len(matches) != 0 {
			t.Errorf("Expected no matches, got %d", len(matches))
		}
	})
}