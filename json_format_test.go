package mtlog_test

import (
	"strings"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/sinks"
)

func TestJSONFormatSpecifier(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	tests := []struct {
		name     string
		template string
		args     []any
		expected string
	}{
		{
			name:     "Simple map to JSON",
			template: "Config: {Settings:j}",
			args:     []any{map[string]any{"debug": true, "port": 8080}},
			expected: `{"debug":true,"port":8080}`,
		},
		{
			name:     "Struct to JSON",
			template: "User: {User:j}",
			args: []any{struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{ID: 123, Name: "Alice"}},
			expected: `{"id":123,"name":"Alice"}`,
		},
		{
			name:     "Nil value to JSON null",
			template: "Value: {Val:j}",
			args:     []any{nil},
			expected: "Value: null",  // Include prefix to match full output
		},
		{
			name:     "Array to JSON",
			template: "Tags: {Tags:j}",
			args:     []any{[]string{"admin", "user", "beta"}},
			expected: `["admin","user","beta"]`,
		},
		{
			name:     "Complex nested structure",
			template: "Data: {Data:j}",
			args: []any{map[string]any{
				"user": map[string]any{
					"id":   123,
					"name": "Bob",
				},
				"active": true,
			}},
			expected: `{"active":true,"user":{"id":123,"name":"Bob"}}`,
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

			// Parse and render the message template
			event := events[0]
			mt, err := parser.Parse(event.MessageTemplate)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			rendered := mt.Render(event.Properties)
			
			// Check if the JSON is in the output
			if !strings.Contains(rendered, tt.expected) {
				t.Errorf("Expected JSON %q in output, got: %q", tt.expected, rendered)
			}
		})
	}
}

func TestJSONFormatWithQuotedStrings(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	// Test combining :q and :j formats
	logger.Information("String normal: {Name}, quoted: {Name:q}, JSON: {Name:j}", "Alice", "Alice", "Alice")

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	mt, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	rendered := mt.Render(event.Properties)
	expected := `String normal: Alice, quoted: "Alice", JSON: "Alice"`
	
	if rendered != expected {
		t.Errorf("Expected: %q\nGot: %q", expected, rendered)
	}
}