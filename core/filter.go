package core

// LogEventFilter determines which events proceed through the pipeline.
type LogEventFilter interface {
	// IsEnabled returns true if the event should be logged.
	IsEnabled(event *LogEvent) bool
}