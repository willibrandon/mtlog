package sinks

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/formatters"
)

func TestSeqSink_CLEFFormat(t *testing.T) {
	// This test verifies that events would be formatted correctly for Seq
	// without actually sending them to a Seq instance
	
	formatter := formatters.NewCLEFFormatter()
	
	event := &core.LogEvent{
		Timestamp:       time.Date(2025, 1, 22, 10, 30, 45, 0, time.UTC),
		Level:           core.InformationLevel,
		MessageTemplate: "User {UserId} logged in from {IP}",
		Properties: map[string]interface{}{
			"UserId":      123,
			"IP":          "192.168.1.1",
			"Application": "mtlog-test",
		},
	}
	
	formatted, err := formatter.Format(event)
	if err != nil {
		t.Fatalf("Failed to format event: %v", err)
	}
	
	// Parse the formatted JSON
	var clef map[string]interface{}
	if err := json.Unmarshal(formatted, &clef); err != nil {
		t.Fatalf("Invalid CLEF JSON: %v", err)
	}
	
	// Verify required CLEF fields
	if clef["@t"] != "2025-01-22T10:30:45.0000000Z" {
		t.Errorf("Incorrect timestamp format: %v", clef["@t"])
	}
	
	if clef["@mt"] != "User {UserId} logged in from {IP}" {
		t.Errorf("Message template not preserved: %v", clef["@mt"])
	}
	
	if clef["@l"] != "Information" {
		t.Errorf("Incorrect level: %v", clef["@l"])
	}
	
	// Verify properties
	if clef["UserId"] != float64(123) { // JSON numbers are float64
		t.Errorf("UserId property incorrect: %v", clef["UserId"])
	}
	
	if clef["IP"] != "192.168.1.1" {
		t.Errorf("IP property incorrect: %v", clef["IP"])
	}
	
	if clef["Application"] != "mtlog-test" {
		t.Errorf("Application property incorrect: %v", clef["Application"])
	}
}

func TestSeqSink_BatchFormatting(t *testing.T) {
	formatter := formatters.NewCLEFBatchFormatter()
	
	events := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event 1",
			Properties:      map[string]interface{}{"id": 1},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.WarningLevel,
			MessageTemplate: "Event 2",
			Properties:      map[string]interface{}{"id": 2},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.ErrorLevel,
			MessageTemplate: "Event 3",
			Properties:      map[string]interface{}{"id": 3},
		},
	}
	
	formatted, err := formatter.FormatBatch(events)
	if err != nil {
		t.Fatalf("Failed to format batch: %v", err)
	}
	
	// Should be newline-delimited JSON
	lines := strings.Split(strings.TrimSpace(string(formatted)), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
	
	// Verify each line is valid CLEF
	levels := []string{"Information", "Warning", "Error"}
	for i, line := range lines {
		var clef map[string]interface{}
		if err := json.Unmarshal([]byte(line), &clef); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
		
		// Check level
		if clef["@l"] != levels[i] {
			t.Errorf("Line %d: expected level %s, got %v", i+1, levels[i], clef["@l"])
		}
		
		// Check id property
		if id, ok := clef["id"].(float64); !ok || int(id) != i+1 {
			t.Errorf("Line %d: expected id=%d, got %v", i+1, i+1, clef["id"])
		}
	}
}

func TestSeqIngestionFormat(t *testing.T) {
	events := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test event 1",
			Properties:      map[string]interface{}{"test": true},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.DebugLevel,
			MessageTemplate: "Test event 2",
			Properties:      map[string]interface{}{"debug": true},
		},
	}
	
	formatted, err := formatters.FormatForSeqIngestion(events)
	if err != nil {
		t.Fatalf("Failed to format for ingestion: %v", err)
	}
	
	// Should be newline-delimited CLEF
	lines := strings.Split(strings.TrimSpace(string(formatted)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
	
	// Verify each event is valid CLEF
	for i, line := range lines {
		var clef map[string]interface{}
		if err := json.Unmarshal([]byte(line), &clef); err != nil {
			t.Errorf("Event %d is not valid JSON: %v", i+1, err)
		}
		
		// Check required CLEF fields
		if _, ok := clef["@t"]; !ok {
			t.Errorf("Event %d missing @t field", i+1)
		}
		if _, ok := clef["@mt"]; !ok {
			t.Errorf("Event %d missing @mt field", i+1)
		}
		if _, ok := clef["@l"]; !ok {
			t.Errorf("Event %d missing @l field", i+1)
		}
	}
}