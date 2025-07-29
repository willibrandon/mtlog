package logr

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/willibrandon/mtlog/core"
)

// LogrSink implements logr.LogSink backed by mtlog's logger.
//
// This sink adapts logr's logging interface to mtlog's structured logging pipeline,
// preserving all the power of mtlog while maintaining full compatibility with logr.
//
// The sink handles:
//   - V-level mapping to mtlog levels (V0→Info, V1→Debug, V2+→Verbose)
//   - Key/value pair conversion to mtlog properties
//   - Logger name hierarchies (controller.reconciler)
//   - Persistent values via WithValues
//   - Error logging with proper error context
type LogrSink struct {
	logger core.Logger
	name   string
	values []interface{}
}

var _ logr.LogSink = (*LogrSink)(nil)

// NewLogrSink creates a new logr.LogSink that writes to the provided mtlog logger.
//
// The returned sink can be passed to logr.New() to create a logr.Logger:
//
//	mtlogLogger := mtlog.New(mtlog.WithConsole())
//	logrLogger := logr.New(mtlogr.NewLogrSink(mtlogLogger))
//
// All log events from logr will be processed through mtlog's pipeline,
// including enrichment, filtering, destructuring, and output to configured sinks.
func NewLogrSink(logger core.Logger) *LogrSink {
	return &LogrSink{
		logger: logger,
		values: []interface{}{},
	}
}

// Init receives optional information about the logr library for the LogSink.
//
// Currently this is a no-op as mtlog handles caller information through
// its own enrichers (e.g., WithCallers).
func (s *LogrSink) Init(info logr.RuntimeInfo) {
	// Store caller info if needed
}

// Enabled tests whether this LogSink is enabled at the given V-level.
//
// V-levels are mapped as follows:
//   - V(0) → Information (always enabled unless filtered)
//   - V(1) → Debug
//   - V(2+) → Verbose
//
// This allows fine-grained control over verbosity using mtlog's level system.
func (s *LogrSink) Enabled(level int) bool {
	// logr levels are inverted: 0 is info, higher is more verbose
	// Map to mtlog levels
	mtlogLevel := logrLevelToMtlog(level)
	return s.logger.IsEnabled(mtlogLevel)
}

// Info logs a non-error message with the given key/value pairs.
//
// The message is logged at the appropriate mtlog level based on the V-level.
// All key/value pairs (both persistent values and those passed here) are
// added as properties to the log event.
func (s *LogrSink) Info(level int, msg string, keysAndValues ...interface{}) {
	mtlogLevel := logrLevelToMtlog(level)
	
	// Build logger with context from stored values and new key/values
	logger := s.applyKeysAndValues(s.logger, append(s.values, keysAndValues...)...)
	
	// Log the message
	switch mtlogLevel {
	case core.VerboseLevel:
		logger.Verbose(msg)
	case core.DebugLevel:
		logger.Debug(msg)
	default:
		logger.Information(msg)
	}
}

// Error logs an error message with the given key/value pairs.
//
// The error is added to the log event context under the "error" property,
// and the message is logged at Error level. All key/value pairs are included
// as properties.
func (s *LogrSink) Error(err error, msg string, keysAndValues ...interface{}) {
	// Add error to context
	logger := s.logger.ForContext("error", err)
	
	// Apply stored values and new key/values
	logger = s.applyKeysAndValues(logger, append(s.values, keysAndValues...)...)
	
	// Log as error
	logger.Error(msg)
}

// WithValues returns a new LogSink with additional key/value pairs.
//
// These values will be included in all subsequent log messages from the
// returned LogSink. This is useful for adding persistent context like
// request IDs or user information.
//
// Example:
//
//	logger = logger.WithValues("request_id", "123", "user", "alice")
//	logger.Info("processing") // includes request_id and user
func (s *LogrSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &LogrSink{
		logger: s.logger,
		name:   s.name,
		values: append(s.values, keysAndValues...),
	}
}

// WithName returns a new LogSink with the specified name appended.
//
// Names create a hierarchy separated by dots. The full logger name is
// included as the "logger" property in all log events.
//
// Example:
//
//	logger = logger.WithName("controller").WithName("reconciler")
//	logger.Info("starting") // includes logger="controller.reconciler"
func (s *LogrSink) WithName(name string) logr.LogSink {
	var newName string
	if s.name == "" {
		newName = name
	} else {
		newName = s.name + "." + name
	}
	
	// Add logger name to context
	logger := s.logger.ForContext("logger", newName)
	
	return &LogrSink{
		logger: logger,
		name:   newName,
		values: s.values,
	}
}

// applyKeysAndValues adds key/value pairs to the logger context
func (s *LogrSink) applyKeysAndValues(logger core.Logger, keysAndValues ...interface{}) core.Logger {
	// Process key/value pairs
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			// Odd number of arguments, add the key with nil value
			logger = logger.ForContext(fmt.Sprint(keysAndValues[i]), nil)
			break
		}
		
		key := fmt.Sprint(keysAndValues[i])
		value := keysAndValues[i+1]
		logger = logger.ForContext(key, value)
	}
	
	return logger
}

// logrLevelToMtlog converts logr V-levels to mtlog levels
// logr levels: 0=info, 1=debug, 2+=verbose
func logrLevelToMtlog(level int) core.LogEventLevel {
	switch level {
	case 0:
		return core.InformationLevel
	case 1:
		return core.DebugLevel
	default:
		return core.VerboseLevel
	}
}