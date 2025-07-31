package sinks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

// Custom typed data for testing
type UserActivity struct {
	UserID string
	Action string
}

// failingTypedSink is a TypedSink that always returns an error
type failingTypedSink struct{}

func (f *failingTypedSink) EmitTyped(event *core.LogEvent, data UserActivity) error {
	return fmt.Errorf("typed emit failed")
}

func (f *failingTypedSink) Close() error {
	return nil
}

func TestTypedSinkSelfLog(t *testing.T) {
	t.Run("typed batching sink emit failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create typed batching sink with failing inner sink
		innerSink := &failingTypedSink{}
		batchingSink := sinks.NewTypedBatchingSink[UserActivity](innerSink, 2)

		// Create test events
		event1 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "User logged in",
			Properties:      map[string]any{},
		}
		data1 := UserActivity{UserID: "user1", Action: "login"}

		event2 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "User logged out",
			Properties:      map[string]any{},
		}
		data2 := UserActivity{UserID: "user1", Action: "logout"}

		// Emit events - second one should trigger batch flush and fail
		if err := batchingSink.EmitTyped(event1, data1); err != nil {
			t.Fatalf("unexpected error emitting first event: %v", err)
		}
		err := batchingSink.EmitTyped(event2, data2)

		if err == nil {
			t.Fatal("expected error from typed batching sink")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[typed-batching] failed to emit typed event 1/2") {
			t.Errorf("expected typed batching error in selflog, got: %s", output)
		}
	})

	t.Run("typed batching sink close failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create typed batching sink with failing inner sink
		innerSink := &failingTypedSink{}
		batchingSink := sinks.NewTypedBatchingSink[UserActivity](innerSink, 10)

		// Add one event to the batch
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "User action",
			Properties:      map[string]any{},
		}
		data := UserActivity{UserID: "user1", Action: "test"}

		if err := batchingSink.EmitTyped(event, data); err != nil {
			t.Fatalf("unexpected error emitting event: %v", err)
		}

		// Close should trigger flush which will fail
		err := batchingSink.Close()
		if err == nil {
			t.Fatal("expected error from close")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[typed-batching] failed to flush on close") {
			t.Errorf("expected flush error in selflog, got: %s", output)
		}
	})

	t.Run("filtered sink emit failure", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create filtered sink with failing inner sink
		innerSink := &failingTypedSink{}
		filteredSink := sinks.NewFilteredSink[UserActivity](innerSink, func(event *core.LogEvent, data UserActivity) bool {
			// Only emit login events
			return data.Action == "login"
		})

		// Create test event that passes filter
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "User logged in",
			Properties:      map[string]any{},
		}
		data := UserActivity{UserID: "user1", Action: "login"}

		// Emit event - should pass filter but fail in inner sink
		err := filteredSink.EmitTyped(event, data)
		if err == nil {
			t.Fatal("expected error from filtered sink")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[typed-filtered] failed to emit typed event") {
			t.Errorf("expected filtered sink error in selflog, got: %s", output)
		}
	})
}
