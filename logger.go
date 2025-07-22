package mtlog

import (
	"context"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/enrichers"
	"github.com/willibrandon/mtlog/parser"
)

// logger is the default implementation of core.Logger.
type logger struct {
	minimumLevel core.LogEventLevel
	pipeline     *pipeline
	properties   map[string]interface{}
	mu           sync.RWMutex
}

// New creates a new logger with the specified options.
func New(opts ...Option) *logger {
	// Apply default configuration
	cfg := &config{
		minimumLevel: core.InformationLevel,
		enrichers:    []core.LogEventEnricher{},
		filters:      []core.LogEventFilter{},
		sinks:        []core.LogEventSink{},
		properties:   make(map[string]interface{}),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}
	
	// Create the pipeline
	p := newPipeline(cfg.enrichers, cfg.filters, cfg.destructurer, cfg.sinks)
	
	return &logger{
		minimumLevel: cfg.minimumLevel,
		pipeline:     p,
		properties:   cfg.properties,
	}
}

// Verbose writes a verbose-level log event.
func (l *logger) Verbose(messageTemplate string, args ...interface{}) {
	l.Write(core.VerboseLevel, messageTemplate, args...)
}

// Debug writes a debug-level log event.
func (l *logger) Debug(messageTemplate string, args ...interface{}) {
	l.Write(core.DebugLevel, messageTemplate, args...)
}

// Information writes an information-level log event.
func (l *logger) Information(messageTemplate string, args ...interface{}) {
	l.Write(core.InformationLevel, messageTemplate, args...)
}

// Warning writes a warning-level log event.
func (l *logger) Warning(messageTemplate string, args ...interface{}) {
	l.Write(core.WarningLevel, messageTemplate, args...)
}

// Error writes an error-level log event.
func (l *logger) Error(messageTemplate string, args ...interface{}) {
	l.Write(core.ErrorLevel, messageTemplate, args...)
}

// Fatal writes a fatal-level log event.
func (l *logger) Fatal(messageTemplate string, args ...interface{}) {
	l.Write(core.FatalLevel, messageTemplate, args...)
}

// Write writes a log event at the specified level.
func (l *logger) Write(level core.LogEventLevel, messageTemplate string, args ...interface{}) {
	// Check minimum level
	if level < l.minimumLevel {
		return
	}
	
	// Fast path for simple messages (no args, no properties, no enrichers, no filters)
	if len(args) == 0 && len(l.properties) == 0 && !hasPropertyTokens(messageTemplate) && 
		len(l.pipeline.enrichers) == 0 && len(l.pipeline.filters) == 0 {
		l.pipeline.processSimple(time.Now(), level, messageTemplate)
		return
	}
	
	// Parse the template with caching
	tmpl, err := parser.ParseCached(messageTemplate)
	if err != nil {
		// Log parsing error and continue with raw template
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
func (l *logger) ForContext(propertyName string, value interface{}) core.Logger {
	newLogger := &logger{
		minimumLevel: l.minimumLevel,
		pipeline:     l.pipeline, // Share the same immutable pipeline
		properties:   make(map[string]interface{}),
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

// WithContext creates a logger that enriches events with context values.
func (l *logger) WithContext(ctx context.Context) core.Logger {
	// Create a new logger with the same configuration but additional context enricher
	newConfig := &config{
		minimumLevel: l.minimumLevel,
		enrichers:    make([]core.LogEventEnricher, len(l.pipeline.enrichers)+1),
		filters:      l.pipeline.filters,
		destructurer: l.pipeline.destructurer,
		sinks:        l.pipeline.sinks,
		properties:   make(map[string]interface{}),
	}
	
	// Copy existing enrichers
	copy(newConfig.enrichers, l.pipeline.enrichers)
	
	// Add context enricher
	newConfig.enrichers[len(l.pipeline.enrichers)] = enrichers.NewContextEnricher(ctx)
	
	// Copy existing properties
	l.mu.RLock()
	for k, v := range l.properties {
		newConfig.properties[k] = v
	}
	l.mu.RUnlock()
	
	// Create new pipeline
	p := newPipeline(newConfig.enrichers, newConfig.filters, newConfig.destructurer, newConfig.sinks)
	
	return &logger{
		minimumLevel: l.minimumLevel,
		pipeline:     p,
		properties:   newConfig.properties,
	}
}

// extractPropertiesInto extracts properties from the template and arguments into an existing map.
func (l *logger) extractPropertiesInto(tmpl *parser.MessageTemplate, args []interface{}, properties map[string]interface{}) {
	// Extract property names from already parsed template
	propNames := parser.ExtractPropertyNamesFromTemplate(tmpl)
	
	// Also check which properties need destructuring
	destructureProps := make(map[string]bool)
	for _, token := range tmpl.Tokens {
		if prop, ok := token.(*parser.PropertyToken); ok {
			if prop.Destructuring == parser.Destructure {
				destructureProps[prop.PropertyName] = true
			}
		}
	}
	
	// Match arguments to properties positionally
	for i, name := range propNames {
		if i < len(args) {
			value := args[i]
			
			// Apply destructuring if needed and destructurer is available
			if destructureProps[name] && l.pipeline.destructurer != nil {
				factory := &propertyFactory{}
				if prop, ok := l.pipeline.destructurer.TryDestructure(value, factory); ok {
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
func (l *logger) extractProperties(tmpl *parser.MessageTemplate, args []interface{}) map[string]interface{} {
	properties := make(map[string]interface{})
	l.extractPropertiesInto(tmpl, args, properties)
	return properties
}

// Note: The logger is immutable once created. To modify configuration,
// create a new logger with the desired options.

// propertyFactory is a simple implementation of LogEventPropertyFactory.
type propertyFactory struct{}

// CreateProperty creates a new log event property.
func (pf *propertyFactory) CreateProperty(name string, value interface{}) *core.LogEventProperty {
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