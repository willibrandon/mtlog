package parser

import (
	"testing"
	"time"
)

func TestParsePropertyWithFormat(t *testing.T) {
	tests := []struct {
		name          string
		propertyText  string
		expectedName  string
		expectedFormat string
		expectedAlign int
	}{
		{
			name:          "No format",
			propertyText:  "UserId",
			expectedName:  "UserId",
			expectedFormat: "",
			expectedAlign: 0,
		},
		{
			name:          "With numeric format",
			propertyText:  "Count:000",
			expectedName:  "Count",
			expectedFormat: "000",
			expectedAlign: 0,
		},
		{
			name:          "With alignment only",
			propertyText:  "Name,10",
			expectedName:  "Name",
			expectedFormat: "",
			expectedAlign: 10,
		},
		{
			name:          "With negative alignment",
			propertyText:  "Name,-10",
			expectedName:  "Name",
			expectedFormat: "",
			expectedAlign: -10,
		},
		{
			name:          "With alignment and format",
			propertyText:  "Price,8:F2",
			expectedName:  "Price",
			expectedFormat: "F2",
			expectedAlign: 8,
		},
		{
			name:          "With capturing and format",
			propertyText:  "@User:json",
			expectedName:  "User",
			expectedFormat: "json",
			expectedAlign: 0,
		},
		{
			name:          "Timestamp format",
			propertyText:  "Timestamp:yyyy-MM-dd HH:mm:ss",
			expectedName:  "Timestamp",
			expectedFormat: "yyyy-MM-dd HH:mm:ss",
			expectedAlign: 0,
		},
		{
			name:          "Hex format",
			propertyText:  "Id:X",
			expectedName:  "Id",
			expectedFormat: "X",
			expectedAlign: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := parsePropertyToken(tt.propertyText)
			if token.PropertyName != tt.expectedName {
				t.Errorf("Expected property name %q, got %q", tt.expectedName, token.PropertyName)
			}
			if token.Format != tt.expectedFormat {
				t.Errorf("Expected format %q, got %q", tt.expectedFormat, token.Format)
			}
			if token.Alignment != tt.expectedAlign {
				t.Errorf("Expected alignment %d, got %d", tt.expectedAlign, token.Alignment)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		value    interface{}
		expected string
	}{
		{"No format", "", 42, "42"},
		{"Zero padding 3", "000", 5, "005"},
		{"Zero padding 4", "0000", 42, "0042"},
		{"Zero padding large", "000", 12345, "12345"},
		{"Hex lowercase", "x", 255, "ff"},
		{"Hex uppercase", "X", 255, "FF"},
		{"Default", "D", 42, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PropertyToken{Format: tt.format}
			result := token.formatNumber(tt.value)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		value    float64
		expected string
	}{
		{"No format", "", 3.14159, "3.14159"},
		{"Fixed 2 decimals", "F2", 3.14159, "3.14"},
		{"Fixed 0 decimals", "F0", 3.14159, "3"},
		{"Exponential", "E2", 1234.5, "1.23e+03"},
		{"General", "G3", 1234.5, "1.23e+03"},
		{"Percentage", "P", 0.125, "12.50%"},
		{"Percentage 1 decimal", "P1", 0.125, "12.5%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PropertyToken{Format: tt.format}
			result := token.formatFloat(tt.value)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	// Use a fixed time for testing
	testTime := time.Date(2025, 1, 22, 15, 30, 45, 0, time.UTC)

	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{"Date only", "yyyy-MM-dd", "2025-01-22"},
		{"Time only", "HH:mm:ss", "15:30:45"},
		{"Date and time", "yyyy-MM-dd HH:mm:ss", "2025-01-22 15:30:45"},
		{"Short date", "yy-M-d", "25-1-22"},
		{"Month name", "MMM dd, yyyy", "Jan 22, 2025"},
		{"Full month", "MMMM dd, yyyy", "January 22, 2025"},
		{"12-hour format", "hh:mm tt", "03:30 PM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PropertyToken{Format: tt.format}
			result := token.formatTime(testTime)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestApplyAlignment(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		alignment int
		expected  string
	}{
		{"No alignment", "test", 0, "test"},
		{"Right align 10", "test", 10, "      test"},
		{"Left align 10", "test", -10, "test      "},
		{"Text longer than width", "verylongtext", 5, "verylongtext"},
		{"Exact width", "12345", 5, "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PropertyToken{Alignment: tt.alignment}
			result := token.applyAlignment(tt.text)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
			if tt.alignment != 0 && len(result) < abs(tt.alignment) {
				t.Errorf("Result length %d is less than expected width %d", len(result), abs(tt.alignment))
			}
		})
	}
}

func TestRenderWithFormat(t *testing.T) {
	tests := []struct {
		name       string
		token      *PropertyToken
		properties map[string]interface{}
		expected   string
	}{
		{
			name: "Number with zero padding",
			token: &PropertyToken{
				PropertyName: "Count",
				Format:      "000",
			},
			properties: map[string]interface{}{"Count": 5},
			expected:   "005",
		},
		{
			name: "Float with precision",
			token: &PropertyToken{
				PropertyName: "Price",
				Format:      "F2",
			},
			properties: map[string]interface{}{"Price": 19.995},
			expected:   "20.00",
		},
		{
			name: "String with right alignment",
			token: &PropertyToken{
				PropertyName: "Name",
				Alignment:   10,
			},
			properties: map[string]interface{}{"Name": "Alice"},
			expected:   "     Alice",
		},
		{
			name: "Number with alignment and format",
			token: &PropertyToken{
				PropertyName: "Id",
				Format:      "000",
				Alignment:   8,
			},
			properties: map[string]interface{}{"Id": 42},
			expected:   "     042",
		},
		{
			name: "Time with format",
			token: &PropertyToken{
				PropertyName: "Timestamp",
				Format:      "yyyy-MM-dd",
			},
			properties: map[string]interface{}{
				"Timestamp": time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
			},
			expected: "2025-01-22",
		},
		{
			name: "Missing property",
			token: &PropertyToken{
				PropertyName: "Missing",
				Format:      "000",
			},
			properties: map[string]interface{}{},
			expected:   "{Missing}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.Render(tt.properties)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseCompleteTemplate(t *testing.T) {
	template := "User {UserId:000} spent {Amount,8:F2} at {Timestamp:yyyy-MM-dd HH:mm:ss}"
	mt, err := Parse(template)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	if len(mt.Tokens) != 6 {
		for i, token := range mt.Tokens {
			switch tok := token.(type) {
			case *TextToken:
				t.Logf("Token %d: TextToken(%q)", i, tok.Text)
			case *PropertyToken:
				t.Logf("Token %d: PropertyToken(name=%q, format=%q, align=%d)", i, tok.PropertyName, tok.Format, tok.Alignment)
			}
		}
		t.Fatalf("Expected 6 tokens, got %d", len(mt.Tokens))
	}

	// Check text tokens
	if text, ok := mt.Tokens[0].(*TextToken); !ok || text.Text != "User " {
		t.Errorf("Token 0: expected text 'User ', got %v", mt.Tokens[0])
	}

	// Check UserId property
	if prop, ok := mt.Tokens[1].(*PropertyToken); !ok {
		t.Errorf("Token 1: expected PropertyToken, got %T", mt.Tokens[1])
	} else {
		if prop.PropertyName != "UserId" || prop.Format != "000" {
			t.Errorf("Token 1: expected UserId:000, got %s:%s", prop.PropertyName, prop.Format)
		}
	}

	// Check Amount property
	if prop, ok := mt.Tokens[3].(*PropertyToken); !ok {
		t.Errorf("Token 3: expected PropertyToken, got %T", mt.Tokens[3])
	} else {
		if prop.PropertyName != "Amount" || prop.Format != "F2" || prop.Alignment != 8 {
			t.Errorf("Token 3: expected Amount,8:F2, got %s,%d:%s", prop.PropertyName, prop.Alignment, prop.Format)
		}
	}

	// Check Timestamp property
	if prop, ok := mt.Tokens[5].(*PropertyToken); !ok {
		t.Errorf("Token 5: expected PropertyToken, got %T", mt.Tokens[5])
	} else {
		if prop.PropertyName != "Timestamp" || prop.Format != "yyyy-MM-dd HH:mm:ss" {
			t.Errorf("Token 5: expected Timestamp:yyyy-MM-dd HH:mm:ss, got %s:%s", prop.PropertyName, prop.Format)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}