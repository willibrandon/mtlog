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