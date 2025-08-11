package main

import (
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Test the new WithConsoleTemplateAndTheme convenience method
	log := mtlog.New(
		mtlog.WithConsoleTemplateAndTheme(
			"[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}",
			sinks.LiterateTheme(),
		),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithSourceContext("TestApp"),
	)

	// Test different log levels with the Literate theme
	log.Debug("Debug message with {Property}", "value1")
	log.Information("User {UserId} logged in from {IP}", 123, "192.168.1.1")
	log.Warning("Disk usage at {Percentage:P1}", 0.85)
	log.Error("Failed to connect to {Service}", "database")

	// Test with complex object
	type Order struct {
		ID    int
		Total float64
	}
	order := Order{ID: 456, Total: 99.95}
	log.Information("Processing {@Order} for customer {CustomerId}", order, "CUST789")
}