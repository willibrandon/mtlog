package output

import (
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
		tokens   int
	}{
		{
			name:     "Simple text",
			template: "Hello World",
			tokens:   1,
		},
		{
			name:     "Single property",
			template: "{UserId}",
			tokens:   1,
		},
		{
			name:     "Built-in element",
			template: "${Message}",
			tokens:   1,
		},
		{
			name:     "Text with built-in",
			template: "[${Timestamp}] ${Message}",
			tokens:   4, // "[", ${Timestamp}, "] ", ${Message}
		},
		{
			name:     "Mixed properties and built-ins",
			template: "[${Timestamp}] User {UserId}: ${Message}",
			tokens:   6, // "[", ${Timestamp}, "] User ", {UserId}, ": ", ${Message}
		},
		{
			name:     "Property with format",
			template: "{Level:u3}",
			tokens:   1,
		},
		{
			name:     "Complex template",
			template: "[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}${NewLine}",
			tokens:   9,
		},
		{
			name:     "Escaped braces",
			template: "{{escaped}}",
			tokens:   2, // "{" and "escaped}}"
		},
		{
			name:     "Unclosed property",
			template: "{Unclosed",
			wantErr:  true,
		},
		{
			name:     "Unclosed built-in",
			template: "${Unclosed",
			wantErr:  true,
		},
		{
			name:     "Built-in with format",
			template: "${Level:u3} ${Timestamp:yyyy-MM-dd}",
			tokens:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(tmpl.Tokens) != tt.tokens {
				t.Errorf("Expected %d tokens, got %d", tt.tokens, len(tmpl.Tokens))
			}
		})
	}
}

func TestBuiltInTokenRender(t *testing.T) {
	timestamp := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	event := &core.LogEvent{
		Timestamp:       timestamp,
		Level:           core.InformationLevel,
		MessageTemplate: "User {UserId} logged in",
		Properties: map[string]interface{}{
			"UserId":        12345,
			"SourceContext": "TestContext",
			"Count":         42,
			"Price":         123.456,
			"Percentage":    0.85,
		},
		Exception: nil,
	}

	tests := []struct {
		name     string
		builtIn  string
		format   string
		expected string
	}{
		// Timestamp formats
		{
			name:     "Timestamp default",
			builtIn:  "Timestamp",
			format:   "",
			expected: "2024-01-02 15:04:05",
		},
		{
			name:     "Timestamp HH:mm:ss",
			builtIn:  "Timestamp",
			format:       "HH:mm:ss",
			expected:     "15:04:05",
		},
		{
			name:         "Timestamp with date",
			builtIn: "Timestamp",
			format:       "yyyy-MM-dd",
			expected:     "2024-01-02",
		},
		// Level formats
		{
			name:         "Level uppercase 3",
			builtIn: "Level",
			format:       "u3",
			expected:     "INF",
		},
		{
			name:         "Level uppercase full",
			builtIn: "Level",
			format:       "u",
			expected:     "INFORMATION",
		},
		{
			name:         "Level lowercase",
			builtIn: "Level",
			format:       "l",
			expected:     "information",
		},
		{
			name:         "Level default",
			builtIn: "Level",
			format:       "",
			expected:     "Information",
		},
		// Message rendering
		{
			name:         "Message",
			builtIn: "Message",
			format:       "",
			expected:     "User 12345 logged in",
		},
		// Special built-ins
		{
			name:     "NewLine",
			builtIn:  "NewLine",
			format:   "",
			expected: "\n",
		},
		{
			name:     "Properties",
			builtIn:  "Properties",
			format:   "",
			expected: "Count=42 Percentage=0.85 Price=123.456 SourceContext=TestContext UserId=12345",
		},
		{
			name:     "Unknown built-in",
			builtIn:  "Unknown",
			format:   "",
			expected: "${Unknown}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &BuiltInToken{
				Name:   tt.builtIn,
				Format: tt.format,
			}
			result := token.Render(event)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPropertyTokenRender(t *testing.T) {
	event := &core.LogEvent{
		Properties: map[string]interface{}{
			"UserId":        12345,
			"SourceContext": "TestContext",
			"Count":         42,
			"Price":         123.456,
			"Percentage":    0.85,
		},
	}

	tests := []struct {
		name     string
		property string
		format   string
		expected string
	}{
		{
			name:     "String property",
			property: "SourceContext",
			format:   "",
			expected: "TestContext",
		},
		{
			name:     "Integer property",
			property: "Count",
			format:   "",
			expected: "42",
		},
		{
			name:     "Integer with padding",
			property: "Count",
			format:   "000",
			expected: "042",
		},
		{
			name:     "Float with fixed decimals",
			property: "Price",
			format:   "F2",
			expected: "123.46",
		},
		{
			name:     "Percentage",
			property: "Percentage",
			format:   "P1",
			expected: "85.0%",
		},
		{
			name:     "Missing property",
			property: "Missing",
			format:   "",
			expected: "{Missing}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PropertyToken{
				PropertyName: tt.property,
				Format:       tt.format,
			}
			result := token.Render(event)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTemplateRender(t *testing.T) {
	timestamp := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	event := &core.LogEvent{
		Timestamp:       timestamp,
		Level:           core.WarningLevel,
		MessageTemplate: "Disk usage at {Percentage}",
		Properties: map[string]interface{}{
			"Percentage":    0.85,
			"SourceContext": "Monitor",
		},
	}

	// Test with new built-in syntax
	template := "[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}${NewLine}"
	tmpl, err := Parse(template)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	result := tmpl.Render(event)
	expected := "[15:04:05 WRN] Monitor: Disk usage at 0.85\n"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestFormatConversions(t *testing.T) {
	// Test level conversions
	levels := []struct {
		level    core.LogEventLevel
		format   string
		expected string
	}{
		{core.VerboseLevel, "u3", "VRB"},
		{core.DebugLevel, "u3", "DBG"},
		{core.InformationLevel, "u3", "INF"},
		{core.WarningLevel, "u3", "WRN"},
		{core.ErrorLevel, "u3", "ERR"},
		{core.FatalLevel, "u3", "FTL"},
		{core.InformationLevel, "u", "INFORMATION"},
		{core.InformationLevel, "l", "information"},
	}

	for _, tt := range levels {
		result := formatLevel(tt.level, tt.format)
		if result != tt.expected {
			t.Errorf("formatLevel(%v, %q) = %q, want %q", tt.level, tt.format, result, tt.expected)
		}
	}
}

func TestTimeFormatConversion(t *testing.T) {
	tests := []struct {
		dotnet string
		golang string
	}{
		{"yyyy-MM-dd", "2006-01-02"},
		{"HH:mm:ss", "15:04:05"},
		{"yyyy-MM-dd HH:mm:ss", "2006-01-02 15:04:05"},
		{"HH:mm:ss.fff", "15:04:05.000"},
		{"yyyy-MM-dd'T'HH:mm:ss.fffzzz", "2006-01-02'T'15:04:05.000-07:00"},
	}

	for _, tt := range tests {
		result := convertTimeFormat(tt.dotnet)
		if result != tt.golang {
			t.Errorf("convertTimeFormat(%q) = %q, want %q", tt.dotnet, result, tt.golang)
		}
	}
}