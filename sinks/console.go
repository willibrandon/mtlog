package sinks

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/parser"
)

// ConsoleSink writes log events to the console.
type ConsoleSink struct {
	output         io.Writer
	mu             sync.Mutex
	showProperties bool
	theme          *ConsoleTheme
	useColor       bool
}

// NewConsoleSink creates a new console sink that writes to stdout.
func NewConsoleSink() *ConsoleSink {
	sink := &ConsoleSink{
		output:   os.Stdout,
		theme:    DefaultTheme(),
		useColor: shouldUseColor(os.Stdout),
	}
	return sink
}

// NewConsoleSinkWithProperties creates a new console sink that displays properties.
func NewConsoleSinkWithProperties() *ConsoleSink {
	return &ConsoleSink{
		output:         os.Stdout,
		showProperties: true,
		theme:          DefaultTheme(),
		useColor:       shouldUseColor(os.Stdout),
	}
}

// NewConsoleSinkWithWriter creates a new console sink with a custom writer.
func NewConsoleSinkWithWriter(w io.Writer) *ConsoleSink {
	return &ConsoleSink{
		output:   w,
		theme:    DefaultTheme(),
		useColor: shouldUseColor(w),
	}
}

// NewConsoleSinkWithTheme creates a new console sink with a custom theme.
func NewConsoleSinkWithTheme(theme *ConsoleTheme) *ConsoleSink {
	return &ConsoleSink{
		output:   os.Stdout,
		theme:    theme,
		useColor: shouldUseColor(os.Stdout),
	}
}

// SetTheme updates the console theme.
func (cs *ConsoleSink) SetTheme(theme *ConsoleTheme) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.theme = theme
}

// SetUseColor enables or disables color output.
func (cs *ConsoleSink) SetUseColor(useColor bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.useColor = useColor
}

// ShowProperties enables or disables property display.
func (cs *ConsoleSink) ShowProperties(show bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.showProperties = show
}

// Emit writes the log event to the console.
func (cs *ConsoleSink) Emit(event *core.LogEvent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Format the log event
	message := cs.formatEvent(event)
	
	// Write to output
	fmt.Fprintln(cs.output, message)
}

// Close releases any resources held by the sink.
func (cs *ConsoleSink) Close() error {
	// Nothing to close for console sink
	return nil
}

// formatEvent formats a log event for console output.
func (cs *ConsoleSink) formatEvent(event *core.LogEvent) string {
	// Parse template to render message
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		// Fallback to raw template
		tmpl = &parser.MessageTemplate{
			Raw:    event.MessageTemplate,
			Tokens: []parser.MessageTemplateToken{&parser.TextToken{Text: event.MessageTemplate}},
		}
	}
	
	// Render the message
	message := tmpl.Render(event.Properties)
	
	// Format components with theme
	levelStr := formatLevel(event.Level)
	levelColor := cs.theme.GetLevelColor(event.Level)
	timestamp := event.Timestamp.Format(cs.theme.TimestampFormat)
	
	// Build the formatted output
	var result string
	
	// Timestamp
	timestampPart := fmt.Sprintf("[%s]", timestamp)
	timestampPart = colorize(timestampPart, cs.theme.TimestampColor, cs.useColor)
	
	// Level
	levelPart := fmt.Sprintf(cs.theme.LevelFormat, levelStr)
	levelPart = colorize(levelPart, levelColor, cs.useColor)
	
	// Message
	messagePart := colorize(message, cs.theme.MessageColor, cs.useColor)
	
	result = fmt.Sprintf("%s %s %s", timestampPart, levelPart, messagePart)
	
	// Add properties if enabled and there are any
	if cs.showProperties && len(event.Properties) > 0 {
		// Get properties that weren't used in the message template
		usedProps := make(map[string]bool)
		for _, token := range tmpl.Tokens {
			if prop, ok := token.(*parser.PropertyToken); ok {
				usedProps[prop.PropertyName] = true
			}
		}
		
		// Collect extra properties
		var extras []string
		for k, v := range event.Properties {
			if !usedProps[k] {
				// Format property with theme colors
				key := colorize(k, cs.theme.PropertyKeyColor, cs.useColor)
				val := colorize(fmt.Sprintf("%v", v), cs.theme.PropertyValColor, cs.useColor)
				extras = append(extras, fmt.Sprintf(cs.theme.PropertyFormat, key, val))
			}
		}
		
		// Sort for consistent output
		sort.Strings(extras)
		
		if len(extras) > 0 {
			result += fmt.Sprintf(" {%s}", strings.Join(extras, ", "))
		}
	}
	
	return result
}

// EmitSimple writes a simple log message without allocations.
func (cs *ConsoleSink) EmitSimple(timestamp time.Time, level core.LogEventLevel, message string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Use zero-allocation formatter
	levelStr := formatLevel(level)
	_ = writeSimple(cs.output, timestamp, levelStr, message)
}

// formatLevel converts a log level to its string representation.
func formatLevel(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "VRB"
	case core.DebugLevel:
		return "DBG"
	case core.InformationLevel:
		return "INF"
	case core.WarningLevel:
		return "WRN"
	case core.ErrorLevel:
		return "ERR"
	case core.FatalLevel:
		return "FTL"
	default:
		return "UNK"
	}
}