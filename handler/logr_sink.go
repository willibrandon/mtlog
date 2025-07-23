package handler

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/willibrandon/mtlog/core"
)

// LogrSink implements logr.LogSink backed by mtlog's logger
type LogrSink struct {
	logger core.Logger
	name   string
	values []interface{}
}

var _ logr.LogSink = (*LogrSink)(nil)

// NewLogrSink creates a new logr.LogSink that writes to the provided mtlog logger
func NewLogrSink(logger core.Logger) *LogrSink {
	return &LogrSink{
		logger: logger,
		values: []interface{}{},
	}
}

// Init receives optional information about the logr library
func (s *LogrSink) Init(info logr.RuntimeInfo) {
	// Store caller info if needed
}

// Enabled tests whether this LogSink is enabled at the given V-level
func (s *LogrSink) Enabled(level int) bool {
	// logr levels are inverted: 0 is info, higher is more verbose
	// Map to mtlog levels
	mtlogLevel := logrLevelToMtlog(level)
	return s.logger.IsEnabled(mtlogLevel)
}

// Info logs a non-error message with the given key/value pairs
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

// Error logs an error message with the given key/value pairs
func (s *LogrSink) Error(err error, msg string, keysAndValues ...interface{}) {
	// Add error to context
	logger := s.logger.ForContext("error", err)
	
	// Apply stored values and new key/values
	logger = s.applyKeysAndValues(logger, append(s.values, keysAndValues...)...)
	
	// Log as error
	logger.Error(msg)
}

// WithValues returns a new LogSink with additional key/value pairs
func (s *LogrSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &LogrSink{
		logger: s.logger,
		name:   s.name,
		values: append(s.values, keysAndValues...),
	}
}

// WithName returns a new LogSink with the specified name appended
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