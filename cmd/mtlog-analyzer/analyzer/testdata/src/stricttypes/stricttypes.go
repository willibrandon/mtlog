package stricttypes

// Local Logger type that has the right methods but isn't from mtlog
type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

type MyLogger struct{}
func (l *MyLogger) Information(template string, args ...interface{}) {}
func (l *MyLogger) Error(template string, args ...interface{}) {}
func (l *MyLogger) Warning(template string, args ...interface{}) {}
func (l *MyLogger) Debug(template string, args ...interface{}) {}

func test() {
	// These should NOT be analyzed when strict-logger-types is set
	log1 := &Logger{}
	log1.Information("User {UserId} logged in", 123)
	
	log2 := &MyLogger{}
	log2.Information("User {UserId} logged in from {IP}", 123)
}