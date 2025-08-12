package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== Default Theme ===")
	demoTheme(sinks.DefaultTheme())

	fmt.Println("\n=== Lite Theme ===")
	demoTheme(sinks.LiteTheme())

	fmt.Println("\n=== Dev Theme ===")
	demoTheme(sinks.DevTheme())

	fmt.Println("\n=== No Color Theme ===")
	demoTheme(sinks.NoColorTheme())

	fmt.Println("\n=== Custom Theme ===")
	customTheme := &sinks.ConsoleTheme{
		VerboseColor:     sinks.ColorMagenta,
		DebugColor:       sinks.ColorBrightCyan,
		InformationColor: sinks.ColorBrightGreen,
		WarningColor:     sinks.ColorBrightYellow + sinks.ColorBold,
		ErrorColor:       sinks.ColorBrightRed + sinks.ColorBold,
		FatalColor:       sinks.ColorBgRed + sinks.ColorWhite + sinks.ColorBold,

		TimestampColor:   sinks.ColorBrightBlue,
		MessageColor:     sinks.ColorBrightWhite,
		PropertyKeyColor: sinks.ColorCyan,
		PropertyValColor: sinks.ColorYellow,

		LevelFormat:     "%-5s |", // Left-aligned with separator
		TimestampFormat: "15:04:05.000",
		PropertyFormat:  "%s → %v",
	}
	demoTheme(customTheme)

	fmt.Println("\n=== Literate Theme with Custom Template ===")
	demoThemeWithTemplate()
}

func demoTheme(theme *sinks.ConsoleTheme) {
	// Create logger with the theme
	sink := sinks.NewConsoleSinkWithTheme(theme)
	logger := mtlog.New(mtlog.WithSink(sink))

	// Create a sink with properties displayed
	propsSink := sinks.NewConsoleSinkWithTheme(theme)
	propsSink.ShowProperties(true)
	loggerWithProps := mtlog.New(mtlog.WithSink(propsSink))

	// Demo different log levels
	logger.Verbose("This is a verbose message with details")
	logger.Debug("Debugging application flow")
	logger.Information("User {UserId} logged in successfully", 42)
	logger.Warning("Cache miss for key {Key}", "user:42:profile")
	logger.Error("Failed to connect to database {Database} after {Attempts} attempts", "OrderDB", 3)

	// Demo with extra properties
	loggerWithProps.ForContext("RequestId", "req-123").
		ForContext("UserId", 42).
		Information("Processing order {OrderId} with total ${Total:F2}", "ORD-2025-001", 99.99)

	// Demo structured logging
	order := map[string]any{
		"Id":     "ORD-2025-001",
		"Status": "Processing",
		"Items":  3,
	}
	logger.Information("Order update: {@Order}", order)

	// Simulate error with timing
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	elapsed := time.Since(start)
	logger.Warning("Slow query detected: {Query} took {Duration}ms",
		"SELECT * FROM orders WHERE status = 'pending'",
		elapsed.Milliseconds())
}

func demoThemeWithTemplate() {
	// Demonstrate the new WithConsoleTemplateAndTheme convenience method
	logger := mtlog.New(
		mtlog.WithConsoleTemplateAndTheme(
			"[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}",
			sinks.LiterateTheme(),
		),
		mtlog.WithSourceContext("AppService"),
	)

	// Show how template and theme work together
	logger.Debug("Debug message with {Property}", "value1")
	logger.Information("User {UserId} logged in from {IP}", 123, "192.168.1.1")
	logger.Warning("Disk usage at {Percentage:P1}", 0.85)
	logger.Error("Failed to connect to {Service}", "database")

	// With structured data
	order := map[string]any{
		"Id":     "ORD-2025-002",
		"Status": "Shipped",
		"Total":  149.99,
	}
	logger.Information("Order {@Order} shipped to customer {CustomerId}", order, "CUST789")
}
