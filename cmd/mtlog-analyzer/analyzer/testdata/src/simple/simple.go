package simple_test

// Simple logger type for testing
type Logger struct{}

func (l *Logger) Information(template string, args ...interface{}) {}

func testSimple() {
	log := &Logger{}
	
	// This should produce an error
	log.Information("User {UserId} logged in", 123, 456) // want "template has 1 properties but 2 arguments provided"
}