package core

import (
	"time"

	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/selflog"
)

// LogEvent represents a single log event with all its properties.
type LogEvent struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Level is the severity of the event.
	Level LogEventLevel

	// MessageTemplate is the original message template with placeholders.
	MessageTemplate string

	// Properties contains the event's properties extracted from the template.
	Properties map[string]any

	// Exception associated with the event, if any.
	Exception error
}

// AddPropertyIfAbsent adds a property to the event if it doesn't already exist.
func (e *LogEvent) AddPropertyIfAbsent(property *LogEventProperty) {
	if _, exists := e.Properties[property.Name]; !exists {
		e.Properties[property.Name] = property.Value
	}
}

// AddProperty adds or overwrites a property in the event.
func (e *LogEvent) AddProperty(name string, value any) {
	e.Properties[name] = value
}

// RenderMessage renders the message template with the event's properties.
// This method parses the MessageTemplate and replaces all placeholders with their
// corresponding property values, handling format specifiers, capturing operators,
// and scalar hints.
//
// If parsing fails, the original MessageTemplate is returned as a fallback.
//
// Example:
//
//	event := &LogEvent{
//	    MessageTemplate: "User {UserId} logged in from {City}",
//	    Properties: map[string]any{
//	        "UserId": 123,
//	        "City": "Seattle",
//	    },
//	}
//	message := event.RenderMessage()  // "User 123 logged in from Seattle"
func (e *LogEvent) RenderMessage() string {
	tmpl, err := parser.Parse(e.MessageTemplate)
	if err != nil {
		// Log parsing error to selflog if enabled
		if selflog.IsEnabled() {
			selflog.Printf("[core] template parse error in RenderMessage: %v (template=%q)", err, e.MessageTemplate)
		}
		// Fallback to raw template on parse error
		return e.MessageTemplate
	}

	return tmpl.Render(e.Properties)
}
