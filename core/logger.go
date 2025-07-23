// Package core provides the fundamental interfaces and types for mtlog.
package core

import "context"

// Logger is the main logging interface providing structured logging methods.
type Logger interface {
	// Verbose writes a verbose-level log event.
	Verbose(messageTemplate string, args ...interface{})
	
	// Debug writes a debug-level log event.
	Debug(messageTemplate string, args ...interface{})
	
	// Information writes an information-level log event.
	Information(messageTemplate string, args ...interface{})
	
	// Warning writes a warning-level log event.
	Warning(messageTemplate string, args ...interface{})
	
	// Error writes an error-level log event.
	Error(messageTemplate string, args ...interface{})
	
	// Fatal writes a fatal-level log event.
	Fatal(messageTemplate string, args ...interface{})
	
	// Write writes a log event at the specified level.
	Write(level LogEventLevel, messageTemplate string, args ...interface{})
	
	// ForContext creates a logger that enriches events with the specified property.
	ForContext(propertyName string, value interface{}) Logger
	
	// WithContext creates a logger that enriches events with context values.
	WithContext(ctx context.Context) Logger
	
	// IsEnabled returns true if events at the specified level would be processed.
	IsEnabled(level LogEventLevel) bool
	
	// Short method names for idiomatic Go usage
	
	// Info writes an information-level log event (alias for Information).
	Info(messageTemplate string, args ...interface{})
	
	// Warn writes a warning-level log event (alias for Warning).
	Warn(messageTemplate string, args ...interface{})
}