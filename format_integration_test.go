package mtlog

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/sinks"
)

func TestFormatSpecifiersIntegration(t *testing.T) {
	// Create a buffer to capture output
	buf := &bytes.Buffer{}

	// Create logger with console sink
	logger := New(WithSink(sinks.NewConsoleSinkWithWriter(buf)))

	// Test numeric formatting
	t.Run("Numeric formatting", func(t *testing.T) {
		buf.Reset()
		logger.Information("Order {OrderId:000} has {ItemCount:00} items", 5, 3)

		output := buf.String()
		if !strings.Contains(output, "Order 005 has 03 items") {
			t.Errorf("Expected zero-padded numbers in output, got: %s", output)
		}
	})

	// Test hex formatting
	t.Run("Hex formatting", func(t *testing.T) {
		buf.Reset()
		logger.Information("Color code: {Color:X}", 0xFF00AA)

		output := buf.String()
		if !strings.Contains(output, "Color code: FF00AA") {
			t.Errorf("Expected hex format in output, got: %s", output)
		}
	})

	// Test float formatting
	t.Run("Float formatting", func(t *testing.T) {
		buf.Reset()
		logger.Information("Price: ${Price:F2} (discount: {Discount:P1})", 19.995, 0.125)

		output := buf.String()
		if !strings.Contains(output, "Price: $20.00") {
			t.Errorf("Expected formatted price in output, got: %s", output)
		}
		if !strings.Contains(output, "discount: 12.5%") {
			t.Errorf("Expected percentage format in output, got: %s", output)
		}
	})

	// Test alignment
	t.Run("Alignment", func(t *testing.T) {
		buf.Reset()
		logger.Information("Name: {Name,10} | Status: {Status,-8} | ID: {Id,5:000}", "Alice", "Active", 42)

		output := buf.String()
		if !strings.Contains(output, "Name:      Alice | Status: Active   | ID:   042") {
			t.Errorf("Expected aligned output, got: %s", output)
		}
	})

	// Test time formatting
	t.Run("Time formatting", func(t *testing.T) {
		buf.Reset()
		testTime := time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC)
		logger.Information("Event at {Timestamp:yyyy-MM-dd HH:mm:ss}", testTime)

		output := buf.String()
		if !strings.Contains(output, "Event at 2025-01-22 15:30:45") {
			t.Errorf("Expected formatted time in output, got: %s", output)
		}
	})

	// Test complex formatting
	t.Run("Complex formatting", func(t *testing.T) {
		buf.Reset()
		testTime := time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC)
		logger.Information(
			"Transaction {TxId:00000} for ${Amount,10:F2} at {Time:HH:mm:ss} ({Status,-10})",
			123, 1234.567, testTime, "COMPLETED",
		)

		output := buf.String()
		if !strings.Contains(output, "Transaction 00123 for $   1234.57 at 15:30:45 (COMPLETED )") {
			t.Errorf("Expected complex formatted output, got: %s", output)
		}
	})

	// Test that format specifiers work with ForContext
	t.Run("Format with context", func(t *testing.T) {
		buf.Reset()
		contextLogger := logger.ForContext("RequestId", 42)
		contextLogger.Information("Processing {Count:000} items", 7)

		output := buf.String()
		if !strings.Contains(output, "Processing 007 items") {
			t.Errorf("Expected formatted output with context, got: %s", output)
		}
	})
}

func TestFormatSpecifiersInProperties(t *testing.T) {
	// Create a logger with a custom test sink that captures both message and properties
	type testEvent struct {
		message    string
		properties map[string]any
	}
	var events []testEvent

	testSink := &testCaptureSink{
		capture: func(event *core.LogEvent) {
			// Render the message
			tmpl, _ := parser.Parse(event.MessageTemplate)
			rendered := tmpl.Render(event.Properties)

			events = append(events, testEvent{
				message:    rendered,
				properties: event.Properties,
			})
		},
	}

	logger := New(WithSink(testSink))

	// Log with various format specifiers
	testTime := time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC)
	logger.Information(
		"Order {OrderId:0000} total ${Total:F2} at {OrderTime:yyyy-MM-dd}",
		42, 123.456, testTime,
	)

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]

	// The formatted values should appear in the rendered message
	if event.message != "Order 0042 total $123.46 at 2025-01-22" {
		t.Errorf("Expected formatted message, got: %s", event.message)
	}

	// The properties should contain the original values
	if event.properties["OrderId"] != 42 {
		t.Errorf("Expected original OrderId value 42, got: %v", event.properties["OrderId"])
	}
	if event.properties["Total"] != 123.456 {
		t.Errorf("Expected original Total value 123.456, got: %v", event.properties["Total"])
	}
}

// testCaptureSink captures events for testing
type testCaptureSink struct {
	capture func(*core.LogEvent)
}

func (t *testCaptureSink) Emit(event *core.LogEvent) {
	if t.capture != nil {
		t.capture(event)
	}
}

func (t *testCaptureSink) Close() error { return nil }

func BenchmarkFormatSpecifiers(b *testing.B) {
	logger := New(WithSink(&discardSink{}))
	testTime := time.Now()

	b.Run("NoFormat", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Information("Order {Id} total {Amount} at {Time}", 123, 456.78, testTime)
		}
	})

	b.Run("WithFormat", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Information("Order {Id:00000} total {Amount:F2} at {Time:yyyy-MM-dd}", 123, 456.78, testTime)
		}
	})

	b.Run("WithAlignment", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Information("Status: {Status,-10} Count: {Count,5}", "Active", 42)
		}
	})

	b.Run("ComplexFormat", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			logger.Information(
				"Transaction {Id:00000} for ${Amount,10:F2} at {Time:HH:mm:ss} ({Status,-8})",
				123, 1234.56, testTime, "DONE",
			)
		}
	})
}
