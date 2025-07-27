package invalidflags

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	
	// These should still be analyzed - invalid disable flag values should be ignored
	// But "naming" is a valid check name and is disabled
	log.Information("User {userId} logged in", 123) // naming check is disabled, no diagnostic expected
	
	// Other checks should work normally
	log.Information("User {UserId} logged in from {IP}", 123) // want "template has 2 properties but 1 arguments provided"
}