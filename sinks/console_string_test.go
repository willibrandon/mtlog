package sinks

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestConsoleSinkStringQuoting(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer
	sink := NewConsoleSinkWithWriter(&buf)

	// Test with string property
	event := &core.LogEvent{
		Timestamp:       time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:           core.InformationLevel,
		MessageTemplate: "User {Name} logged in",
		Properties: map[string]any{
			"Name": "John",
		},
	}

	sink.Emit(event)
	output := buf.String()

	t.Logf("Actual output: %s", output)

	// Check what we actually get
	if strings.Contains(output, "User \"John\" logged in") {
		t.Log("✓ String is quoted as expected")
	} else if strings.Contains(output, "User John logged in") {
		t.Error("✗ String is NOT quoted - still rendering as literal")
	} else {
		t.Errorf("✗ Unexpected output: %s", output)
	}
}