package core

// Destructurer converts complex types to log-appropriate representations.
type Destructurer interface {
	// TryDestructure attempts to destructure a value into a log event property.
	// Returns the property and true if successful, nil and false otherwise.
	TryDestructure(value interface{}, propertyFactory LogEventPropertyFactory) (*LogEventProperty, bool)
}