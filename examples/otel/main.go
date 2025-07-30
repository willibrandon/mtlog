// Package main demonstrates using OTEL-style dotted property names with mtlog.
package main

import (
	"time"
	
	"github.com/willibrandon/mtlog"
)

func main() {
	// Create a logger with console output
	log := mtlog.New(
		mtlog.WithConsole(),
	)
	
	// Log with OTEL-style dotted properties
	log.Information("HTTP request {http.method} to {http.url} returned {http.status_code} in {http.duration.ms:F2}ms",
		"GET", "/api/users", 200, 123.456)
	
	// Service metadata using OTEL conventions
	log.Information("Service {service.name} version {service.version} started in {service.namespace}",
		"user-api", "2.1.0", "production")
	
	// Database operations with OTEL conventions
	log.Information("Database {db.system} query {db.operation} on {db.name} took {db.duration.ms:F1}ms",
		"postgresql", "SELECT", "users", 45.8)
	
	// Trace context with dots
	traceLog := log.ForContext("trace.id", "abc123").
		ForContext("trace.span_id", "span456")
	
	traceLog.Information("Processing request in span")
	
	// Mix traditional PascalCase with OTEL-style
	log.Information("User {UserId} made {http.method} request to {http.url} from {client.ip}",
		123, "POST", "/api/orders", "192.168.1.100")
	
	// Error tracking with OTEL conventions
	log.Error("Failed to connect to {db.system} at {db.connection_string}: {error.message}",
		"redis", "redis://localhost:6379", "connection refused")
	
	// Metrics-style properties
	log.Information("Cache hit rate: {cache.hit_rate:P1}, Items: {cache.item_count}",
		0.856, 1542)
	
	time.Sleep(100 * time.Millisecond) // Give console time to flush
}

// Example output:
// 2025-07-29 10:15:23 [INF] HTTP request GET to /api/users returned 200 in 123.46ms
// 2025-07-29 10:15:23 [INF] Service user-api version 2.1.0 started in production
// 2025-07-29 10:15:23 [INF] Database postgresql query SELECT on users took 45.8ms
// 2025-07-29 10:15:23 [INF] Processing request in span (trace.id: abc123, trace.span_id: span456)
// 2025-07-29 10:15:23 [INF] User 123 made POST request to /api/orders from 192.168.1.100
// 2025-07-29 10:15:23 [ERR] Failed to connect to redis at redis://localhost:6379: connection refused
// 2025-07-29 10:15:23 [INF] Cache hit rate: 85.6%, Items: 1542