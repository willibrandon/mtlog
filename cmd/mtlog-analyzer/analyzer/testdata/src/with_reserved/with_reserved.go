package with_reserved

import "fmt"

// Simple logger type for testing With() method
type Logger struct{}

func (l *Logger) Verbose(template string, args ...interface{})     {}
func (l *Logger) V(template string, args ...interface{})           {}
func (l *Logger) Debug(template string, args ...interface{})       {}
func (l *Logger) D(template string, args ...interface{})           {}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Info(template string, args ...interface{})        {}
func (l *Logger) I(template string, args ...interface{})           {}
func (l *Logger) Warning(template string, args ...interface{})     {}
func (l *Logger) Warn(template string, args ...interface{})        {}
func (l *Logger) W(template string, args ...interface{})           {}
func (l *Logger) Error(template string, args ...interface{})       {}
func (l *Logger) Err(template string, args ...interface{})         {}
func (l *Logger) E(template string, args ...interface{})           {}
func (l *Logger) Fatal(template string, args ...interface{})       {}
func (l *Logger) F(template string, args ...interface{})           {}
func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }
func (l *Logger) With(args ...interface{}) *Logger                 { return l }

func testWithReservedProperties() {
	log := &Logger{}
	
	// These should trigger warnings when -check-reserved flag is enabled
	log.With("Timestamp", "custom") // want `\[MTLOG012\] suggestion: property 'Timestamp' shadows a built-in property`
	log.With("Level", "INFO") // want `\[MTLOG012\] suggestion: property 'Level' shadows a built-in property`
	log.With("Message", "custom message") // want `\[MTLOG012\] suggestion: property 'Message' shadows a built-in property`
	log.With("MessageTemplate", "template") // want `\[MTLOG012\] suggestion: property 'MessageTemplate' shadows a built-in property`
	log.With("Exception", fmt.Errorf("error")) // want `\[MTLOG012\] suggestion: property 'Exception' shadows a built-in property`
	log.With("SourceContext", "context") // want `\[MTLOG012\] suggestion: property 'SourceContext' shadows a built-in property`
	
	// Valid cases - not reserved
	log.With("UserId", 123)
	log.With("RequestId", "abc-123")
	log.With("CustomTimestamp", "2024-01-01")
}