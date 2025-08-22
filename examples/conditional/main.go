package main

import (
	"fmt"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create separate sinks for different purposes
	consoleSink := sinks.NewConsoleSink()
	
	// Create an alerting sink that only processes critical errors
	alertSink, _ := sinks.NewFileSink("alerts.log")
	
	// Create a conditional sink that routes critical errors to the alert sink
	criticalAlertSink := sinks.NewNamedConditionalSink(
		"critical-alerts",
		func(event *core.LogEvent) bool {
			// Send to alerts if:
			// 1. Level is Error or Fatal
			// 2. Has an "Alert" property
			return event.Level >= core.ErrorLevel && 
				   event.Properties["Alert"] != nil
		},
		alertSink,
	)
	
	// Create an audit file sink
	auditFileSink, _ := sinks.NewFileSink("audit.log")
	
	// Create a conditional sink for audit events
	auditSink := sinks.NewNamedConditionalSink(
		"audit-events",
		sinks.PropertyPredicate("Audit"),
		auditFileSink,
	)
	
	// Create a performance monitoring sink that only logs slow operations
	perfFileSink, _ := sinks.NewFileSink("performance.log")
	
	perfSink := sinks.NewNamedConditionalSink(
		"slow-operations",
		func(event *core.LogEvent) bool {
			if elapsed, ok := event.Properties["Elapsed"].(time.Duration); ok {
				return elapsed > 100*time.Millisecond
			}
			return false
		},
		perfFileSink,
	)
	
	// Create a logger with all conditional sinks
	log := mtlog.New(
		mtlog.WithSink(consoleSink),           // All events go to console
		mtlog.WithSink(criticalAlertSink),     // Critical alerts go to alerts.log
		mtlog.WithSink(auditSink),             // Audit events go to audit.log
		mtlog.WithSink(perfSink),              // Slow operations go to performance.log
		mtlog.WithProperty("Application", "ConditionalExample"),
	)
	defer log.Close()
	
	// Normal operation
	log.Information("Application started")
	
	// User action (with audit)
	log.With("Audit", true).
		Information("User {UserId} logged in with {Action}", 123, "Login")
	
	// Simulated operation timing
	start := time.Now()
	time.Sleep(50 * time.Millisecond) // Fast operation
	elapsed := time.Since(start)
	log.With("Elapsed", elapsed).
		Information("Database query {Query} completed", "SELECT * FROM users")
	
	// Slow operation
	start = time.Now()
	time.Sleep(150 * time.Millisecond) // Slow operation
	elapsed = time.Since(start)
	log.With("Elapsed", elapsed).
		Warning("Database query {Query} slow", "SELECT * FROM orders JOIN ...")
	
	// Regular error (not critical)
	log.Error("Failed to load optional config")
	
	// Critical error with alert
	log.With("Alert", true).
		Error("Database connection lost to {Database} after {RetryCount} retries", "primary", 3)
	
	// Fatal error with alert
	log.With("Alert", true).
		Fatal("System out of memory: {AvailableMemory} available, {RequiredMemory} required", "12MB", "1GB")
	
	// Security audit event
	log.With("Audit", true).
		Warning("Unauthorized access attempt by {UserId} to {Resource} from {IPAddress}", 456, "/admin/users", "192.168.1.100")
	
	fmt.Println("\nCheck the following files:")
	fmt.Println("- alerts.log: Contains only critical errors with Alert property")
	fmt.Println("- audit.log: Contains only events with Audit property") 
	fmt.Println("- performance.log: Contains only slow operations (>100ms)")
}