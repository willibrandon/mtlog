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