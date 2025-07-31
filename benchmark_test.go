package mtlog

import (
	"io"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// discardSink is a sink that discards all events (for benchmarking)
type discardSink struct{}

func (d *discardSink) Emit(event *core.LogEvent)                                                {}
func (d *discardSink) Close() error                                                             { return nil }
func (d *discardSink) EmitSimple(timestamp time.Time, level core.LogEventLevel, message string) {}

// Benchmark simple logging without properties
func BenchmarkSimpleLog(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("This is a simple log message")
	}
}

// Benchmark logging with properties
func BenchmarkLogWithProperties(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("User {UserId} performed action {Action}", 123, "login")
	}
}

// Benchmark logging with multiple properties
func BenchmarkLogWithManyProperties(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("User {UserId} from {Country} using {Browser} on {OS} performed {Action}",
			123, "USA", "Chrome", "Windows", "login")
	}
}

// Benchmark logging with context
func BenchmarkLogWithContext(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	ctxLogger := logger.ForContext("Environment", "Production").
		ForContext("Version", "1.0.0").
		ForContext("Region", "us-east-1")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctxLogger.Information("Processing request")
	}
}

// Benchmark logging below minimum level (should be very fast)
func BenchmarkLogBelowMinimumLevel(b *testing.B) {
	logger := New(
		WithSink(&discardSink{}),
		WithMinimumLevel(core.InformationLevel),
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Debug("This should not be processed")
	}
}

// Benchmark with enrichers
func BenchmarkLogWithEnrichers(b *testing.B) {
	logger := New(
		WithSink(&discardSink{}),
		// Add test enrichers
		WithEnricher(&benchEnricher{name: "Prop1", value: "Value1"}),
		WithEnricher(&benchEnricher{name: "Prop2", value: "Value2"}),
		WithEnricher(&benchEnricher{name: "Prop3", value: "Value3"}),
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("Test message")
	}
}

// Benchmark console sink
func BenchmarkConsoleSink(b *testing.B) {
	logger := New(WithSink(sinks.NewConsoleSinkWithWriter(io.Discard)))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("User {UserId} logged in", 123)
	}
}

// Benchmark parallel logging
func BenchmarkParallelLogging(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Information("Parallel log message {ThreadId}", 1)
		}
	})
}

// Benchmark template parsing (cached vs uncached)
func BenchmarkTemplateParsing(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	// Use different templates to avoid caching
	templates := []string{
		"User {UserId} logged in",
		"Order {OrderId} processed",
		"Payment {PaymentId} received",
		"Item {ItemId} shipped",
		"Customer {CustomerId} registered",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		template := templates[i%len(templates)]
		logger.Information(template, i)
	}
}

// Benchmark structured object logging
func BenchmarkStructuredObject(b *testing.B) {
	logger := New(WithSink(&discardSink{}))

	user := struct {
		ID    int
		Name  string
		Email string
		Role  string
	}{
		ID:    123,
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  "Admin",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Information("Processing user {@User}", user)
	}
}

// Helper enricher for benchmarks
type benchEnricher struct {
	name  string
	value any
}

func (be *benchEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	prop := propertyFactory.CreateProperty(be.name, be.value)
	event.Properties[prop.Name] = prop.Value
}
