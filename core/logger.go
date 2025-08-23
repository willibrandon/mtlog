// Package core provides the fundamental interfaces and types for mtlog.
package core

import (
	"context"
	"time"
)

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

	// Sampling methods for per-message control

	// Sample creates a logger that samples every nth message.
	Sample(n uint64) Logger

	// SampleDuration creates a logger that samples at most once per duration.
	SampleDuration(duration time.Duration) Logger

	// SampleRate creates a logger that samples a percentage of messages (0.0 to 1.0).
	SampleRate(rate float32) Logger

	// SampleFirst creates a logger that logs only the first n occurrences.
	SampleFirst(n uint64) Logger

	// SampleGroup creates a logger that samples messages within a named group.
	SampleGroup(groupName string, n uint64) Logger

	// SampleWhen creates a logger that samples conditionally based on a predicate.
	SampleWhen(predicate func() bool, n uint64) Logger

	// SampleBackoff creates a logger that samples with exponential backoff.
	SampleBackoff(key string, factor float64) Logger

	// ResetSampling resets all sampling counters for this logger.
	ResetSampling()

	// ResetSamplingGroup resets the sampling counter for a specific group.
	ResetSamplingGroup(groupName string)
	
	// EnableSamplingSummary enables periodic emission of sampling summary events.
	EnableSamplingSummary(period time.Duration) Logger
	
	// GetSamplingStats returns current sampling statistics.
	GetSamplingStats() (sampled uint64, skipped uint64)
	
	// SampleProfile applies a predefined sampling profile optimized for common scenarios.
	SampleProfile(profileName string) Logger
	
	// SampleAdaptive creates a logger that adjusts sampling rates based on target events per second.
	SampleAdaptive(targetEventsPerSecond uint64) Logger
	
	// SampleAdaptiveWithOptions creates a logger with adaptive sampling and custom parameters.
	SampleAdaptiveWithOptions(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration) Logger
}
