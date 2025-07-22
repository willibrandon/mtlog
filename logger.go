package mtlog

import (
	"context"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/parser"
)

// logger is the default implementation of core.Logger.
type logger struct {
	minimumLevel core.LogEventLevel
	enrichers    []core.LogEventEnricher
	filters      []core.LogEventFilter
	sinks        []core.LogEventSink
	properties   map[string]interface{}
	mu           sync.RWMutex
}

// New creates a new logger with default configuration.
func New() *logger {
	return &logger{
		minimumLevel: core.InformationLevel,
		enrichers:    []core.LogEventEnricher{},
		filters:      []core.LogEventFilter{},
		sinks:        []core.LogEventSink{},
		properties:   make(map[string]interface{}),
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
	if len(args) == 0 && len(l.enrichers) == 0 && len(l.properties) == 0 && len(l.filters) == 0 && !hasPropertyTokens(messageTemplate) {
		timestamp := time.Now()
		for _, sink := range l.sinks {
			if simpleSink, ok := sink.(core.SimpleSink); ok {
				simpleSink.EmitSimple(timestamp, level, messageTemplate)
			} else {
				// Fall back to regular emit with a minimal event
				event := &core.LogEvent{
					Timestamp:       timestamp,
					Level:           level,
					MessageTemplate: messageTemplate,
					Properties:      make(map[string]interface{}),
				}
				sink.Emit(event)
			}
		}
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
	
	// Apply enrichers
	factory := &propertyFactory{}
	for _, enricher := range l.enrichers {
		enricher.Enrich(event, factory)
	}
	
	// Apply filters
	for _, filter := range l.filters {
		if !filter.IsEnabled(event) {
			return
		}
	}
	
	// Emit to sinks
	for _, sink := range l.sinks {
		sink.Emit(event)
	}
}

// ForContext creates a logger that enriches events with the specified property.
func (l *logger) ForContext(propertyName string, value interface{}) core.Logger {
	newLogger := &logger{
		minimumLevel: l.minimumLevel,
		enrichers:    l.enrichers,
		filters:      l.filters,
		sinks:        l.sinks,
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
	// TODO: Extract values from context
	return l
}

// extractPropertiesInto extracts properties from the template and arguments into an existing map.
func (l *logger) extractPropertiesInto(tmpl *parser.MessageTemplate, args []interface{}, properties map[string]interface{}) {
	// Extract property names from already parsed template
	propNames := parser.ExtractPropertyNamesFromTemplate(tmpl)
	
	// Match arguments to properties positionally
	for i, name := range propNames {
		if i < len(args) {
			properties[name] = args[i]
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

// AddSink adds a sink to the logger.
func (l *logger) AddSink(sink core.LogEventSink) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sinks = append(l.sinks, sink)
}

// AddEnricher adds an enricher to the logger.
func (l *logger) AddEnricher(enricher core.LogEventEnricher) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enrichers = append(l.enrichers, enricher)
}

// AddFilter adds a filter to the logger.
func (l *logger) AddFilter(filter core.LogEventFilter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.filters = append(l.filters, filter)
}

// SetMinimumLevel sets the minimum log level.
func (l *logger) SetMinimumLevel(level core.LogEventLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minimumLevel = level
}

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