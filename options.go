package mtlog

import (
	"github.com/willibrandon/mtlog/core"
)

// config holds the configuration for building a logger.
type config struct {
	minimumLevel core.LogEventLevel
	enrichers    []core.LogEventEnricher
	filters      []core.LogEventFilter
	destructurer core.Destructurer
	sinks        []core.LogEventSink
	properties   map[string]interface{}
}

// Option is a functional option for configuring a logger.
type Option func(*config)

// WithMinimumLevel sets the minimum log level.
func WithMinimumLevel(level core.LogEventLevel) Option {
	return func(c *config) {
		c.minimumLevel = level
	}
}

// WithEnricher adds an enricher to the pipeline.
func WithEnricher(enricher core.LogEventEnricher) Option {
	return func(c *config) {
		c.enrichers = append(c.enrichers, enricher)
	}
}

// WithFilter adds a filter to the pipeline.
func WithFilter(filter core.LogEventFilter) Option {
	return func(c *config) {
		c.filters = append(c.filters, filter)
	}
}

// WithDestructurer sets the destructurer for the pipeline.
func WithDestructurer(destructurer core.Destructurer) Option {
	return func(c *config) {
		c.destructurer = destructurer
	}
}

// WithSink adds a sink to the pipeline.
func WithSink(sink core.LogEventSink) Option {
	return func(c *config) {
		c.sinks = append(c.sinks, sink)
	}
}

// WithProperty adds a global property to all log events.
func WithProperty(name string, value interface{}) Option {
	return func(c *config) {
		c.properties[name] = value
	}
}

// WithProperties adds multiple global properties.
func WithProperties(properties map[string]interface{}) Option {
	return func(c *config) {
		for k, v := range properties {
			c.properties[k] = v
		}
	}
}