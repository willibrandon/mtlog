package core

// PropertyValue represents a typed property value using generics
type PropertyValue[T any] struct {
	value T
}

// NewPropertyValue creates a new typed property value
func NewPropertyValue[T any](value T) PropertyValue[T] {
	return PropertyValue[T]{value: value}
}

// Value returns the underlying value
func (p PropertyValue[T]) Value() T {
	return p.value
}

// ToLogEventProperty converts to a LogEventProperty
func (p PropertyValue[T]) ToLogEventProperty(name string, factory LogEventPropertyFactory) *LogEventProperty {
	return factory.CreateProperty(name, p.value)
}

// PropertyBag is a type-safe property collection
type PropertyBag struct {
	properties map[string]interface{}
}

// NewPropertyBag creates a new property bag
func NewPropertyBag() *PropertyBag {
	return &PropertyBag{
		properties: make(map[string]interface{}),
	}
}

// Add adds a typed property to the bag
func (pb *PropertyBag) Add(name string, value interface{}) {
	pb.properties[name] = value
}

// Get retrieves a property from the bag
func (pb *PropertyBag) Get(name string) (interface{}, bool) {
	val, ok := pb.properties[name]
	return val, ok
}

// GetTyped retrieves a typed property from the bag
func GetTyped[T any](pb *PropertyBag, name string) (T, bool) {
	var zero T
	if val, ok := pb.properties[name]; ok {
		if typed, ok := val.(T); ok {
			return typed, true
		}
	}
	return zero, false
}

// AddTyped adds a typed property to the bag
func AddTyped[T any](pb *PropertyBag, name string, value T) {
	pb.properties[name] = value
}

// Properties returns all properties as a map
func (pb *PropertyBag) Properties() map[string]interface{} {
	return pb.properties
}