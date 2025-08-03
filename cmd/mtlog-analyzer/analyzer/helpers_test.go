package analyzer

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExtractProperties(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
		wantErr  bool
	}{
		{
			name:     "simple property",
			template: "User {UserId} logged in",
			want:     []string{"UserId"},
		},
		{
			name:     "multiple properties",
			template: "User {UserId} logged in from {IP}",
			want:     []string{"UserId", "IP"},
		},
		{
			name:     "property with format",
			template: "Count: {Count:000}",
			want:     []string{"Count:000"},
		},
		{
			name:     "escaped braces",
			template: "Processing {{batch}} with {Count} items",
			want:     []string{"Count"},
		},
		{
			name:     "empty property",
			template: "User {} logged in",
			want:     []string{}, // Empty property is skipped
		},
		{
			name:     "nested braces",
			template: "Data: {Outer{Inner}}",
			want:     []string{"Outer{Inner}"},
		},
		{
			name:     "deeply nested braces",
			template: "Data: {Outer{Middle{Inner}}}",
			want:     []string{"Outer{Middle{Inner}}"},
		},
		{
			name:     "unbalanced braces",
			template: "User {UserId logged in",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractProperties(tt.template)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("extractProperties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil {
				return // Expected error case
			}
			
			if len(got) != len(tt.want) {
				t.Errorf("extractProperties() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractProperties()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidateFormatSpecifier(t *testing.T) {
	tests := []struct {
		name       string
		property   string
		strictMode bool
		wantErr    bool
	}{
		{"no format", "Count", false, false},
		{"valid number format", "Count:000", false, false},
		{"valid float format", "Price:F2", false, false},
		{"valid percentage", "Percent:P1", false, false},
		{"valid hex", "Value:X4", false, false},
		{"valid time format", "Time:HH:mm:ss", false, false},
		{"valid date format", "Date:yyyy-MM-dd", false, false},
		{"unknown format lenient", "Count:ZZZ", false, false},
		{"unknown format strict", "Count:ZZZ", true, true},
		{"alignment specifier", "Name:-20", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				StrictMode:        tt.strictMode,
				CommonContextKeys: []string{},
				DisabledChecks:    make(map[string]bool),
			}
			err := validateFormatSpecifier(tt.property, config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFormatSpecifier(%q) error = %v, wantErr %v", tt.property, err, tt.wantErr)
			}
		})
	}
}

func TestExtractPropertiesPerformance(t *testing.T) {
	// Test with extremely large template to ensure the analyzer can handle
	// edge cases in generated code or complex logging scenarios.
	// The 1-second threshold ensures reasonable performance for large templates
	// and is adjustable for slower CI runners via MTLOG_ANALYZER_PERF_THRESHOLD.
	t.Run("10000 properties", func(t *testing.T) {
		var builder strings.Builder
		builder.WriteString("Template with many properties: ")
		
		// Build template with 10,000 properties
		for i := 0; i < 10000; i++ {
			builder.WriteString(fmt.Sprintf("{Property%d} ", i))
		}
		
		template := builder.String()
		
		// Measure extraction time
		start := time.Now()
		properties, err := extractProperties(template)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("extractProperties failed: %v", err)
		}
		
		if len(properties) != 10000 {
			t.Errorf("expected 10000 properties, got %d", len(properties))
		}
		
		// Get configurable threshold from environment, default to 1 second
		threshold := time.Second
		if env := os.Getenv("MTLOG_ANALYZER_PERF_THRESHOLD"); env != "" {
			if ms, err := strconv.Atoi(env); err == nil && ms > 0 {
				threshold = time.Duration(ms) * time.Millisecond
			}
		}
		
		// Ensure it completes in reasonable time
		if duration > threshold {
			t.Errorf("extractProperties took too long: %v (threshold: %v)", duration, threshold)
		}
		
		t.Logf("Extracted %d properties in %v", len(properties), duration)
	})
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"user_id", "UserId"},
		{"user-id", "UserId"},
		{"user.id", "UserId"},
		{"user:id", "UserId"},
		{"user/id", "UserId"},
		{"user\\id", "UserId"},
		{"user id", "UserId"},
		
		// Mixed separators
		{"user_id.test-value", "UserIdTestValue"},
		{"request-id:trace_id", "RequestIdTraceId"},
		{"mixed_separator-test.value:here/now", "MixedSeparatorTestValueHereNow"},
		
		// Edge cases
		{"", ""},
		{"single", "Single"},
		{"UPPERCASE", "Uppercase"},
		{"camelCase", "Camelcase"},
		{"123_number", "123Number"},
		{"_leading_underscore", "LeadingUnderscore"},
		{"trailing_underscore_", "TrailingUnderscore"},
		{"multiple___underscores", "MultipleUnderscores"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}