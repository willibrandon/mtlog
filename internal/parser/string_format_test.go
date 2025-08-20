package parser

import (
	"testing"
)

func TestStringFormatting(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		properties map[string]any
		expected   string
	}{
		{
			name:       "String without format - should be quoted",
			template:   "User {Name} logged in",
			properties: map[string]any{"Name": "John"},
			expected:   "User John logged in",
		},
		{
			name:       "String with :l format - no quotes",
			template:   "User {Name:l} logged in",
			properties: map[string]any{"Name": "John"},
			expected:   "User John logged in",
		},
		{
			name:       "Mixed string formats",
			template:   "User {Name:l} said {Message}",
			properties: map[string]any{"Name": "Alice", "Message": "Hello"},
			expected:   "User Alice said Hello",
		},
		{
			name:       "String with special chars - quoted",
			template:   "Error: {Error}",
			properties: map[string]any{"Error": "File \"test.txt\" not found"},
			expected:   "Error: File \"test.txt\" not found",
		},
		{
			name:       "String with special chars - literal",
			template:   "Error: {Error:l}",
			properties: map[string]any{"Error": "File \"test.txt\" not found"},
			expected:   "Error: File \"test.txt\" not found",
		},
		{
			name:       "Integer should not be quoted",
			template:   "User {UserId} logged in",
			properties: map[string]any{"UserId": 123},
			expected:   "User 123 logged in",
		},
		{
			name:       "Empty string - quoted",
			template:   "Value: {Value}",
			properties: map[string]any{"Value": ""},
			expected:   "Value: ",
		},
		{
			name:       "Empty string - literal",
			template:   "Value: {Value:l}",
			properties: map[string]any{"Value": ""},
			expected:   "Value: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			result := tmpl.Render(tt.properties)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStringFormattingCompatibility(t *testing.T) {
	// Test Serilog compatibility examples
	tests := []struct {
		name       string
		template   string
		properties map[string]any
		expected   string
	}{
		{
			name:       "Serilog example - quoted",
			template:   "Could not find documents matching {Term}",
			properties: map[string]any{"Term": "search query"},
			expected:   "Could not find documents matching search query",
		},
		{
			name:       "Serilog example - literal",
			template:   "Could not find documents matching {Term:l}",
			properties: map[string]any{"Term": "search query"},
			expected:   "Could not find documents matching search query",
		},
		{
			name:       "File path - quoted",
			template:   "Processing file {FilePath}",
			properties: map[string]any{"FilePath": "/usr/local/bin/app"},
			expected:   "Processing file /usr/local/bin/app",
		},
		{
			name:       "File path - literal",
			template:   "Processing file {FilePath:l}",
			properties: map[string]any{"FilePath": "/usr/local/bin/app"},
			expected:   "Processing file /usr/local/bin/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			result := tmpl.Render(tt.properties)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}