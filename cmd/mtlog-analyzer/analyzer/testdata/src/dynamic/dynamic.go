package dynamic

import "fmt"

// Simple logger type for testing
type Logger struct{}

func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}

func testDynamicTemplates() {
	log := &Logger{}
	userId := 123
	
	// Dynamic templates should be warned about
	template := fmt.Sprintf("User {%s} logged in", "UserId")
	log.Information(template, userId) // want "warning: dynamic template strings are not analyzed"
	
	// Non-literal templates
	getUserTemplate := func() string { return "User {UserId} logged in" }
	log.Information(getUserTemplate(), userId) // want "warning: dynamic template strings are not analyzed"
	
	// Concatenated strings
	log.Information("User " + "{UserId}" + " logged in", userId) // want "warning: dynamic template strings are not analyzed"
}