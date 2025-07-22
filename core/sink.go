package core

import "time"

// LogEventSink outputs log events to a destination.
type LogEventSink interface {
	// Emit writes the log event to the sink's destination.
	Emit(event *LogEvent)
	
	// Close releases any resources held by the sink.
	Close() error
}

// SimpleSink is an optional interface for sinks that support zero-allocation simple logging.
type SimpleSink interface {
	LogEventSink
	
	// EmitSimple writes a simple log message without allocations.
	EmitSimple(timestamp time.Time, level LogEventLevel, message string)
}