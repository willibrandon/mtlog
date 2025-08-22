package sinks

import (
	"fmt"
	"sync"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

func TestRouterSink(t *testing.T) {
	t.Run("FirstMatch mode stops at first matching route", func(t *testing.T) {
		errorSink := NewMemorySink()
		warningSink := NewMemorySink()
		infoSink := NewMemorySink()
		
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      errorSink,
			},
			Route{
				Name:      "warnings",
				Predicate: LevelPredicate(core.WarningLevel),
				Sink:      warningSink,
			},
			Route{
				Name:      "info",
				Predicate: LevelPredicate(core.InformationLevel),
				Sink:      infoSink,
			},
		)
		
		// Emit an error (matches first route)
		router.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate:    "Error message",
			Properties: make(map[string]any),
		})
		
		// Emit a warning (matches second route)
		router.Emit(&core.LogEvent{
			Level:      core.WarningLevel,
			MessageTemplate:    "Warning message",
			Properties: make(map[string]any),
		})
		
		// Check that error went to error sink only
		if len(errorSink.Events()) != 1 {
			t.Errorf("Expected 1 error event, got %d", len(errorSink.Events()))
		}
		if len(warningSink.Events()) != 1 {
			t.Errorf("Expected 1 warning event, got %d", len(warningSink.Events()))
		}
		if len(infoSink.Events()) != 0 {
			t.Errorf("Expected 0 info events, got %d", len(infoSink.Events()))
		}
	})
	
	t.Run("AllMatch mode sends to all matching routes", func(t *testing.T) {
		alertSink := NewMemorySink()
		errorSink := NewMemorySink()
		auditSink := NewMemorySink()
		
		router := NewRouterSink(AllMatch,
			Route{
				Name:      "alerts",
				Predicate: PropertyPredicate("Alert"),
				Sink:      alertSink,
			},
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      errorSink,
			},
			Route{
				Name:      "audit",
				Predicate: PropertyPredicate("Audit"),
				Sink:      auditSink,
			},
		)
		
		// Emit an error with alert and audit properties
		router.Emit(&core.LogEvent{
			Level:   core.ErrorLevel,
			MessageTemplate: "Critical error",
			Properties: map[string]any{
				"Alert": true,
				"Audit": true,
			},
		})
		
		// Should go to all three sinks
		if len(alertSink.Events()) != 1 {
			t.Errorf("Expected 1 alert event, got %d", len(alertSink.Events()))
		}
		if len(errorSink.Events()) != 1 {
			t.Errorf("Expected 1 error event, got %d", len(errorSink.Events()))
		}
		if len(auditSink.Events()) != 1 {
			t.Errorf("Expected 1 audit event, got %d", len(auditSink.Events()))
		}
	})
	
	t.Run("default sink receives non-matching events", func(t *testing.T) {
		errorSink := NewMemorySink()
		defaultSink := NewMemorySink()
		
		router := NewRouterSinkWithDefault(FirstMatch, defaultSink,
			Route{
				Name:      "errors",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      errorSink,
			},
		)
		
		// Emit an info event (doesn't match route)
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Info message",
			Properties: make(map[string]any),
		})
		
		// Should go to default sink
		if len(defaultSink.Events()) != 1 {
			t.Errorf("Expected 1 default event, got %d", len(defaultSink.Events()))
		}
		if len(errorSink.Events()) != 0 {
			t.Errorf("Expected 0 error events, got %d", len(errorSink.Events()))
		}
	})
	
	t.Run("handles nil event", func(t *testing.T) {
		sink := NewMemorySink()
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "all",
				Predicate: func(e *core.LogEvent) bool { return true },
				Sink:      sink,
			},
		)
		
		// Should not panic
		router.Emit(nil)
		
		if len(sink.Events()) != 0 {
			t.Error("Expected no events for nil input")
		}
	})
	
	t.Run("handles predicate panic", func(t *testing.T) {
		goodSink := NewMemorySink()
		badSink := NewMemorySink()
		
		router := NewRouterSink(AllMatch,
			Route{
				Name: "panicking",
				Predicate: func(e *core.LogEvent) bool {
					panic("predicate error")
				},
				Sink: badSink,
			},
			Route{
				Name:      "good",
				Predicate: func(e *core.LogEvent) bool { return true },
				Sink:      goodSink,
			},
		)
		
		// Should not panic, good route should still work
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Test",
			Properties: make(map[string]any),
		})
		
		if len(badSink.Events()) != 0 {
			t.Error("Panicking route should not receive events")
		}
		if len(goodSink.Events()) != 1 {
			t.Error("Good route should still receive events")
		}
	})
	
	t.Run("runtime route management", func(t *testing.T) {
		sink1 := NewMemorySink()
		sink2 := NewMemorySink()
		
		router := NewRouterSink(FirstMatch)
		
		// Add first route
		router.AddRoute(Route{
			Name:      "route1",
			Predicate: PropertyPredicate("Type1"),
			Sink:      sink1,
		})
		
		// Test first route
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Type1 event",
			Properties: map[string]any{"Type1": true},
		})
		
		if len(sink1.Events()) != 1 {
			t.Error("First route should work after adding")
		}
		
		// Add second route
		router.AddRoute(Route{
			Name:      "route2",
			Predicate: PropertyPredicate("Type2"),
			Sink:      sink2,
		})
		
		// Test second route
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Type2 event",
			Properties: map[string]any{"Type2": true},
		})
		
		if len(sink2.Events()) != 1 {
			t.Error("Second route should work after adding")
		}
		
		// Remove first route
		removed := router.RemoveRoute("route1")
		if !removed {
			t.Error("Should have removed route1")
		}
		
		// First route should no longer work
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Another Type1 event",
			Properties: map[string]any{"Type1": true},
		})
		
		if len(sink1.Events()) != 1 {
			t.Error("Removed route should not receive new events")
		}
	})
	
	t.Run("Close closes all sinks", func(t *testing.T) {
		sink1 := &mockCloseableSink{}
		sink2 := &mockCloseableSink{}
		defaultSink := &mockCloseableSink{}
		
		router := NewRouterSinkWithDefault(FirstMatch, defaultSink,
			Route{Name: "route1", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink1},
			Route{Name: "route2", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink2},
		)
		
		err := router.Close()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !sink1.closed || !sink2.closed || !defaultSink.closed {
			t.Error("All sinks should be closed")
		}
	})
}

func TestRouteBuilder(t *testing.T) {
	t.Run("builds route with fluent API", func(t *testing.T) {
		sink := NewMemorySink()
		
		route := NewRoute("test").
			When(LevelPredicate(core.ErrorLevel)).
			To(sink)
		
		if route.Name != "test" {
			t.Errorf("Expected name 'test', got %s", route.Name)
		}
		if route.Sink != sink {
			t.Error("Sink not set correctly")
		}
		if route.Predicate == nil {
			t.Error("Predicate not set")
		}
	})
	
	t.Run("panics without predicate", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for missing predicate")
			}
		}()
		
		NewRoute("test").To(NewMemorySink())
	})
	
	t.Run("panics without sink", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for missing sink")
			}
		}()
		
		NewRoute("test").
			When(func(e *core.LogEvent) bool { return true }).
			To(nil)
	})
}

func TestCommonRoutes(t *testing.T) {
	t.Run("ErrorRoute", func(t *testing.T) {
		sink := NewMemorySink()
		route := ErrorRoute("errors", sink)
		
		router := NewRouterSink(FirstMatch, route)
		
		// Should route errors
		router.Emit(&core.LogEvent{
			Level:      core.ErrorLevel,
			MessageTemplate:    "Error",
			Properties: make(map[string]any),
		})
		
		// Should not route info
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Info",
			Properties: make(map[string]any),
		})
		
		if len(sink.Events()) != 1 {
			t.Errorf("Expected 1 error event, got %d", len(sink.Events()))
		}
	})
	
	t.Run("AuditRoute", func(t *testing.T) {
		sink := NewMemorySink()
		route := AuditRoute("audit", sink)
		
		router := NewRouterSink(FirstMatch, route)
		
		// Should route audit events
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Audit event",
			Properties: map[string]any{"Audit": true},
		})
		
		// Should not route non-audit
		router.Emit(&core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Regular event",
			Properties: make(map[string]any),
		})
		
		if len(sink.Events()) != 1 {
			t.Errorf("Expected 1 audit event, got %d", len(sink.Events()))
		}
	})
}

func TestRouterSinkConcurrency(t *testing.T) {
	t.Run("concurrent routing", func(t *testing.T) {
		sink1 := NewMemorySink()
		sink2 := NewMemorySink()
		
		router := NewRouterSink(AllMatch,
			Route{
				Name:      "evens",
				Predicate: func(e *core.LogEvent) bool {
					if num, ok := e.Properties["Number"].(int); ok {
						return num%2 == 0
					}
					return false
				},
				Sink: sink1,
			},
			Route{
				Name:      "odds",
				Predicate: func(e *core.LogEvent) bool {
					if num, ok := e.Properties["Number"].(int); ok {
						return num%2 == 1
					}
					return false
				},
				Sink: sink2,
			},
		)
		
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(num int) {
				defer wg.Done()
				router.Emit(&core.LogEvent{
					Level:      core.InformationLevel,
					MessageTemplate:    "Test",
					Properties: map[string]any{"Number": num},
				})
			}(i)
		}
		
		wg.Wait()
		
		// Should have 50 even and 50 odd
		if len(sink1.Events()) != 50 {
			t.Errorf("Expected 50 even events, got %d", len(sink1.Events()))
		}
		if len(sink2.Events()) != 50 {
			t.Errorf("Expected 50 odd events, got %d", len(sink2.Events()))
		}
	})
	
	t.Run("concurrent route management", func(t *testing.T) {
		router := NewRouterSink(FirstMatch)
		
		var wg sync.WaitGroup
		
		// Add routes concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				router.AddRoute(Route{
					Name:      fmt.Sprintf("route%d", n),
					Predicate: func(e *core.LogEvent) bool { return false },
					Sink:      NewMemorySink(),
				})
			}(i)
		}
		
		// Remove routes concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				time.Sleep(time.Millisecond) // Let adds happen first
				router.RemoveRoute(fmt.Sprintf("route%d", n))
			}(i)
		}
		
		// Emit events concurrently
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				router.Emit(&core.LogEvent{
					Level:      core.InformationLevel,
					MessageTemplate:    "Test",
					Properties: make(map[string]any),
				})
			}()
		}
		
		wg.Wait()
		// Should complete without race conditions or panics
	})
}

func BenchmarkRouterSink(b *testing.B) {
	b.Run("FirstMatch with 3 routes", func(b *testing.B) {
		sink := NewMemorySink()
		router := NewRouterSink(FirstMatch,
			Route{Name: "route1", Predicate: func(e *core.LogEvent) bool { return false }, Sink: sink},
			Route{Name: "route2", Predicate: func(e *core.LogEvent) bool { return false }, Sink: sink},
			Route{Name: "route3", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink},
		)
		
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Test",
			Properties: make(map[string]any),
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			router.Emit(event)
		}
	})
	
	b.Run("AllMatch with 3 routes", func(b *testing.B) {
		sink := NewMemorySink()
		router := NewRouterSink(AllMatch,
			Route{Name: "route1", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink},
			Route{Name: "route2", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink},
			Route{Name: "route3", Predicate: func(e *core.LogEvent) bool { return true }, Sink: sink},
		)
		
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			MessageTemplate:    "Test",
			Properties: make(map[string]any),
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			router.Emit(event)
		}
	})
}