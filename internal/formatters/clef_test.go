package formatters

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestCLEFFormatter_Format(t *testing.T) {
	formatter := NewCLEFFormatter()
	timestamp := time.Date(2025, 1, 15, 10, 30, 45, 123456700, time.UTC)

	tests := []struct {
		name     string
		event    *core.LogEvent
		validate func(t *testing.T, clef map[string]any)
	}{
		{
			name: "basic event",
			event: &core.LogEvent{
				Timestamp:       timestamp,
				Level:           core.InformationLevel,
				MessageTemplate: "User {UserId} logged in",
				Properties: map[string]any{
					"UserId": 123,
				},
			},
			validate: func(t *testing.T, clef map[string]any) {
				if clef["@t"] != "2025-01-15T10:30:45.1234567Z" {
					t.Errorf("Expected timestamp '2025-01-15T10:30:45.1234567Z', got %v", clef["@t"])
				}
				if clef["@mt"] != "User {UserId} logged in" {
					t.Errorf("Expected template 'User {UserId} logged in', got %v", clef["@mt"])
				}
				if clef["@l"] != "Information" {
					t.Errorf("Expected level 'Information', got %v", clef["@l"])
				}
				if clef["UserId"] != float64(123) { // JSON unmarshal converts to float64
					t.Errorf("Expected UserId=123, got %v", clef["UserId"])
				}
			},
		},
		{
			name: "with exception",
			event: &core.LogEvent{
				Timestamp:       timestamp,
				Level:           core.ErrorLevel,
				MessageTemplate: "Operation failed",
				Properties: map[string]any{
					"Exception": errors.New("database connection failed"),
					"Operation": "SaveUser",
				},
			},
			validate: func(t *testing.T, clef map[string]any) {
				if clef["@l"] != "Error" {
					t.Errorf("Expected level 'Error', got %v", clef["@l"])
				}
				if clef["@x"] != "database connection failed" {
					t.Errorf("Expected exception 'database connection failed', got %v", clef["@x"])
				}
				if clef["Operation"] != "SaveUser" {
					t.Errorf("Expected Operation='SaveUser', got %v", clef["Operation"])
				}
			},
		},
		{
			name: "with rendered message",
			event: &core.LogEvent{
				Timestamp:       timestamp,
				Level:           core.DebugLevel,
				MessageTemplate: "Processing {Count} items from {Source}",
				Properties: map[string]any{
					"Count":  42,
					"Source": "API",
				},
			},
			validate: func(t *testing.T, clef map[string]any) {
				if clef["@m"] != "Processing 42 items from API" {
					t.Errorf("Expected rendered message 'Processing 42 items from API', got %v", clef["@m"])
				}
			},
		},
		{
			name: "all log levels",
			event: &core.LogEvent{
				Timestamp:       timestamp,
				Level:           core.VerboseLevel,
				MessageTemplate: "Test",
				Properties:      map[string]any{},
			},
			validate: func(t *testing.T, clef map[string]any) {
				if clef["@l"] != "Verbose" {
					t.Errorf("Expected level 'Verbose', got %v", clef["@l"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted, err := formatter.Format(tt.event)
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}

			var clef map[string]any
			if err := json.Unmarshal(formatted, &clef); err != nil {
				t.Fatalf("Invalid JSON: %v", err)
			}

			tt.validate(t, clef)
		})
	}
}

func TestCLEFFormatter_RenderMessage(t *testing.T) {
	formatter := NewCLEFFormatter()

	tests := []struct {
		template   string
		properties map[string]any
		expected   string
	}{
		{
			template: "User {UserId} logged in from {IP}",
			properties: map[string]any{
				"UserId": 123,
				"IP":     "192.168.1.1",
			},
			expected: "User 123 logged in from 192.168.1.1",
		},
		{
			template: "Processing {@Order} with total {$Total}",
			properties: map[string]any{
				"Order": map[string]any{"id": 456},
				"Total": 99.99,
			},
			expected: "Processing map[id:456] with total 99.99",
		},
		{
			template: "Missing {Property} in template",
			properties: map[string]any{
				"Other": "value",
			},
			expected: "Missing {Property} in template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			event := &core.LogEvent{
				MessageTemplate: tt.template,
				Properties:      tt.properties,
			}

			rendered, err := formatter.renderMessage(event)
			if err != nil {
				t.Fatalf("renderMessage failed: %v", err)
			}

			if rendered != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, rendered)
			}
		})
	}
}

func TestCLEFBatchFormatter(t *testing.T) {
	formatter := NewCLEFBatchFormatter()

	events := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event 1",
			Properties:      map[string]any{"id": 1},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.WarningLevel,
			MessageTemplate: "Event 2",
			Properties:      map[string]any{"id": 2},
		},
	}

	formatted, err := formatter.FormatBatch(events)
	if err != nil {
		t.Fatalf("FormatBatch failed: %v", err)
	}

	// Should be newline-delimited JSON
	lines := strings.Split(strings.TrimSpace(string(formatted)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var clef map[string]any
		if err := json.Unmarshal([]byte(line), &clef); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
	}
}

func TestFormatForSeqIngestion(t *testing.T) {
	events := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test event",
			Properties:      map[string]any{"test": true},
		},
	}

	formatted, err := FormatForSeqIngestion(events)
	if err != nil {
		t.Fatalf("FormatForSeqIngestion failed: %v", err)
	}

	// Should be newline-delimited CLEF
	lines := strings.Split(strings.TrimSpace(string(formatted)), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
	}

	// Verify the event is valid CLEF
	var clef map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &clef); err != nil {
		t.Errorf("Event is not valid JSON: %v", err)
	}

	if clef["@mt"] != "Test event" {
		t.Errorf("Expected template 'Test event', got %v", clef["@mt"])
	}
}
