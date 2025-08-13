package otel

import (
	"fmt"
	"github.com/willibrandon/mtlog/selflog"
)

// logLevel represents the severity of internal log messages
type logLevel int

const (
	logDebug logLevel = iota
	logInfo
	logWarn
	logError
)

// internalLogger provides consistent logging for OTEL components
type internalLogger struct {
	prefix string
}

// newInternalLogger creates a logger with the given prefix
func newInternalLogger(prefix string) *internalLogger {
	return &internalLogger{
		prefix: prefix,
	}
}

// log outputs a message if selflog is enabled
func (l *internalLogger) log(level logLevel, format string, args ...interface{}) {
	if !selflog.IsEnabled() {
		return
	}
	
	var levelStr string
	switch level {
	case logDebug:
		levelStr = "DEBUG"
	case logInfo:
		levelStr = "INFO"
	case logWarn:
		levelStr = "WARN"
	case logError:
		levelStr = "ERROR"
	default:
		levelStr = "UNKNOWN"
	}
	
	msg := fmt.Sprintf(format, args...)
	selflog.Printf("[%s] %s %s", l.prefix, levelStr, msg)
}

// Debug logs a debug message
func (l *internalLogger) Debug(format string, args ...interface{}) {
	l.log(logDebug, format, args...)
}

// Info logs an info message
func (l *internalLogger) Info(format string, args ...interface{}) {
	l.log(logInfo, format, args...)
}

// Warn logs a warning message
func (l *internalLogger) Warn(format string, args ...interface{}) {
	l.log(logWarn, format, args...)
}

// Error logs an error message
func (l *internalLogger) Error(format string, args ...interface{}) {
	l.log(logError, format, args...)
}

// Package-level loggers for different components
var (
	enricherLog = newInternalLogger("otel-enricher")
	sinkLog     = newInternalLogger("otlp")
	bridgeLog   = newInternalLogger("otel-bridge")
)