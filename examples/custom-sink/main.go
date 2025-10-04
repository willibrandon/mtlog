package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// CustomSink demonstrates using RenderMessage() to properly render message templates
type CustomSink struct {
	output *os.File
}

func (s *CustomSink) Emit(event *core.LogEvent) {
	// Use RenderMessage() to properly render the message template
	// This handles format specifiers, capturing operators, and scalar hints
	message := event.RenderMessage()

	// Format and write the log entry
	timestamp := event.Timestamp.Format("15:04:05")
	levelStr := formatLevel(event.Level)

	fmt.Fprintf(s.output, "[%s] %s: %s\n", timestamp, levelStr, message)

	// Optionally include extra properties not in the template
	for key, value := range event.Properties {
		// Check for complete placeholder patterns to avoid false positives
		// (e.g., "User" should not match "{UserId}")
		placeholder1 := "{" + key + "}"        // Simple: {Key}
		placeholder2 := "{" + key + ":"        // With format: {Key:format}
		placeholder3 := "{@" + key + "}"       // Capturing: {@Key}
		placeholder4 := "{$" + key + "}"       // Scalar: {$Key}
		placeholder5 := "{" + key + ","        // Alignment: {Key,10}

		if !strings.Contains(event.MessageTemplate, placeholder1) &&
			!strings.Contains(event.MessageTemplate, placeholder2) &&
			!strings.Contains(event.MessageTemplate, placeholder3) &&
			!strings.Contains(event.MessageTemplate, placeholder4) &&
			!strings.Contains(event.MessageTemplate, placeholder5) {
			fmt.Fprintf(s.output, "  → %s: %v\n", key, value)
		}
	}
}

func (s *CustomSink) Close() error {
	return nil
}

func formatLevel(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "VRB"
	case core.DebugLevel:
		return "DBG"
	case core.InformationLevel:
		return "INF"
	case core.WarningLevel:
		return "WRN"
	case core.ErrorLevel:
		return "ERR"
	case core.FatalLevel:
		return "FTL"
	default:
		return "???"
	}
}

func main() {
	// Create logger with custom sink
	log := mtlog.New(
		mtlog.WithSink(&CustomSink{output: os.Stdout}),
		mtlog.WithMinimumLevel(core.DebugLevel),
	)

	// Simple message with properties
	log.Info("Application started on port {Port}", 8080)

	// Capturing operator - renders the entire struct
	config := map[string]any{
		"debug":   true,
		"port":    8080,
		"timeout": 30,
	}
	log.Debug("Configuration: {@Config}", config)

	// Scalar hint - renders as simple value
	values := []int{10, 20, 30, 40, 50}
	log.Info("Processing values: {$Values}", values)

	// Format specifier - applies padding
	log.Info("Order {OrderId:00000} processed", 42)

	// Time formatting
	log.Info("Event occurred at {Time:15:04:05}", time.Now())

	// Multiple properties with format specifiers
	log.Info("User {UserId:000} spent ${Amount:F2} on {ItemCount} items",
		123, 49.95, 3)

	// Properties not in template are shown separately
	log.With("host", "api.example.com", "port", 443, "retry", 3).
		Warning("Connection failed")

	// Complex nested structures
	user := struct {
		ID   int
		Name string
		Tags []string
	}{
		ID:   789,
		Name: "Alice",
		Tags: []string{"premium", "verified"},
	}
	log.Info("User {@User} logged in", user)

	fmt.Println("\n✅ Custom sink successfully using RenderMessage() to handle all template features!")
}