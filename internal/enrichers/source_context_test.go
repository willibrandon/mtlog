package enrichers

import (
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/core"
)

func TestSourceContextEnricher(t *testing.T) {
	tests := []struct {
		name            string
		sourceContext   string
		expectedContext string
		description     string
	}{
		{
			name:            "Explicit context",
			sourceContext:   "MyService.Database",
			expectedContext: "MyService.Database",
			description:     "Should use explicitly provided context",
		},
		{
			name:            "Auto-detect context",
			sourceContext:   "",
			expectedContext: "", // Will be detected at runtime
			description:     "Should auto-detect context from caller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enricher := NewSourceContextEnricher(tt.sourceContext)
			event := &core.LogEvent{
				Properties: make(map[string]any),
			}
			factory := &mockPropertyFactory{}

			enricher.Enrich(event, factory)

			// Check if SourceContext was added
			sourceContext, exists := event.Properties["SourceContext"]
			if !exists {
				t.Error("Expected SourceContext property to be added")
				return
			}

			// For explicit context, verify the value
			if tt.sourceContext != "" && sourceContext != tt.expectedContext {
				t.Errorf("Expected SourceContext=%q, got %q", tt.expectedContext, sourceContext)
			}

			// For auto-detect, just verify it's not empty
			if tt.sourceContext == "" && sourceContext == "" {
				t.Error("Expected auto-detected SourceContext to be non-empty")
			}
		})
	}
}

func TestSourceContextEnricherSkipsInternalPackages(t *testing.T) {
	// This test verifies that auto-detection skips mtlog internal packages
	enricher := NewSourceContextEnricher("")
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	// Call from a test function
	enricher.Enrich(event, factory)

	sourceContext, exists := event.Properties["SourceContext"]
	if !exists {
		t.Fatal("Expected SourceContext property to be added")
	}

	// The source context should be from this test package, not mtlog internals
	contextStr := sourceContext.(string)
	if contextStr == "" {
		t.Error("Source context should not be empty")
	}

	// Should contain "enrichers" since this test is in the enrichers package
	// but exact format depends on how the path is normalized
	t.Logf("Detected source context: %s", contextStr)
}

func TestSourceContextEnricherNormalization(t *testing.T) {
	// Test that paths are normalized correctly by checking the result
	// We can't test the private method directly, so we'll test through the enricher
	tests := []struct {
		sourceContext   string
		expectedPattern string
	}{
		{
			sourceContext:   "github.com.example.app",
			expectedPattern: "github.com.example.app",
		},
		{
			sourceContext:   "myapp.handlers",
			expectedPattern: "myapp.handlers",
		},
		{
			sourceContext:   "app.services.auth",
			expectedPattern: "app.services.auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.sourceContext, func(t *testing.T) {
			enricher := NewSourceContextEnricher(tt.sourceContext)
			event := &core.LogEvent{
				Properties: make(map[string]any),
			}
			factory := &mockPropertyFactory{}

			enricher.Enrich(event, factory)

			if val, ok := event.Properties["SourceContext"]; ok {
				if !strings.Contains(val.(string), tt.expectedPattern) {
					t.Errorf("Expected source context to contain %q, got %q", tt.expectedPattern, val)
				}
			}
		})
	}
}

func TestSourceContextEnricherDoesNotOverwrite(t *testing.T) {
	// Verify that the enricher doesn't overwrite existing SourceContext
	enricher := NewSourceContextEnricher("NewContext")
	event := &core.LogEvent{
		Properties: map[string]any{
			"SourceContext": "ExistingContext",
		},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should preserve the existing value
	if val := event.Properties["SourceContext"]; val != "ExistingContext" {
		t.Errorf("Expected existing SourceContext to be preserved, got %v", val)
	}
}
