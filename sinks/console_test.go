package sinks

import (
	"bytes"
	"strings"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

func TestConsoleSink(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer
	sink := NewConsoleSinkWithWriter(&buf)
	
	// Create a test event
	event := &core.LogEvent{
		Timestamp:       time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:           core.InformationLevel,
		MessageTemplate: "User {UserId} logged in",
		Properties: map[string]interface{}{
			"UserId": 123,
		},
	}
	
	sink.Emit(event)
	
	output := buf.String()
	
	// Check output contains expected elements
	if !strings.Contains(output, "[2024-01-15 10:30:45.000]") {
		t.Error("Output should contain timestamp")
	}
	
	if !strings.Contains(output, "[INF]") {
		t.Error("Output should contain log level")
	}
	
	if !strings.Contains(output, "User 123 logged in") {
		t.Error("Output should contain rendered message")
	}
}

func TestConsoleSinkLevels(t *testing.T) {
	tests := []struct {
		level    core.LogEventLevel
		expected string
	}{
		{core.VerboseLevel, "[VRB]"},
		{core.DebugLevel, "[DBG]"},
		{core.InformationLevel, "[INF]"},
		{core.WarningLevel, "[WRN]"},
		{core.ErrorLevel, "[ERR]"},
		{core.FatalLevel, "[FTL]"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			sink := NewConsoleSinkWithWriter(&buf)
			
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           tt.level,
				MessageTemplate: "Test message",
				Properties:      map[string]interface{}{},
			}
			
			sink.Emit(event)
			
			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %s, got: %s", tt.expected, output)
			}
		})
	}
}

func TestConsoleSinkClose(t *testing.T) {
	sink := NewConsoleSink()
	
	// Close should not error
	if err := sink.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestConsoleSinkWithTemplate(t *testing.T) {
	// Test custom output template
	var buf bytes.Buffer
	template := "[{Timestamp:HH:mm:ss} {Level:u3}] {SourceContext}: {Message}"
	
	sink, err := NewConsoleSinkWithTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create console sink with template: %v", err)
	}
	sink.output = &buf
	sink.useColor = false // Disable colors for testing
	
	event := &core.LogEvent{
		Timestamp:       time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:           core.WarningLevel,
		MessageTemplate: "Configuration value '{Key}' not found, using default: {Default}",
		Properties: map[string]interface{}{
			"Key":           "MaxRetries",
			"Default":       3,
			"SourceContext": "MyApp.Config",
		},
	}
	
	sink.Emit(event)
	
	output := strings.TrimSpace(buf.String())
	expected := "[10:30:45 WRN] MyApp.Config: Configuration value 'MaxRetries' not found, using default: 3"
	
	if output != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, output)
	}
}

func TestConsoleSinkTemplateWithColors(t *testing.T) {
	// Test that template rendering applies colors correctly
	var buf bytes.Buffer
	template := "[{Timestamp:HH:mm:ss} {Level:u3}] {Message}"
	
	sink, err := NewConsoleSinkWithTemplateAndTheme(template, LiterateTheme())
	if err != nil {
		t.Fatalf("Failed to create console sink with template and theme: %v", err)
	}
	sink.output = &buf
	sink.useColor = true
	
	event := &core.LogEvent{
		Timestamp:       time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:           core.ErrorLevel,
		MessageTemplate: "Failed to process {Count} items",
		Properties: map[string]interface{}{
			"Count": 5,
		},
	}
	
	sink.Emit(event)
	
	output := buf.String()
	
	// Check for ANSI color codes
	if !strings.Contains(output, "\033[38;5;7m") { // Gray for timestamp
		t.Error("Output should contain gray color for timestamp")
	}
	
	if !strings.Contains(output, "\033[38;5;9m") { // Red for error level
		t.Error("Output should contain red color for error level")
	}
	
	if !strings.Contains(output, "\033[38;5;51m") { // Cyan for property values
		t.Error("Output should contain cyan color for property values")
	}
}

func TestConsoleSinkTemplateWithMissingProperty(t *testing.T) {
	// Test handling of missing properties in template
	var buf bytes.Buffer
	template := "[{Level:u3}] {SourceContext}: {Message}"
	
	sink, err := NewConsoleSinkWithTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create console sink with template: %v", err)
	}
	sink.output = &buf
	sink.useColor = false
	
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Test message",
		Properties:      map[string]interface{}{}, // No SourceContext
	}
	
	sink.Emit(event)
	
	output := strings.TrimSpace(buf.String())
	
	// Should show placeholder for missing property
	if !strings.Contains(output, "{SourceContext}") {
		t.Error("Output should contain placeholder for missing SourceContext")
	}
}

func TestConsoleSinkNoColorTheme(t *testing.T) {
	// Test that NoColorTheme produces no ANSI codes
	var buf bytes.Buffer
	template := "[{Timestamp:HH:mm:ss} {Level:u3}] {Message}"
	
	sink, err := NewConsoleSinkWithTemplateAndTheme(template, NoColorTheme())
	if err != nil {
		t.Fatalf("Failed to create console sink with template and theme: %v", err)
	}
	sink.output = &buf
	
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.ErrorLevel,
		MessageTemplate: "Error occurred",
		Properties:      map[string]interface{}{},
	}
	
	sink.Emit(event)
	
	output := buf.String()
	
	// Should not contain any ANSI escape codes
	if strings.Contains(output, "\033[") {
		t.Error("NoColorTheme output should not contain ANSI escape codes")
	}
}