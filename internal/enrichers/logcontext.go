package enrichers

import (
	"context"
	
	"github.com/willibrandon/mtlog/core"
)

// LogContextEnricher enriches log events with properties from LogContext.
type LogContextEnricher struct {
	ctx context.Context
	getProperties func(context.Context) map[string]interface{}
}

// NewLogContextEnricher creates an enricher that extracts properties from LogContext.
func NewLogContextEnricher(ctx context.Context, getProperties func(context.Context) map[string]interface{}) *LogContextEnricher {
	return &LogContextEnricher{
		ctx:           ctx,
		getProperties: getProperties,
	}
}

// Enrich adds LogContext properties to the log event.
func (e *LogContextEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if e.ctx == nil || e.getProperties == nil {
		return
	}
	
	properties := e.getProperties(e.ctx)
	for name, value := range properties {
		// Only add if not already present (allows event-specific overrides)
		if _, exists := event.Properties[name]; !exists {
			prop := propertyFactory.CreateProperty(name, value)
			event.Properties[prop.Name] = prop.Value
		}
	}
}

