package mtlog

import (
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// config holds the configuration for building a logger.
type config struct {
	minimumLevel core.LogEventLevel
	levelSwitch  *LoggingLevelSwitch
	enrichers    []core.LogEventEnricher
	filters      []core.LogEventFilter
	capturer     core.Capturer
	sinks        []core.LogEventSink
	properties   map[string]any
	err          error // First error encountered during configuration
}

// Option is a functional option for configuring a logger.
type Option func(*config)

// WithMinimumLevel sets the minimum log level.
func WithMinimumLevel(level core.LogEventLevel) Option {
	return func(c *config) {
		c.minimumLevel = level
	}
}

// WithLevelSwitch enables dynamic level control using the specified level switch.
// When a level switch is provided, it takes precedence over the static minimum level.
func WithLevelSwitch(levelSwitch *LoggingLevelSwitch) Option {
	return func(c *config) {
		c.levelSwitch = levelSwitch
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

// WithCapturer sets the capturer for the pipeline.
func WithCapturer(capturer core.Capturer) Option {
	return func(c *config) {
		c.capturer = capturer
	}
}

// WithSink adds a sink to the pipeline.
func WithSink(sink core.LogEventSink) Option {
	return func(c *config) {
		c.sinks = append(c.sinks, sink)
	}
}

// WithProperty adds a global property to all log events.
func WithProperty(name string, value any) Option {
	return func(c *config) {
		c.properties[name] = value
	}
}

// WithProperties adds multiple global properties.
func WithProperties(properties map[string]any) Option {
	return func(c *config) {
		for k, v := range properties {
			c.properties[k] = v
		}
	}
}

// WithConditional adds a conditional sink that only forwards events matching the predicate.
func WithConditional(predicate func(*core.LogEvent) bool, sink core.LogEventSink) Option {
	return func(c *config) {
		conditionalSink := sinks.NewConditionalSink(predicate, sink)
		c.sinks = append(c.sinks, conditionalSink)
	}
}

// WithRouter adds a router sink for sophisticated event routing.
func WithRouter(routes ...sinks.Route) Option {
	return WithRouterMode(sinks.FirstMatch, routes...)
}

// WithRouterMode adds a router sink with a specific routing mode.
func WithRouterMode(mode sinks.RoutingMode, routes ...sinks.Route) Option {
	return func(c *config) {
		routerSink := sinks.NewRouterSink(mode, routes...)
		c.sinks = append(c.sinks, routerSink)
	}
}

// WithRouterDefault adds a router sink with a default fallback sink.
func WithRouterDefault(mode sinks.RoutingMode, defaultSink core.LogEventSink, routes ...sinks.Route) Option {
	return func(c *config) {
		routerSink := sinks.NewRouterSinkWithDefault(mode, defaultSink, routes...)
		c.sinks = append(c.sinks, routerSink)
	}
}

// Route creates a new route builder for fluent route configuration.
func Route(name string) *sinks.RouteBuilder {
	return sinks.NewRoute(name)
}
