package sentry

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
)

// Option configures the Sentry sink.
type Option func(*SentrySink)

// WithEnvironment sets the environment (e.g., "production", "staging").
func WithEnvironment(env string) Option {
	return func(s *SentrySink) {
		s.environment = env
	}
}

// WithRelease sets the release version.
func WithRelease(release string) Option {
	return func(s *SentrySink) {
		s.release = release
	}
}

// WithServerName sets the server name.
func WithServerName(name string) Option {
	return func(s *SentrySink) {
		s.serverName = name
	}
}

// WithMinLevel sets the minimum level for events to be sent to Sentry.
// Events below this level may still be captured as breadcrumbs.
func WithMinLevel(level core.LogEventLevel) Option {
	return func(s *SentrySink) {
		s.minLevel = level
	}
}

// WithBreadcrumbLevel sets the minimum level for breadcrumb collection.
// Events at or above this level but below MinLevel become breadcrumbs.
func WithBreadcrumbLevel(level core.LogEventLevel) Option {
	return func(s *SentrySink) {
		s.breadcrumbLevel = level
	}
}

// WithSampleRate sets the sample rate (0.0 to 1.0).
func WithSampleRate(rate float64) Option {
	return func(s *SentrySink) {
		if rate < 0 {
			rate = 0
		} else if rate > 1 {
			rate = 1
		}
		s.sampleRate = rate
	}
}

// WithMaxBreadcrumbs sets the maximum number of breadcrumbs to keep.
func WithMaxBreadcrumbs(max int) Option {
	return func(s *SentrySink) {
		if max < 0 {
			max = 0
		}
		s.maxBreadcrumbs = max
	}
}

// WithBatchSize sets the batch size for event sending.
func WithBatchSize(size int) Option {
	return func(s *SentrySink) {
		if size < 1 {
			size = 1
		}
		s.batchSize = size
	}
}

// WithBatchTimeout sets the timeout for batch sending.
func WithBatchTimeout(timeout time.Duration) Option {
	return func(s *SentrySink) {
		if timeout < time.Second {
			timeout = time.Second
		}
		s.batchTimeout = timeout
	}
}

// WithFingerprinter sets a custom fingerprinting function for error grouping.
func WithFingerprinter(f Fingerprinter) Option {
	return func(s *SentrySink) {
		s.fingerprinter = f
	}
}

// WithBeforeSend sets a function to modify or filter events before sending.
// Return nil to drop the event.
func WithBeforeSend(processor sentry.EventProcessor) Option {
	return func(s *SentrySink) {
		s.beforeSend = processor
	}
}

// WithIgnoreErrors configures errors to ignore.
func WithIgnoreErrors(errors ...error) Option {
	return func(s *SentrySink) {
		originalBeforeSend := s.beforeSend
		s.beforeSend = func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Check if we should ignore this error
			if hint != nil && hint.OriginalException != nil {
				for _, ignoreErr := range errors {
					if err, ok := hint.OriginalException.(error); ok {
						if err == ignoreErr || err.Error() == ignoreErr.Error() {
							return nil // Drop the event
						}
					}
				}
			}

			// Call original beforeSend if set
			if originalBeforeSend != nil {
				return originalBeforeSend(event, hint)
			}
			return event
		}
	}
}

// WithAttachStacktrace enables stack trace attachment for all levels.
func WithAttachStacktrace(attach bool) Option {
	return func(s *SentrySink) {
		// This is set in client options during initialization
		// We'll need to track this for client creation
	}
}

// Common Fingerprinting Helpers

// ByTemplate groups errors by message template only.
// This is useful when you want all instances of the same log message
// to be grouped together regardless of the actual values.
func ByTemplate() Fingerprinter {
	return func(event *core.LogEvent) []string {
		return []string{event.MessageTemplate}
	}
}

// ByErrorType groups by template and error type.
// This creates separate groups for different error types even if they
// have the same message template.
func ByErrorType() Fingerprinter {
	return func(event *core.LogEvent) []string {
		fp := []string{event.MessageTemplate}
		
		// Check common property names for errors
		for _, key := range []string{"Error", "error", "err", "Exception"} {
			if err, ok := event.Properties[key].(error); ok {
				fp = append(fp, fmt.Sprintf("%T", err))
				break
			}
		}
		
		return fp
	}
}

// ByProperty groups by template and a specific property value.
// This is useful for grouping by user ID, tenant ID, or other identifiers.
func ByProperty(propertyName string) Fingerprinter {
	return func(event *core.LogEvent) []string {
		fp := []string{event.MessageTemplate}
		
		if val, ok := event.Properties[propertyName]; ok {
			fp = append(fp, fmt.Sprint(val))
		}
		
		return fp
	}
}

// ByMultipleProperties groups by template and multiple property values.
// This allows for fine-grained grouping based on multiple dimensions.
func ByMultipleProperties(propertyNames ...string) Fingerprinter {
	return func(event *core.LogEvent) []string {
		fp := []string{event.MessageTemplate}
		
		for _, name := range propertyNames {
			if val, ok := event.Properties[name]; ok {
				fp = append(fp, fmt.Sprint(val))
			}
		}
		
		return fp
	}
}

// Custom creates a fingerprinter that uses a custom function to generate fingerprints.
// The function receives the event and should return a unique identifier string.
func Custom(fn func(*core.LogEvent) string) Fingerprinter {
	return func(event *core.LogEvent) []string {
		return []string{fn(event)}
	}
}