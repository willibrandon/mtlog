package emptyflags

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	
	// These should still be analyzed - empty flag values should be handled gracefully
	log.Information("User {userId} logged in", 123) // want "suggestion: consider using PascalCase for property 'userId'"
	log.Information("User {UserId} logged in from {IP}", 123) // want "template has 2 properties but 1 arguments provided"
}