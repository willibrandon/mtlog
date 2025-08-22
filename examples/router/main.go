package main

import (
	"fmt"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Example 1: FirstMatch routing - exclusive routing to different files by level
	fmt.Println("=== FirstMatch Routing Example ===")
	demoFirstMatchRouting()
	
	// Example 2: AllMatch routing - broadcast events to multiple destinations
	fmt.Println("\n=== AllMatch Routing Example ===")
	demoAllMatchRouting()
	
	// Example 3: Dynamic route management
	fmt.Println("\n=== Dynamic Route Management Example ===")
	demoDynamicRouting()
}

func demoFirstMatchRouting() {
	// Create separate files for different log levels
	errorSink, _ := sinks.NewFileSink("errors.log")
	warningSink, _ := sinks.NewFileSink("warnings.log")
	infoSink, _ := sinks.NewFileSink("info.log")
	
	// Create a router with FirstMatch mode (exclusive routing)
	router := sinks.NewRouterSink(
		sinks.FirstMatch,
		sinks.Route{
			Name:      "errors",
			Predicate: sinks.LevelPredicate(core.ErrorLevel),
			Sink:      errorSink,
		},
		sinks.Route{
			Name:      "warnings", 
			Predicate: sinks.LevelPredicate(core.WarningLevel),
			Sink:      warningSink,
		},
		sinks.Route{
			Name:      "info",
			Predicate: sinks.LevelPredicate(core.InformationLevel),
			Sink:      infoSink,
		},
	)
	
	log := mtlog.New(
		mtlog.WithSink(router),
		mtlog.WithSink(sinks.NewConsoleSink()), // Also log to console
	)
	defer log.Close()
	
	// These will go to different files based on level
	log.Information("Application started")
	log.Warning("Configuration file not found, using defaults")
	log.Error("Failed to connect to external service")
	log.Fatal("Critical system failure")
	
	fmt.Println("FirstMatch: Each event went to exactly one file based on level")
}

func demoAllMatchRouting() {
	// Create different sinks for different purposes
	metricsSink, _ := sinks.NewFileSink("metrics.log")
	auditSink, _ := sinks.NewFileSink("audit_trail.log")
	alertsSink, _ := sinks.NewFileSink("critical_alerts.log")
	allEventsSink, _ := sinks.NewFileSink("all_events.log")
	
	// Create a router with AllMatch mode (broadcast routing)
	router := sinks.NewRouterSinkWithDefault(
		sinks.AllMatch,
		allEventsSink, // Default sink for unmatched events
		// Route metrics to metrics file
		sinks.MetricRoute("metrics", metricsSink),
		// Route audit events to audit file
		sinks.AuditRoute("audit", auditSink),
		// Route critical errors to alerts file
		sinks.NewRoute("critical-alerts").
			When(sinks.AndPredicate(
				sinks.LevelPredicate(core.ErrorLevel),
				sinks.PropertyPredicate("Critical"),
			)).
			To(alertsSink),
	)
	
	log := mtlog.New(
		mtlog.WithSink(router),
		mtlog.WithSink(sinks.NewConsoleSink()),
	)
	defer log.Close()
	
	// This goes only to all_events.log (no matching routes)
	log.Information("Regular application event")
	
	// This goes to metrics.log AND all_events.log
	log.With("Metric", true).
		Information("Request processed in {Duration}ms with {RequestsPerSecond} RPS", 45, 1250)
	
	// This goes to audit.log AND all_events.log
	log.With("Audit", true).
		Information("User {UserId} modified resource {ResourceId} with action {Action}", 123, "RES-456", "UPDATE")
	
	// This goes to critical_alerts.log AND all_events.log
	log.With("Critical", true).
		Error("System memory critically low: {AvailableMemory} available", "50MB")
	
	// This goes to metrics.log, audit.log, critical_alerts.log AND all_events.log
	log.With("Metric", true, "Audit", true, "Critical", true).
		Error("Security breach detected: {AttackType} from {SourceIP}", "SQLInjection", "192.168.1.100")
	
	fmt.Println("AllMatch: Events were broadcast to all matching routes")
}

func demoDynamicRouting() {
	// Start with a basic router
	router := sinks.NewRouterSink(sinks.FirstMatch)
	
	// Create console sink for real-time monitoring
	consoleSink := sinks.NewConsoleSink()
	
	log := mtlog.New(
		mtlog.WithSink(router),
		mtlog.WithSink(consoleSink),
	)
	defer log.Close()
	
	// Initially, no routes configured
	log.Information("Starting with no routes - events go nowhere")
	
	// Add a debug route dynamically
	debugSink, _ := sinks.NewFileSink("debug.log")
	router.AddRoute(sinks.Route{
		Name:      "debug",
		Predicate: func(e *core.LogEvent) bool { return e.Level <= core.DebugLevel },
		Sink:      debugSink,
	})
	
	log.Debug("Debug route added - this message is captured")
	
	// Add a production route for important events
	prodSink, _ := sinks.NewFileSink("production.log")
	router.AddRoute(sinks.Route{
		Name: "production",
		Predicate: func(e *core.LogEvent) bool {
			env, _ := e.Properties["Environment"].(string)
			return env == "production" && e.Level >= core.WarningLevel
		},
		Sink: prodSink,
	})
	
	log.With("Environment", "production").
		Warning("Production issue detected in {Service}", "API")
	
	// Remove debug route in production
	if router.RemoveRoute("debug") {
		fmt.Println("Debug route removed for production deployment")
	}
	
	log.Debug("This debug message is not captured after route removal")
	
	// Add specialized route using the fluent builder
	specialSink, _ := sinks.NewFileSink("special.log")
	router.AddRoute(
		sinks.NewRoute("special-events").
			When(func(e *core.LogEvent) bool {
				category, _ := e.Properties["Category"].(string)
				return category == "Special"
			}).
			To(specialSink),
	)
	
	log.With("Category", "Special").
		Information("Special event occurred with value {Value}", 42)
	
	fmt.Println("Dynamic routing: Routes were modified at runtime")
}