package downgrade

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	
	// These would normally be errors, but with -downgrade-errors they become warnings
	log.Information("User {UserId} logged in from {IP}", 123) // want "warning: template has 2 properties but 1 arguments provided"
	log.Information("User {UserId} {UserId} logged in", 123, 456) // want "warning: duplicate property 'UserId' in template"
	log.Information("User {User Id} logged in", 123) // want "warning: property name 'User Id' contains spaces"
	log.Information("Count: {Count:reallyinvalidformat}", 10) // want "warning: invalid format specifier in property 'Count:reallyinvalidformat': unknown format specifier: reallyinvalidformat"
	
	// These remain as warnings/suggestions even with downgrade
	log.Information("User {userId} logged in", 123) // want "suggestion: consider using PascalCase for property 'userId'"
	log.Error("Something failed") // want "suggestion: Error level log without error value, consider including the error or using Warning level"
}