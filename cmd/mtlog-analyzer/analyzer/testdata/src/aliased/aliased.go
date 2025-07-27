package aliased

// Define a local Logger type to simulate mtlog.Logger
type Logger struct{}

func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

// Test aliased logger types
type MyLogger = Logger

func testAliasedLogger() {
	var log MyLogger
	
	// These should still be caught by the analyzer
	log.Information("User {UserId} logged in from {IP}", 123) // want "template has 2 properties but 1 arguments provided"
	log.Error("Failed to process")                            // want "suggestion: Error level log without error value, consider including the error or using Warning level"
}

// Test interface-based calls
type LoggerInterface interface {
	Information(template string, args ...interface{})
	Error(template string, args ...interface{})
}

func testInterfaceLogger(log LoggerInterface) {
	// These won't be caught (documented limitation)
	log.Information("User {UserId} logged in from {IP}", 123)
	log.Error("Failed to process")
}

// Test unrelated logger type with different name
type UnrelatedLogger struct {
	Name string
}

func (l *UnrelatedLogger) Print(msg string) {}

func testUnrelatedLogger() {
	log := &UnrelatedLogger{Name: "test"}
	// This should NOT be analyzed (different logger type)
	log.Print("This is not mtlog")
}