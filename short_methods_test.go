package mtlog_test

import (
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestShortMethods(t *testing.T) {
	// Create memory sink to capture events
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Test Info method
	logger.Info("Information message with {Value}", 42)

	// Test Warn method
	logger.Warn("Warning message with {Count}", 5)

	// Verify events were captured
	events := memSink.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify Info method created Information level event
	if events[0].Level != core.InformationLevel {
		t.Errorf("expected Information level, got %v", events[0].Level)
	}
	if events[0].Properties["Value"] != 42 {
		t.Errorf("expected Value=42, got %v", events[0].Properties["Value"])
	}

	// Verify Warn method created Warning level event
	if events[1].Level != core.WarningLevel {
		t.Errorf("expected Warning level, got %v", events[1].Level)
	}
	if events[1].Properties["Count"] != 5 {
		t.Errorf("expected Count=5, got %v", events[1].Properties["Count"])
	}
}

func TestShortMethodsWithGenericLogger(t *testing.T) {
	// Create memory sink to capture events
	memSink := sinks.NewMemorySink()
	
	// Create typed logger for User type
	type User struct {
		ID   int
		Name string
	}
	
	userLogger := mtlog.NewTyped[User](
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	user := User{ID: 123, Name: "Alice"}

	// Test InfoT method
	userLogger.InfoT("User {ID} logged in", user)

	// Test WarnT method
	userLogger.WarnT("User {Name} has invalid session", user, user.Name)

	// Verify events
	events := memSink.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify InfoT created Information level event
	if events[0].Level != core.InformationLevel {
		t.Errorf("expected Information level, got %v", events[0].Level)
	}

	// Verify WarnT created Warning level event
	if events[1].Level != core.WarningLevel {
		t.Errorf("expected Warning level, got %v", events[1].Level)
	}
}

func TestShortMethodsEquivalence(t *testing.T) {
	// Test that short methods produce identical output to long methods
	memSink1 := sinks.NewMemorySink()
	logger1 := mtlog.New(mtlog.WithSink(memSink1))

	memSink2 := sinks.NewMemorySink()
	logger2 := mtlog.New(mtlog.WithSink(memSink2))

	// Log with long methods
	logger1.Information("Test message {Value}", 123)
	logger1.Warning("Warning {Type}", "test")

	// Log with short methods
	logger2.Info("Test message {Value}", 123)
	logger2.Warn("Warning {Type}", "test")

	// Get events
	events1 := memSink1.Events()
	events2 := memSink2.Events()

	if len(events1) != len(events2) {
		t.Fatalf("expected same number of events, got %d vs %d", len(events1), len(events2))
	}

	// Compare events
	for i := range events1 {
		if events1[i].Level != events2[i].Level {
			t.Errorf("event %d: level mismatch: %v vs %v", i, events1[i].Level, events2[i].Level)
		}
		if events1[i].MessageTemplate != events2[i].MessageTemplate {
			t.Errorf("event %d: template mismatch: %v vs %v", i, 
				events1[i].MessageTemplate, events2[i].MessageTemplate)
		}
		// Check properties
		for k, v1 := range events1[i].Properties {
			if v2, ok := events2[i].Properties[k]; !ok || v1 != v2 {
				t.Errorf("event %d: property %s mismatch: %v vs %v", i, k, v1, v2)
			}
		}
	}
}