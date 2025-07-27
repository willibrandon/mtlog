//go:build go1.18
// +build go1.18

package output

import (
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// FuzzParseOutputTemplate tests the output template parser with random inputs
func FuzzParseOutputTemplate(f *testing.F) {
	// Add seed corpus with various edge cases
	testCases := []string{
		// Basic templates
		"[{Timestamp}] {Message}",
		"{Level} - {Message}",
		"[{Timestamp:HH:mm:ss} {Level:u3}] {Message}",
		
		// Serilog-style templates
		"[{Timestamp:HH:mm:ss} {Level:u3}] {SourceContext}: {Message:lj}",
		"{Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} [{Level:u3}] {Message}",
		
		// Go template syntax
		"{{.Timestamp}} {{.Level}} {{.Message}}",
		"User {{.UserId}} logged in from {{.IP}}",
		"{{.Level}}: {{.Message}} [{{.SourceContext}}]",
		
		// Mixed syntax
		"{Timestamp} {{.Level}} {Message}",
		"[{{.Timestamp}}] {Level:u3} - {{.Message}}",
		
		// Edge cases
		"",
		"{}",
		"{{}}",
		"{{.}}",
		"{",
		"}",
		"{{",
		"}}",
		"{{{}}}",
		"{{{",
		"}}}",
		
		// Format specifiers
		"{Level:}",
		"{Level:u}",
		"{Level:u3}",
		"{Level:w}",
		"{Level:U}",
		"{Level:U3}",
		"{Level:W}",
		"{Level:t}",
		"{Level:T}",
		"{Level:m}",
		"{Level:M}",
		
		// Time format specifiers
		"{Timestamp:HH:mm:ss}",
		"{Timestamp:yyyy-MM-dd}",
		"{Timestamp:yyyy-MM-dd HH:mm:ss.fff}",
		"{Timestamp:dd/MM/yyyy HH:mm:ss}",
		"{Timestamp:2006-01-02 15:04:05}",
		
		// Special properties
		"{NewLine}",
		"{Tab}",
		"{Exception}",
		"{Properties}",
		
		// Alignment
		"{Message:lj}",
		"{Message,10}",
		"{Message,-10}",
		"{Level,5}",
		
		// Invalid formats
		"{Level:xyz}",
		"{Timestamp:invalid}",
		"{Message:}",
		"{:format}",
		"{ }",
		
		// Complex nested
		"[{Timestamp}] {{{Level}}} {Message}",
		"{{.Level}} in {{{SourceContext}}}",
		
		// Unicode
		"时间: {Timestamp} 级别: {Level} 消息: {Message}",
		"📅 {Timestamp} 📊 {Level} 💬 {Message}",
		
		// Long templates
		"[{Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz}] [{Level:u3}] [{MachineName}] [{ThreadId}] [{SourceContext}] {Message:lj} {Properties}",
		
		// Repeated properties
		"{Level} {Level} {Level}",
		"{Timestamp} - {Message} - {Timestamp}",
	}
	
	for _, tc := range testCases {
		f.Add(tc)
	}
	
	f.Fuzz(func(t *testing.T, input string) {
		// The parser should never panic, regardless of input
		template, err := Parse(input)
		
		if err != nil {
			// Error is acceptable for malformed input
			return
		}
		
		// If parsing succeeded, validate the result
		if template == nil {
			t.Error("Parse returned nil template without error")
			return
		}
		
		// Tokens should not be nil (even for empty templates)
		if template.Tokens == nil {
			// Empty template might have empty tokens array, which is ok
			if input != "" {
				t.Error("Template tokens is nil for non-empty input")
				return
			}
		}
		
		// Create a test event to render
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test message",
			Properties: map[string]interface{}{
				"UserId":        42,
				"IP":            "192.168.1.1",
				"SourceContext": "TestContext",
				"MachineName":   "TestMachine",
				"ThreadId":      12345,
			},
		}
		
		// Rendering should not panic
		rendered := template.Render(event)
		
		// Rendered result should be a string (even if empty)
		_ = rendered // Mark as used to satisfy linter
	})
}

// FuzzTimeFormatting tests time formatting with random format strings
func FuzzTimeFormatting(f *testing.F) {
	// Add seed corpus with various time formats
	formats := []string{
		"HH:mm:ss",
		"yyyy-MM-dd",
		"yyyy-MM-dd HH:mm:ss",
		"dd/MM/yyyy",
		"MM/dd/yyyy",
		"HH:mm:ss.fff",
		"yyyy-MM-dd HH:mm:ss.fff zzz",
		"2006-01-02 15:04:05",
		"",
		"invalid",
		"HH",
		"mm",
		"ss",
		"fff",
		"zzz",
		"MMMM dd, yyyy",
		"dddd",
	}
	
	for _, fmt := range formats {
		f.Add(fmt)
	}
	
	f.Fuzz(func(t *testing.T, format string) {
		// Test time formatting with the given format
		now := time.Now()
		
		// Should not panic
		formatted := formatTimestamp(now, format)
		
		// Result should always be a string
		_ = formatted // Mark as used to satisfy linter
	})
}

// FuzzLevelFormatting tests level formatting with random format strings
func FuzzLevelFormatting(f *testing.F) {
	// Add seed corpus
	formats := []string{
		"",
		"u",
		"u3",
		"U",
		"U3", 
		"w",
		"W",
		"t",
		"T",
		"m",
		"M",
		"invalid",
		"u10",
		"U0",
		"xyz",
	}
	
	for _, fmt := range formats {
		f.Add(fmt)
	}
	
	levels := []core.LogEventLevel{
		core.VerboseLevel,
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}
	
	f.Fuzz(func(t *testing.T, format string) {
		// Test each level with the format
		for _, level := range levels {
			// Should not panic
			formatted := formatLevel(level, format)
			
			// Result should always be a string
			if formatted == "" {
				t.Errorf("formatLevel returned empty string for level %v with format %q", level, format)
			}
		}
	})
}