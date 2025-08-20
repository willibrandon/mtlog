package suppress_multiple_with

// Logger with all required methods including With
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

func testSuppressMultipleWithDiagnostics() {
	log := &Logger{}
	
	// MTLOG009 and MTLOG010 are suppressed via analyzer flags, so these should NOT produce diagnostics
	log.With("key1", "value1", "key2") // No diagnostic expected (MTLOG009 suppressed)
	log.With(123, "value")            // No diagnostic expected (MTLOG010 suppressed)
	
	// Other With diagnostics should still work
	log.With("", "value")  // want `\[MTLOG013\] With\(\) key is empty and will be ignored`
	
	// Non-With diagnostics should still work  
	log.Information("User {UserId} logged in", 123, 456) // want `\[MTLOG001\] template has 1 properties but 2 arguments provided`
	log.Error("Something went wrong") // want `\[MTLOG006\] suggestion: Error level log without error value, consider including the error or using Warning level`
}