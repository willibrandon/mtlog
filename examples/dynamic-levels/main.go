package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== Dynamic Level Control Example ===")

	// Example 1: Basic dynamic level control
	fmt.Println("\n--- Example 1: Basic Dynamic Level Control ---")
	basicDynamicLevelExample()

	// Example 2: Multiple loggers sharing a level switch
	fmt.Println("\n--- Example 2: Shared Level Switch ---")
	sharedLevelSwitchExample()

	// Example 3: Runtime level changes
	fmt.Println("\n--- Example 3: Runtime Level Changes ---")
	runtimeLevelChangesExample()

	// Example 4: HTTP endpoint for level control (simulation)
	fmt.Println("\n--- Example 4: HTTP Level Control ---")
	httpLevelControlExample()

	// Example 5: Performance comparison
	fmt.Println("\n--- Example 5: Performance Comparison ---")
	performanceComparisonExample()
}

func basicDynamicLevelExample() {
	// Create level switch starting at Information level
	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

	// Create logger with dynamic level control
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Example", "BasicDynamic"),
	)

	fmt.Printf("Current level: %v\n", levelSwitch.Level())

	// Log at various levels
	logger.Debug("This debug message will be filtered")
	logger.Information("This information message will appear")
	logger.Warning("This warning message will appear")

	fmt.Println("\nChanging level to Debug...")
	levelSwitch.SetLevel(core.DebugLevel)
	fmt.Printf("New level: %v\n", levelSwitch.Level())

	// Now debug messages should appear
	logger.Debug("This debug message will now appear")
	logger.Information("This information message will still appear")

	fmt.Println("\nChanging level to Error...")
	levelSwitch.SetLevel(core.ErrorLevel)

	logger.Debug("This debug message will be filtered")
	logger.Information("This information message will be filtered")
	logger.Warning("This warning message will be filtered")
	logger.Error("This error message will appear")
}

func sharedLevelSwitchExample() {
	// Create shared level switch
	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

	// Create multiple loggers for different components
	userLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Component", "User"),
	)

	dbLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Component", "Database"),
	)

	httpLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Component", "HTTP"),
	)

	fmt.Printf("All loggers using level: %v\n", levelSwitch.Level())

	// Log from different components
	userLogger.Information("User authentication successful")
	dbLogger.Information("Database connection established")
	httpLogger.Information("HTTP server started on port 8080")

	fmt.Println("\nChanging all loggers to Warning level...")
	levelSwitch.SetLevel(core.WarningLevel)

	// Information messages should now be filtered for all loggers
	userLogger.Information("This info message will be filtered")
	dbLogger.Warning("Database connection pool exhausted")
	httpLogger.Error("HTTP request timeout")
}

func runtimeLevelChangesExample() {
	// Use the convenience method to create controlled level
	option, levelSwitch := mtlog.WithControlledLevel(core.InformationLevel)

	logger := mtlog.New(
		mtlog.WithConsole(),
		option,
		mtlog.WithProperty("Example", "Runtime"),
	)

	// Simulate application lifecycle with different log levels
	phases := []struct {
		phase       string
		level       core.LogEventLevel
		description string
	}{
		{"Startup", core.DebugLevel, "Enable debug logging during startup"},
		{"Normal Operation", core.InformationLevel, "Standard logging during normal operation"},
		{"High Load", core.WarningLevel, "Reduce logging during high load"},
		{"Troubleshooting", core.VerboseLevel, "Enable verbose logging for troubleshooting"},
		{"Shutdown", core.ErrorLevel, "Only log errors during shutdown"},
	}

	for _, phase := range phases {
		fmt.Printf("\n=== %s Phase ===\n", phase.phase)
		fmt.Printf("Setting level to %v - %s\n", phase.level, phase.description)

		levelSwitch.SetLevel(phase.level)

		// Simulate logging at different levels
		logger.Verbose("Verbose: Detailed trace information")
		logger.Debug("Debug: Development diagnostic info")
		logger.Information("Information: General application flow")
		logger.Warning("Warning: Something unexpected happened")
		logger.Error("Error: Operation failed")

		time.Sleep(500 * time.Millisecond)
	}

	// Demonstrate fluent interface
	fmt.Println("\n=== Fluent Interface ===")
	levelSwitch.Information().Warning().Debug()
	fmt.Printf("Final level after chaining: %v\n", levelSwitch.Level())
	logger.Debug("This debug message shows fluent interface worked")
}

func httpLevelControlExample() {
	// Simulate HTTP endpoint for dynamic level control
	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Service", "WebAPI"),
	)

	// Simulate HTTP handler for level control
	handleLevelChange := func(newLevel string) {
		fmt.Printf("Received HTTP request to change level to: %s\n", newLevel)

		var level core.LogEventLevel
		switch newLevel {
		case "verbose":
			level = core.VerboseLevel
		case "debug":
			level = core.DebugLevel
		case "information":
			level = core.InformationLevel
		case "warning":
			level = core.WarningLevel
		case "error":
			level = core.ErrorLevel
		case "fatal":
			level = core.FatalLevel
		default:
			logger.Error("Invalid level requested: {Level}", newLevel)
			return
		}

		oldLevel := levelSwitch.Level()
		levelSwitch.SetLevel(level)
		logger.Information("Log level changed from {OldLevel} to {NewLevel}", oldLevel, level)
	}

	// Simulate some HTTP requests to change log levels
	requests := []string{"debug", "warning", "verbose", "error", "invalid", "information"}

	for _, req := range requests {
		handleLevelChange(req)

		// Log some messages to demonstrate the level change
		logger.Debug("Debug message after level change")
		logger.Information("Information message after level change")
		logger.Warning("Warning message after level change")

		fmt.Println()
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Final logging level: %v\n", levelSwitch.Level())
}

func performanceComparisonExample() {
	// Create loggers with different level control mechanisms
	staticLogger := mtlog.New(
		mtlog.WithSink(sinks.NewMemorySink()), // Use memory sink for performance testing
		mtlog.WithMinimumLevel(core.WarningLevel),
	)

	levelSwitch := mtlog.NewLoggingLevelSwitch(core.WarningLevel)
	dynamicLogger := mtlog.New(
		mtlog.WithSink(sinks.NewMemorySink()),
		mtlog.WithLevelSwitch(levelSwitch),
	)

	const iterations = 100000

	// Benchmark static level filtering
	start := time.Now()
	for range iterations {
		staticLogger.Debug("Filtered debug message")
	}
	staticDuration := time.Since(start)

	// Benchmark dynamic level filtering
	start = time.Now()
	for range iterations {
		dynamicLogger.Debug("Filtered debug message")
	}
	dynamicDuration := time.Since(start)

	fmt.Printf("Performance comparison (%d iterations):\n", iterations)
	fmt.Printf("Static level filtering:  %v (%d ns/op)\n", staticDuration, staticDuration.Nanoseconds()/iterations)
	fmt.Printf("Dynamic level filtering: %v (%d ns/op)\n", dynamicDuration, dynamicDuration.Nanoseconds()/iterations)

	overhead := float64(dynamicDuration.Nanoseconds()-staticDuration.Nanoseconds()) / float64(staticDuration.Nanoseconds()) * 100
	fmt.Printf("Dynamic overhead: %.1f%%\n", overhead)

	// Demonstrate that both can be changed at runtime (only dynamic actually works)
	fmt.Println("\nTesting runtime level changes:")

	// This won't change anything for static logger
	fmt.Println("Attempting to change static logger level (no effect)...")
	staticLogger.Information("Static logger info message") // Still filtered

	// This will work for dynamic logger
	fmt.Println("Changing dynamic logger level to Information...")
	levelSwitch.SetLevel(core.InformationLevel)
	dynamicLogger.Information("Dynamic logger info message") // Should appear
}

// demonstrateHTTPServer shows how you might integrate level switching with a real HTTP server
func demonstrateHTTPServer() {
	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithLevelSwitch(levelSwitch),
		mtlog.WithProperty("Service", "HTTP"),
	)

	// HTTP handler for changing log level
	http.HandleFunc("/admin/loglevel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		newLevel := r.FormValue("level")
		if newLevel == "" {
			http.Error(w, "Missing level parameter", http.StatusBadRequest)
			return
		}

		var level core.LogEventLevel
		switch newLevel {
		case "verbose":
			level = core.VerboseLevel
		case "debug":
			level = core.DebugLevel
		case "information":
			level = core.InformationLevel
		case "warning":
			level = core.WarningLevel
		case "error":
			level = core.ErrorLevel
		case "fatal":
			level = core.FatalLevel
		default:
			logger.Warning("Invalid log level requested: {Level}", newLevel)
			http.Error(w, "Invalid level", http.StatusBadRequest)
			return
		}

		oldLevel := levelSwitch.Level()
		levelSwitch.SetLevel(level)

		logger.Information("Log level changed via HTTP from {OldLevel} to {NewLevel} by {RemoteAddr}",
			oldLevel, level, r.RemoteAddr)

		fmt.Fprintf(w, "Log level changed from %v to %v\n", oldLevel, level)
	})

	// HTTP handler to get current log level
	http.HandleFunc("/admin/loglevel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		currentLevel := levelSwitch.Level()
		fmt.Fprintf(w, "Current log level: %v\n", currentLevel)
	})

	logger.Information("HTTP admin server would be available at http://localhost:8080/admin/loglevel")
	logger.Information("Use POST with 'level' parameter to change level")
	logger.Information("Use GET to check current level")

	// Note: We don't actually start the server in this example
	// fmt.Println("Starting HTTP server on :8080...")
	// log.Fatal(http.ListenAndServe(":8080", nil))
}
