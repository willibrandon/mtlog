package enrichers

import (
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// TimestampEnricher adds a high-precision timestamp to log events.
type TimestampEnricher struct {
	propertyName string
}

// NewTimestampEnricher creates a new timestamp enricher.
func NewTimestampEnricher() *TimestampEnricher {
	return &TimestampEnricher{
		propertyName: "Timestamp",
	}
}

// NewTimestampEnricherWithName creates a new timestamp enricher with a custom property name.
func NewTimestampEnricherWithName(propertyName string) *TimestampEnricher {
	return &TimestampEnricher{
		propertyName: propertyName,
	}
}

// Enrich adds the current timestamp to the log event.
func (te *TimestampEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	// Add high-precision timestamp
	prop := propertyFactory.CreateProperty(te.propertyName, time.Now().Format(time.RFC3339Nano))
	event.Properties[prop.Name] = prop.Value
}