package mtlog

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

// TestGetAvailableProfileDescriptions tests the profile discovery feature
func TestGetAvailableProfileDescriptions(t *testing.T) {
	descriptions := GetAvailableProfileDescriptions()
	
	// Check that we have at least the predefined profiles
	expectedProfiles := []string{
		"HighTrafficAPI",
		"BackgroundWorker",
		"DebugVerbose",
		"ProductionErrors",
		"HealthChecks",
		"CriticalAlerts",
	}
	
	for _, expected := range expectedProfiles {
		if desc, exists := descriptions[expected]; !exists {
			t.Errorf("Expected profile %s not found in descriptions", expected)
		} else if desc == "" {
			t.Errorf("Profile %s has empty description", expected)
		}
	}
	
	// Verify that descriptions are non-empty
	for name, desc := range descriptions {
		if desc == "" {
			t.Errorf("Profile %s has empty description", name)
		}
	}
}

// TestGetSamplingMetrics tests the sampling metrics feature
func TestGetSamplingMetrics(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Use Sample method which properly sets up the sampling filter
	sampledLogger := logger.Sample(2) // Sample every 2nd message
	
	// Log some messages
	for i := 0; i < 10; i++ {
		sampledLogger.Info("Test message %d", i)
	}
	
	// Get stats from the sampled logger
	sampled, skipped := sampledLogger.GetSamplingStats()
	
	// We should have some sampled and skipped messages
	if sampled == 0 {
		t.Error("Expected some sampled messages")
	}
	
	if skipped == 0 {
		t.Error("Expected some skipped messages")
	}
	
	// Total should be 10
	total := sampled + skipped
	if total != 10 {
		t.Errorf("Expected total of 10 messages, got %d", total)
	}
}

// TestEnableSamplingDebug tests the sampling debug logging feature
func TestEnableSamplingDebug(t *testing.T) {
	// Capture selflog output
	var buf bytes.Buffer
	selflog.Enable(&buf)
	defer selflog.Disable()
	
	// Enable sampling debug
	EnableSamplingDebug()
	defer DisableSamplingDebug()
	
	// Create logger with sampling
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	sampledLogger := logger.Sample(3) // Every 3rd message
	
	// Log some messages
	for i := 1; i <= 5; i++ {
		sampledLogger.Info("Debug test message %d", i)
	}
	
	// Check that we got debug output
	output := buf.String()
	if !strings.Contains(output, "[Sampling]") {
		t.Error("Expected sampling debug output in selflog")
	}
	
	// Should see both SAMPLE and SKIP decisions
	if !strings.Contains(output, "Decision=SAMPLE") {
		t.Error("Expected to see SAMPLE decisions in debug output")
	}
	
	if !strings.Contains(output, "Decision=SKIP") {
		t.Error("Expected to see SKIP decisions in debug output")
	}
	
	// Should see the mode
	if !strings.Contains(output, "Mode=Counter") {
		t.Error("Expected to see Counter mode in debug output")
	}
	
	// Should see the template
	if !strings.Contains(output, "Template=\"Debug test message") {
		t.Error("Expected to see message template in debug output")
	}
}

// TestIsSamplingDebugEnabled tests the debug state query
func TestIsSamplingDebugEnabled(t *testing.T) {
	// Initially should be disabled
	if IsSamplingDebugEnabled() {
		t.Error("Sampling debug should be disabled by default")
	}
	
	// Enable and check
	EnableSamplingDebug()
	if !IsSamplingDebugEnabled() {
		t.Error("Sampling debug should be enabled after calling EnableSamplingDebug")
	}
	
	// Disable and check
	DisableSamplingDebug()
	if IsSamplingDebugEnabled() {
		t.Error("Sampling debug should be disabled after calling DisableSamplingDebug")
	}
}

// TestSamplingMetricsWithMultipleStrategies tests metrics with various sampling strategies
func TestSamplingMetricsWithMultipleStrategies(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Test different sampling strategies
	strategies := []struct {
		name   string
		logger core.Logger
		count  int
	}{
		{"Counter", logger.Sample(2), 10},
		{"Rate", logger.SampleRate(0.5), 20},
		{"First", logger.SampleFirst(5), 10},
	}
	
	for _, strategy := range strategies {
		t.Run(strategy.name, func(t *testing.T) {
			// Clear any previous state by creating new logger
			testSink := sinks.NewMemorySink()
			testLogger := New(WithSink(testSink))
			var sampledLogger core.Logger
			
			switch strategy.name {
			case "Counter":
				sampledLogger = testLogger.Sample(2)
			case "Rate":
				sampledLogger = testLogger.SampleRate(0.5)
			case "First":
				sampledLogger = testLogger.SampleFirst(5)
			}
			
			// Log messages
			for i := 0; i < strategy.count; i++ {
				sampledLogger.Info("Test %s message %d", strategy.name, i)
			}
			
			// Get stats - using GetSamplingStats from the interface
			sampled, skipped := sampledLogger.GetSamplingStats()
			
			// Verify we have some activity
			total := sampled + skipped
			if total == 0 {
				t.Errorf("%s: Expected some messages in stats", strategy.name)
			}
		})
	}
}