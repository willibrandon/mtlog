package middleware

import (
	"context"
)

// Context-based logging helpers that safely handle nil loggers

// VerboseContext logs a verbose message using the logger from context
func VerboseContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Verbose(template, args...)
	}
}

// DebugContext logs a debug message using the logger from context
func DebugContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Debug(template, args...)
	}
}

// InfoContext logs an information message using the logger from context
func InfoContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Information(template, args...)
	}
}

// InformationContext is an alias for InfoContext
func InformationContext(ctx context.Context, template string, args ...any) {
	InfoContext(ctx, template, args...)
}

// WarnContext logs a warning message using the logger from context
func WarnContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Warning(template, args...)
	}
}

// WarningContext is an alias for WarnContext
func WarningContext(ctx context.Context, template string, args ...any) {
	WarnContext(ctx, template, args...)
}

// ErrorContext logs an error message using the logger from context
func ErrorContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Error(template, args...)
	}
}

// FatalContext logs a fatal message using the logger from context
func FatalContext(ctx context.Context, template string, args ...any) {
	if logger := FromContext(ctx); logger != nil {
		logger.Fatal(template, args...)
	}
}

// WithContext adds structured fields to the logger in context and returns a new context
func WithContext(ctx context.Context, key string, value any) context.Context {
	if logger := FromContext(ctx); logger != nil {
		newLogger := logger.With(key, value)
		return context.WithValue(ctx, LoggerContextKey, newLogger)
	}
	return ctx
}

// WithFieldsContext adds multiple structured fields to the logger in context
func WithFieldsContext(ctx context.Context, fields map[string]any) context.Context {
	if logger := FromContext(ctx); logger != nil {
		for k, v := range fields {
			logger = logger.With(k, v)
		}
		return context.WithValue(ctx, LoggerContextKey, logger)
	}
	return ctx
}