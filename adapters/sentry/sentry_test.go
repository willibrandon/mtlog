package sentry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
)

func TestBreadcrumbBuffer(t *testing.T) {
	t.Run("AddAndRetrieve", func(t *testing.T) {
		buffer := NewBreadcrumbBuffer(5)

		// Add breadcrumbs
		for i := 0; i < 3; i++ {
			buffer.Add(sentry.Breadcrumb{
				Message: fmt.Sprintf("Breadcrumb %d", i),
			})
		}

		// Check size
		if buffer.Size() != 3 {
			t.Errorf("Expected size 3, got %d", buffer.Size())
		}

		// Get all breadcrumbs
		breadcrumbs := buffer.GetAll()
		if len(breadcrumbs) != 3 {
			t.Errorf("Expected 3 breadcrumbs, got %d", len(breadcrumbs))
		}
	})

	t.Run("Overflow", func(t *testing.T) {
		buffer := NewBreadcrumbBuffer(3)

		// Add more than capacity
		for i := 0; i < 5; i++ {
			buffer.Add(sentry.Breadcrumb{
				Message: fmt.Sprintf("Breadcrumb %d", i),
			})
		}

		// Should only keep last 3
		if buffer.Size() != 3 {
			t.Errorf("Expected size 3, got %d", buffer.Size())
		}

		breadcrumbs := buffer.GetAll()
		if len(breadcrumbs) != 3 {
			t.Errorf("Expected 3 breadcrumbs, got %d", len(breadcrumbs))
		}

		// Check that we have the latest ones
		if breadcrumbs[0].Message != "Breadcrumb 2" {
			t.Errorf("Expected oldest to be 'Breadcrumb 2', got %s", breadcrumbs[0].Message)
		}
	})

	t.Run("AgeEviction", func(t *testing.T) {
		buffer := NewBreadcrumbBuffer(10)
		buffer.SetMaxAge(100 * time.Millisecond)

		// Add old breadcrumb
		buffer.Add(sentry.Breadcrumb{
			Message: "Old",
		})

		// Wait for it to age out
		time.Sleep(150 * time.Millisecond)

		// Add new breadcrumb
		buffer.Add(sentry.Breadcrumb{
			Message: "New",
		})

		// Should only get the new one
		breadcrumbs := buffer.GetAll()
		if len(breadcrumbs) != 1 {
			t.Errorf("Expected 1 breadcrumb, got %d", len(breadcrumbs))
		}
		if breadcrumbs[0].Message != "New" {
			t.Errorf("Expected 'New', got %s", breadcrumbs[0].Message)
		}
	})

	t.Run("Clear", func(t *testing.T) {
		buffer := NewBreadcrumbBuffer(5)

		// Add breadcrumbs
		for i := 0; i < 3; i++ {
			buffer.Add(sentry.Breadcrumb{
				Message: fmt.Sprintf("Breadcrumb %d", i),
			})
		}

		// Clear
		buffer.Clear()

		// Should be empty
		if buffer.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", buffer.Size())
		}

		breadcrumbs := buffer.GetAll()
		if len(breadcrumbs) != 0 {
			t.Errorf("Expected 0 breadcrumbs after clear, got %d", len(breadcrumbs))
		}
	})
}

func TestLevelConversion(t *testing.T) {
	tests := []struct {
		mtlogLevel   core.LogEventLevel
		sentryLevel  sentry.Level
		category     string
	}{
		{core.VerboseLevel, sentry.LevelDebug, "debug"},
		{core.DebugLevel, sentry.LevelDebug, "debug"},
		{core.InformationLevel, sentry.LevelInfo, "info"},
		{core.WarningLevel, sentry.LevelWarning, "warning"},
		{core.ErrorLevel, sentry.LevelError, "error"},
		{core.FatalLevel, sentry.LevelFatal, "fatal"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Level_%d", tt.mtlogLevel), func(t *testing.T) {
			sentryLevel := levelToSentryLevel(tt.mtlogLevel)
			if sentryLevel != tt.sentryLevel {
				t.Errorf("Expected Sentry level %v, got %v", tt.sentryLevel, sentryLevel)
			}

			category := levelToCategory(tt.mtlogLevel)
			if category != tt.category {
				t.Errorf("Expected category %s, got %s", tt.category, category)
			}
		})
	}
}

func TestContextEnrichment(t *testing.T) {
	t.Run("UserContext", func(t *testing.T) {
		ctx := context.Background()
		user := sentry.User{
			ID:    "123",
			Email: "test@example.com",
		}

		ctx = WithUser(ctx, user)
		retrievedUser, ok := UserFromContext(ctx)
		if !ok {
			t.Error("Expected to retrieve user from context")
		}
		if retrievedUser.ID != user.ID {
			t.Errorf("Expected user ID %s, got %s", user.ID, retrievedUser.ID)
		}
	})

	t.Run("TagsContext", func(t *testing.T) {
		ctx := context.Background()
		tags := map[string]string{
			"env":     "test",
			"version": "1.0",
		}

		ctx = WithTags(ctx, tags)
		retrievedTags := TagsFromContext(ctx)
		if len(retrievedTags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(retrievedTags))
		}
		if retrievedTags["env"] != "test" {
			t.Errorf("Expected env=test, got %s", retrievedTags["env"])
		}
	})

	t.Run("MergeTags", func(t *testing.T) {
		ctx := context.Background()
		
		// Add initial tags
		ctx = WithTags(ctx, map[string]string{
			"env": "test",
			"foo": "bar",
		})
		
		// Add more tags
		ctx = WithTags(ctx, map[string]string{
			"version": "1.0",
			"foo":     "baz", // Override
		})
		
		retrievedTags := TagsFromContext(ctx)
		if len(retrievedTags) != 3 {
			t.Errorf("Expected 3 tags, got %d", len(retrievedTags))
		}
		if retrievedTags["foo"] != "baz" {
			t.Errorf("Expected foo=baz (overridden), got %s", retrievedTags["foo"])
		}
	})

	t.Run("EventEnrichment", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUser(ctx, sentry.User{ID: "123"})
		ctx = WithTags(ctx, map[string]string{"env": "test"})
		ctx = WithContext(ctx, "device", map[string]interface{}{
			"model": "test-device",
		})

		event := &sentry.Event{
			Message: "Test",
		}

		enrichEventFromContext(ctx, event)

		if event.User.ID != "123" {
			t.Errorf("Expected user ID 123, got %s", event.User.ID)
		}
		if event.Tags["env"] != "test" {
			t.Errorf("Expected env tag, got %v", event.Tags)
		}
		if event.Contexts["device"] == nil {
			t.Error("Expected device context")
		}
	})
}

func TestSentrySinkOptions(t *testing.T) {
	t.Run("MinLevel", func(t *testing.T) {
		sink := &SentrySink{
			minLevel: core.InformationLevel,
		}
		WithMinLevel(core.ErrorLevel)(sink)
		if sink.minLevel != core.ErrorLevel {
			t.Errorf("Expected min level Error, got %v", sink.minLevel)
		}
	})

	t.Run("SampleRate", func(t *testing.T) {
		sink := &SentrySink{}
		
		// Normal range
		WithSampleRate(0.5)(sink)
		if sink.sampleRate != 0.5 {
			t.Errorf("Expected sample rate 0.5, got %f", sink.sampleRate)
		}
		
		// Below range
		WithSampleRate(-0.1)(sink)
		if sink.sampleRate != 0 {
			t.Errorf("Expected sample rate 0, got %f", sink.sampleRate)
		}
		
		// Above range
		WithSampleRate(1.5)(sink)
		if sink.sampleRate != 1 {
			t.Errorf("Expected sample rate 1, got %f", sink.sampleRate)
		}
	})

	t.Run("BatchSize", func(t *testing.T) {
		sink := &SentrySink{}
		
		WithBatchSize(50)(sink)
		if sink.batchSize != 50 {
			t.Errorf("Expected batch size 50, got %d", sink.batchSize)
		}
		
		// Below minimum
		WithBatchSize(0)(sink)
		if sink.batchSize != 1 {
			t.Errorf("Expected batch size 1, got %d", sink.batchSize)
		}
	})

	t.Run("BatchTimeout", func(t *testing.T) {
		sink := &SentrySink{}
		
		WithBatchTimeout(10 * time.Second)(sink)
		if sink.batchTimeout != 10*time.Second {
			t.Errorf("Expected batch timeout 10s, got %v", sink.batchTimeout)
		}
		
		// Below minimum
		WithBatchTimeout(500 * time.Millisecond)(sink)
		if sink.batchTimeout != time.Second {
			t.Errorf("Expected batch timeout 1s, got %v", sink.batchTimeout)
		}
	})
}

func TestEventConversion(t *testing.T) {
	sink := &SentrySink{}

	t.Run("BasicEvent", func(t *testing.T) {
		logEvent := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Test message {Value}",
			Properties: map[string]interface{}{
				"Value": 42,
			},
		}

		sentryEvent := sink.convertToSentryEvent(logEvent)

		if sentryEvent.Level != sentry.LevelError {
			t.Errorf("Expected error level, got %v", sentryEvent.Level)
		}
		if sentryEvent.Message != "Test message {Value}" {
			t.Errorf("Expected template message, got %s", sentryEvent.Message)
		}
		if sentryEvent.Tags["message.template"] != "Test message {Value}" {
			t.Errorf("Expected template tag, got %s", sentryEvent.Tags["message.template"])
		}
		if sentryEvent.Extra["Value"] != 42 {
			t.Errorf("Expected Value in extra, got %v", sentryEvent.Extra["Value"])
		}
	})

	t.Run("EventWithError", func(t *testing.T) {
		testErr := errors.New("test error")
		logEvent := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Error occurred: {Error}",
			Properties: map[string]interface{}{
				"Error": testErr,
			},
		}

		sentryEvent := sink.convertToSentryEvent(logEvent)

		if len(sentryEvent.Exception) == 0 {
			t.Error("Expected exception to be extracted")
		}
		if sentryEvent.Exception[0].Value != "test error" {
			t.Errorf("Expected error message, got %s", sentryEvent.Exception[0].Value)
		}
	})

	t.Run("EventWithUser", func(t *testing.T) {
		logEvent := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "User action",
			Properties: map[string]interface{}{
				"User": sentry.User{
					ID:    "123",
					Email: "test@example.com",
				},
			},
		}

		sentryEvent := sink.convertToSentryEvent(logEvent)

		if sentryEvent.User.ID != "123" {
			t.Errorf("Expected user ID 123, got %s", sentryEvent.User.ID)
		}
	})

	t.Run("CustomFingerprint", func(t *testing.T) {
		sink.fingerprinter = func(event *core.LogEvent) []string {
			return []string{"custom", "fingerprint"}
		}

		logEvent := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Test",
			Properties:      map[string]interface{}{},
		}

		sentryEvent := sink.convertToSentryEvent(logEvent)

		if len(sentryEvent.Fingerprint) != 2 {
			t.Errorf("Expected 2 fingerprint parts, got %d", len(sentryEvent.Fingerprint))
		}
		if sentryEvent.Fingerprint[0] != "custom" {
			t.Errorf("Expected 'custom' fingerprint, got %s", sentryEvent.Fingerprint[0])
		}
	})
}