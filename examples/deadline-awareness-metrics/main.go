// Package main demonstrates integrating mtlog deadline awareness with metrics systems.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// MockMetrics simulates a metrics client (Prometheus, StatsD, etc)
type MockMetrics struct {
	gauges map[string]float64
	mu     sync.Mutex
}

func (m *MockMetrics) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = make(map[string]float64)
	}
	m.gauges[name] = value
	fmt.Printf("[METRICS] %s = %.2f\n", name, value)
}

func main() {
	fmt.Println("=== Deadline Awareness Metrics Integration ===")
	fmt.Println()

	metrics := &MockMetrics{}

	// Create logger with deadline awareness
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithContextDeadlineWarning(50*time.Millisecond),
	)

	// Simulate operations
	simulateOperations(logger)

	// Export metrics
	exportDeadlineMetrics(logger, metrics)

	// Example: Prometheus integration
	prometheusExample(logger)

	// Example: Custom middleware with metrics
	middlewareExample(logger, metrics)
}

func simulateOperations(logger core.Logger) {
	fmt.Println("Simulating operations to populate cache...")
	// Create multiple contexts to populate cache
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		logger.InfoContext(ctx, "Operation {Id} started", i)
		time.Sleep(60 * time.Millisecond)
		logger.InfoContext(ctx, "Operation {Id} approaching deadline", i)
		cancel()
	}
	fmt.Println()
}

func exportDeadlineMetrics(logger core.Logger, metrics *MockMetrics) {
	fmt.Println("Exporting deadline metrics:")

	if statsInterface := logger.DeadlineStats(); statsInterface != nil {
		stats := statsInterface.(core.DeadlineStats)

		// Export to metrics system
		metrics.SetGauge("mtlog_deadline_cache_size", float64(stats.CacheSize))
		metrics.SetGauge("mtlog_deadline_cache_capacity", float64(stats.CacheCapacity))
		metrics.SetGauge("mtlog_deadline_cache_utilization",
			float64(stats.CacheSize)/float64(stats.CacheCapacity)*100)
		metrics.SetGauge("mtlog_deadline_first_warnings", float64(stats.FirstWarningCount))
		metrics.SetGauge("mtlog_deadline_cache_ttl_seconds", stats.CacheTTL.Seconds())
	}
	fmt.Println()
}

func prometheusExample(logger core.Logger) {
	fmt.Println("Prometheus metrics example:")
	fmt.Println("```")

	if statsInterface := logger.DeadlineStats(); statsInterface != nil {
		stats := statsInterface.(core.DeadlineStats)

		// Prometheus format
		fmt.Printf("# HELP mtlog_deadline_cache_size Current number of contexts in deadline cache\n")
		fmt.Printf("# TYPE mtlog_deadline_cache_size gauge\n")
		fmt.Printf("mtlog_deadline_cache_size %d\n", stats.CacheSize)

		fmt.Printf("# HELP mtlog_deadline_warnings_total Total first warnings issued\n")
		fmt.Printf("# TYPE mtlog_deadline_warnings_total counter\n")
		fmt.Printf("mtlog_deadline_warnings_total %d\n", stats.FirstWarningCount)
		
		fmt.Printf("# HELP mtlog_deadline_cache_utilization_percent Cache utilization percentage\n")
		fmt.Printf("# TYPE mtlog_deadline_cache_utilization_percent gauge\n")
		fmt.Printf("mtlog_deadline_cache_utilization_percent %.2f\n", 
			float64(stats.CacheSize)/float64(stats.CacheCapacity)*100)
	}
	fmt.Println("```")
	fmt.Println()
}

func middlewareExample(logger core.Logger, metrics *MockMetrics) {
	fmt.Println("Middleware with metrics:")

	// Track deadline warnings per endpoint
	deadlineWarnings := make(map[string]int)

	// Simulate requests
	endpoints := []string{"/api/users", "/api/orders", "/api/products"}

	for _, endpoint := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		// Log and check if warning triggered
		logger.InfoContext(ctx, "Request to {Endpoint}", endpoint)
		time.Sleep(60 * time.Millisecond)

		// This will trigger a warning
		logger.InfoContext(ctx, "Still processing {Endpoint}", endpoint)
		deadlineWarnings[endpoint]++

		cancel()
	}

	// Export per-endpoint metrics
	fmt.Println("\nPer-endpoint deadline warnings:")
	for endpoint, count := range deadlineWarnings {
		metricName := fmt.Sprintf("http_deadline_warnings{endpoint=\"%s\"}", endpoint)
		metrics.SetGauge(metricName, float64(count))
	}
	fmt.Println()
}