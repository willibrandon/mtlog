package sinks

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/formatters/output"
	"github.com/willibrandon/mtlog/parser"
)

// ConsoleSink writes log events to the console.
type ConsoleSink struct {
	output         io.Writer
	mu             sync.Mutex
	showProperties bool
	theme          *ConsoleTheme
	useColor       bool
	template       string
	parsedTemplate *output.Template
	// Template analysis for optimization
	hasBrackets       bool // Template contains bracket tokens
	potentialStatuses []string // Property names that might be status codes
}

// NewConsoleSink creates a new console sink that writes to stdout.
func NewConsoleSink() *ConsoleSink {
	// Enable VT processing on Windows for ANSI colors
	enableWindowsVTProcessing()
	
	sink := &ConsoleSink{
		output:   os.Stdout,
		theme:    DefaultTheme(),
		useColor: shouldUseColor(os.Stdout),
	}
	return sink
}

// NewConsoleSinkWithProperties creates a new console sink that displays properties.
func NewConsoleSinkWithProperties() *ConsoleSink {
	// Enable VT processing on Windows for ANSI colors
	enableWindowsVTProcessing()
	
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
	// Enable VT processing on Windows for ANSI colors
	enableWindowsVTProcessing()
	
	return &ConsoleSink{
		output:   os.Stdout,
		theme:    theme,
		useColor: shouldUseColor(os.Stdout),
	}
}

// NewConsoleSinkWithTemplate creates a new console sink with a custom output template.
func NewConsoleSinkWithTemplate(template string) (*ConsoleSink, error) {
	// Enable VT processing on Windows for ANSI colors
	enableWindowsVTProcessing()
	
	parsedTemplate, err := output.Parse(template)
	if err != nil {
		return nil, fmt.Errorf("invalid output template: %w", err)
	}
	sink := &ConsoleSink{
		output:         os.Stdout,
		theme:          DefaultTheme(),
		useColor:       shouldUseColor(os.Stdout),
		template:       template,
		parsedTemplate: parsedTemplate,
	}
	
	// Analyze template for optimizations
	sink.hasBrackets, sink.potentialStatuses = analyzeTemplate(parsedTemplate)
	
	return sink, nil
}

// NewConsoleSinkWithTemplateAndTheme creates a new console sink with both template and theme.
func NewConsoleSinkWithTemplateAndTheme(template string, theme *ConsoleTheme) (*ConsoleSink, error) {
	// Enable VT processing on Windows for ANSI colors
	enableWindowsVTProcessing()
	
	parsedTemplate, err := output.Parse(template)
	if err != nil {
		return nil, fmt.Errorf("invalid output template: %w", err)
	}
	sink := &ConsoleSink{
		output:         os.Stdout,
		theme:          theme,
		useColor:       shouldUseColor(os.Stdout),
		template:       template,
		parsedTemplate: parsedTemplate,
	}
	
	// If theme has no colors, disable color output entirely
	if theme.InformationColor == "" && theme.WarningColor == "" && theme.ErrorColor == "" {
		sink.useColor = false
	}
	
	// Analyze template for optimizations
	sink.hasBrackets, sink.potentialStatuses = analyzeTemplate(parsedTemplate)
	
	return sink, nil
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
	
	var message string
	if cs.parsedTemplate != nil {
		// Use template-based formatting with per-token coloring
		message = cs.renderTemplateWithColors(event)
	} else {
		// Use default formatting
		message = cs.formatEvent(event)
	}
	
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

// renderTemplateWithColors renders the template with proper per-token coloring.
func (cs *ConsoleSink) renderTemplateWithColors(event *core.LogEvent) string {
	if cs.parsedTemplate == nil {
		return cs.formatEvent(event)
	}
	
	var sb strings.Builder
	
	for _, token := range cs.parsedTemplate.Tokens {
		text := token.Render(event)
		
		// Apply coloring based on token type
		if propToken, ok := token.(*output.PropertyToken); ok {
			switch propToken.PropertyName {
			case "Timestamp":
				text = colorize(text, cs.theme.TimestampColor, cs.useColor)
			case "Level":
				levelColor := cs.theme.GetLevelColor(event.Level)
				text = colorize(text, levelColor, cs.useColor)
			case "Message":
				// Parse and render message with property coloring
				text = cs.renderMessageWithPropertyColors(event, propToken.Format)
			case "SourceContext":
				// SourceContext uses property color
				text = colorize(text, cs.theme.PropertyValColor, cs.useColor)
			default:
				// Other properties - check for status codes if property might be one
				if cs.isPotentialStatus(propToken.PropertyName) {
					if statusColor := cs.getStatusCodeColor(text); statusColor != "" {
						text = colorize(text, statusColor, cs.useColor)
					} else {
						text = colorize(text, cs.theme.PropertyValColor, cs.useColor)
					}
				} else {
					text = colorize(text, cs.theme.PropertyValColor, cs.useColor)
				}
			}
		}
		// TextToken - check for brackets and apply bracket color
		if cs.hasBrackets {
			if textToken, ok := token.(*output.TextToken); ok {
				text = cs.colorizeTextWithBrackets(text, textToken.Text)
			}
		}
		
		sb.WriteString(text)
	}
	
	return sb.String()
}

// renderMessageWithPropertyColors renders the message with colored property values.
func (cs *ConsoleSink) renderMessageWithPropertyColors(event *core.LogEvent, format string) string {
	// Parse the message template
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		return colorize(event.MessageTemplate, cs.theme.MessageColor, cs.useColor)
	}
	
	var sb strings.Builder
	
	for _, token := range tmpl.Tokens {
		switch t := token.(type) {
		case *parser.TextToken:
			// Regular text uses message color
			sb.WriteString(colorize(t.Text, cs.theme.MessageColor, cs.useColor))
		case *parser.PropertyToken:
			// Property values use property value color
			propText := t.Render(event.Properties)
			if propText != "{"+t.PropertyName+"}" {
				// Property was found and rendered
				// Check if it's a status code and apply appropriate color
				if cs.isPotentialStatus(t.PropertyName) {
					if statusColor := cs.getStatusCodeColor(propText); statusColor != "" {
						sb.WriteString(colorize(propText, statusColor, cs.useColor))
					} else {
						sb.WriteString(colorize(propText, cs.theme.PropertyValColor, cs.useColor))
					}
				} else {
					sb.WriteString(colorize(propText, cs.theme.PropertyValColor, cs.useColor))
				}
			} else {
				// Missing property - show placeholder in message color
				sb.WriteString(colorize(propText, cs.theme.MessageColor, cs.useColor))
			}
		}
	}
	
	return sb.String()
}

// colorizeTextWithBrackets applies bracket coloring to text that contains brackets
func (cs *ConsoleSink) colorizeTextWithBrackets(rendered, original string) string {
	// Common bracket patterns to colorize
	brackets := []string{"[", "]", "(", ")", "{", "}", ":", ","}
	
	for _, bracket := range brackets {
		if strings.Contains(original, bracket) {
			// Apply bracket color to the entire text token if it contains brackets
			return colorize(rendered, cs.theme.BracketColor, cs.useColor)
		}
	}
	
	// No brackets found, return as-is
	return rendered
}

// getStatusCodeColor returns the appropriate color for HTTP status codes
func (cs *ConsoleSink) getStatusCodeColor(value string) Color {
	// Try to parse as integer
	trimmed := strings.TrimSpace(value)
	
	// Check if it looks like a status code (3 digits)
	if len(trimmed) == 3 {
		if code, err := strconv.Atoi(trimmed); err == nil {
			switch {
			case code >= 200 && code < 300:
				// 2xx Success - Green
				return ColorGreen
			case code >= 300 && code < 400:
				// 3xx Redirection - Blue
				return ColorBlue
			case code >= 400 && code < 500:
				// 4xx Client Error - Yellow
				return ColorYellow
			case code >= 500 && code < 600:
				// 5xx Server Error - Red
				return ColorRed
			}
		}
	}
	
	return ""
}

// analyzeTemplate analyzes a template for optimization hints
func analyzeTemplate(parsedTemplate *output.Template) (hasBrackets bool, potentialStatuses []string) {
	if parsedTemplate == nil {
		return false, nil
	}
	
	statusNames := make(map[string]bool)
	
	for _, token := range parsedTemplate.Tokens {
		// Check for brackets in text tokens
		if textToken, ok := token.(*output.TextToken); ok {
			if strings.ContainsAny(textToken.Text, "[](){}:,") {
				hasBrackets = true
			}
		}
		
		// Check for properties that might be status codes
		if propToken, ok := token.(*output.PropertyToken); ok {
			name := strings.ToLower(propToken.PropertyName)
			if strings.Contains(name, "status") || 
			   strings.Contains(name, "code") ||
			   strings.Contains(name, "http") ||
			   name == "response" {
				statusNames[propToken.PropertyName] = true
			}
		}
	}
	
	// Convert map to slice
	for name := range statusNames {
		potentialStatuses = append(potentialStatuses, name)
	}
	
	return hasBrackets, potentialStatuses
}

// isPotentialStatus checks if a property name might contain a status code
func (cs *ConsoleSink) isPotentialStatus(propertyName string) bool {
	if len(cs.potentialStatuses) == 0 {
		// No analysis done, fall back to checking name
		name := strings.ToLower(propertyName)
		return strings.Contains(name, "status") || 
		       strings.Contains(name, "code") ||
		       strings.Contains(name, "http") ||
		       name == "response"
	}
	
	for _, status := range cs.potentialStatuses {
		if status == propertyName {
			return true
		}
	}
	return false
}