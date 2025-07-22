package filters

import (
	"fmt"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

func TestLevelFilter(t *testing.T) {
	filter := NewLevelFilter(core.InformationLevel)
	
	tests := []struct {
		level    core.LogEventLevel
		expected bool
	}{
		{core.VerboseLevel, false},
		{core.DebugLevel, false},
		{core.InformationLevel, true},
		{core.WarningLevel, true},
		{core.ErrorLevel, true},
		{core.FatalLevel, true},
	}
	
	for _, tt := range tests {
		event := &core.LogEvent{Level: tt.level}
		result := filter.IsEnabled(event)
		if result != tt.expected {
			t.Errorf("Level %v: expected %v, got %v", tt.level, tt.expected, result)
		}
	}
}

func TestPredicateFilter(t *testing.T) {
	// Test basic predicate
	filter := NewPredicateFilter(func(event *core.LogEvent) bool {
		return event.Level >= core.WarningLevel
	})
	
	event1 := &core.LogEvent{Level: core.InformationLevel}
	if filter.IsEnabled(event1) {
		t.Error("Expected Information level to be filtered out")
	}
	
	event2 := &core.LogEvent{Level: core.WarningLevel}
	if !filter.IsEnabled(event2) {
		t.Error("Expected Warning level to pass")
	}
	
	// Test ByExcluding
	excludeFilter := ByExcluding(func(event *core.LogEvent) bool {
		msg, ok := event.Properties["Message"].(string)
		return ok && msg == "exclude me"
	})
	
	event3 := &core.LogEvent{
		Properties: map[string]interface{}{"Message": "exclude me"},
	}
	if excludeFilter.IsEnabled(event3) {
		t.Error("Expected event to be excluded")
	}
	
	event4 := &core.LogEvent{
		Properties: map[string]interface{}{"Message": "include me"},
	}
	if !excludeFilter.IsEnabled(event4) {
		t.Error("Expected event to be included")
	}
}

func TestExpressionFilter(t *testing.T) {
	// Test MatchProperty
	filter1 := MatchProperty("Environment", "Production")
	
	event1 := &core.LogEvent{
		Properties: map[string]interface{}{"Environment": "Production"},
	}
	if !filter1.IsEnabled(event1) {
		t.Error("Expected property match to pass")
	}
	
	event2 := &core.LogEvent{
		Properties: map[string]interface{}{"Environment": "Development"},
	}
	if filter1.IsEnabled(event2) {
		t.Error("Expected property mismatch to fail")
	}
	
	// Test MatchPropertyRegex
	filter2 := MatchPropertyRegex("Path", "^/api/.*")
	
	event3 := &core.LogEvent{
		Properties: map[string]interface{}{"Path": "/api/users"},
	}
	if !filter2.IsEnabled(event3) {
		t.Error("Expected regex match to pass")
	}
	
	event4 := &core.LogEvent{
		Properties: map[string]interface{}{"Path": "/health"},
	}
	if filter2.IsEnabled(event4) {
		t.Error("Expected regex mismatch to fail")
	}
	
	// Test MatchPropertyContains
	filter3 := MatchPropertyContains("Message", "error")
	
	event5 := &core.LogEvent{
		Properties: map[string]interface{}{"Message": "An error occurred"},
	}
	if !filter3.IsEnabled(event5) {
		t.Error("Expected substring match to pass")
	}
	
	// Test MatchPropertyExists
	filter4 := MatchPropertyExists("UserId")
	
	event6 := &core.LogEvent{
		Properties: map[string]interface{}{"UserId": 123},
	}
	if !filter4.IsEnabled(event6) {
		t.Error("Expected property existence check to pass")
	}
	
	event7 := &core.LogEvent{
		Properties: map[string]interface{}{},
	}
	if filter4.IsEnabled(event7) {
		t.Error("Expected property absence to fail")
	}
}

func TestSamplingFilter(t *testing.T) {
	// Test 50% sampling
	filter := NewSamplingFilter(0.5)
	
	passed := 0
	total := 1000
	
	for i := 0; i < total; i++ {
		event := &core.LogEvent{}
		if filter.IsEnabled(event) {
			passed++
		}
	}
	
	// Should be approximately 500 (allow 10% margin)
	expectedMin := int(float32(total) * 0.4)
	expectedMax := int(float32(total) * 0.6)
	
	if passed < expectedMin || passed > expectedMax {
		t.Errorf("Expected ~50%% sampling, got %d/%d", passed, total)
	}
	
	// Test 0% sampling
	filter0 := NewSamplingFilter(0.0)
	event := &core.LogEvent{}
	if filter0.IsEnabled(event) {
		t.Error("Expected 0% sampling to block all events")
	}
	
	// Test 100% sampling
	filter100 := NewSamplingFilter(1.0)
	if !filter100.IsEnabled(event) {
		t.Error("Expected 100% sampling to pass all events")
	}
}

func TestHashSamplingFilter(t *testing.T) {
	filter := NewHashSamplingFilter("UserId", 0.5)
	
	// Same user ID should always get same result
	userId := "user123"
	event1 := &core.LogEvent{
		Properties: map[string]interface{}{"UserId": userId},
	}
	event2 := &core.LogEvent{
		Properties: map[string]interface{}{"UserId": userId},
	}
	
	result1 := filter.IsEnabled(event1)
	result2 := filter.IsEnabled(event2)
	
	if result1 != result2 {
		t.Error("Expected same UserId to get consistent sampling result")
	}
	
	// Test distribution
	passed := 0
	total := 1000
	
	for i := 0; i < total; i++ {
		event := &core.LogEvent{
			Properties: map[string]interface{}{"UserId": fmt.Sprintf("user%d", i)},
		}
		if filter.IsEnabled(event) {
			passed++
		}
	}
	
	// Should be approximately 500 (allow 15% margin for hash distribution)
	expectedMin := int(float32(total) * 0.35)
	expectedMax := int(float32(total) * 0.65)
	
	if passed < expectedMin || passed > expectedMax {
		t.Errorf("Expected ~50%% hash sampling, got %d/%d", passed, total)
	}
}

func TestRateLimitFilter(t *testing.T) {
	// 5 events per 100ms window
	filter := NewRateLimitFilter(5, 100*int64(time.Millisecond))
	
	now := time.Now()
	
	// First 5 events should pass
	for i := 0; i < 5; i++ {
		event := &core.LogEvent{Timestamp: now}
		if !filter.IsEnabled(event) {
			t.Errorf("Expected event %d to pass rate limit", i+1)
		}
	}
	
	// 6th event should be blocked
	event := &core.LogEvent{Timestamp: now}
	if filter.IsEnabled(event) {
		t.Error("Expected 6th event to be blocked by rate limit")
	}
	
	// After window expires, events should pass again
	future := now.Add(150 * time.Millisecond)
	event2 := &core.LogEvent{Timestamp: future}
	if !filter.IsEnabled(event2) {
		t.Error("Expected event in new window to pass")
	}
}

func TestCompositeFilters(t *testing.T) {
	// Test AND composite (all must pass)
	levelFilter := NewLevelFilter(core.InformationLevel)
	propFilter := MatchProperty("Environment", "Production")
	
	andFilter := NewCompositeFilter(levelFilter, propFilter)
	
	// Should pass both filters
	event1 := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]interface{}{"Environment": "Production"},
	}
	if !andFilter.IsEnabled(event1) {
		t.Error("Expected event to pass both filters")
	}
	
	// Fails level filter
	event2 := &core.LogEvent{
		Level:      core.DebugLevel,
		Properties: map[string]interface{}{"Environment": "Production"},
	}
	if andFilter.IsEnabled(event2) {
		t.Error("Expected event to fail level filter")
	}
	
	// Test OR filter
	orFilter := NewOrFilter(
		MatchProperty("Priority", "High"),
		NewLevelFilter(core.ErrorLevel),
	)
	
	// Passes first condition
	event3 := &core.LogEvent{
		Level:      core.InformationLevel,
		Properties: map[string]interface{}{"Priority": "High"},
	}
	if !orFilter.IsEnabled(event3) {
		t.Error("Expected event to pass OR filter via priority")
	}
	
	// Passes second condition
	event4 := &core.LogEvent{
		Level:      core.ErrorLevel,
		Properties: map[string]interface{}{"Priority": "Low"},
	}
	if !orFilter.IsEnabled(event4) {
		t.Error("Expected event to pass OR filter via level")
	}
	
	// Test NOT filter
	notFilter := NewNotFilter(MatchProperty("Exclude", true))
	
	event5 := &core.LogEvent{
		Properties: map[string]interface{}{"Exclude": true},
	}
	if notFilter.IsEnabled(event5) {
		t.Error("Expected NOT filter to invert result")
	}
	
	event6 := &core.LogEvent{
		Properties: map[string]interface{}{},
	}
	if !notFilter.IsEnabled(event6) {
		t.Error("Expected NOT filter to pass when inner filter fails")
	}
}