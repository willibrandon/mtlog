package core

// LogEventEnricher adds contextual properties to log events.
type LogEventEnricher interface {
	// Enrich adds properties to the provided log event.
	Enrich(event *LogEvent, propertyFactory LogEventPropertyFactory)
}