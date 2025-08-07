// Stub mtlog package for tests
package mtlog

// Logger is a stub for the mtlog logger
type Logger struct{}

// New creates a new logger
func New() *Logger { return &Logger{} }

// Debug logs at debug level
func (l *Logger) Debug(template string, args ...interface{}) {}

// Information logs at information level  
func (l *Logger) Information(template string, args ...interface{}) {}

// Warning logs at warning level
func (l *Logger) Warning(template string, args ...interface{}) {}

// Error logs at error level
func (l *Logger) Error(template string, args ...interface{}) {}

// Fatal logs at fatal level
func (l *Logger) Fatal(template string, args ...interface{}) {}

// Verbose logs at verbose level
func (l *Logger) Verbose(template string, args ...interface{}) {}

// Short method names
func (l *Logger) V(template string, args ...interface{}) {}
func (l *Logger) D(template string, args ...interface{}) {}
func (l *Logger) I(template string, args ...interface{}) {}
func (l *Logger) W(template string, args ...interface{}) {}
func (l *Logger) E(template string, args ...interface{}) {}
func (l *Logger) F(template string, args ...interface{}) {}

// ForContext returns a logger for the given context
func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }