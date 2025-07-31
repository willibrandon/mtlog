package mtlog

import (
	"context"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/selflog"
)

// logger is the default implementation of core.Logger.
type logger struct {
	minimumLevel core.LogEventLevel
	levelSwitch  *LoggingLevelSwitch
	pipeline     *pipeline
	properties   map[string]any
	mu           sync.RWMutex
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
		minimumLevel: cfg.minimumLevel,
		levelSwitch:  cfg.levelSwitch,
		pipeline:     p,
		properties:   cfg.properties,
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
	if len(args) == 0 && len(l.properties) == 0 && !hasPropertyTokens(messageTemplate) &&
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

// ForContext creates a logger that enriches events with the specified property.
func (l *logger) ForContext(propertyName string, value any) core.Logger {
	newLogger := &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     l.pipeline, // Share the same immutable pipeline
		properties:   make(map[string]any),
	}

	// Copy existing properties
	l.mu.RLock()
	for k, v := range l.properties {
		newLogger.properties[k] = v
	}
	l.mu.RUnlock()

	// Add new property
	newLogger.properties[propertyName] = value

	return newLogger
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
	for k, v := range l.properties {
		newConfig.properties[k] = v
	}
	l.mu.RUnlock()

	// Create new pipeline
	p := newPipeline(newConfig.enrichers, newConfig.filters, newConfig.capturer, newConfig.sinks)

	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     p,
		properties:   newConfig.properties,
	}
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

	// Match arguments to properties positionally
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
