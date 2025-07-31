package mtlog_test

import (
	"strings"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// Test types for ForType functionality
type ForTypeUser struct {
	ID   int
	Name string
}

type ForTypeProduct struct {
	SKU   string
	Price float64
}

type ForTypeOrder struct {
	ID       string
	UserID   int
	Products []ForTypeProduct
}

func TestForTypeBasic(t *testing.T) {
	// Clear cache to ensure test isolation
	mtlog.ResetTypeNameCache()

	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test basic struct type
	userLogger := mtlog.ForType[ForTypeUser](logger)
	userLogger.Information("User operation performed")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "ForTypeUser" {
		t.Errorf("expected SourceContext=ForTypeUser, got %v", sourceContext)
	}
}

func TestForTypePointer(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test pointer type - should dereference to base type
	userLogger := mtlog.ForType[*ForTypeUser](logger)
	userLogger.Information("User pointer operation")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "ForTypeUser" {
		t.Errorf("expected SourceContext=ForTypeUser (dereferenced), got %v", sourceContext)
	}
}

func TestForTypeMultiplePointers(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test multiple pointer levels
	userLogger := mtlog.ForType[**ForTypeUser](logger)
	userLogger.Information("Double pointer operation")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "ForTypeUser" {
		t.Errorf("expected SourceContext=ForTypeUser (double dereferenced), got %v", sourceContext)
	}
}

func TestForTypeDifferentTypes(t *testing.T) {
	// Test different types
	tests := []struct {
		name           string
		logFunc        func(logger core.Logger)
		expectedSource string
	}{
		{
			name: "ForTypeUser",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[ForTypeUser](logger).Information("User test")
			},
			expectedSource: "ForTypeUser",
		},
		{
			name: "ForTypeProduct",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[ForTypeProduct](logger).Information("Product test")
			},
			expectedSource: "ForTypeProduct",
		},
		{
			name: "ForTypeOrder",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[ForTypeOrder](logger).Information("Order test")
			},
			expectedSource: "ForTypeOrder",
		},
		{
			name: "UserRepository",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[mtlog.UserRepository](logger).Information("Repository test")
			},
			expectedSource: "UserRepository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh memory sink for each subtest
			sink := sinks.NewMemorySink()
			logger := mtlog.New(mtlog.WithSink(sink))

			// Execute the log function
			tt.logFunc(logger)

			events := sink.Events()
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			event := events[0]
			if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != tt.expectedSource {
				t.Errorf("expected SourceContext=%s, got %v", tt.expectedSource, sourceContext)
			}
		})
	}
}

func TestForTypeBuiltinTypes(t *testing.T) {
	// Test built-in types
	tests := []struct {
		name           string
		logFunc        func(logger core.Logger)
		expectedSource string
	}{
		{
			name: "string",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[string](logger).Information("String test")
			},
			expectedSource: "string",
		},
		{
			name: "int",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[int](logger).Information("Int test")
			},
			expectedSource: "int",
		},
		{
			name: "slice",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[[]string](logger).Information("Slice test")
			},
			expectedSource: "[]string",
		},
		{
			name: "map",
			logFunc: func(logger core.Logger) {
				mtlog.ForType[map[string]int](logger).Information("Map test")
			},
			expectedSource: "map[string]int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh memory sink for each subtest
			sink := sinks.NewMemorySink()
			logger := mtlog.New(mtlog.WithSink(sink))

			// Execute the log function
			tt.logFunc(logger)

			events := sink.Events()
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			event := events[0]
			if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != tt.expectedSource {
				t.Errorf("expected SourceContext=%s, got %v", tt.expectedSource, sourceContext)
			}
		})
	}
}

func TestForTypeWithOtherProperties(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test ForType combined with other properties
	userLogger := mtlog.ForType[ForTypeUser](logger).ForContext("Operation", "Create")
	userLogger.Information("Creating user {UserId}", 123)

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Check SourceContext
	if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "ForTypeUser" {
		t.Errorf("expected SourceContext=ForTypeUser, got %v", sourceContext)
	}

	// Check Operation property
	if operation, ok := event.Properties["Operation"]; !ok || operation != "Create" {
		t.Errorf("expected Operation=Create, got %v", operation)
	}

	// Check template argument
	if userId, ok := event.Properties["UserId"]; !ok || userId != 123 {
		t.Errorf("expected UserId=123, got %v", userId)
	}
}

func TestForTypeChaining(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test method chaining
	mtlog.ForType[ForTypeUser](logger).
		ForContext("Action", "Update").
		ForContext("RequestId", "req-456").
		Information("User update completed")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	expected := map[string]any{
		"SourceContext": "ForTypeUser",
		"Action":        "Update",
		"RequestId":     "req-456",
	}

	for key, expectedValue := range expected {
		if actualValue, ok := event.Properties[key]; !ok || actualValue != expectedValue {
			t.Errorf("expected %s=%v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestForTypeEquivalentToForSourceContext(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test that ForType[ForTypeUser] is equivalent to ForSourceContext("ForTypeUser")
	mtlog.ForType[ForTypeUser](logger).Information("ForType message")
	logger.ForSourceContext("ForTypeUser").Information("ForSourceContext message")

	events := sink.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Both events should have the same SourceContext
	for i, event := range events {
		if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "ForTypeUser" {
			t.Errorf("event %d: expected SourceContext=ForTypeUser, got %v", i, sourceContext)
		}
	}
}

func TestForTypeAnonymousStructs(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test anonymous struct
	mtlog.ForType[struct{ Name string }](logger).Information("Anonymous struct operation")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	sourceContext, ok := event.Properties["SourceContext"]
	if !ok {
		t.Error("SourceContext property not found")
	}

	// Anonymous structs should include their definition
	sourceContextStr, isString := sourceContext.(string)
	if !isString {
		t.Errorf("expected SourceContext to be string, got %T", sourceContext)
	}

	if !strings.Contains(sourceContextStr, "struct") {
		t.Errorf("expected SourceContext to contain 'struct', got %v", sourceContext)
	}
}
