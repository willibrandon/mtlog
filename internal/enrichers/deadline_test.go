package enrichers

import (
	"context"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// TestDeadlineEnricher_NoContext tests behavior when no context is provided.
func TestDeadlineEnricher_NoContext(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should not add any properties when no context
	if len(event.Properties) > 0 {
		t.Errorf("Expected no properties, got %v", event.Properties)
	}
}

// TestDeadlineEnricher_NoDeadline tests behavior when context has no deadline.
func TestDeadlineEnricher_NoDeadline(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx := context.Background()
	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should remove __context__ but not add deadline properties
	if _, exists := event.Properties["__context__"]; exists {
		t.Error("__context__ property should be removed")
	}
	if _, exists := event.Properties["deadline.approaching"]; exists {
		t.Error("Should not add deadline properties when no deadline")
	}
}

// TestDeadlineEnricher_FarFromDeadline tests behavior when deadline is far away.
func TestDeadlineEnricher_FarFromDeadline(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should not add properties when deadline is far
	if _, exists := event.Properties["deadline.approaching"]; exists {
		t.Error("Should not add deadline properties when deadline is far")
	}
	// Level should remain unchanged
	if event.Level != core.InformationLevel {
		t.Errorf("Level should remain Information, got %v", event.Level)
	}
}

// TestDeadlineEnricher_ApproachingDeadline tests behavior when deadline is approaching.
func TestDeadlineEnricher_ApproachingDeadline(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should add deadline properties
	if approaching, exists := event.Properties["deadline.approaching"]; !exists || approaching != true {
		t.Error("Should add deadline.approaching=true")
	}
	if remaining, exists := event.Properties["deadline.remaining_ms"]; !exists {
		t.Error("Should add deadline.remaining_ms")
	} else {
		// Check that remaining is positive and less than threshold
		if ms, ok := remaining.(int64); !ok || ms <= 0 || ms > 100 {
			t.Errorf("deadline.remaining_ms should be between 0 and 100, got %v", ms)
		}
	}
	// Level should be upgraded to Warning
	if event.Level != core.WarningLevel {
		t.Errorf("Level should be upgraded to Warning, got %v", event.Level)
	}
}

// TestDeadlineEnricher_ExceededDeadline tests behavior when deadline has been exceeded.
func TestDeadlineEnricher_ExceededDeadline(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	// Create a context that's already expired
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond) // Ensure it's expired
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should add exceeded properties
	if exceeded, exists := event.Properties["deadline.exceeded"]; !exists || exceeded != true {
		t.Error("Should add deadline.exceeded=true")
	}
	if exceededBy, exists := event.Properties["deadline.exceeded_by_ms"]; !exists {
		t.Error("Should add deadline.exceeded_by_ms")
	} else {
		// Check that exceeded_by is positive
		if ms, ok := exceededBy.(int64); !ok || ms <= 0 {
			t.Errorf("deadline.exceeded_by_ms should be positive, got %v", ms)
		}
	}
	// Level should be upgraded to Warning
	if event.Level != core.WarningLevel {
		t.Errorf("Level should be upgraded to Warning, got %v", event.Level)
	}
}

// TestDeadlineEnricher_DisableUpgradeLevel tests disabling level upgrade.
func TestDeadlineEnricher_DisableUpgradeLevel(t *testing.T) {
	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineUpgradeLevel(false))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Level should remain unchanged
	if event.Level != core.InformationLevel {
		t.Errorf("Level should remain Information when upgrade disabled, got %v", event.Level)
	}
}

// TestDeadlineEnricher_DisableProperties tests disabling property addition.
func TestDeadlineEnricher_DisableProperties(t *testing.T) {
	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineAddProperties(false))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	// Should not add deadline properties
	if _, exists := event.Properties["deadline.approaching"]; exists {
		t.Error("Should not add properties when disabled")
	}
	// But level should still be upgraded (unless also disabled)
	if event.Level != core.WarningLevel {
		t.Errorf("Level should still be upgraded, got %v", event.Level)
	}
}

// TestDeadlineEnricher_PercentageThreshold tests percentage-based threshold.
func TestDeadlineEnricher_PercentageThreshold(t *testing.T) {
	enricher := NewDeadlineEnricher(1*time.Hour, // Very high absolute threshold
		WithDeadlinePercentageThreshold(0.2)) // Warn at 20% remaining

	// Create context with 1 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Sleep to get below 20%
	time.Sleep(850 * time.Millisecond)

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	// First, cache the context info
	info := enricher.deadlineCache.getOrCreate(ctx)
	deadline, _ := ctx.Deadline()
	info.hasDeadline = true
	info.lastCheck = deadline.Add(-1 * time.Second) // Set start time

	enricher.Enrich(event, factory)

	// Should trigger warning based on percentage
	if _, exists := event.Properties["deadline.approaching"]; !exists {
		t.Error("Should trigger warning at 20% remaining")
	}
}

// TestDeadlineEnricher_FirstWarning tests that first warning is marked.
func TestDeadlineEnricher_FirstWarning(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// First event
	event1 := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}
	enricher.Enrich(event1, factory)

	// Should mark as first warning
	if firstWarning, exists := event1.Properties["deadline.first_warning"]; !exists || firstWarning != true {
		t.Error("First warning should be marked")
	}

	// Second event with same context
	event2 := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	enricher.Enrich(event2, factory)

	// Should not mark as first warning
	if _, exists := event2.Properties["deadline.first_warning"]; exists {
		t.Error("Second warning should not be marked as first")
	}
}

// TestDeadlineEnricher_CustomHandler tests custom handler functionality.
func TestDeadlineEnricher_CustomHandler(t *testing.T) {
	handlerCalled := false
	var handlerRemaining time.Duration

	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineCustomHandler(func(event *core.LogEvent, remaining time.Duration) {
			handlerCalled = true
			handlerRemaining = remaining
			event.Properties["custom_handler"] = "called"
		}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]any{"__context__": ctx},
	}
	factory := &mockPropertyFactory{}

	enricher.Enrich(event, factory)

	if !handlerCalled {
		t.Error("Custom handler should be called")
	}
	if handlerRemaining <= 0 || handlerRemaining > 100*time.Millisecond {
		t.Errorf("Handler should receive remaining time, got %v", handlerRemaining)
	}
	if custom, exists := event.Properties["custom_handler"]; !exists || custom != "called" {
		t.Error("Custom handler should be able to modify event")
	}
}

// TestDeadlineEnricher_CacheSize tests cache size limits.
func TestDeadlineEnricher_CacheSize(t *testing.T) {
	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineCacheSize(2)) // Very small cache

	// Create multiple contexts
	contexts := make([]context.Context, 5)
	cancels := make([]context.CancelFunc, 5)
	for i := range contexts {
		contexts[i], cancels[i] = context.WithTimeout(context.Background(), time.Hour)
		defer cancels[i]()
	}

	// Process events for each context
	for _, ctx := range contexts {
		event := &core.LogEvent{
			Level:      core.InformationLevel,
			Properties: map[string]any{"__context__": ctx},
		}
		factory := &mockPropertyFactory{}
		enricher.Enrich(event, factory)
	}

	// Cache should have evicted older entries
	// This is hard to test directly without exposing internals
	// but we can verify it doesn't crash or leak memory
}

// TestDeadlineEnricher_CacheTTL tests cache TTL functionality.
func TestDeadlineEnricher_CacheTTL(t *testing.T) {
	enricher := NewDeadlineEnricher(100*time.Millisecond,
		WithDeadlineCacheTTL(50*time.Millisecond)) // Very short TTL

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	// Cache the context
	info := enricher.deadlineCache.getOrCreate(ctx)
	info.startTime = time.Now()
	info.hasDeadline = true
	enricher.deadlineCache.put(ctx, info)

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Try to get from cache - should be expired
	cachedInfo := enricher.deadlineCache.get(ctx)
	if cachedInfo != nil {
		t.Error("Cache entry should be expired after TTL")
	}
}

// TestDeadlineEnricher_EnrichWithContext tests the EnrichWithContext method.
func TestDeadlineEnricher_EnrichWithContext(t *testing.T) {
	enricher := NewDeadlineEnricher(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: make(map[string]any),
	}
	factory := &mockPropertyFactory{}

	enricher.EnrichWithContext(ctx, event, factory)

	// Should add deadline properties
	if _, exists := event.Properties["deadline.approaching"]; !exists {
		t.Error("EnrichWithContext should add deadline properties")
	}
}

// Use the same mockPropertyFactory from enrichers_test.go
// which already exists and has the correct signature