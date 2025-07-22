package core

// LogValue is an optional interface that types can implement to provide
// custom log representations. When a type implements this interface,
// the destructurer will use the returned value instead of using reflection.
type LogValue interface {
	// LogValue returns the value to be logged. This can be a simple type
	// (string, number, bool) or a complex type (struct, map, slice).
	// The returned value may itself be destructured if it's complex.
	LogValue() interface{}
}