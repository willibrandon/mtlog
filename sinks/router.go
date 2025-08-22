package sinks

import (
	"fmt"
	"sort"
	"sync"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// RoutingMode determines how the router processes matching routes.
type RoutingMode int

const (
	// FirstMatch stops at the first matching route (exclusive routing).
	FirstMatch RoutingMode = iota
	// AllMatch sends to all matching routes (broadcast routing).
	AllMatch
)

// Route represents a named routing rule with optional priority.
type Route struct {
	Name      string
	Priority  int // Lower values = higher priority (0 = highest)
	Predicate func(*core.LogEvent) bool
	Sink      core.LogEventSink
}

// RouterSink routes events to different sinks based on configured routes.
type RouterSink struct {
	routes      []Route
	mode        RoutingMode
	defaultSink core.LogEventSink // Optional fallback sink
	mu          sync.RWMutex      // Protects routes during runtime updates
	stats       *RouterStats      // Statistics tracking
}

// RouterStats tracks routing statistics.
type RouterStats struct {
	RouteHits    map[string]uint64 // Hit count per route
	DefaultHits  uint64            // Events sent to default sink
	TotalEvents  uint64            // Total events processed
	DroppedEvents uint64           // Events that matched no routes (no default sink)
	mu           sync.RWMutex
}

// NewRouterSink creates a new router sink with the specified routing mode.
func NewRouterSink(mode RoutingMode, routes ...Route) *RouterSink {
	stats := &RouterStats{
		RouteHits: make(map[string]uint64),
	}
	// Sort routes by priority (lower = higher priority)
	sortedRoutes := make([]Route, len(routes))
	copy(sortedRoutes, routes)
	sortRoutesByPriority(sortedRoutes)
	
	// Initialize route hit counters
	for _, route := range sortedRoutes {
		stats.RouteHits[route.Name] = 0
	}
	return &RouterSink{
		routes: sortedRoutes,
		mode:   mode,
		stats:  stats,
	}
}

// NewRouterSinkWithDefault creates a router with a default sink for non-matching events.
func NewRouterSinkWithDefault(mode RoutingMode, defaultSink core.LogEventSink, routes ...Route) *RouterSink {
	stats := &RouterStats{
		RouteHits: make(map[string]uint64),
	}
	// Sort routes by priority (lower = higher priority)
	sortedRoutes := make([]Route, len(routes))
	copy(sortedRoutes, routes)
	sortRoutesByPriority(sortedRoutes)
	
	// Initialize route hit counters
	for _, route := range sortedRoutes {
		stats.RouteHits[route.Name] = 0
	}
	return &RouterSink{
		routes:      sortedRoutes,
		mode:        mode,
		defaultSink: defaultSink,
		stats:       stats,
	}
}

// Emit routes the event according to the configured rules.
func (r *RouterSink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}
	
	// Increment total events counter
	if r.stats != nil {
		r.stats.mu.Lock()
		r.stats.TotalEvents++
		r.stats.mu.Unlock()
	}
	
	r.mu.RLock()
	routes := r.routes
	defaultSink := r.defaultSink
	mode := r.mode
	r.mu.RUnlock()
	
	matched := false
	
	for _, route := range routes {
		shouldRoute := false
		
		// Safely evaluate predicate
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					if selflog.IsEnabled() {
						selflog.Printf("[RouterSink] route '%s' predicate panic: %v", route.Name, rec)
					}
					shouldRoute = false
				}
			}()
			shouldRoute = route.Predicate(event)
		}()
		
		if shouldRoute {
			matched = true
			route.Sink.Emit(event)
			
			// Track route hit
			if r.stats != nil {
				r.stats.mu.Lock()
				r.stats.RouteHits[route.Name]++
				r.stats.mu.Unlock()
			}
			
			if mode == FirstMatch {
				return // Stop at first match
			}
			// Continue for AllMatch mode
		}
	}
	
	// Send to default sink if no routes matched
	if !matched {
		if defaultSink != nil {
			defaultSink.Emit(event)
			// Track default hit
			if r.stats != nil {
				r.stats.mu.Lock()
				r.stats.DefaultHits++
				r.stats.mu.Unlock()
			}
		} else {
			// Track dropped event
			if r.stats != nil {
				r.stats.mu.Lock()
				r.stats.DroppedEvents++
				r.stats.mu.Unlock()
			}
		}
	}
}

// Close closes all route sinks and the default sink if present.
func (r *RouterSink) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var firstErr error
	
	// Close all route sinks
	for _, route := range r.routes {
		if closer, ok := route.Sink.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				if selflog.IsEnabled() {
					selflog.Printf("[RouterSink] failed to close route '%s': %v", route.Name, err)
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	
	// Close default sink
	if r.defaultSink != nil {
		if closer, ok := r.defaultSink.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				if selflog.IsEnabled() {
					selflog.Printf("[RouterSink] failed to close default sink: %v", err)
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	
	return firstErr
}

// AddRoute adds a new route at runtime, maintaining priority order.
func (r *RouterSink) AddRoute(route Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
	sortRoutesByPriority(r.routes)
	
	// Initialize stats for new route
	if r.stats != nil {
		r.stats.mu.Lock()
		r.stats.RouteHits[route.Name] = 0
		r.stats.mu.Unlock()
	}
}

// RemoveRoute removes a route by name at runtime.
func (r *RouterSink) RemoveRoute(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for i, route := range r.routes {
		if route.Name == name {
			// Close the sink if possible
			if closer, ok := route.Sink.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					if selflog.IsEnabled() {
						selflog.Printf("[RouterSink] failed to close removed route '%s': %v", name, err)
					}
				}
			}
			
			// Remove from slice
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			
			// Remove from stats
			if r.stats != nil {
				r.stats.mu.Lock()
				delete(r.stats.RouteHits, name)
				r.stats.mu.Unlock()
			}
			
			return true
		}
	}
	return false
}

// GetStats returns a copy of the current routing statistics.
func (r *RouterSink) GetStats() RouterStats {
	if r.stats == nil {
		return RouterStats{}
	}
	
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()
	
	// Create a copy of the stats
	statsCopy := RouterStats{
		RouteHits:     make(map[string]uint64),
		DefaultHits:   r.stats.DefaultHits,
		TotalEvents:   r.stats.TotalEvents,
		DroppedEvents: r.stats.DroppedEvents,
	}
	
	for name, count := range r.stats.RouteHits {
		statsCopy.RouteHits[name] = count
	}
	
	return statsCopy
}

// ResetStats resets all routing statistics to zero.
func (r *RouterSink) ResetStats() {
	if r.stats == nil {
		return
	}
	
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	
	for name := range r.stats.RouteHits {
		r.stats.RouteHits[name] = 0
	}
	r.stats.DefaultHits = 0
	r.stats.TotalEvents = 0
	r.stats.DroppedEvents = 0
}

// TestEvent tests which routes an event would match without actually emitting it.
// Returns a slice of route names that would receive the event.
func (r *RouterSink) TestEvent(event *core.LogEvent) []string {
	if event == nil {
		return nil
	}
	
	r.mu.RLock()
	routes := r.routes
	mode := r.mode
	defaultSink := r.defaultSink
	r.mu.RUnlock()
	
	var matches []string
	
	for _, route := range routes {
		shouldRoute := false
		
		// Safely evaluate predicate
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					shouldRoute = false
				}
			}()
			shouldRoute = route.Predicate(event)
		}()
		
		if shouldRoute {
			matches = append(matches, route.Name)
			
			if mode == FirstMatch {
				break // Stop at first match
			}
			// Continue for AllMatch mode
		}
	}
	
	// Check if it would go to default sink
	if len(matches) == 0 && defaultSink != nil {
		matches = append(matches, "<default>")
	}
	
	return matches
}

// RouteBuilder provides a fluent API for building routes.
type RouteBuilder struct {
	name      string
	priority  int
	predicate func(*core.LogEvent) bool
}

// NewRoute starts building a new route with the given name.
func NewRoute(name string) *RouteBuilder {
	return &RouteBuilder{name: name, priority: 100} // Default priority
}

// WithPriority sets the priority for the route (lower = higher priority).
func (b *RouteBuilder) WithPriority(priority int) *RouteBuilder {
	b.priority = priority
	return b
}

// When sets the predicate for the route.
func (b *RouteBuilder) When(predicate func(*core.LogEvent) bool) *RouteBuilder {
	b.predicate = predicate
	return b
}

// To completes the route with the target sink.
func (b *RouteBuilder) To(sink core.LogEventSink) Route {
	if b.predicate == nil {
		panic(fmt.Sprintf("route '%s' requires a predicate", b.name))
	}
	if sink == nil {
		panic(fmt.Sprintf("route '%s' requires a sink", b.name))
	}
	
	return Route{
		Name:      b.name,
		Priority:  b.priority,
		Predicate: b.predicate,
		Sink:      sink,
	}
}

// Common route predicates

// ErrorRoute creates a route for error-level events.
func ErrorRoute(name string, sink core.LogEventSink) Route {
	return NewRoute(name).
		When(LevelPredicate(core.ErrorLevel)).
		To(sink)
}

// AuditRoute creates a route for events with an "Audit" property.
func AuditRoute(name string, sink core.LogEventSink) Route {
	return NewRoute(name).
		When(PropertyPredicate("Audit")).
		To(sink)
}

// MetricRoute creates a route for events with a "Metric" property.
func MetricRoute(name string, sink core.LogEventSink) Route {
	return NewRoute(name).
		When(PropertyPredicate("Metric")).
		To(sink)
}

// sortRoutesByPriority sorts routes by priority (lower values first).
// Routes with the same priority maintain their relative order.
func sortRoutesByPriority(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		return routes[i].Priority < routes[j].Priority
	})
}

// RouteGroup represents a named collection of related routes.
type RouteGroup struct {
	Name   string
	Routes []Route
}

// NewRouteGroup creates a new route group with the specified name and routes.
func NewRouteGroup(name string, routes ...Route) RouteGroup {
	return RouteGroup{
		Name:   name,
		Routes: routes,
	}
}

// AddRouteGroup adds all routes from a group to the router at runtime.
func (r *RouterSink) AddRouteGroup(group RouteGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Add all routes from the group
	for _, route := range group.Routes {
		// Prefix route name with group name for clarity
		route.Name = fmt.Sprintf("%s.%s", group.Name, route.Name)
		r.routes = append(r.routes, route)
		
		// Initialize stats for new route
		if r.stats != nil {
			r.stats.mu.Lock()
			r.stats.RouteHits[route.Name] = 0
			r.stats.mu.Unlock()
		}
	}
	
	// Re-sort all routes by priority
	sortRoutesByPriority(r.routes)
}

// RemoveRouteGroup removes all routes belonging to a group.
func (r *RouterSink) RemoveRouteGroup(groupName string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	prefix := groupName + "."
	removed := 0
	newRoutes := make([]Route, 0, len(r.routes))
	
	for _, route := range r.routes {
		if len(route.Name) > len(prefix) && route.Name[:len(prefix)] == prefix {
			// This route belongs to the group - close and skip it
			if closer, ok := route.Sink.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					if selflog.IsEnabled() {
						selflog.Printf("[RouterSink] failed to close group route '%s': %v", route.Name, err)
					}
				}
			}
			
			// Remove from stats
			if r.stats != nil {
				r.stats.mu.Lock()
				delete(r.stats.RouteHits, route.Name)
				r.stats.mu.Unlock()
			}
			
			removed++
		} else {
			// Keep this route
			newRoutes = append(newRoutes, route)
		}
	}
	
	r.routes = newRoutes
	return removed
}