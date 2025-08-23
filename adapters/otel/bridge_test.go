package otel

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	olog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	"go.opentelemetry.io/otel/trace"
)

// mockLogger implements core.Logger for testing
type mockLogger struct {
	events []*core.LogEvent
	level  core.LogEventLevel
}

func (m *mockLogger) Log(timestamp time.Time, level core.LogEventLevel, template string, err error, props []core.LogEventProperty) {
	event := &core.LogEvent{
		Timestamp:       timestamp,
		Level:           level,
		MessageTemplate: template,
		Exception:       err,
		Properties:      make(map[string]any),
	}
	for _, prop := range props {
		event.Properties[prop.Name] = prop.Value
	}
	m.events = append(m.events, event)
}

func (m *mockLogger) IsEnabled(level core.LogEventLevel) bool {
	return level >= m.level
}

func (m *mockLogger) Verbose(template string, args ...any) {
	m.Log(time.Now(), core.VerboseLevel, template, nil, nil)
}

func (m *mockLogger) Debug(template string, args ...any) {
	m.Log(time.Now(), core.DebugLevel, template, nil, nil)
}

func (m *mockLogger) Information(template string, args ...any) {
	m.Log(time.Now(), core.InformationLevel, template, nil, nil)
}

func (m *mockLogger) Warning(template string, args ...any) {
	m.Log(time.Now(), core.WarningLevel, template, nil, nil)
}

func (m *mockLogger) Error(template string, args ...any) {
	m.Log(time.Now(), core.ErrorLevel, template, nil, nil)
}

func (m *mockLogger) Fatal(template string, args ...any) {
	m.Log(time.Now(), core.FatalLevel, template, nil, nil)
}

func (m *mockLogger) Write(level core.LogEventLevel, template string, args ...any) {
	m.Log(time.Now(), level, template, nil, nil)
}

func (m *mockLogger) ForContext(propertyName string, value any) core.Logger {
	// For testing, just return self
	return m
}

func (m *mockLogger) WithContext(ctx context.Context) core.Logger {
	// For testing, just return self
	return m
}

func (m *mockLogger) With(args ...any) core.Logger {
	// For testing, just return self
	return m
}

func (m *mockLogger) Info(template string, args ...any) {
	m.Information(template, args...)
}

func (m *mockLogger) Warn(template string, args ...any) {
	m.Warning(template, args...)
}

// Sampling methods - all return self for testing
func (m *mockLogger) Sample(n uint64) core.Logger                          { return m }
func (m *mockLogger) SampleDuration(duration time.Duration) core.Logger    { return m }
func (m *mockLogger) SampleRate(rate float32) core.Logger                  { return m }
func (m *mockLogger) SampleFirst(n uint64) core.Logger                     { return m }
func (m *mockLogger) SampleGroup(groupName string, n uint64) core.Logger   { return m }
func (m *mockLogger) SampleWhen(predicate func() bool, n uint64) core.Logger { return m }
func (m *mockLogger) SampleBackoff(key string, factor float64) core.Logger { return m }
func (m *mockLogger) ResetSampling()                                       {}
func (m *mockLogger) ResetSamplingGroup(groupName string)                  {}
func (m *mockLogger) EnableSamplingSummary(period time.Duration) core.Logger { return m }
func (m *mockLogger) GetSamplingStats() (sampled uint64, skipped uint64)   { return 0, 0 }
func (m *mockLogger) SampleProfile(profileName string) core.Logger         { return m }
func (m *mockLogger) SampleAdaptive(targetEventsPerSecond uint64) core.Logger { return m }
func (m *mockLogger) SampleAdaptiveWithOptions(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration) core.Logger {
	return m
}

// Context-aware methods
func (m *mockLogger) VerboseContext(ctx context.Context, template string, args ...any) {
	m.Verbose(template, args...)
}

func (m *mockLogger) DebugContext(ctx context.Context, template string, args ...any) {
	m.Debug(template, args...)
}

func (m *mockLogger) InfoContext(ctx context.Context, template string, args ...any) {
	m.Information(template, args...)
}

func (m *mockLogger) WarnContext(ctx context.Context, template string, args ...any) {
	m.Warning(template, args...)
}

func (m *mockLogger) ErrorContext(ctx context.Context, template string, args ...any) {
	m.Error(template, args...)
}

func (m *mockLogger) FatalContext(ctx context.Context, template string, args ...any) {
	m.Fatal(template, args...)
}

func (m *mockLogger) WriteContext(ctx context.Context, level core.LogEventLevel, template string, args ...any) {
	m.Write(level, template, args...)
}

func (m *mockLogger) EnableSamplingSummaryWithCleanup(period time.Duration) (core.Logger, func()) {
	return m, func() {}
}

func (m *mockLogger) GetSamplingMetrics() core.SamplingMetrics {
	return core.SamplingMetrics{}
}

// New methods for deadline awareness
func (m *mockLogger) DeadlineStats() interface{} {
	return nil
}

func (m *mockLogger) WithDeadlineWarning(threshold time.Duration, opts ...interface{}) core.Logger {
	return m
}

func TestBridge(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{level: core.InformationLevel}

	// Create bridge
	bridge := NewBridge(logger)

	// Test Emit
	ctx := context.Background()
	var record olog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(olog.SeverityInfo1)
	record.SetBody(olog.StringValue("Test message"))
	record.AddAttributes(olog.String("key", "value"))

	bridge.Emit(ctx, record)

	// Verify log was captured
	if len(logger.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(logger.events))
	}

	event := logger.events[0]
	if event.MessageTemplate != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", event.MessageTemplate)
	}
}

func TestBridgeEnabled(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{level: core.WarningLevel}

	// Create bridge
	bridge := NewBridge(logger)

	ctx := context.Background()

	// Test with Info level (should be disabled)
	params := olog.EnabledParameters{
		Severity: olog.SeverityInfo1,
	}
	if bridge.Enabled(ctx, params) {
		t.Error("Expected Info to be disabled with Warning minimum level")
	}

	// Test with Error level (should be enabled)
	params.Severity = olog.SeverityError1
	if !bridge.Enabled(ctx, params) {
		t.Error("Expected Error to be enabled with Warning minimum level")
	}
}

func TestBridgeWithTraceContext(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{level: core.InformationLevel}

	// Create bridge with trace context enabled
	bridge := NewBridge(logger, WithTraceContext(true))

	// Create a context with a span
	ctx := context.Background()
	traceID, _ := trace.TraceIDFromHex("01020304050607080102040810203040")
	spanID, _ := trace.SpanIDFromHex("0102040810203040")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  false,
	})
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)

	// Create a mock span
	mockSpan := &mockSpan{spanContext: spanCtx}
	ctx = trace.ContextWithSpan(ctx, mockSpan)

	// Test Emit with trace context
	var record olog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(olog.SeverityInfo1)
	record.SetBody(olog.StringValue("Test with trace"))

	bridge.Emit(ctx, record)

	// Verify log was captured
	if len(logger.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(logger.events))
	}
}

func TestHandler(t *testing.T) {
	// Create a mock OTEL logger
	mockLogger := &mockOTELLogger{}

	// Create handler
	handler := NewHandler(mockLogger)

	// Create a log event
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Test {Property}",
		Properties: map[string]any{
			"Property": "value",
			"Number":   42,
		},
	}

	// Emit event
	handler.Emit(event)

	// Verify the event was forwarded
	if len(mockLogger.records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(mockLogger.records))
	}

	record := mockLogger.records[0]
	body := record.Body().AsString()
	// The body should contain the template, but may have been rendered
	if !strings.Contains(body, "Test") {
		t.Errorf("Expected body to contain 'Test', got '%s'", body)
	}
}

func TestHandlerWithException(t *testing.T) {
	// Create a mock OTEL logger
	mockLogger := &mockOTELLogger{}

	// Create handler
	handler := NewHandler(mockLogger)

	// Create a log event with exception
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.ErrorLevel,
		MessageTemplate: "Error occurred",
		Exception:       &testError{msg: "test error"},
		Properties:      make(map[string]any),
	}

	// Emit event
	handler.Emit(event)

	// Verify the event was forwarded
	if len(mockLogger.records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(mockLogger.records))
	}

	// Check exception was added as attributes
	record := mockLogger.records[0]
	found := false
	record.WalkAttributes(func(kv olog.KeyValue) bool {
		if kv.Key == "exception.message" && kv.Value.AsString() == "test error" {
			found = true
		}
		return true
	})
	if !found {
		t.Error("Expected exception.message attribute")
	}
}

func TestLoggerProvider(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{level: core.InformationLevel}

	// Create logger provider
	provider := NewLoggerProvider(logger)

	// Get a logger
	otelLogger := provider.Logger("test")
	if otelLogger == nil {
		t.Fatal("Expected logger, got nil")
	}

	// Use the logger
	ctx := context.Background()
	var record olog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(olog.SeverityWarn1)
	record.SetBody(olog.StringValue("Warning message"))

	otelLogger.Emit(ctx, record)

	// Verify log was captured
	if len(logger.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(logger.events))
	}

	event := logger.events[0]
	if event.Level != core.WarningLevel {
		t.Errorf("Expected warning level, got %v", event.Level)
	}
}

func TestOTLPSink(t *testing.T) {
	// This is a basic test to ensure OTLP sink compiles and initializes
	// Actual OTLP testing would require a collector endpoint
	
	sink, err := NewOTLPSink(
		WithOTLPEndpoint("localhost:4317"),
		WithOTLPBatching(10, 1*time.Second),
	)
	if err != nil {
		// Skip if there's a schema conflict (this can happen in test environments)
		if strings.Contains(err.Error(), "conflicting Schema URL") {
			t.Skip("Skipping due to schema URL conflict in test environment")
		}
		t.Fatalf("Failed to create OTLP sink: %v", err)
	}
	defer sink.Close()

	// Test emitting an event (it won't actually send anywhere)
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Test OTLP",
		Properties: map[string]any{
			"test": "value",
		},
	}
	
	sink.Emit(event)
	
	// Test metrics
	metrics := sink.GetMetrics()
	if metrics == nil {
		t.Error("Expected metrics, got nil")
	}
}

// Mock implementations for testing

type mockSpan struct {
	trace.Span
	spanContext trace.SpanContext
}

func (s *mockSpan) SpanContext() trace.SpanContext {
	return s.spanContext
}

type mockOTELLogger struct {
	embedded.Logger
	records []olog.Record
}

func (l *mockOTELLogger) Emit(ctx context.Context, record olog.Record) {
	l.records = append(l.records, record)
}

func (l *mockOTELLogger) Enabled(ctx context.Context, param olog.EnabledParameters) bool {
	return true
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}