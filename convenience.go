package mtlog

import (
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/enrichers"
	"github.com/willibrandon/mtlog/filters"
	"github.com/willibrandon/mtlog/sinks"
)

// Convenience options for common configurations

// WithConsole adds a console sink.
func WithConsole() Option {
	return WithSink(sinks.NewConsoleSink())
}

// WithConsoleProperties adds a console sink that displays properties.
func WithConsoleProperties() Option {
	return WithSink(sinks.NewConsoleSinkWithProperties())
}

// WithFile adds a file sink.
func WithFile(path string) Option {
	return func(c *config) {
		sink, err := sinks.NewFileSink(path)
		if err != nil {
			panic(err) // Configuration errors should fail fast
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithMachineName adds machine name enrichment.
func WithMachineName() Option {
	return WithEnricher(enrichers.NewMachineNameEnricher())
}

// WithTimestamp adds timestamp enrichment.
func WithTimestamp() Option {
	return WithEnricher(enrichers.NewTimestampEnricher())
}

// WithProcess adds process information enrichment.
func WithProcess() Option {
	return WithEnricher(enrichers.NewProcessEnricher())
}

// WithEnvironment adds environment variable enrichment.
func WithEnvironment(variableName, propertyName string) Option {
	return WithEnricher(enrichers.NewEnvironmentEnricherCached(variableName, propertyName))
}

// WithCommonEnvironment adds enrichers for common environment variables.
func WithCommonEnvironment() Option {
	return func(c *config) {
		for _, enricher := range enrichers.CommonEnvironmentEnrichers() {
			c.enrichers = append(c.enrichers, enricher)
		}
	}
}

// WithThreadId adds goroutine ID enrichment.
func WithThreadId() Option {
	return WithEnricher(enrichers.NewThreadIdEnricher())
}

// WithCallers adds caller information enrichment.
func WithCallers(skip int) Option {
	return WithEnricher(enrichers.NewCallersEnricher(skip))
}

// WithCorrelationId adds a fixed correlation ID to all log events.
func WithCorrelationId(correlationId string) Option {
	return WithEnricher(enrichers.NewCorrelationIdEnricher(correlationId))
}

// Filter convenience options

// WithLevelFilter adds a minimum level filter.
func WithLevelFilter(minimumLevel core.LogEventLevel) Option {
	return WithFilter(filters.NewLevelFilter(minimumLevel))
}

// WithPropertyFilter adds a filter that matches a specific property value.
func WithPropertyFilter(propertyName string, expectedValue interface{}) Option {
	return WithFilter(filters.MatchProperty(propertyName, expectedValue))
}

// WithExcludeFilter adds a filter that excludes events matching the predicate.
func WithExcludeFilter(predicate func(*core.LogEvent) bool) Option {
	return WithFilter(filters.ByExcluding(predicate))
}

// WithSampling adds a sampling filter.
func WithSampling(rate float32) Option {
	return WithFilter(filters.NewSamplingFilter(rate))
}

// WithHashSampling adds a hash-based sampling filter.
func WithHashSampling(propertyName string, rate float32) Option {
	return WithFilter(filters.NewHashSamplingFilter(propertyName, rate))
}

// WithRateLimit adds a rate limiting filter.
func WithRateLimit(maxEvents int, windowNanos int64) Option {
	return WithFilter(filters.NewRateLimitFilter(maxEvents, windowNanos))
}

// Level convenience options

// Debug sets the minimum level to Debug.
func Debug() Option {
	return WithMinimumLevel(core.DebugLevel)
}

// Verbose sets the minimum level to Verbose.
func Verbose() Option {
	return WithMinimumLevel(core.VerboseLevel)
}

// Information sets the minimum level to Information.
func Information() Option {
	return WithMinimumLevel(core.InformationLevel)
}

// Warning sets the minimum level to Warning.
func Warning() Option {
	return WithMinimumLevel(core.WarningLevel)
}

// Error sets the minimum level to Error.
func Error() Option {
	return WithMinimumLevel(core.ErrorLevel)
}