package mtlog

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/sinks"
)

func TestNumericIndexing(t *testing.T) {
	// Create a buffer to capture output
	buf := &bytes.Buffer{}
	logger := New(WithSink(sinks.NewConsoleSinkWithWriter(buf)))

	tests := []struct {
		name     string
		template string
		args     []any
		expected string
	}{
		{
			name:     "Simple numeric indexing",
			template: "Processing {0} of {1}",
			args:     []any{5, 10},
			expected: "Processing 5 of 10",
		},
		{
			name:     "Mixed numeric and named properties",
			template: "User {UserId} processed {0} of {1} items",
			args:     []any{123, 50, 100},
			expected: "User 123 processed 50 of 100 items",
		},
		{
			name:     "String values with numeric indexing",
			template: "File {0} moved to {1}",
			args:     []any{"test.txt", "archive/"},
			expected: "File test.txt moved to archive/",
		},
		{
			name:     "Numeric indexing with format specifiers",
			template: "Progress: {0:000}/{1:000} ({2:P1})",
			args:     []any{42, 100, 0.42},
			expected: "Progress: 042/100 (42.0%)",
		},
		{
			name:     "Out of order numeric indices",
			template: "Result: {1} - {0}",
			args:     []any{"first", "second"},
			expected: "Result: second - first", // Should use actual index values
		},
		{
			name:     "Numeric index with literal format",
			template: "Command: {0:l} {1:l}",
			args:     []any{"git", "status"},
			expected: "Command: git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			logger.Information(tt.template, tt.args...)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

func TestNumericIndexingWithMemorySink(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Test that numeric indices are treated as property names
	logger.Information("Item {0} has value {1}", "ABC", 123)

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Check that numeric indices are stored as properties
	if val, ok := event.Properties["0"]; !ok || val != "ABC" {
		t.Errorf("Expected property '0' = 'ABC', got %v", val)
	}

	if val, ok := event.Properties["1"]; !ok || val != 123 {
		t.Errorf("Expected property '1' = 123, got %v", val)
	}
}

func TestNumericIndexingCompatibility(t *testing.T) {
	// Test .NET/Serilog style numeric indexing
	buf := &bytes.Buffer{}
	logger := New(WithSink(sinks.NewConsoleSinkWithWriter(buf)))

	// This is how it works in .NET String.Format and Serilog
	logger.Information("The {0} {1} {2} jumped over the {3} {4}",
		"quick", "brown", "fox", "lazy", "dog")

	output := buf.String()
	expected := "The quick brown fox jumped over the lazy dog"
	if !strings.Contains(output, expected) {
		t.Errorf("Expected Serilog-compatible output: %q, got: %s", expected, output)
	}
}

func TestNumericIndexingEdgeCases(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	tests := []struct {
		name       string
		template   string
		args       []any
		checkProps func(t *testing.T, props map[string]any)
	}{
		{
			name:     "Missing arguments",
			template: "Values: {0}, {1}, {2}",
			args:     []any{"first"},
			checkProps: func(t *testing.T, props map[string]any) {
				if props["0"] != "first" {
					t.Errorf("Expected property '0' = 'first'")
				}
				// Properties 1 and 2 should not exist
				if _, ok := props["1"]; ok {
					t.Error("Property '1' should not exist")
				}
				if _, ok := props["2"]; ok {
					t.Error("Property '2' should not exist")
				}
			},
		},
		{
			name:     "Large numeric index",
			template: "Value at {999}",
			args:     []any{"test"},
			checkProps: func(t *testing.T, props map[string]any) {
				// Index 999 is out of bounds, so property should not exist
				if _, ok := props["999"]; ok {
					t.Error("Property '999' should not exist when index is out of bounds")
				}
			},
		},
		{
			name:     "Zero index",
			template: "First: {0}",
			args:     []any{"zero"},
			checkProps: func(t *testing.T, props map[string]any) {
				if props["0"] != "zero" {
					t.Errorf("Expected property '0' = 'zero'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink.Clear()
			logger.Information(tt.template, tt.args...)

			events := sink.Events()
			if len(events) != 1 {
				t.Fatalf("Expected 1 event, got %d", len(events))
			}

			tt.checkProps(t, events[0].Properties)
		})
	}
}