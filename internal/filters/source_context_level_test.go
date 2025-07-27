package filters

import (
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestSourceContextLevelFilter(t *testing.T) {
	overrides := map[string]core.LogEventLevel{
		"Microsoft":                   core.WarningLevel,
		"Microsoft.EntityFramework":   core.InformationLevel,
		"Microsoft.Hosting.Lifetime":  core.InformationLevel,
		"System":                      core.WarningLevel,
		"MyApp.Critical":              core.VerboseLevel,
	}

	filter := NewSourceContextLevelFilter(core.InformationLevel, overrides)

	tests := []struct {
		name          string
		level         core.LogEventLevel
		sourceContext string
		expected      bool
	}{
		// Default level tests
		{
			name:          "Default allows Information",
			level:         core.InformationLevel,
			sourceContext: "UnknownContext",
			expected:      true,
		},
		{
			name:          "Default blocks Debug",
			level:         core.DebugLevel,
			sourceContext: "UnknownContext",
			expected:      false,
		},
		{
			name:          "Default allows Warning",
			level:         core.WarningLevel,
			sourceContext: "UnknownContext",
			expected:      true,
		},

		// Microsoft overrides
		{
			name:          "Microsoft blocks Information",
			level:         core.InformationLevel,
			sourceContext: "Microsoft.AspNetCore.Routing",
			expected:      false,
		},
		{
			name:          "Microsoft allows Warning",
			level:         core.WarningLevel,
			sourceContext: "Microsoft.AspNetCore.Routing",
			expected:      true,
		},

		// More specific overrides take precedence
		{
			name:          "EntityFramework allows Information",
			level:         core.InformationLevel,
			sourceContext: "Microsoft.EntityFrameworkCore.Database",
			expected:      true,
		},
		{
			name:          "EntityFramework blocks Debug",
			level:         core.DebugLevel,
			sourceContext: "Microsoft.EntityFrameworkCore.Query",
			expected:      false,
		},

		// Exact match
		{
			name:          "Hosting.Lifetime exact match allows Information",
			level:         core.InformationLevel,
			sourceContext: "Microsoft.Hosting.Lifetime",
			expected:      true,
		},

		// Critical namespace allows everything
		{
			name:          "Critical allows Verbose",
			level:         core.VerboseLevel,
			sourceContext: "MyApp.Critical.Service",
			expected:      true,
		},
		{
			name:          "Critical allows Debug",
			level:         core.DebugLevel,
			sourceContext: "MyApp.Critical.Database",
			expected:      true,
		},

		// No source context uses default
		{
			name:          "No context uses default",
			level:         core.InformationLevel,
			sourceContext: "",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &core.LogEvent{
				Timestamp: time.Now(),
				Level:     tt.level,
				Properties: map[string]interface{}{
					"SourceContext": tt.sourceContext,
				},
			}

			result := filter.IsEnabled(event)
			if result != tt.expected {
				t.Errorf("Expected IsEnabled=%v for level=%v, context=%q", tt.expected, tt.level, tt.sourceContext)
			}
		})
	}
}

func TestSourceContextLevelFilterPrefixMatching(t *testing.T) {
	// Test that prefix matching works correctly
	overrides := map[string]core.LogEventLevel{
		"A":     core.ErrorLevel,
		"A.B":   core.WarningLevel,
		"A.B.C": core.InformationLevel,
	}

	filter := NewSourceContextLevelFilter(core.DebugLevel, overrides)

	tests := []struct {
		context  string
		level    core.LogEventLevel
		expected bool
	}{
		// Most specific match wins
		{"A.B.C.D", core.InformationLevel, true},
		{"A.B.C.D", core.DebugLevel, false},
		{"A.B.C", core.InformationLevel, true},
		{"A.B.D", core.WarningLevel, true},
		{"A.B.D", core.InformationLevel, false},
		{"A.C", core.ErrorLevel, true},
		{"A.C", core.WarningLevel, false},
		{"B", core.DebugLevel, true}, // Uses default
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			event := &core.LogEvent{
				Level: tt.level,
				Properties: map[string]interface{}{
					"SourceContext": tt.context,
				},
			}
			result := filter.IsEnabled(event)
			if result != tt.expected {
				t.Errorf("Context %q at level %v: expected %v, got %v", tt.context, tt.level, tt.expected, result)
			}
		})
	}
}

func TestSourceContextLevelFilterNoOverrides(t *testing.T) {
	// Test filter with no overrides behaves like standard level filter
	filter := NewSourceContextLevelFilter(core.WarningLevel, nil)

	tests := []struct {
		level    core.LogEventLevel
		expected bool
	}{
		{core.VerboseLevel, false},
		{core.DebugLevel, false},
		{core.InformationLevel, false},
		{core.WarningLevel, true},
		{core.ErrorLevel, true},
		{core.FatalLevel, true},
	}

	for _, tt := range tests {
		event := &core.LogEvent{
			Level: tt.level,
			Properties: map[string]interface{}{
				"SourceContext": "Any.Context",
			},
		}
		result := filter.IsEnabled(event)
		if result != tt.expected {
			t.Errorf("Level %v: expected %v, got %v", tt.level, tt.expected, result)
		}
	}
}

func TestSourceContextLevelFilterCaseSensitive(t *testing.T) {
	// Verify that source context matching is case-sensitive
	overrides := map[string]core.LogEventLevel{
		"MyApp": core.DebugLevel,
	}

	filter := NewSourceContextLevelFilter(core.WarningLevel, overrides)

	tests := []struct {
		context  string
		level    core.LogEventLevel
		expected bool
	}{
		{"MyApp.Service", core.DebugLevel, true},
		{"myapp.Service", core.DebugLevel, false}, // Different case, uses default
		{"MYAPP.Service", core.DebugLevel, false}, // Different case, uses default
	}

	for _, tt := range tests {
		event := &core.LogEvent{
			Level: tt.level,
			Properties: map[string]interface{}{
				"SourceContext": tt.context,
			},
		}
		result := filter.IsEnabled(event)
		if result != tt.expected {
			t.Errorf("Context %q: expected %v, got %v", tt.context, tt.expected, result)
		}
	}
}