package parser

import (
	"strings"
	"testing"
)

func TestParseGoTemplateSyntax(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected []string // Expected property names
	}{
		{
			name:     "Simple Go template",
			template: "User {{.UserId}} logged in",
			expected: []string{"UserId"},
		},
		{
			name:     "Multiple Go templates",
			template: "{{.User}} performed {{.Action}} at {{.Time}}",
			expected: []string{"User", "Action", "Time"},
		},
		{
			name:     "Mixed syntax",
			template: "User {UserId} ({{.Username}}) logged in",
			expected: []string{"UserId", "Username"},
		},
		{
			name:     "Go template with underscores",
			template: "Error in {{.error_code}} module",
			expected: []string{"error_code"},
		},
		{
			name:     "Escaped braces",
			template: "Use {{{{ for literal brace",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Extract property names
			var propNames []string
			for _, token := range mt.Tokens {
				if prop, ok := token.(*PropertyToken); ok {
					propNames = append(propNames, prop.PropertyName)
				}
			}

			// Compare
			if len(propNames) != len(tt.expected) {
				t.Errorf("Expected %d properties, got %d", len(tt.expected), len(propNames))
			}

			for i, name := range propNames {
				if i < len(tt.expected) && name != tt.expected[i] {
					t.Errorf("Property %d: expected %q, got %q", i, tt.expected[i], name)
				}
			}
		})
	}
}

func TestGoTemplateEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		properties map[string]any
		expected   string
	}{
		{
			name:       "Empty Go template property",
			template:   "Value: {{.}}",
			properties: map[string]any{},
			expected:   "Value: {}", // Empty property name
		},
		{
			name:       "Go template without dot",
			template:   "Value: {{Property}}",
			properties: map[string]any{"Property": "test"},
			expected:   "Value: test", // Treated as regular property without dot
		},
		{
			name:       "Nested Go templates",
			template:   "{{.Outer{{.Inner}}}}",
			properties: map[string]any{"Outer": "out", "Inner": "in"},
			expected:   "{Outer{{.Inner}}", // Invalid nesting, treated as property with invalid content
		},
		{
			name:       "Go template with spaces",
			template:   "User {{ .UserId }} logged in",
			properties: map[string]any{"UserId": 123},
			expected:   "User { .UserId } logged in", // Spaces make it a regular property
		},
		{
			name:       "Unclosed Go template",
			template:   "User {{.UserId logged in",
			properties: map[string]any{"UserId": 123},
			expected:   "User {{.UserId logged in", // Treated as text
		},
		{
			name:       "Go template with special characters",
			template:   "Value: {{.user-id}}",
			properties: map[string]any{"user-id": 123},
			expected:   "Value: 123", // Now allowing hyphens in property names
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := mt.Render(tt.properties)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGoTemplateCapturing(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		properties map[string]any
		expected   string
		checkType  string // To verify capturing hint is parsed
	}{
		{
			name:       "Go template with @ capturing",
			template:   "User {{@.User}} created",
			properties: map[string]any{"User": map[string]any{"id": 1, "name": "Alice"}},
			expected:   "User ",
			checkType:  "capture",
		},
		{
			name:       "Go template with $ scalar",
			template:   "Error: {{$.Error}}",
			properties: map[string]any{"Error": map[string]any{"code": 500}},
			expected:   "Error: ",
			checkType:  "scalar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Check capturing hint was parsed correctly
			for _, token := range mt.Tokens {
				if prop, ok := token.(*PropertyToken); ok {
					if tt.checkType == "capture" && prop.Capturing != Capture {
						t.Errorf("Expected capturing hint, got %v", prop.Capturing)
					}
					if tt.checkType == "scalar" && prop.Capturing != AsScalar {
						t.Errorf("Expected scalar hint, got %v", prop.Capturing)
					}
				}
			}

			result := mt.Render(tt.properties)
			// For capturing tests, just check that it starts with expected prefix
			if !strings.HasPrefix(result, tt.expected) {
				t.Errorf("Expected result to start with %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMixedTemplateSyntax(t *testing.T) {
	template := "User {UserId} ({{.Username}}) performed {Action} on {{.Resource}} at {Time}"
	properties := map[string]any{
		"UserId":   123,
		"Username": "alice",
		"Action":   "update",
		"Resource": "document",
		"Time":     "15:04:05",
	}

	mt, err := Parse(template)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Count property tokens
	propCount := 0
	for _, token := range mt.Tokens {
		if _, ok := token.(*PropertyToken); ok {
			propCount++
		}
	}

	if propCount != 5 {
		t.Errorf("Expected 5 property tokens, got %d", propCount)
	}

	result := mt.Render(properties)
	expected := "User 123 (alice) performed update on document at 15:04:05"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func BenchmarkParseGoTemplate(b *testing.B) {
	template := "User {{.UserId}} ({{.Username}}) logged in from {{.IP}}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(template)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseTraditional(b *testing.B) {
	template := "User {UserId} ({Username}) logged in from {IP}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(template)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMixed(b *testing.B) {
	template := "User {UserId} ({{.Username}}) logged in from {IP}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(template)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestRenderGoTemplateSyntax(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		properties map[string]any
		expected   string
	}{
		{
			name:     "Simple Go template",
			template: "User {{.UserId}} logged in",
			properties: map[string]any{
				"UserId": 123,
			},
			expected: "User 123 logged in",
		},
		{
			name:     "Multiple Go templates",
			template: "{{.User}} performed {{.Action}}",
			properties: map[string]any{
				"User":   "Alice",
				"Action": "login",
			},
			expected: "Alice performed login",
		},
		{
			name:     "Mixed syntax",
			template: "User {UserId} ({{.Username}}) logged in",
			properties: map[string]any{
				"UserId":   123,
				"Username": "alice",
			},
			expected: "User 123 (alice) logged in",
		},
		{
			name:     "Missing property",
			template: "User {{.UserId}} status {{.Status}}",
			properties: map[string]any{
				"UserId": 123,
			},
			expected: "User 123 status {Status}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := mt.Render(tt.properties)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
