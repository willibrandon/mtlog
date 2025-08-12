package mtlog

import (
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/capture"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/internal/filters"
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

// WithConsoleTemplate adds a console sink with a custom output template.
func WithConsoleTemplate(template string) Option {
	return func(c *config) {
		if c.err != nil {
			return // Don't process if already errored
		}
		sink, err := sinks.NewConsoleSinkWithTemplate(template)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithConsoleTheme adds a console sink with a custom theme.
func WithConsoleTheme(theme *sinks.ConsoleTheme) Option {
	return WithSink(sinks.NewConsoleSinkWithTheme(theme))
}

// WithConsoleTemplateAndTheme adds a console sink with both a custom output template and theme.
func WithConsoleTemplateAndTheme(template string, theme *sinks.ConsoleTheme) Option {
	return func(c *config) {
		if c.err != nil {
			return // Don't process if already errored
		}
		sink, err := sinks.NewConsoleSinkWithTemplateAndTheme(template, theme)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithFile adds a file sink.
func WithFile(path string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewFileSink(path)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithFileTemplate adds a file sink with a custom output template.
func WithFileTemplate(path string, template string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewFileSinkWithTemplate(path, template)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithSeq adds a Seq sink with default configuration.
func WithSeq(serverURL string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewSeqSink(serverURL)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithSeqAPIKey adds a Seq sink with API key authentication.
func WithSeqAPIKey(serverURL, apiKey string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewSeqSink(serverURL, sinks.WithSeqAPIKey(apiKey))
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithSeqAdvanced adds a Seq sink with advanced options.
func WithSeqAdvanced(serverURL string, opts ...sinks.SeqOption) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewSeqSink(serverURL, opts...)
		if err != nil {
			c.err = err
			return
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

// WithProcessInfo is an alias for WithProcess.
func WithProcessInfo() Option {
	return WithProcess()
}

// WithEnvironment adds environment variable enrichment.
func WithEnvironment(variableName, propertyName string) Option {
	return WithEnricher(enrichers.NewEnvironmentEnricherCached(variableName, propertyName))
}

// WithEnvironmentVariables adds enrichers for multiple environment variables.
func WithEnvironmentVariables(variables ...string) Option {
	return func(c *config) {
		for _, v := range variables {
			c.enrichers = append(c.enrichers, enrichers.NewEnvironmentEnricherCached(v, v))
		}
	}
}

// WithCommonEnvironment adds enrichers for common environment variables.
func WithCommonEnvironment() Option {
	return func(c *config) {
		c.enrichers = append(c.enrichers, enrichers.CommonEnvironmentEnrichers()...)
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

// WithCallersInfo adds caller information enrichment with default skip.
func WithCallersInfo() Option {
	return WithEnricher(enrichers.NewCallersEnricher(3))
}

// WithCorrelationId adds a fixed correlation ID to all log events.
func WithCorrelationId(correlationId string) Option {
	return WithEnricher(enrichers.NewCorrelationIdEnricher(correlationId))
}

// WithSourceContext adds source context enrichment with the specified context.
func WithSourceContext(sourceContext string) Option {
	return WithEnricher(enrichers.NewSourceContextEnricher(sourceContext))
}

// WithAutoSourceContext adds automatic source context detection.
func WithAutoSourceContext() Option {
	return WithEnricher(enrichers.NewAutoSourceContextEnricher())
}

// Filter convenience options

// WithLevelFilter adds a minimum level filter.
func WithLevelFilter(minimumLevel core.LogEventLevel) Option {
	return WithFilter(filters.NewLevelFilter(minimumLevel))
}

// WithPropertyFilter adds a filter that matches a specific property value.
func WithPropertyFilter(propertyName string, expectedValue any) Option {
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

// WithMinimumLevelOverrides adds source context-based level filtering.
func WithMinimumLevelOverrides(defaultLevel core.LogEventLevel, overrides map[string]core.LogEventLevel) Option {
	return WithFilter(filters.NewSourceContextLevelFilter(defaultLevel, overrides))
}

// Capturing options

// WithCapturing adds the cached capturer for better performance.
func WithCapturing() Option {
	return WithCapturer(capture.NewCachedCapturer())
}

// WithCapturingDepth adds capturing with a specific max depth.
func WithCapturingDepth(maxDepth int) Option {
	d := capture.NewCachedCapturer()
	d.DefaultCapturer = capture.NewCapturer(maxDepth, 1000, 100)
	return WithCapturer(d)
}

// WithCustomCapturing adds a capturer with custom limits.
func WithCustomCapturing(maxDepth, maxStringLength, maxCollectionCount int) Option {
	return WithCapturer(capture.NewCapturer(maxDepth, maxStringLength, maxCollectionCount))
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

// WithControlledLevel creates a level switch and applies it to the logger.
// Returns both the option and the level switch for external control.
func WithControlledLevel(initialLevel core.LogEventLevel) (Option, *LoggingLevelSwitch) {
	levelSwitch := NewLoggingLevelSwitch(initialLevel)
	return WithLevelSwitch(levelSwitch), levelSwitch
}

// WithDynamicLevel enables dynamic level control using a level switch.
// This is an alias for WithLevelSwitch for better readability.
func WithDynamicLevel(levelSwitch *LoggingLevelSwitch) Option {
	return WithLevelSwitch(levelSwitch)
}

// WithElasticsearch adds an Elasticsearch sink with default configuration.
func WithElasticsearch(url string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewElasticsearchSink(url)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithElasticsearchBasicAuth adds an Elasticsearch sink with basic authentication.
func WithElasticsearchBasicAuth(url, username, password string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewElasticsearchSink(url,
			sinks.WithElasticsearchBasicAuth(username, password))
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithElasticsearchAPIKey adds an Elasticsearch sink with API key authentication.
func WithElasticsearchAPIKey(url, apiKey string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewElasticsearchSink(url,
			sinks.WithElasticsearchAPIKey(apiKey))
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithElasticsearchAdvanced adds an Elasticsearch sink with advanced options.
func WithElasticsearchAdvanced(url string, opts ...sinks.ElasticsearchOption) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewElasticsearchSink(url, opts...)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithSplunk adds a Splunk sink to the logger.
func WithSplunk(url, token string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewSplunkSink(url, token)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithSplunkAdvanced adds a Splunk sink with advanced options.
func WithSplunkAdvanced(url, token string, opts ...sinks.SplunkOption) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewSplunkSink(url, token, opts...)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithDurableBuffer adds durable buffering to a sink for reliability.
func WithDurableBuffer(wrapped core.LogEventSink, bufferPath string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewDurableSink(wrapped, sinks.DurableOptions{
			BufferPath: bufferPath,
		})
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithDurableBufferAdvanced adds durable buffering with advanced options.
func WithDurableBufferAdvanced(wrapped core.LogEventSink, options sinks.DurableOptions) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		sink, err := sinks.NewDurableSink(wrapped, options)
		if err != nil {
			c.err = err
			return
		}
		c.sinks = append(c.sinks, sink)
	}
}

// WithDurableSeq adds a Seq sink with durable buffering.
func WithDurableSeq(serverURL, bufferPath string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		seqSink, err := sinks.NewSeqSink(serverURL)
		if err != nil {
			c.err = err
			return
		}

		durableSink, err := sinks.NewDurableSink(seqSink, sinks.DurableOptions{
			BufferPath: bufferPath,
		})
		if err != nil {
			c.err = err
			return
		}

		c.sinks = append(c.sinks, durableSink)
	}
}

// WithDurableElasticsearch adds an Elasticsearch sink with durable buffering.
func WithDurableElasticsearch(url, bufferPath string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		esSink, err := sinks.NewElasticsearchSink(url)
		if err != nil {
			c.err = err
			return
		}

		durableSink, err := sinks.NewDurableSink(esSink, sinks.DurableOptions{
			BufferPath: bufferPath,
		})
		if err != nil {
			c.err = err
			return
		}

		c.sinks = append(c.sinks, durableSink)
	}
}

// WithDurableSplunk adds a Splunk sink with durable buffering.
func WithDurableSplunk(url, token, bufferPath string) Option {
	return func(c *config) {
		if c.err != nil {
			return
		}
		splunkSink, err := sinks.NewSplunkSink(url, token)
		if err != nil {
			c.err = err
			return
		}

		durableSink, err := sinks.NewDurableSink(splunkSink, sinks.DurableOptions{
			BufferPath: bufferPath,
		})
		if err != nil {
			c.err = err
			return
		}

		c.sinks = append(c.sinks, durableSink)
	}
}
