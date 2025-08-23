package mtlog

import (
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
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
	
	// Deadline awareness configuration
	deadlineEnricher *enrichers.DeadlineEnricher
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

// WithContextDeadlineWarning enables automatic context deadline detection and warning.
// When a context deadline is approaching (within the specified threshold), the logger
// will automatically add deadline information to log events and optionally upgrade
// their level to Warning.
//
// Example:
//   logger := mtlog.New(
//       mtlog.WithConsole(),
//       mtlog.WithContextDeadlineWarning(100*time.Millisecond),
//   )
//
// This will warn when operations are within 100ms of their deadline.
func WithContextDeadlineWarning(threshold time.Duration, opts ...enrichers.DeadlineOption) Option {
	return func(c *config) {
		c.deadlineEnricher = enrichers.NewDeadlineEnricher(threshold, opts...)
		// Add the deadline enricher to the pipeline
		c.enrichers = append(c.enrichers, c.deadlineEnricher)
	}
}

// WithDeadlinePercentageThreshold configures deadline warnings based on percentage of time remaining.
// For example, 0.1 means warn when 10% of the total time remains.
//
// This can be used together with absolute threshold - warnings will trigger when either
// condition is met.
func WithDeadlinePercentageThreshold(threshold time.Duration, percent float64, opts ...enrichers.DeadlineOption) Option {
	allOpts := append([]enrichers.DeadlineOption{
		enrichers.WithDeadlinePercentageThreshold(percent),
	}, opts...)
	return WithContextDeadlineWarning(threshold, allOpts...)
}

// WithDeadlinePercentageOnly configures deadline warnings based only on percentage
// of time remaining, without requiring an absolute threshold.
// For example, 0.2 means warn when 20% of time remains.
//
// Example:
//   logger := mtlog.New(
//       mtlog.WithDeadlinePercentageOnly(0.2), // Warn at 20% remaining
//   )
func WithDeadlinePercentageOnly(percent float64, opts ...enrichers.DeadlineOption) Option {
	// Use a very large duration to effectively disable absolute threshold
	// while still allowing the percentage threshold to work
	return WithDeadlinePercentageThreshold(time.Duration(1<<62), percent, opts...)
}

// WithDeadlineOptions applies additional deadline enricher options to an existing configuration.
// This is useful for fine-tuning deadline behavior without recreating the entire enricher.
func WithDeadlineOptions(opts ...enrichers.DeadlineOption) Option {
	return func(c *config) {
		if c.deadlineEnricher == nil {
			// If no deadline enricher exists, create one with default 100ms threshold
			c.deadlineEnricher = enrichers.NewDeadlineEnricher(100*time.Millisecond, opts...)
			c.enrichers = append(c.enrichers, c.deadlineEnricher)
		} else {
			// Apply options to existing enricher
			for _, opt := range opts {
				opt(c.deadlineEnricher)
			}
		}
	}
}
