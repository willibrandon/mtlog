package core

import "time"

// LogEvent represents a single log event with all its properties.
type LogEvent struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time
	
	// Level is the severity of the event.
	Level LogEventLevel
	
	// MessageTemplate is the original message template with placeholders.
	MessageTemplate string
	
	// Properties contains the event's properties extracted from the template.
	Properties map[string]interface{}
	
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
func (e *LogEvent) AddProperty(name string, value interface{}) {
	e.Properties[name] = value
}