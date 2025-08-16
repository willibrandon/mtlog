// Package core provides the fundamental interfaces and types for mtlog.
package core

import "context"

// Logger is the main logging interface providing structured logging methods.
type Logger interface {
	// Verbose writes a verbose-level log event.
	Verbose(messageTemplate string, args ...any)

	// Debug writes a debug-level log event.
	Debug(messageTemplate string, args ...any)

	// Information writes an information-level log event.
	Information(messageTemplate string, args ...any)

	// Warning writes a warning-level log event.
	Warning(messageTemplate string, args ...any)

	// Error writes an error-level log event.
	Error(messageTemplate string, args ...any)

	// Fatal writes a fatal-level log event.
	Fatal(messageTemplate string, args ...any)

	// Write writes a log event at the specified level.
	Write(level LogEventLevel, messageTemplate string, args ...any)

	// ForContext creates a logger that enriches events with the specified property.
	ForContext(propertyName string, value any) Logger

	// WithContext creates a logger that enriches events with context values.
	WithContext(ctx context.Context) Logger

	// With creates a logger that enriches events with the specified key-value pairs.
	// Keys must be strings. Values can be any type.
	// The key-value pairs should be provided in the order: key1, value1, key2, value2, ...
	// If an odd number of arguments is provided, the last argument is ignored.
	With(args ...any) Logger

	// IsEnabled returns true if events at the specified level would be processed.
	IsEnabled(level LogEventLevel) bool

	// Short method names for idiomatic Go usage

	// Info writes an information-level log event (alias for Information).
	Info(messageTemplate string, args ...any)

	// Warn writes a warning-level log event (alias for Warning).
	Warn(messageTemplate string, args ...any)
}
