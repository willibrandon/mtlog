package with_cross_call

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

func testDirectChaining() {
	log := &Logger{}
	
	// Direct chaining - should detect override
	log.With("id", 1).With("id", 2).Information("test") // want `\[MTLOG011\] warning: With\(\) overrides property 'id' set in previous call`
	
	// Multiple properties with override
	log.With("user", "alice", "id", 1).With("name", "bob", "id", 2).Info("test") // want `\[MTLOG011\] warning: With\(\) overrides property 'id' set in previous call`
	
	// Three-level chaining
	log.With("service", "api").With("version", "1.0").With("service", "auth").Debug("test") // want `\[MTLOG011\] warning: With\(\) overrides property 'service' set in previous call`
}

func testVariableAssignment() {
	log := &Logger{}
	
	// Variable assignment tracking
	baseLogger := log.With("service", "api", "version", "1.0")
	baseLogger.With("service", "auth").Information("override") // want `\[MTLOG011\] warning: With\(\) overrides property 'service' set in previous call`
	
	// Multi-level assignment
	logger2 := baseLogger.With("env", "prod")
	logger2.With("version", "2.0").Info("another override") // want `\[MTLOG011\] warning: With\(\) overrides property 'version' set in previous call`
	
	// No override - different properties
	baseLogger.With("user", "alice").Debug("no override")
}

func testCrossMethodOverride() {
	log := &Logger{}
	
	// With followed by ForContext
	log.With("user", "alice").ForContext("user", "bob").Information("cross-method") // want `\[MTLOG011\] warning: ForContext\(\) overrides property 'user' set in previous call`
	
	// ForContext followed by With
	log.ForContext("request_id", "123").With("request_id", "456").Debug("override") // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'request_id'` `\[MTLOG011\] warning: With\(\) overrides property 'request_id' set in previous call`
	
	// Mixed chaining
	log.With("a", 1).ForContext("b", 2).With("a", 3).Info("mixed") // want `\[MTLOG011\] warning: With\(\) overrides property 'a' set in previous call`
}

func testComplexScenarios() {
	log := &Logger{}
	
	// Complex chain with multiple overrides
	log.With("id", 1, "name", "alice").
		With("age", 30).
		With("id", 2). // want `\[MTLOG011\] warning: With\(\) overrides property 'id' set in previous call`
		With("name", "bob"). // want `\[MTLOG011\] warning: With\(\) overrides property 'name' set in previous call`
		Information("multiple overrides")
	
	// Assignment with chaining
	logger := log.With("base", "value")
	logger.With("x", 1).With("base", "new").With("x", 2).Error("chained overrides", fmt.Errorf("test")) // want `\[MTLOG011\] warning: With\(\) overrides property 'base' set in previous call` `\[MTLOG011\] warning: With\(\) overrides property 'x' set in previous call`
}

func testNoFalsePositives() {
	log := &Logger{}
	
	// Different loggers - no cross-contamination
	logger1 := log.With("id", 1)
	logger2 := log.With("id", 2) // No warning - different base
	
	// Use both
	logger1.Information("logger1")
	logger2.Information("logger2")
	
	// Unique properties - no warnings
	log.With("a", 1).With("b", 2).With("c", 3).Debug("unique props")
	
	// Empty With() calls
	log.With().With().Information("empty calls")
}