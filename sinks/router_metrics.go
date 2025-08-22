package sinks

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// RouterMetricsExporter exports router statistics in Prometheus format.
type RouterMetricsExporter struct {
	routers map[string]*RouterSink
	mu      sync.RWMutex
}

// NewRouterMetricsExporter creates a new metrics exporter.
func NewRouterMetricsExporter() *RouterMetricsExporter {
	return &RouterMetricsExporter{
		routers: make(map[string]*RouterSink),
	}
}

// RegisterRouter registers a router for metrics export.
func (e *RouterMetricsExporter) RegisterRouter(name string, router *RouterSink) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.routers[name] = router
}

// UnregisterRouter removes a router from metrics export.
func (e *RouterMetricsExporter) UnregisterRouter(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.routers, name)
}

// ServeHTTP implements http.Handler for Prometheus scraping.
func (e *RouterMetricsExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var metrics []string
	
	// Add metric help and type information
	metrics = append(metrics, 
		"# HELP mtlog_router_events_total Total number of events processed by router",
		"# TYPE mtlog_router_events_total counter",
		"# HELP mtlog_router_route_hits_total Number of events matched by each route",
		"# TYPE mtlog_router_route_hits_total counter",
		"# HELP mtlog_router_default_hits_total Number of events sent to default sink",
		"# TYPE mtlog_router_default_hits_total counter",
		"# HELP mtlog_router_dropped_events_total Number of events dropped (no match, no default)",
		"# TYPE mtlog_router_dropped_events_total counter",
	)
	
	// Export metrics for each registered router
	for routerName, router := range e.routers {
		stats := router.GetStats()
		
		// Total events
		metrics = append(metrics, fmt.Sprintf(
			`mtlog_router_events_total{router="%s"} %d`,
			routerName, stats.TotalEvents,
		))
		
		// Route hits
		for routeName, hits := range stats.RouteHits {
			metrics = append(metrics, fmt.Sprintf(
				`mtlog_router_route_hits_total{router="%s",route="%s"} %d`,
				routerName, routeName, hits,
			))
		}
		
		// Default sink hits
		metrics = append(metrics, fmt.Sprintf(
			`mtlog_router_default_hits_total{router="%s"} %d`,
			routerName, stats.DefaultHits,
		))
		
		// Dropped events
		metrics = append(metrics, fmt.Sprintf(
			`mtlog_router_dropped_events_total{router="%s"} %d`,
			routerName, stats.DroppedEvents,
		))
	}
	
	// Write all metrics
	fmt.Fprintln(w, strings.Join(metrics, "\n"))
}

// StartMetricsServer starts an HTTP server for Prometheus metrics.
func StartMetricsServer(addr string, exporter *RouterMetricsExporter) error {
	http.Handle("/metrics", exporter)
	return http.ListenAndServe(addr, nil)
}