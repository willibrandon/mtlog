package mtlog

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/internal/filters"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/selflog"
)

// propertyPair represents a key-value pair for properties
type propertyPair struct {
	key   string
	value any
}

// logger is the default implementation of core.Logger.
type logger struct {
	minimumLevel core.LogEventLevel
	levelSwitch  *LoggingLevelSwitch
	pipeline     *pipeline
	
	// Use slice instead of map for properties to minimize allocations
	// Achieves 2 allocations (logger struct + fields array) for common cases
	fields []propertyPair
	
	// Fallback to map only for very large numbers of properties (>64)
	// This is rare in practice
	properties map[string]any
	
	// Sampling state (per-logger instance)
	samplingFilter *filters.PerMessageSamplingFilter
	
	// Deadline awareness
	deadlineEnricher *enrichers.DeadlineEnricher
	
	mu sync.RWMutex
}

// isReservedProperty checks if a property name conflicts with built-in properties
func isReservedProperty(key string) bool {
	switch key {
	case "Timestamp", "Level", "Message", "MessageTemplate", "Exception", "SourceContext":
		return true
	default:
		return false
	}
}

// New creates a new logger with the specified options.
// If any option returns an error during configuration, New will panic.
// Use Build() for non-panicking initialization.
func New(opts ...Option) *logger {
	log, err := Build(opts...)
	if err != nil {
		panic(err)
	}
	return log
}

// Build creates a new logger with the specified options.
// Returns an error if any option fails during configuration.
func Build(opts ...Option) (*logger, error) {
	// Apply default configuration
	cfg := &config{
		minimumLevel: core.InformationLevel,
		enrichers:    []core.LogEventEnricher{},
		filters:      []core.LogEventFilter{},
		sinks:        []core.LogEventSink{},
		properties:   make(map[string]any),
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Check for configuration errors
	if cfg.err != nil {
		return nil, cfg.err
	}

	// Create the pipeline
	p := newPipeline(cfg.enrichers, cfg.filters, cfg.capturer, cfg.sinks)

	return &logger{
		minimumLevel:     cfg.minimumLevel,
		levelSwitch:      cfg.levelSwitch,
		pipeline:         p,
		properties:       cfg.properties,
		deadlineEnricher: cfg.deadlineEnricher,
	}, nil
}

// Verbose writes a verbose-level log event.
func (l *logger) Verbose(messageTemplate string, args ...any) {
	l.Write(core.VerboseLevel, messageTemplate, args...)
}

// Debug writes a debug-level log event.
func (l *logger) Debug(messageTemplate string, args ...any) {
	l.Write(core.DebugLevel, messageTemplate, args...)
}

// Information writes an information-level log event.
func (l *logger) Information(messageTemplate string, args ...any) {
	l.Write(core.InformationLevel, messageTemplate, args...)
}

// Warning writes a warning-level log event.
func (l *logger) Warning(messageTemplate string, args ...any) {
	l.Write(core.WarningLevel, messageTemplate, args...)
}

// Error writes an error-level log event.
func (l *logger) Error(messageTemplate string, args ...any) {
	l.Write(core.ErrorLevel, messageTemplate, args...)
}

// Fatal writes a fatal-level log event.
func (l *logger) Fatal(messageTemplate string, args ...any) {
	l.Write(core.FatalLevel, messageTemplate, args...)
}

// Info writes an information-level log event (alias for Information).
func (l *logger) Info(messageTemplate string, args ...any) {
	l.Write(core.InformationLevel, messageTemplate, args...)
}

// Warn writes a warning-level log event (alias for Warning).
func (l *logger) Warn(messageTemplate string, args ...any) {
	l.Write(core.WarningLevel, messageTemplate, args...)
}

// VerboseContext writes a verbose-level log event with context awareness.
func (l *logger) VerboseContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.VerboseLevel, messageTemplate, args...)
}

// DebugContext writes a debug-level log event with context awareness.
func (l *logger) DebugContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.DebugLevel, messageTemplate, args...)
}

// InfoContext writes an information-level log event with context awareness.
func (l *logger) InfoContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.InformationLevel, messageTemplate, args...)
}

// WarnContext writes a warning-level log event with context awareness.
func (l *logger) WarnContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.WarningLevel, messageTemplate, args...)
}

// ErrorContext writes an error-level log event with context awareness.
func (l *logger) ErrorContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.ErrorLevel, messageTemplate, args...)
}

// FatalContext writes a fatal-level log event with context awareness.
func (l *logger) FatalContext(ctx context.Context, messageTemplate string, args ...any) {
	l.writeWithContext(ctx, core.FatalLevel, messageTemplate, args...)
}

// Write writes a log event at the specified level.
func (l *logger) Write(level core.LogEventLevel, messageTemplate string, args ...any) {
	// Check minimum level (dynamic level switch takes precedence)
	var minimumLevel core.LogEventLevel
	if l.levelSwitch != nil {
		minimumLevel = l.levelSwitch.Level()
	} else {
		minimumLevel = l.minimumLevel
	}

	if level < minimumLevel {
		return
	}

	// Fast path for simple messages (no args, no properties, no enrichers, no filters)
	if len(args) == 0 && len(l.properties) == 0 && len(l.fields) == 0 && !hasPropertyTokens(messageTemplate) &&
		len(l.pipeline.enrichers) == 0 && len(l.pipeline.filters) == 0 {
		l.pipeline.processSimple(time.Now(), level, messageTemplate)
		return
	}

	// Validate template for selflog
	if selflog.IsEnabled() {
		if err := parser.ValidateTemplate(messageTemplate); err != nil {
			selflog.Printf("[parser] template validation error: %v (template=%q)", err, messageTemplate)
		}
	}

	// Parse the template with caching
	tmpl, err := parser.ParseCached(messageTemplate)
	if err != nil {
		// This shouldn't happen as the parser doesn't return errors, but keep for safety
		tmpl = &parser.MessageTemplate{
			Raw:    messageTemplate,
			Tokens: []parser.MessageTemplateToken{&parser.TextToken{Text: messageTemplate}},
		}
	}

	// Create log event - we can't pool these because sinks may retain references
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           level,
		MessageTemplate: messageTemplate,
		Properties:      getPropertyMap(),
	}

	// Extract properties directly into event
	l.extractPropertiesInto(tmpl, args, event.Properties)

	// Add context properties
	l.mu.RLock()
	// Add from fields slice
	for _, field := range l.fields {
		if _, exists := event.Properties[field.key]; !exists {
			event.Properties[field.key] = field.value
		}
	}
	// Add from map (if used for large field counts)
	for k, v := range l.properties {
		if _, exists := event.Properties[k]; !exists {
			event.Properties[k] = v
		}
	}
	l.mu.RUnlock()

	// Process through pipeline
	factory := &propertyFactory{}
	l.pipeline.process(event, factory)
}

// writeWithContext writes a log event at the specified level with context awareness.
// This is the internal implementation for all context-aware logging methods.
func (l *logger) writeWithContext(ctx context.Context, level core.LogEventLevel, messageTemplate string, args ...any) {
	// If context is nil, fall back to Write
	if ctx == nil {
		l.Write(level, messageTemplate, args...)
		return
	}

	// Create a logger with context enrichers
	// The deadline enricher (if configured) is already in the pipeline
	contextLogger := l.WithContext(ctx)
	
	// If we have a deadline enricher, we need to pass the context to it
	// We'll do this by temporarily adding it to the event properties
	if l.deadlineEnricher != nil {
		// The deadline enricher will look for this special property
		contextLogger = contextLogger.ForContext("__context__", ctx)
	}
	
	contextLogger.Write(level, messageTemplate, args...)
}

// hasDeadlineAwareness checks if deadline awareness is configured for this logger.
func (l *logger) hasDeadlineAwareness() bool {
	return l.deadlineEnricher != nil
}

// ForContext creates a logger that enriches events with the specified property.
func (l *logger) ForContext(propertyName string, value any) core.Logger {
	// This is essentially With() with a single key-value pair
	// Reuse the optimized With implementation
	return l.With(propertyName, value)
}

// ForSourceContext creates a logger with the specified source context.
// This is equivalent to ForContext("SourceContext", sourceContext).
func (l *logger) ForSourceContext(sourceContext string) core.Logger {
	return l.ForContext("SourceContext", sourceContext)
}

// WithContext creates a logger that enriches events with context values from both
// standard context and LogContext.
//
// This method adds two enrichers to the logger:
// 1. ContextEnricher - Extracts standard context values like trace IDs
// 2. LogContextEnricher - Extracts properties added via PushProperty
//
// Property precedence (highest to lowest priority):
// 1. Event-specific properties (passed directly to log methods like Information)
// 2. ForContext properties (added via ForContext method)
// 3. LogContext properties (added via PushProperty)
// 4. Standard context values
//
// This ensures that properties can be overridden at more specific scopes while
// maintaining defaults from broader contexts.
//
// Example:
//
//	ctx := context.Background()
//	ctx = mtlog.PushProperty(ctx, "UserId", 123)
//	ctx = mtlog.PushProperty(ctx, "TenantId", "acme")
//
//	// Both UserId and TenantId will be included
//	logger.WithContext(ctx).Information("User action")
//
//	// ForContext overrides LogContext
//	logger.WithContext(ctx).ForContext("UserId", 456).Information("Override test")
//	// Results in UserId=456, TenantId=acme
//
//	// Event properties override everything
//	logger.WithContext(ctx).Information("User {UserId} action", 789)
//	// Results in UserId=789, TenantId=acme
func (l *logger) WithContext(ctx context.Context) core.Logger {
	// Create a new logger with the same configuration but additional context enrichers
	newConfig := &config{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		enrichers:    make([]core.LogEventEnricher, len(l.pipeline.enrichers)+2),
		filters:      l.pipeline.filters,
		capturer:     l.pipeline.capturer,
		sinks:        l.pipeline.sinks,
		properties:   make(map[string]any),
	}

	// Copy existing enrichers
	copy(newConfig.enrichers, l.pipeline.enrichers)

	// Add context enrichers
	newConfig.enrichers[len(l.pipeline.enrichers)] = enrichers.NewContextEnricher(ctx)
	newConfig.enrichers[len(l.pipeline.enrichers)+1] = enrichers.NewLogContextEnricher(ctx, getLogContextProperties)

	// Copy existing properties
	l.mu.RLock()
	// Copy from fields slice
	for _, field := range l.fields {
		newConfig.properties[field.key] = field.value
	}
	// Copy from map
	for k, v := range l.properties {
		newConfig.properties[k] = v
	}
	l.mu.RUnlock()

	// Create new pipeline
	p := newPipeline(newConfig.enrichers, newConfig.filters, newConfig.capturer, newConfig.sinks)

	// Convert properties to fields slice for efficiency
	var fields []propertyPair
	if len(newConfig.properties) <= 64 {
		fields = make([]propertyPair, 0, len(newConfig.properties))
		for k, v := range newConfig.properties {
			fields = append(fields, propertyPair{key: k, value: v})
		}
		newConfig.properties = nil // Clear map to use fields instead
	}

	return &logger{
		minimumLevel:     l.minimumLevel,
		levelSwitch:      l.levelSwitch,
		pipeline:         p,
		fields:           fields,
		properties:       newConfig.properties,
		deadlineEnricher: l.deadlineEnricher,
	}
}

// With creates a logger that enriches events with the specified key-value pairs.
// This method follows the slog convention of accepting variadic key-value pairs.
// 
// Keys must be strings (either string literals or string-typed variables).
// Values can be any type.
//
// The key-value pairs should be provided in alternating order:
//   logger.With("user_id", 123, "request_id", "abc-123")
//
// If an odd number of arguments is provided, the last argument is ignored.
// Non-string keys are skipped with their corresponding values.
//
// Performance:
// - 0 allocations when no fields (returns same logger)
// - 2 allocations for common cases (≤64 fields): logger struct + fields array
// - 3+ allocations for very large field counts or complex scenarios
// 
// Note: Unlike zap which pre-serializes fields into JSON for 1 allocation,
// mtlog maintains structured properties for Serilog compatibility. This enables:
// - Property-based filtering in the pipeline
// - Dynamic enrichment based on property values  
// - Multiple output formats from the same properties
// - Property inspection and manipulation by sinks
//
// The trade-off of 1 extra allocation maintains the flexibility expected
// by both Serilog users and Go developers familiar with slog.
//
// Example:
//   logger.With("service", "auth", "version", "1.0").Info("Service started")
//   logger.With("user_id", 123).With("request_id", "abc").Info("Request processed")
func (l *logger) With(args ...any) core.Logger {
	// Fast path: no arguments
	if len(args) == 0 {
		return l
	}

	// Calculate the number of valid pairs
	numPairs := len(args) / 2
	if numPairs == 0 {
		return l
	}

	// Count valid string keys
	validPairs := 0
	for i := 0; i < numPairs*2; i += 2 {
		if key, ok := args[i].(string); ok && key != "" {
			validPairs++
		} else if selflog.IsEnabled() {
			if !ok {
				selflog.Printf("With: skipping non-string key at position %d: %T", i, args[i])
			} else {
				selflog.Printf("With: skipping empty key at position %d", i)
			}
		}
	}

	if validPairs == 0 {
		return l
	}

	// Calculate total fields needed
	l.mu.RLock()
	existingFieldCount := len(l.fields)
	existingMapCount := len(l.properties)
	l.mu.RUnlock()

	totalFields := existingFieldCount + validPairs

	// Use slice for reasonable field counts (<= 64)
	// This is the common case and results in 2 allocations (logger struct + fields slice)
	if totalFields <= 64 && existingMapCount == 0 {
		// Allocate slice with capacity for worst-case (no overrides)
		// This may over-allocate when fields are overridden, but that's acceptable
		// because it avoids an extra allocation for calculating exact capacity
		newFields := make([]propertyPair, 0, totalFields)
		
		// Copy existing fields that aren't being overridden
		// O(n*m) complexity is acceptable here because:
		// - n and m are small (≤64 fields total)
		// - Avoiding a map saves an allocation
		// - For 64 fields, this is still very fast (~4096 comparisons worst case)
		l.mu.RLock()
		outer:
		for _, existing := range l.fields {
			// Check if this key is overridden by new args
			for i := 0; i < numPairs*2; i += 2 {
				if key, ok := args[i].(string); ok && key == existing.key {
					continue outer // Skip this field, it's overridden
				}
			}
			newFields = append(newFields, existing)
		}
		l.mu.RUnlock()
		
		// Add all new fields (they naturally override due to order)
		for i := 0; i < numPairs*2; i += 2 {
			if key, ok := args[i].(string); ok && key != "" {
				if isReservedProperty(key) && selflog.IsEnabled() {
					selflog.Printf("With: property '%s' shadows built-in property", key)
				}
				newFields = append(newFields, propertyPair{
					key:   key,
					value: args[i+1],
				})
			}
		}
		
		// Create new logger with the single allocation for fields
		return &logger{
			minimumLevel:     l.minimumLevel,
			levelSwitch:      l.levelSwitch,
			pipeline:         l.pipeline,
			fields:           newFields,
			deadlineEnricher: l.deadlineEnricher,
		}
	}

	// Fallback for very large field counts or when already using map
	// This is rare in practice
	return l.withMap(args, numPairs, totalFields + existingMapCount)
}

// withMap creates a logger using map for large numbers of properties
func (l *logger) withMap(args []any, numPairs, capacity int) core.Logger {
	newLogger := &logger{
		minimumLevel:     l.minimumLevel,
		levelSwitch:      l.levelSwitch,
		pipeline:         l.pipeline,
		properties:       make(map[string]any, capacity),
		deadlineEnricher: l.deadlineEnricher,
	}

	// Copy from fields array to map
	l.mu.RLock()
	for _, field := range l.fields {
		newLogger.properties[field.key] = field.value
	}
	// Copy from existing map
	for k, v := range l.properties {
		newLogger.properties[k] = v
	}
	l.mu.RUnlock()

	// Add new properties
	for i := 0; i < numPairs*2; i += 2 {
		if key, ok := args[i].(string); ok && key != "" {
			newLogger.properties[key] = args[i+1]
		}
	}

	return newLogger
}

// extractPropertiesInto extracts properties from the template and arguments into an existing map.
func (l *logger) extractPropertiesInto(tmpl *parser.MessageTemplate, args []any, properties map[string]any) {
	// Extract property names from already parsed template
	propNames := parser.ExtractPropertyNamesFromTemplate(tmpl)

	// Also check which properties need capturing
	captureProps := make(map[string]bool)
	for _, token := range tmpl.Tokens {
		if prop, ok := token.(*parser.PropertyToken); ok {
			if prop.Capturing == parser.Capture {
				captureProps[prop.PropertyName] = true
			}
		}
	}

	// Check if all property names are numeric.
	// This determines whether to use index-based matching (like string.Format)
	// or left-to-right positional matching for assigning argument values to properties.
	allNumeric := true
	for _, name := range propNames {
		if _, err := strconv.Atoi(name); err != nil {
			allNumeric = false
			break
		}
	}

	// Match arguments to properties using two strategies:
	// 1. If all property names are numeric, use index-based matching (like string.Format).
	// 2. Otherwise, use left-to-right positional matching for named or mixed properties.
	if allNumeric && len(propNames) > 0 {
		// All numeric: use index-based matching (like string.Format)
		for _, name := range propNames {
			idx, _ := strconv.Atoi(name) // We know it's numeric from the check above
			if idx >= 0 && idx < len(args) {
				value := args[idx]

				// Apply capturing if needed and capturer is available
				if captureProps[name] && l.pipeline.capturer != nil {
					factory := &propertyFactory{}
					if prop, ok := l.pipeline.capturer.TryCapture(value, factory); ok {
						value = prop.Value
					}
				}

				properties[name] = value
			}
		}
	} else {
		// Mixed or all named: use left-to-right positional matching
		for i, name := range propNames {
			if i < len(args) {
				value := args[i]

				// Apply capturing if needed and capturer is available
				if captureProps[name] && l.pipeline.capturer != nil {
					factory := &propertyFactory{}
					if prop, ok := l.pipeline.capturer.TryCapture(value, factory); ok {
						value = prop.Value
					}
				}

				properties[name] = value
			}
		}
	}

	// Add any extra arguments as positional properties
	for i := len(propNames); i < len(args); i++ {
		properties[string(rune('0'+i))] = args[i]
	}
}

// extractProperties extracts properties from the template and arguments.
func (l *logger) extractProperties(tmpl *parser.MessageTemplate, args []any) map[string]any {
	properties := make(map[string]any)
	l.extractPropertiesInto(tmpl, args, properties)
	return properties
}

// Note: The logger is immutable once created. To modify configuration,
// create a new logger with the desired options.

// propertyFactory is a simple implementation of LogEventPropertyFactory.
type propertyFactory struct{}

// CreateProperty creates a new log event property.
func (pf *propertyFactory) CreateProperty(name string, value any) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

// hasPropertyTokens quickly checks if a template contains property tokens.
func hasPropertyTokens(template string) bool {
	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			// Check if it's an escaped brace
			if i+1 < len(template) && template[i+1] == '{' {
				i++ // Skip the escaped brace
				continue
			}
			// Found a potential property token
			return true
		}
	}
	return false
}

// Close closes all closeable sinks in the pipeline.
func (l *logger) Close() error {
	return l.pipeline.Close()
}

// GetMinimumLevel returns the current effective minimum level.
// If a level switch is configured, it returns the switch's current level.
// Otherwise, it returns the static minimum level.
func (l *logger) GetMinimumLevel() core.LogEventLevel {
	if l.levelSwitch != nil {
		return l.levelSwitch.Level()
	}
	return l.minimumLevel
}

// GetLevelSwitch returns the level switch if one is configured, otherwise nil.
func (l *logger) GetLevelSwitch() *LoggingLevelSwitch {
	return l.levelSwitch
}

// IsEnabled returns true if events at the specified level would be processed.
func (l *logger) IsEnabled(level core.LogEventLevel) bool {
	return level >= l.GetMinimumLevel()
}

// Global managers for sampling state with LRU eviction
var (
	globalSamplingGroupManager = filters.NewSamplingGroupManager(10000)
	globalBackoffState         = filters.NewBackoffState(10000)
)

// cloneWithSamplingFilter creates a new logger with a sampling filter
func (l *logger) cloneWithSamplingFilter(filter *filters.PerMessageSamplingFilter) core.Logger {
	// Create new pipeline with the sampling filter
	var newFilters []core.LogEventFilter
	if filter != nil {
		// Add the sampling filter first in the pipeline for efficiency
		newFilters = make([]core.LogEventFilter, len(l.pipeline.filters)+1)
		newFilters[0] = filter
		copy(newFilters[1:], l.pipeline.filters)
	} else {
		newFilters = l.pipeline.filters
	}

	p := newPipeline(l.pipeline.enrichers, newFilters, l.pipeline.capturer, l.pipeline.sinks)

	// Create new logger with sampling filter
	return &logger{
		minimumLevel:   l.minimumLevel,
		levelSwitch:    l.levelSwitch,
		pipeline:       p,
		fields:         l.fields,
		properties:     l.properties,
		samplingFilter: filter,
	}
}

// Sample creates a logger that samples every nth message.
func (l *logger) Sample(n uint64) core.Logger {
	filter := filters.NewCounterSamplingFilter(n)
	return l.cloneWithSamplingFilter(filter)
}

// SampleDuration creates a logger that samples at most once per duration.
func (l *logger) SampleDuration(duration time.Duration) core.Logger {
	filter := filters.NewDurationSamplingFilter(duration)
	return l.cloneWithSamplingFilter(filter)
}

// SampleRate creates a logger that samples a percentage of messages (0.0 to 1.0).
func (l *logger) SampleRate(rate float32) core.Logger {
	filter := filters.NewRateSamplingFilter(rate)
	return l.cloneWithSamplingFilter(filter)
}

// SampleFirst creates a logger that logs only the first n occurrences.
// If n is 0, no messages will be logged.
func (l *logger) SampleFirst(n uint64) core.Logger {
	if n == 0 && selflog.IsEnabled() {
		selflog.Printf("SampleFirst(0) will not log any messages - use a positive value to log the first N occurrences")
	}
	filter := filters.NewFirstNSamplingFilter(n)
	return l.cloneWithSamplingFilter(filter)
}

// SampleGroup creates a logger that samples messages within a named group.
func (l *logger) SampleGroup(groupName string, n uint64) core.Logger {
	filter := filters.NewGroupSamplingFilter(groupName, n, globalSamplingGroupManager)
	return l.cloneWithSamplingFilter(filter)
}

// SampleWhen creates a logger that samples conditionally based on a predicate.
func (l *logger) SampleWhen(predicate func() bool, n uint64) core.Logger {
	filter := filters.NewConditionalSamplingFilter(predicate, n)
	return l.cloneWithSamplingFilter(filter)
}

// SampleBackoff creates a logger that samples with exponential backoff.
func (l *logger) SampleBackoff(key string, factor float64) core.Logger {
	factor = validateBackoffFactor(factor)
	filter := filters.NewBackoffSamplingFilter(key, factor, globalBackoffState)
	return l.cloneWithSamplingFilter(filter)
}


// ResetSampling resets all sampling counters for this logger.
func (l *logger) ResetSampling() {
	if l.samplingFilter != nil {
		l.samplingFilter.Reset()
	}
}

// ResetSamplingGroup resets the sampling counter for a specific group.
func (l *logger) ResetSamplingGroup(groupName string) {
	globalSamplingGroupManager.ResetGroup(groupName)
}

// EnableSamplingSummary enables periodic emission of sampling summary events.
// Note: This starts a goroutine that runs indefinitely. Consider using EnableSamplingSummaryWithCleanup 
// for better lifecycle management.
func (l *logger) EnableSamplingSummary(period time.Duration) core.Logger {
	if l.samplingFilter != nil {
		// Start a goroutine to emit summaries periodically
		ctx, cancel := context.WithCancel(context.Background())
		go l.emitSamplingSummariesWithContext(ctx, period)
		
		// Store cancel function in logger for potential cleanup (though interface doesn't expose it)
		// This is not ideal but maintains backward compatibility
		_ = cancel // Mark as used to avoid compiler warnings
	}
	return l
}

// EnableSamplingSummaryWithCleanup enables periodic emission of sampling summary events
// and returns a cleanup function to stop the background goroutine.
func (l *logger) EnableSamplingSummaryWithCleanup(period time.Duration) (core.Logger, func()) {
	if l.samplingFilter == nil {
		return l, func() {} // No-op cleanup
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	go l.emitSamplingSummariesWithContext(ctx, period)
	
	return l, cancel
}

// GetSamplingStats returns current sampling statistics.
func (l *logger) GetSamplingStats() (sampled uint64, skipped uint64) {
	if l.samplingFilter != nil {
		stats := l.samplingFilter.GetStats()
		return stats.Sampled, stats.Skipped
	}
	return 0, 0
}

// GetSamplingMetrics returns detailed metrics about sampling cache performance.
// This helps operators tune cache limits and understand sampling behavior.
func (l *logger) GetSamplingMetrics() core.SamplingMetrics {
	metrics := core.SamplingMetrics{}
	
	// Get overall sampling stats if we have a sampling filter
	if l.samplingFilter != nil {
		stats := l.samplingFilter.GetStats()
		metrics.TotalSampled = stats.Sampled
		metrics.TotalSkipped = stats.Skipped
		
		// TODO: Collect cache metrics from individual filters
		// This would require extending the filter interfaces to expose cache stats
		// For now, return basic stats
	}
	
	return metrics
}

// DeadlineStats returns deadline tracking statistics if deadline awareness is enabled.
// Returns nil if deadline awareness is not configured.
func (l *logger) DeadlineStats() interface{} {
	if l.deadlineEnricher == nil {
		return nil
	}
	return l.deadlineEnricher.Stats()
}

// WithDeadlineWarning creates a new logger with modified deadline warning threshold.
// This allows creating derived loggers with different deadline configurations.
func (l *logger) WithDeadlineWarning(threshold time.Duration, opts ...interface{}) core.Logger {
	// Convert interface{} options to enricher options
	var enricherOpts []enrichers.DeadlineOption
	for _, opt := range opts {
		if deadlineOpt, ok := opt.(enrichers.DeadlineOption); ok {
			enricherOpts = append(enricherOpts, deadlineOpt)
		}
	}
	
	// Create new deadline enricher
	newDeadlineEnricher := enrichers.NewDeadlineEnricher(threshold, enricherOpts...)
	
	// Create new list of enrichers, replacing any existing deadline enricher
	var newEnrichers []core.LogEventEnricher
	for _, e := range l.pipeline.enrichers {
		// Skip old deadline enricher if present
		if _, ok := e.(*enrichers.DeadlineEnricher); !ok {
			newEnrichers = append(newEnrichers, e)
		}
	}
	newEnrichers = append(newEnrichers, newDeadlineEnricher)
	
	// Create new logger with updated pipeline
	return &logger{
		minimumLevel:     l.minimumLevel,
		levelSwitch:      l.levelSwitch,
		pipeline:         newPipeline(newEnrichers, l.pipeline.filters, l.pipeline.capturer, l.pipeline.sinks),
		fields:           l.fields,
		properties:       l.properties,
		deadlineEnricher: newDeadlineEnricher,
		samplingFilter:   l.samplingFilter,
	}
}

// emitSamplingSummariesWithContext periodically emits sampling summary events with context support
func (l *logger) emitSamplingSummariesWithContext(ctx context.Context, period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	
	var lastSampled, lastSkipped uint64
	
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, clean exit
			return
		case <-ticker.C:
			if l.samplingFilter == nil {
				return
			}
			
			stats := l.samplingFilter.GetStats()
			sampled := stats.Sampled - lastSampled
			skipped := stats.Skipped - lastSkipped
			
			if sampled > 0 || skipped > 0 {
				// Emit a dedicated summary event
				l.Information("Sampling summary for last {Period}: {Sampled} messages logged, {Skipped} messages skipped",
					period.String(), sampled, skipped)
			}
			
			lastSampled = stats.Sampled
			lastSkipped = stats.Skipped
		}
	}
}
