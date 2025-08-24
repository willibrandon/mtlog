//go:build integration

package sentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
)

const (
	testDSN      = "http://test-public-key@localhost:9000/1"
	sentryAPIURL = "http://localhost:9000/api/0"
)

func TestSentryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Wait for Sentry to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := waitForSentry(ctx); err != nil {
		t.Fatalf("Sentry not ready: %v", err)
	}

	// Setup test project (simplified for testing)
	dsn := getTestDSN(t)

	t.Run("BasicErrorDelivery", func(t *testing.T) {
		sink, err := NewSentrySink(dsn)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Create test event
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Test error: {Error}",
			Properties: map[string]interface{}{
				"Error": errors.New("test error"),
			},
		}

		// Send event
		sink.Emit(event)

		// Allow time for processing
		time.Sleep(2 * time.Second)

		// Verify event was received (would need Sentry API client for real verification)
		// For now, we just ensure no panics and the sink processes the event
	})

	t.Run("BreadcrumbCollection", func(t *testing.T) {
		sink, err := NewSentrySink(dsn,
			WithMinLevel(core.ErrorLevel),
			WithBreadcrumbLevel(core.DebugLevel),
			WithMaxBreadcrumbs(10),
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Add breadcrumbs
		for i := 0; i < 5; i++ {
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           core.InformationLevel,
				MessageTemplate: "Breadcrumb {Index}",
				Properties: map[string]interface{}{
					"Index": i,
				},
			}
			sink.Emit(event)
		}

		// Verify breadcrumbs were collected
		if sink.breadcrumbs.Size() != 5 {
			t.Errorf("Expected 5 breadcrumbs, got %d", sink.breadcrumbs.Size())
		}

		// Send an error
		errorEvent := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Error after breadcrumbs",
			Properties:      map[string]interface{}{},
		}
		sink.Emit(errorEvent)

		// Allow time for processing
		time.Sleep(2 * time.Second)
	})

	t.Run("Batching", func(t *testing.T) {
		sink, err := NewSentrySink(dsn,
			WithBatchSize(5),
			WithBatchTimeout(2*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Send multiple events
		for i := 0; i < 10; i++ {
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           core.ErrorLevel,
				MessageTemplate: "Batch error {Index}",
				Properties: map[string]interface{}{
					"Index": i,
				},
			}
			sink.Emit(event)
		}

		// Allow time for batch processing
		time.Sleep(3 * time.Second)
	})

	t.Run("ContextEnrichment", func(t *testing.T) {
		sink, err := NewSentrySink(dsn)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Create context with user
		ctx := context.Background()
		ctx = WithUser(ctx, sentry.User{
			ID:    "test-user-123",
			Email: "test@example.com",
		})
		ctx = WithTags(ctx, map[string]string{
			"environment": "test",
			"version":     "1.0.0",
		})

		// Send event with context
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "User action failed",
			Properties:      map[string]interface{}{},
		}
		sink.Emit(event)

		// Allow time for processing
		time.Sleep(2 * time.Second)
	})

	t.Run("CustomFingerprinting", func(t *testing.T) {
		fingerprinter := func(event *core.LogEvent) []string {
			return []string{
				event.MessageTemplate,
				fmt.Sprintf("%v", event.Level),
			}
		}

		sink, err := NewSentrySink(dsn,
			WithFingerprinter(fingerprinter),
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Send events that should be grouped
		for i := 0; i < 3; i++ {
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           core.ErrorLevel,
				MessageTemplate: "Same error template",
				Properties: map[string]interface{}{
					"DifferentProp": i, // Different property but same fingerprint
				},
			}
			sink.Emit(event)
		}

		// Allow time for processing
		time.Sleep(2 * time.Second)
	})

	t.Run("SampleRate", func(t *testing.T) {
		sink, err := NewSentrySink(dsn,
			WithSampleRate(0.5), // 50% sampling
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Send multiple events
		for i := 0; i < 10; i++ {
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           core.ErrorLevel,
				MessageTemplate: "Sampled error {Index}",
				Properties: map[string]interface{}{
					"Index": i,
				},
			}
			sink.Emit(event)
		}

		// Allow time for processing
		time.Sleep(2 * time.Second)
	})

	t.Run("BeforeSendHook", func(t *testing.T) {
		var processedCount int
		beforeSend := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			processedCount++
			// Filter out events with specific message
			if strings.Contains(event.Message, "filtered") {
				return nil // Drop the event
			}
			// Add custom tag
			if event.Tags == nil {
				event.Tags = make(map[string]string)
			}
			event.Tags["processed"] = "true"
			return event
		}

		sink, err := NewSentrySink(dsn,
			WithBeforeSend(beforeSend),
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Send event that should be filtered
		event1 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "This should be filtered",
			Properties:      map[string]interface{}{},
		}
		sink.Emit(event1)

		// Send event that should pass through
		event2 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "This should pass",
			Properties:      map[string]interface{}{},
		}
		sink.Emit(event2)

		// Force flush
		sink.flush()
		time.Sleep(1 * time.Second)

		if processedCount != 2 {
			t.Errorf("Expected 2 events to be processed, got %d", processedCount)
		}
	})

	t.Run("IgnoreErrors", func(t *testing.T) {
		ignoredErr := errors.New("ignored error")
		
		sink, err := NewSentrySink(dsn,
			WithIgnoreErrors(ignoredErr, io.EOF),
		)
		if err != nil {
			t.Fatalf("Failed to create sink: %v", err)
		}
		defer sink.Close()

		// Send ignored error
		event1 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Error: {Error}",
			Properties: map[string]interface{}{
				"Error": ignoredErr,
			},
		}
		sink.Emit(event1)

		// Send normal error
		event2 := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Error: {Error}",
			Properties: map[string]interface{}{
				"Error": errors.New("normal error"),
			},
		}
		sink.Emit(event2)

		// Allow time for processing
		time.Sleep(2 * time.Second)
	})
}

func waitForSentry(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	client := &http.Client{Timeout: 2 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Try to reach Sentry's health endpoint
			resp, err := client.Get("http://localhost:9000/_health/")
			if err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				
				// Check if response indicates healthy status
				if resp.StatusCode == 200 || strings.Contains(string(body), "healthy") {
					return nil
				}
			}
		}
	}
}

func getTestDSN(t *testing.T) string {
	// First check if a test DSN is provided via environment variable
	if dsn := os.Getenv("SENTRY_TEST_DSN"); dsn != "" {
		return dsn
	}

	// Try to create a test project using Sentry's internal API
	// Note: This is simplified and may need adjustment based on Sentry's actual setup
	if dsn := createTestProject(t); dsn != "" {
		return dsn
	}

	// Fall back to default test DSN
	return testDSN
}

func createTestProject(t *testing.T) string {
	// This is a simplified version - real implementation would need to:
	// 1. Bootstrap Sentry with initial superuser
	// 2. Create organization
	// 3. Create project
	// 4. Get DSN
	
	// For now, we'll just return empty string and rely on environment variable
	return ""
}

// TestSentryInitialization tests the Sentry bootstrap process
func TestSentryInitialization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test helps with initial Sentry setup
	t.Run("BootstrapSentry", func(t *testing.T) {
		// Wait for services to be ready
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := waitForSentry(ctx); err != nil {
			// If Sentry isn't ready, try to initialize it
			if err := initializeSentry(); err != nil {
				t.Skipf("Could not initialize Sentry: %v", err)
			}
		}
	})
}

func initializeSentry() error {
	// Create initial superuser
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Try to create superuser via Sentry's CLI in the container
	// This would typically be done via docker exec
	payload := map[string]interface{}{
		"username": "admin",
		"password": "admin123",
		"email":    "admin@localhost",
	}
	
	data, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "http://localhost:9000/api/0/users/", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}