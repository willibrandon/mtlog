package core

// Capturer converts complex types to log-appropriate representations.
type Capturer interface {
	// TryCapture attempts to capture a value into a log event property.
	// Returns the property and true if successful, nil and false otherwise.
	TryCapture(value interface{}, propertyFactory LogEventPropertyFactory) (*LogEventProperty, bool)
}