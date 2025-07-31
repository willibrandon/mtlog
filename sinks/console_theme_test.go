package sinks

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestConsoleThemes(t *testing.T) {
	// Create a test event
	event := &core.LogEvent{
		Timestamp:       time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC),
		Level:          core.InformationLevel,
		MessageTemplate: "User {UserId} logged in from {IP}",
		Properties: map[string]interface{}{
			"UserId": 123,
			"IP":     "192.168.1.1",
		},
	}

	tests := []struct {
		name           string
		theme          *ConsoleTheme
		expectedParts  []string
		notExpected    []string
		checkNoColor   bool
	}{
		{
			name:  "Default theme",
			theme: DefaultTheme(),
			expectedParts: []string{
				"[2025-01-22 15:30:45.000]",
				"[INF]",
				"User 123 logged in from 192.168.1.1",
			},
			checkNoColor: true,
		},
		{
			name:  "Lite theme",
			theme: LiteTheme(),
			expectedParts: []string{
				"[15:30:45]",
				"INF",
				"User 123 logged in from 192.168.1.1",
			},
			checkNoColor: true,
		},
		{
			name:  "Dev theme",
			theme: DevTheme(),
			expectedParts: []string{
				"[2025-01-22 15:30:45.000]",
				"[INF  ]", // Fixed width
				"User 123 logged in from 192.168.1.1",
			},
			checkNoColor: true,
		},
		{
			name:  "No color theme",
			theme: NoColorTheme(),
			expectedParts: []string{
				"[2025-01-22 15:30:45.000]",
				"[INF]",
				"User 123 logged in from 192.168.1.1",
			},
			notExpected: []string{
				"\033[", // No ANSI escape codes
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with color enabled
			buf := &bytes.Buffer{}
			sink := &ConsoleSink{
				output:   buf,
				theme:    tt.theme,
				useColor: true,
			}

			sink.Emit(event)
			output := buf.String()

			// Check expected parts
			for _, part := range tt.expectedParts {
				if !strings.Contains(output, part) {
					t.Errorf("Expected output to contain %q, got: %s", part, output)
				}
			}

			// Check not expected parts
			for _, part := range tt.notExpected {
				if strings.Contains(output, part) {
					t.Errorf("Expected output NOT to contain %q, got: %s", part, output)
				}
			}

			// Test with color disabled if applicable
			if tt.checkNoColor {
				buf.Reset()
				sink.useColor = false
				sink.Emit(event)
				output = buf.String()

				// Should not contain any ANSI escape codes
				if strings.Contains(output, "\033[") {
					t.Errorf("Expected no ANSI codes with color disabled, got: %s", output)
				}
			}
		})
	}
}

func TestConsoleThemeWithProperties(t *testing.T) {
	event := &core.LogEvent{
		Timestamp:       time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC),
		Level:          core.InformationLevel,
		MessageTemplate: "Processing order {OrderId}",
		Properties: map[string]interface{}{
			"OrderId":    "ORD-123",
			"CustomerId": "CUST-456",
			"Total":      99.99,
		},
	}

	buf := &bytes.Buffer{}
	sink := &ConsoleSink{
		output:         buf,
		theme:          DefaultTheme(),
		useColor:       true,
		showProperties: true,
	}

	sink.Emit(event)
	output := buf.String()

	// Should show the rendered message
	if !strings.Contains(output, "Processing order ORD-123") {
		t.Errorf("Expected rendered message, got: %s", output)
	}

	// Should show extra properties
	if !strings.Contains(output, "CustomerId") {
		t.Errorf("Expected CustomerId property, got: %s", output)
	}
	if !strings.Contains(output, "Total") {
		t.Errorf("Expected Total property, got: %s", output)
	}
}

func TestConsoleThemeLevels(t *testing.T) {
	levels := []struct {
		level    core.LogEventLevel
		levelStr string
	}{
		{core.VerboseLevel, "VRB"},
		{core.DebugLevel, "DBG"},
		{core.InformationLevel, "INF"},
		{core.WarningLevel, "WRN"},
		{core.ErrorLevel, "ERR"},
		{core.FatalLevel, "FTL"},
	}

	theme := DefaultTheme()
	buf := &bytes.Buffer{}
	sink := &ConsoleSink{
		output:   buf,
		theme:    theme,
		useColor: true,
	}

	for _, lvl := range levels {
		t.Run(lvl.levelStr, func(t *testing.T) {
			buf.Reset()
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:          lvl.level,
				MessageTemplate: "Test message",
				Properties:     map[string]interface{}{},
			}

			sink.Emit(event)
			output := buf.String()

			// Check that the level string appears
			if !strings.Contains(output, lvl.levelStr) {
				t.Errorf("Expected level %s in output, got: %s", lvl.levelStr, output)
			}

			// For error and fatal levels, check they use appropriate colors
			if lvl.level >= core.ErrorLevel {
				// Should contain red color code
				if !strings.Contains(output, "\033[") {
					t.Errorf("Expected color codes for %s level", lvl.levelStr)
				}
			}
		})
	}
}

func TestSetTheme(t *testing.T) {
	buf := &bytes.Buffer{}
	sink := NewConsoleSinkWithWriter(buf)

	// Start with default theme
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:          core.InformationLevel,
		MessageTemplate: "Test",
		Properties:     map[string]interface{}{},
	}

	sink.Emit(event)
	output1 := buf.String()

	// Change to lite theme
	buf.Reset()
	sink.SetTheme(LiteTheme())
	sink.Emit(event)
	output2 := buf.String()

	// Outputs should be different (different timestamp formats)
	if output1 == output2 {
		t.Error("Expected different output with different themes")
	}

	// Lite theme should have shorter timestamp
	if len(output2) >= len(output1) {
		t.Error("Expected lite theme to have shorter output")
	}
}

func TestShouldUseColor(t *testing.T) {
	// This is hard to test comprehensively without mocking runtime
	// Just ensure it doesn't panic
	_ = shouldUseColor(&bytes.Buffer{})
	_ = shouldUseColor(nil)
}

func TestColorize(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		color    Color
		useColor bool
		expected string
	}{
		{
			name:     "With color",
			text:     "Hello",
			color:    ColorRed,
			useColor: true,
			expected: "\033[31mHello\033[0m",
		},
		{
			name:     "Without color",
			text:     "Hello",
			color:    ColorRed,
			useColor: false,
			expected: "Hello",
		},
		{
			name:     "Empty color",
			text:     "Hello",
			color:    "",
			useColor: true,
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := colorize(tt.text, tt.color, tt.useColor)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}