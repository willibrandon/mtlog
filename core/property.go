package core

// LogEventProperty represents a single property of a log event.
type LogEventProperty struct {
	// Name is the property name.
	Name string

	// Value is the property value.
	Value any
}

// LogEventPropertyFactory creates log event properties.
type LogEventPropertyFactory interface {
	// CreateProperty creates a new log event property.
	CreateProperty(name string, value any) *LogEventProperty
}
