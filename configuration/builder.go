package configuration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/internal/filters"
	"github.com/willibrandon/mtlog/selflog"
	"github.com/willibrandon/mtlog/sinks"
)

// LoggerBuilder builds a logger from configuration.
type LoggerBuilder struct {
	sinkFactories     map[string]SinkFactory
	enricherFactories map[string]EnricherFactory
	filterFactories   map[string]FilterFactory
}

// SinkFactory creates a sink from configuration.
type SinkFactory func(args map[string]any) (core.LogEventSink, error)

// EnricherFactory creates an enricher from configuration.
type EnricherFactory func(args map[string]any) (core.LogEventEnricher, error)

// FilterFactory creates a filter from configuration.
type FilterFactory func(args map[string]any) (core.LogEventFilter, error)

// NewLoggerBuilder creates a new logger builder with default factories.
func NewLoggerBuilder() *LoggerBuilder {
	lb := &LoggerBuilder{
		sinkFactories:     make(map[string]SinkFactory),
		enricherFactories: make(map[string]EnricherFactory),
		filterFactories:   make(map[string]FilterFactory),
	}

	// Register default sinks
	lb.RegisterSink("Console", createConsoleSink)
	lb.RegisterSink("File", createFileSink)
	lb.RegisterSink("RollingFile", createRollingFileSink)
	lb.RegisterSink("Seq", createSeqSink)
	lb.RegisterSink("Elasticsearch", createElasticsearchSink)
	lb.RegisterSink("Splunk", createSplunkSink)
	lb.RegisterSink("Async", createAsyncSink)
	lb.RegisterSink("Durable", createDurableSink)

	// Register default enrichers
	lb.RegisterEnricher("WithMachineName", func(args map[string]any) (core.LogEventEnricher, error) {
		return enrichers.NewMachineNameEnricher(), nil
	})
	lb.RegisterEnricher("WithThreadId", func(args map[string]any) (core.LogEventEnricher, error) {
		return enrichers.NewThreadIdEnricher(), nil
	})
	lb.RegisterEnricher("WithProcessId", func(args map[string]any) (core.LogEventEnricher, error) {
		return enrichers.NewProcessEnricher(), nil
	})
	lb.RegisterEnricher("WithEnvironmentName", func(args map[string]any) (core.LogEventEnricher, error) {
		name := GetString(args, "environmentName", "Production")
		// Create a simple enricher that adds a fixed "EnvironmentName" property
		return &fixedPropertyEnricher{propertyName: "EnvironmentName", value: name}, nil
	})

	// Register default filters
	lb.RegisterFilter("ByLevel", func(args map[string]any) (core.LogEventFilter, error) {
		levelStr := GetString(args, "minimumLevel", "Information")
		level, err := ParseLevel(levelStr)
		if err != nil {
			return nil, err
		}
		return filters.MinimumLevelFilter(level), nil
	})

	return lb
}

// RegisterSink registers a sink factory.
func (lb *LoggerBuilder) RegisterSink(name string, factory SinkFactory) {
	lb.sinkFactories[name] = factory
}

// RegisterEnricher registers an enricher factory.
func (lb *LoggerBuilder) RegisterEnricher(name string, factory EnricherFactory) {
	lb.enricherFactories[name] = factory
}

// RegisterFilter registers a filter factory.
func (lb *LoggerBuilder) RegisterFilter(name string, factory FilterFactory) {
	lb.filterFactories[name] = factory
}

// Build creates a logger from configuration.
func (lb *LoggerBuilder) Build(config *Configuration) (core.Logger, error) {
	var options []mtlog.Option

	// Set minimum level
	if config.Mtlog.MinimumLevel != "" {
		level, err := ParseLevel(config.Mtlog.MinimumLevel)
		if err != nil {
			return nil, fmt.Errorf("invalid minimum level: %w", err)
		}
		options = append(options, mtlog.WithMinimumLevel(level))
	}

	// Add sinks
	for _, sinkConfig := range config.Mtlog.WriteTo {
		sink, err := lb.createSink(sinkConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create sink %s: %w", sinkConfig.Name, err)
		}
		options = append(options, mtlog.WithSink(sink))
	}

	// Add enrichers
	for _, enricherName := range config.Mtlog.Enrich {
		enricher, err := lb.createEnricherByName(enricherName)
		if err != nil {
			return nil, fmt.Errorf("failed to create enricher %s: %w", enricherName, err)
		}
		options = append(options, mtlog.WithEnricher(enricher))
	}

	// Add enrichers with args
	for _, enricherConfig := range config.Mtlog.EnrichWith {
		enricher, err := lb.createEnricher(enricherConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create enricher %s: %w", enricherConfig.Name, err)
		}
		options = append(options, mtlog.WithEnricher(enricher))
	}

	// Add filters
	for _, filterConfig := range config.Mtlog.Filter {
		filter, err := lb.createFilter(filterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create filter %s: %w", filterConfig.Name, err)
		}
		options = append(options, mtlog.WithFilter(filter))
	}

	// Add properties
	for key, value := range config.Mtlog.Properties {
		options = append(options, mtlog.WithProperty(key, value))
	}

	return mtlog.New(options...), nil
}

// createSink creates a sink from configuration.
func (lb *LoggerBuilder) createSink(config SinkConfiguration) (core.LogEventSink, error) {
	factory, ok := lb.sinkFactories[config.Name]
	if !ok {
		if selflog.IsEnabled() {
			selflog.Printf("[configuration] unknown sink type '%s', available sinks: %v", config.Name, lb.getAvailableSinkNames())
		}
		return nil, fmt.Errorf("unknown sink: %s", config.Name)
	}

	return factory(config.Args)
}

// createEnricherByName creates an enricher by name without args.
func (lb *LoggerBuilder) createEnricherByName(name string) (core.LogEventEnricher, error) {
	factory, ok := lb.enricherFactories[name]
	if !ok {
		if selflog.IsEnabled() {
			selflog.Printf("[configuration] unknown enricher '%s', available enrichers: %v", name, lb.getAvailableEnricherNames())
		}
		return nil, fmt.Errorf("unknown enricher: %s", name)
	}

	return factory(nil)
}

// createEnricher creates an enricher from configuration.
func (lb *LoggerBuilder) createEnricher(config EnricherConfiguration) (core.LogEventEnricher, error) {
	factory, ok := lb.enricherFactories[config.Name]
	if !ok {
		if selflog.IsEnabled() {
			selflog.Printf("[configuration] unknown enricher '%s', available enrichers: %v", config.Name, lb.getAvailableEnricherNames())
		}
		return nil, fmt.Errorf("unknown enricher: %s", config.Name)
	}

	return factory(config.Args)
}

// createFilter creates a filter from configuration.
func (lb *LoggerBuilder) createFilter(config FilterConfiguration) (core.LogEventFilter, error) {
	factory, ok := lb.filterFactories[config.Name]
	if !ok {
		if selflog.IsEnabled() {
			selflog.Printf("[configuration] unknown filter '%s', available filters: %v", config.Name, lb.getAvailableFilterNames())
		}
		return nil, fmt.Errorf("unknown filter: %s", config.Name)
	}

	return factory(config.Args)
}

// getAvailableSinkNames returns a list of registered sink names
func (lb *LoggerBuilder) getAvailableSinkNames() []string {
	names := make([]string, 0, len(lb.sinkFactories))
	for name := range lb.sinkFactories {
		names = append(names, name)
	}
	return names
}

// getAvailableEnricherNames returns a list of registered enricher names
func (lb *LoggerBuilder) getAvailableEnricherNames() []string {
	names := make([]string, 0, len(lb.enricherFactories))
	for name := range lb.enricherFactories {
		names = append(names, name)
	}
	return names
}

// getAvailableFilterNames returns a list of registered filter names
func (lb *LoggerBuilder) getAvailableFilterNames() []string {
	names := make([]string, 0, len(lb.filterFactories))
	for name := range lb.filterFactories {
		names = append(names, name)
	}
	return names
}

// Default sink factories

func createConsoleSink(args map[string]any) (core.LogEventSink, error) {
	themeName := GetString(args, "theme", "Default")

	var theme *sinks.ConsoleTheme
	switch themeName {
	case "Default":
		theme = sinks.DefaultTheme()
	case "Lite":
		theme = sinks.LiteTheme()
	case "Dev":
		theme = sinks.DevTheme()
	case "NoColor":
		theme = sinks.NoColorTheme()
	default:
		if selflog.IsEnabled() {
			selflog.Printf("[configuration] unknown console theme '%s', using Default theme", themeName)
		}
		theme = sinks.DefaultTheme()
	}

	sink := sinks.NewConsoleSinkWithTheme(theme)

	// Check for property display
	if GetBool(args, "showProperties", false) {
		sink.ShowProperties(true)
	}

	return sink, nil
}

func createFileSink(args map[string]any) (core.LogEventSink, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return nil, fmt.Errorf("file sink requires 'path' argument")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return sinks.NewFileSink(path)
}

func createRollingFileSink(args map[string]any) (core.LogEventSink, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return nil, fmt.Errorf("rolling file sink requires 'path' argument")
	}

	options := sinks.RollingFileOptions{
		FilePath:            path,
		MaxFileSize:         GetInt64(args, "fileSizeLimitBytes", 0),
		RetainFileCount:     GetInt(args, "retainedFileCount", 31),
		CompressRolledFiles: GetBool(args, "compress", false),
		BufferSize:          GetInt(args, "bufferSize", 64*1024),
	}

	// Parse rolling interval
	interval := GetString(args, "rollingInterval", "")
	switch interval {
	case "Hour", "Hourly":
		options.RollingInterval = sinks.RollingIntervalHourly
	case "Day", "Daily":
		options.RollingInterval = sinks.RollingIntervalDaily
	case "Week", "Weekly":
		options.RollingInterval = sinks.RollingIntervalWeekly
	case "Month", "Monthly":
		options.RollingInterval = sinks.RollingIntervalMonthly
	}

	return sinks.NewRollingFileSink(options)
}

func createSeqSink(args map[string]any) (core.LogEventSink, error) {
	url := GetString(args, "serverUrl", "")
	if url == "" {
		return nil, fmt.Errorf("Seq sink requires 'serverUrl' argument")
	}

	var options []sinks.SeqOption

	// Add API key if provided
	apiKey := GetString(args, "apiKey", "")
	if apiKey != "" {
		options = append(options, sinks.WithSeqAPIKey(apiKey))
	}

	// Add batch size
	batchSize := GetInt(args, "batchSize", 100)
	options = append(options, sinks.WithSeqBatchSize(batchSize))

	// Add flush interval
	flushInterval := parseDuration(GetString(args, "period", "5s"), 5*time.Second)
	options = append(options, sinks.WithSeqBatchTimeout(flushInterval))

	// Add compression option
	compress := GetBool(args, "compress", false)
	options = append(options, sinks.WithSeqCompression(compress))

	// Add retry options
	retryCount := GetInt(args, "maxRetries", 3)
	retryDelay := parseDuration(GetString(args, "retryBackoff", "1s"), time.Second)
	options = append(options, sinks.WithSeqRetry(retryCount, retryDelay))

	return sinks.NewSeqSink(url, options...)
}

func createAsyncSink(args map[string]any) (core.LogEventSink, error) {
	// Get the wrapped sink configuration
	wrappedConfig, ok := args["writeTo"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("async sink requires 'writeTo' configuration")
	}

	// Create wrapped sink
	sinkName, ok := wrappedConfig["Name"].(string)
	if !ok {
		return nil, fmt.Errorf("wrapped sink must have 'Name'")
	}

	// Get wrapped sink args
	wrappedArgs, _ := wrappedConfig["Args"].(map[string]any)

	// Use a temporary builder to create the wrapped sink
	tempBuilder := NewLoggerBuilder()
	wrapped, err := tempBuilder.createSink(SinkConfiguration{
		Name: sinkName,
		Args: wrappedArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create wrapped sink: %w", err)
	}

	// Parse overflow strategy
	strategy := sinks.OverflowBlock
	strategyStr := GetString(args, "overflowStrategy", "Block")
	switch strategyStr {
	case "Drop":
		strategy = sinks.OverflowDrop
	case "DropOldest":
		strategy = sinks.OverflowDropOldest
	}

	options := sinks.AsyncOptions{
		BufferSize:       GetInt(args, "bufferSize", 1000),
		OverflowStrategy: strategy,
		BatchSize:        GetInt(args, "batchSize", 0),
		FlushInterval:    parseDuration(GetString(args, "flushInterval", "0"), 0),
		ShutdownTimeout:  parseDuration(GetString(args, "shutdownTimeout", "30s"), 30*time.Second),
	}

	return sinks.NewAsyncSink(wrapped, options), nil
}

func createElasticsearchSink(args map[string]any) (core.LogEventSink, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return nil, fmt.Errorf("Elasticsearch sink requires 'url' argument")
	}

	var options []sinks.ElasticsearchOption

	// Set index name if provided
	if index := GetString(args, "index", ""); index != "" {
		options = append(options, sinks.WithElasticsearchIndex(index))
	}

	// Authentication
	apiKey := GetString(args, "apiKey", "")
	username := GetString(args, "username", "")
	password := GetString(args, "password", "")

	if apiKey != "" {
		options = append(options, sinks.WithElasticsearchAPIKey(apiKey))
	} else if username != "" && password != "" {
		options = append(options, sinks.WithElasticsearchBasicAuth(username, password))
	}

	// Batching options
	batchSize := GetInt(args, "batchSize", 100)
	options = append(options, sinks.WithElasticsearchBatchSize(batchSize))

	batchTimeout := parseDuration(GetString(args, "batchTimeout", "5s"), 5*time.Second)
	options = append(options, sinks.WithElasticsearchBatchTimeout(batchTimeout))

	// Data streams
	if GetBool(args, "dataStreams", false) {
		options = append(options, sinks.WithElasticsearchDataStreams())
	}

	// Pipeline
	if pipeline := GetString(args, "pipeline", ""); pipeline != "" {
		options = append(options, sinks.WithElasticsearchPipeline(pipeline))
	}

	return sinks.NewElasticsearchSink(url, options...)
}

func createSplunkSink(args map[string]any) (core.LogEventSink, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return nil, fmt.Errorf("Splunk sink requires 'url' argument")
	}

	token := GetString(args, "token", "")
	if token == "" {
		return nil, fmt.Errorf("Splunk sink requires 'token' argument")
	}

	var options []sinks.SplunkOption

	// Set metadata fields if provided
	if index := GetString(args, "index", ""); index != "" {
		options = append(options, sinks.WithSplunkIndex(index))
	}
	if source := GetString(args, "source", ""); source != "" {
		options = append(options, sinks.WithSplunkSource(source))
	}
	if sourceType := GetString(args, "sourceType", ""); sourceType != "" {
		options = append(options, sinks.WithSplunkSourceType(sourceType))
	}
	if host := GetString(args, "host", ""); host != "" {
		options = append(options, sinks.WithSplunkHost(host))
	}

	// Batching options
	batchSize := GetInt(args, "batchSize", 100)
	options = append(options, sinks.WithSplunkBatchSize(batchSize))

	if batchTimeout := GetString(args, "batchTimeout", ""); batchTimeout != "" {
		if d, err := time.ParseDuration(batchTimeout); err == nil {
			options = append(options, sinks.WithSplunkBatchTimeout(d))
		}
	}

	return sinks.NewSplunkSink(url, token, options...)
}

func createDurableSink(args map[string]any) (core.LogEventSink, error) {
	bufferPath := GetString(args, "bufferPath", "")
	if bufferPath == "" {
		return nil, fmt.Errorf("durable sink requires 'bufferPath' argument")
	}

	// Get the wrapped sink configuration
	wrappedConfig, ok := args["writeTo"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("durable sink requires 'writeTo' configuration")
	}

	// Create wrapped sink
	sinkName, ok := wrappedConfig["Name"].(string)
	if !ok {
		return nil, fmt.Errorf("wrapped sink must have 'Name'")
	}

	// Get wrapped sink args
	wrappedArgs, _ := wrappedConfig["Args"].(map[string]any)

	// Use a temporary builder to create the wrapped sink
	tempBuilder := NewLoggerBuilder()
	wrapped, err := tempBuilder.createSink(SinkConfiguration{
		Name: sinkName,
		Args: wrappedArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create wrapped sink: %w", err)
	}

	// Create durable options
	options := sinks.DurableOptions{
		BufferPath:      bufferPath,
		MaxBufferSize:   GetInt64(args, "maxBufferSize", 0),
		MaxBufferFiles:  GetInt(args, "maxBufferFiles", 0),
		BatchSize:       GetInt(args, "batchSize", 0),
		RetryInterval:   parseDuration(GetString(args, "retryInterval", ""), 0),
		FlushInterval:   parseDuration(GetString(args, "flushInterval", ""), 0),
		ShutdownTimeout: parseDuration(GetString(args, "shutdownTimeout", ""), 0),
	}

	return sinks.NewDurableSink(wrapped, options)
}

// Helper to parse duration strings
func parseDuration(s string, defaultValue time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return defaultValue
}

// fixedPropertyEnricher adds a fixed property to log events.
type fixedPropertyEnricher struct {
	propertyName string
	value        any
}

// Enrich adds the fixed property to the log event.
func (e *fixedPropertyEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	prop := propertyFactory.CreateProperty(e.propertyName, e.value)
	event.Properties[prop.Name] = prop.Value
}
