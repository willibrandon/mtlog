package enrichers

import (
	"context"
	"os"
	"testing"

	"github.com/willibrandon/mtlog/core"
)

// mockPropertyFactory is a test implementation of LogEventPropertyFactory.
type mockPropertyFactory struct{}

func (m *mockPropertyFactory) CreateProperty(name string, value any) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

func TestEnvironmentEnricher(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	// Test non-cached enricher
	enricher := NewEnvironmentEnricher("TEST_ENV_VAR", "TestProperty")
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	if val, ok := event.Properties["TestProperty"]; !ok || val != "test_value" {
		t.Errorf("Expected TestProperty='test_value', got %v", val)
	}

	// Test cached enricher
	cachedEnricher := NewEnvironmentEnricherCached("TEST_ENV_VAR", "CachedProperty")
	event2 := &core.LogEvent{
		Properties: make(map[string]any),
	}

	cachedEnricher.Enrich(event2, factory)

	if val, ok := event2.Properties["CachedProperty"]; !ok || val != "test_value" {
		t.Errorf("Expected CachedProperty='test_value', got %v", val)
	}

	// Change environment variable - cached enricher should still use old value
	os.Setenv("TEST_ENV_VAR", "new_value")

	event3 := &core.LogEvent{
		Properties: make(map[string]any),
	}

	cachedEnricher.Enrich(event3, factory)

	if val, ok := event3.Properties["CachedProperty"]; !ok || val != "test_value" {
		t.Errorf("Cached enricher should still use old value, got %v", val)
	}
}

func TestThreadIdEnricher(t *testing.T) {
	enricher := NewThreadIdEnricher()
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should have ThreadId property
	if _, ok := event.Properties["ThreadId"]; !ok {
		t.Error("Expected ThreadId property")
	}

	// Value should be an integer
	if val, ok := event.Properties["ThreadId"].(int); !ok || val <= 0 {
		t.Errorf("Expected positive integer ThreadId, got %v", event.Properties["ThreadId"])
	}
}

func TestCallersEnricher(t *testing.T) {
	enricher := NewCallersEnricher(1) // Skip 1 frame
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should have caller properties
	if _, ok := event.Properties["Caller"]; !ok {
		t.Error("Expected Caller property")
	}

	if _, ok := event.Properties["SourceFile"]; !ok {
		t.Error("Expected SourceFile property")
	}

	if _, ok := event.Properties["SourceLine"]; !ok {
		t.Error("Expected SourceLine property")
	}

	// Line number should be positive
	if line, ok := event.Properties["SourceLine"].(int); !ok || line <= 0 {
		t.Errorf("Expected positive line number, got %v", event.Properties["SourceLine"])
	}
}

func TestContextEnricher(t *testing.T) {
	// Create context with values
	ctx := context.Background()
	ctx = WithCorrelationId(ctx, "test-correlation-id")
	ctx = WithRequestId(ctx, "test-request-id")
	ctx = WithUserId(ctx, 12345)
	ctx = WithSessionId(ctx, "test-session-id")

	enricher := NewContextEnricher(ctx)
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Check all context values were added
	tests := []struct {
		property string
		expected any
	}{
		{"CorrelationId", "test-correlation-id"},
		{"RequestId", "test-request-id"},
		{"UserId", 12345},
		{"SessionId", "test-session-id"},
	}

	for _, tt := range tests {
		if val, ok := event.Properties[tt.property]; !ok || val != tt.expected {
			t.Errorf("Expected %s=%v, got %v", tt.property, tt.expected, val)
		}
	}
}

func TestContextValueEnricher(t *testing.T) {
	type customKey string
	const myKey customKey = "myCustomKey"

	ctx := context.WithValue(context.Background(), myKey, "custom-value")

	enricher := NewContextValueEnricher(ctx, myKey, "CustomProperty")
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	if val, ok := event.Properties["CustomProperty"]; !ok || val != "custom-value" {
		t.Errorf("Expected CustomProperty='custom-value', got %v", val)
	}
}

func TestCorrelationIdEnricher(t *testing.T) {
	enricher := NewCorrelationIdEnricher("fixed-correlation-id")
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	if val, ok := event.Properties["CorrelationId"]; !ok || val != "fixed-correlation-id" {
		t.Errorf("Expected CorrelationId='fixed-correlation-id', got %v", val)
	}
}

func TestCommonEnvironmentEnrichers(t *testing.T) {
	// Set some test environment variables
	os.Setenv("ENVIRONMENT", "test")
	os.Setenv("SERVICE_NAME", "test-service")
	defer os.Unsetenv("ENVIRONMENT")
	defer os.Unsetenv("SERVICE_NAME")

	enrichers := CommonEnvironmentEnrichers()

	// Should return multiple enrichers
	if len(enrichers) == 0 {
		t.Error("Expected multiple enrichers")
	}

	// Apply all enrichers
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	for _, enricher := range enrichers {
		enricher.Enrich(event, factory)
	}

	// Should have at least the environment variables we set
	if val, ok := event.Properties["Environment"]; !ok || val != "test" {
		t.Errorf("Expected Environment='test', got %v", val)
	}

	if val, ok := event.Properties["ServiceName"]; !ok || val != "test-service" {
		t.Errorf("Expected ServiceName='test-service', got %v", val)
	}
}
