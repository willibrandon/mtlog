package output

import (
	"testing"
	
	"github.com/willibrandon/mtlog/core"
)

func TestLevelFormatShortcuts(t *testing.T) {
	tests := []struct {
		name     string
		level    core.LogEventLevel
		format   string
		expected string
	}{
		// u3 - uppercase 3-letter
		{name: "Verbose u3", level: core.VerboseLevel, format: "u3", expected: "VRB"},
		{name: "Debug u3", level: core.DebugLevel, format: "u3", expected: "DBG"},
		{name: "Information u3", level: core.InformationLevel, format: "u3", expected: "INF"},
		{name: "Warning u3", level: core.WarningLevel, format: "u3", expected: "WRN"},
		{name: "Error u3", level: core.ErrorLevel, format: "u3", expected: "ERR"},
		{name: "Fatal u3", level: core.FatalLevel, format: "u3", expected: "FTL"},
		
		// w3 - lowercase 3-letter
		{name: "Verbose w3", level: core.VerboseLevel, format: "w3", expected: "vrb"},
		{name: "Debug w3", level: core.DebugLevel, format: "w3", expected: "dbg"},
		{name: "Information w3", level: core.InformationLevel, format: "w3", expected: "inf"},
		{name: "Warning w3", level: core.WarningLevel, format: "w3", expected: "wrn"},
		{name: "Error w3", level: core.ErrorLevel, format: "w3", expected: "err"},
		{name: "Fatal w3", level: core.FatalLevel, format: "w3", expected: "ftl"},
		
		// u - uppercase full
		{name: "Verbose u", level: core.VerboseLevel, format: "u", expected: "VERBOSE"},
		{name: "Debug u", level: core.DebugLevel, format: "u", expected: "DEBUG"},
		{name: "Information u", level: core.InformationLevel, format: "u", expected: "INFORMATION"},
		{name: "Warning u", level: core.WarningLevel, format: "u", expected: "WARNING"},
		{name: "Error u", level: core.ErrorLevel, format: "u", expected: "ERROR"},
		{name: "Fatal u", level: core.FatalLevel, format: "u", expected: "FATAL"},
		
		// w - lowercase full
		{name: "Verbose w", level: core.VerboseLevel, format: "w", expected: "verbose"},
		{name: "Debug w", level: core.DebugLevel, format: "w", expected: "debug"},
		{name: "Information w", level: core.InformationLevel, format: "w", expected: "information"},
		{name: "Warning w", level: core.WarningLevel, format: "w", expected: "warning"},
		{name: "Error w", level: core.ErrorLevel, format: "w", expected: "error"},
		{name: "Fatal w", level: core.FatalLevel, format: "w", expected: "fatal"},
		
		// l - lowercase full (backward compat)
		{name: "Information l", level: core.InformationLevel, format: "l", expected: "information"},
		
		// Default (no format)
		{name: "Information default", level: core.InformationLevel, format: "", expected: "Information"},
		{name: "Warning default", level: core.WarningLevel, format: "", expected: "Warning"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLevel(tt.level, tt.format)
			if result != tt.expected {
				t.Errorf("formatLevel(%v, %q) = %q, want %q", tt.level, tt.format, result, tt.expected)
			}
		})
	}
}

func TestLevelFormatInTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		level    core.LogEventLevel
		expected string
	}{
		{
			name:     "Level u3 in template",
			template: "[${Level:u3}] ${Message}",
			level:    core.InformationLevel,
			expected: "[INF] Test message",
		},
		{
			name:     "Level w3 in template",
			template: "[${Level:w3}] ${Message}",
			level:    core.WarningLevel,
			expected: "[wrn] Test message",
		},
		{
			name:     "Level u in template",
			template: "Level: ${Level:u}",
			level:    core.ErrorLevel,
			expected: "Level: ERROR",
		},
		{
			name:     "Level w in template",
			template: "Level: ${Level:w}",
			level:    core.DebugLevel,
			expected: "Level: debug",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}
			
			event := &core.LogEvent{
				Level:           tt.level,
				MessageTemplate: "Test message",
				Properties:      map[string]any{},
			}
			
			result := tmpl.Render(event)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}