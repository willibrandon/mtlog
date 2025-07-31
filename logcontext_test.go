package mtlog_test

import (
	"context"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/sinks"
)

func TestPushProperty(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test single property
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "UserId", 123)

	logger.WithContext(ctx).Information("User action")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if userId, ok := event.Properties["UserId"]; !ok || userId != 123 {
		t.Errorf("expected UserId=123, got %v", userId)
	}
}

func TestPushPropertyInheritance(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test property inheritance
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "TenantId", "acme-corp")
	ctx = mtlog.PushProperty(ctx, "UserId", 456)
	ctx = mtlog.PushProperty(ctx, "RequestId", "req-789")

	logger.WithContext(ctx).Information("Multi-property test")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	expected := map[string]any{
		"TenantId":  "acme-corp",
		"UserId":    456,
		"RequestId": "req-789",
	}

	for key, expectedValue := range expected {
		if actualValue, ok := event.Properties[key]; !ok || actualValue != expectedValue {
			t.Errorf("expected %s=%v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestPushPropertyOverwrite(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test property overwriting
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "Version", "1.0")
	ctx = mtlog.PushProperty(ctx, "Version", "2.0") // Should overwrite

	logger.WithContext(ctx).Information("Overwrite test")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if version, ok := event.Properties["Version"]; !ok || version != "2.0" {
		t.Errorf("expected Version=2.0, got %v", version)
	}
}

func TestPushPropertyWithForContext(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test interaction with ForContext
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "TenantId", "tenant-123")

	// ForContext properties should take precedence
	logger.WithContext(ctx).ForContext("TenantId", "tenant-override").Information("Override test")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if tenantId, ok := event.Properties["TenantId"]; !ok || tenantId != "tenant-override" {
		t.Errorf("expected TenantId=tenant-override, got %v", tenantId)
	}
}

func TestPushPropertyScoping(t *testing.T) {
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test proper scoping
	ctx := context.Background()

	// Parent context
	parentCtx := mtlog.PushProperty(ctx, "Level", "parent")

	// Child context adds more properties
	childCtx := mtlog.PushProperty(parentCtx, "Level", "child")
	childCtx = mtlog.PushProperty(childCtx, "Extra", "value")

	// Log with parent context
	logger.WithContext(parentCtx).Information("Parent log")

	// Log with child context
	logger.WithContext(childCtx).Information("Child log")

	events := sink.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Check parent event
	parentEvent := events[0]
	if level, ok := parentEvent.Properties["Level"]; !ok || level != "parent" {
		t.Errorf("parent event: expected Level=parent, got %v", level)
	}
	if _, ok := parentEvent.Properties["Extra"]; ok {
		t.Error("parent event should not have Extra property")
	}

	// Check child event
	childEvent := events[1]
	if level, ok := childEvent.Properties["Level"]; !ok || level != "child" {
		t.Errorf("child event: expected Level=child, got %v", level)
	}
	if extra, ok := childEvent.Properties["Extra"]; !ok || extra != "value" {
		t.Errorf("child event: expected Extra=value, got %v", extra)
	}
}

func TestPushPropertyNilContext(t *testing.T) {
	// Test that PushProperty handles nil context gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PushProperty should not panic with nil context: %v", r)
		}
	}()

	// This should not panic
	ctx := mtlog.PushProperty(context.TODO(), "Key", "Value")
	if ctx == nil {
		t.Error("PushProperty should return a valid context even with nil input")
	}
}

func TestPushPropertyIntegration(t *testing.T) {
	// Integration test simulating real usage
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Simulate a request handler
	handleRequest := func(ctx context.Context, userId int, operation string) {
		// Add operation-specific context
		ctx = mtlog.PushProperty(ctx, "Operation", operation)
		log := logger.WithContext(ctx)

		log.Information("Starting operation")

		// Simulate some work
		if operation == "delete" {
			log.Warning("Performing delete operation")
		}

		log.Information("Operation completed")
	}

	// Simulate request processing
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "RequestId", "req-001")
	ctx = mtlog.PushProperty(ctx, "UserId", 789)

	handleRequest(ctx, 789, "update")
	handleRequest(ctx, 789, "delete")

	// Should have 5 events total (2 start, 2 complete, 1 warning)
	events := sink.Events()
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// All events should have RequestId and UserId
	for i, event := range events {
		if requestId, ok := event.Properties["RequestId"]; !ok || requestId != "req-001" {
			t.Errorf("event %d: missing or incorrect RequestId", i)
		}
		if userId, ok := event.Properties["UserId"]; !ok || userId != 789 {
			t.Errorf("event %d: missing or incorrect UserId", i)
		}
	}

	// First two events should have Operation=update
	for i := 0; i < 2; i++ {
		if op, ok := events[i].Properties["Operation"]; !ok || op != "update" {
			t.Errorf("event %d: expected Operation=update, got %v", i, op)
		}
	}

	// Last three events should have Operation=delete
	for i := 2; i < 5; i++ {
		if op, ok := events[i].Properties["Operation"]; !ok || op != "delete" {
			t.Errorf("event %d: expected Operation=delete, got %v", i, op)
		}
	}
}
