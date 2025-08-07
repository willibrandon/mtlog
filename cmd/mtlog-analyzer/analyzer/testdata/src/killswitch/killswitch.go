package killswitch_test

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

func testKillSwitch() {
	log := &Logger{}

	// With disable-all flag, NONE of these should produce diagnostics
	log.Information("User {UserId} logged in", 123, 456) // No diagnostic expected
	log.Warning("User {userId} logged in", 123)          // No diagnostic expected
	log.Error("Something went wrong")                    // No diagnostic expected
	log.Information("User {Id} and {Id}", 1, 1)          // No diagnostic expected (duplicate)
}
