package core

// LogEventSink outputs log events to a destination.
type LogEventSink interface {
	// Emit writes the log event to the sink's destination.
	Emit(event *LogEvent)
	
	// Close releases any resources held by the sink.
	Close() error
}