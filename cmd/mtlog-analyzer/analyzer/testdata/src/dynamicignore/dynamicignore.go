package dynamicignore

import "fmt"

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	userId := 123
	
	// Dynamic templates should NOT be warned about when ignore-dynamic-templates is set
	template := fmt.Sprintf("User {%s} logged in", "UserId")
	log.Information(template, userId)
	
	// Non-literal templates
	getUserTemplate := func() string { return "User {UserId} logged in" }
	log.Information(getUserTemplate(), userId)
	
	// Concatenated strings
	log.Information("User " + "{UserId}" + " logged in", userId)
}