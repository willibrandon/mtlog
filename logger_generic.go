package mtlog

import (
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/parser"
)

// LoggerG is a generic type-safe logger interface
type LoggerG[T any] interface {
	// Logging methods with type safety
	VerboseT(messageTemplate string, value T, args ...any)
	DebugT(messageTemplate string, value T, args ...any)
	InformationT(messageTemplate string, value T, args ...any)
	WarningT(messageTemplate string, value T, args ...any)
	ErrorT(messageTemplate string, value T, args ...any)
	FatalT(messageTemplate string, value T, args ...any)

	// Short method names
	InfoT(messageTemplate string, value T, args ...any)
	WarnT(messageTemplate string, value T, args ...any)

	// ForContextT adds a typed property
	ForContextT(propertyName string, value T) LoggerG[T]
}

// TypedLogger wraps a regular logger with type-safe methods
type TypedLogger[T any] struct {
	logger *logger
}

// NewTyped creates a new typed logger
func NewTyped[T any](opts ...Option) LoggerG[T] {
	return &TypedLogger[T]{
		logger: New(opts...),
	}
}

// VerboseT logs at verbose level with typed value
func (l *TypedLogger[T]) VerboseT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Verbose(messageTemplate, allArgs...)
}

// DebugT logs at debug level with typed value
func (l *TypedLogger[T]) DebugT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Debug(messageTemplate, allArgs...)
}

// InformationT logs at information level with typed value
func (l *TypedLogger[T]) InformationT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Information(messageTemplate, allArgs...)
}

// WarningT logs at warning level with typed value
func (l *TypedLogger[T]) WarningT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Warning(messageTemplate, allArgs...)
}

// ErrorT logs at error level with typed value
func (l *TypedLogger[T]) ErrorT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Error(messageTemplate, allArgs...)
}

// FatalT logs at fatal level with typed value
func (l *TypedLogger[T]) FatalT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Fatal(messageTemplate, allArgs...)
}

// InfoT writes an information-level log event with a typed value (alias for InformationT)
func (l *TypedLogger[T]) InfoT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Info(messageTemplate, allArgs...)
}

// WarnT writes a warning-level log event with a typed value (alias for WarningT)
func (l *TypedLogger[T]) WarnT(messageTemplate string, value T, args ...any) {
	allArgs := append([]any{value}, args...)
	l.logger.Warn(messageTemplate, allArgs...)
}

// ForContextT adds a typed property to the logger context
func (l *TypedLogger[T]) ForContextT(propertyName string, value T) LoggerG[T] {
	// ForContext returns core.Logger interface, but we know it's *logger
	newLogger := l.logger.ForContext(propertyName, value)
	return &TypedLogger[T]{logger: newLogger.(*logger)}
}

// StructuredLogger provides type-safe structured logging
type StructuredLogger struct {
	logger *logger
}

// NewStructured creates a new structured logger
func NewStructured(opts ...Option) *StructuredLogger {
	return &StructuredLogger{
		logger: New(opts...),
	}
}

// Log logs with a typed property bag
func (sl *StructuredLogger) Log(level core.LogEventLevel, messageTemplate string, properties *core.PropertyBag) {
	args := make([]any, 0)
	for k, v := range properties.Properties() {
		args = append(args, v)
		_ = k // Properties are matched positionally in templates
	}
	sl.logger.Write(level, messageTemplate, args...)
}

// LogWith logs with typed properties using a builder pattern
func (sl *StructuredLogger) LogWith() *LogBuilder {
	return &LogBuilder{
		logger:     sl.logger,
		properties: core.NewPropertyBag(),
	}
}

// LogBuilder provides a fluent API for building typed log entries
type LogBuilder struct {
	logger     *logger
	properties *core.PropertyBag
	level      core.LogEventLevel
	template   string
}

// Property adds a property
func (b *LogBuilder) Property(name string, value any) *LogBuilder {
	b.properties.Add(name, value)
	return b
}

// PropertyTyped adds a typed property
func (b *LogBuilder) PropertyTyped(name string, value any) *LogBuilder {
	b.properties.Add(name, value)
	return b
}

// Level sets the log level
func (b *LogBuilder) Level(level core.LogEventLevel) *LogBuilder {
	b.level = level
	return b
}

// Message sets the message template
func (b *LogBuilder) Message(template string) *LogBuilder {
	b.template = template
	return b
}

// Write writes the log entry
func (b *LogBuilder) Write() {
	// For message templates, we need to match properties by name
	// Extract property names from the template
	tmpl, _ := parser.ParseCached(b.template)
	args := make([]any, 0)

	if tmpl != nil {
		// Match properties in template order
		for _, token := range tmpl.Tokens {
			if prop, ok := token.(*parser.PropertyToken); ok {
				if val, exists := b.properties.Get(prop.PropertyName); exists {
					args = append(args, val)
				} else {
					args = append(args, nil)
				}
			}
		}
	}

	b.logger.Write(b.level, b.template, args...)
}
