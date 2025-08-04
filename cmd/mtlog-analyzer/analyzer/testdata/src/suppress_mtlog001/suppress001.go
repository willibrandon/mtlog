package suppress_mtlog001

// Logger with all required methods
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

func testSuppressMTLOG001() {
	log := &Logger{}
	
	// MTLOG001 is suppressed, so no template mismatch errors
	log.Information("User {UserId} logged in", 123, 456) // No diagnostic expected
	log.Warning("User {Name} with {Email}", "John")      // No diagnostic expected
	
	// Other diagnostics should still work
	log.Warning("User {userId} logged in", 123)          // want `\[MTLOG004\] suggestion: consider using PascalCase for property 'userId'`
	log.Error("Something went wrong")                    // want `\[MTLOG006\] suggestion: Error level log without error value, consider including the error or using Warning level`
}