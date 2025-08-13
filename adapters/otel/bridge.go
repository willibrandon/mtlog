// Package otel provides OpenTelemetry integration for mtlog.
package otel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/capture"
	"github.com/willibrandon/mtlog/internal/parser"
	olog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	"go.opentelemetry.io/otel/log/global"
	otrace "go.opentelemetry.io/otel/trace"
)

// Bridge adapts mtlog to work as an OpenTelemetry log.Logger.
// It provides bidirectional compatibility between mtlog and otel/log.
type Bridge struct {
	embedded.Logger
	logger     core.Logger
	capturer   *capture.DefaultCapturer
	
	// Cache for parsed templates
	templateCache sync.Map
	
	// Options
	includeTraceContext bool
	severityMapping     map[olog.Severity]core.LogEventLevel
}

// BridgeOption configures the bridge
type BridgeOption func(*Bridge)

// WithTraceContext enables automatic trace context extraction
func WithTraceContext(enabled bool) BridgeOption {
	return func(b *Bridge) {
		b.includeTraceContext = enabled
	}
}

// WithSeverityMapping sets custom severity level mapping
func WithSeverityMapping(mapping map[olog.Severity]core.LogEventLevel) BridgeOption {
	return func(b *Bridge) {
		b.severityMapping = mapping
	}
}

// NewBridge creates a new bridge from mtlog to otel/log
func NewBridge(logger core.Logger, opts ...BridgeOption) *Bridge {
	b := &Bridge{
		logger:              logger,
		capturer:            capture.NewCapturer(3, 1000, 100),
		includeTraceContext: true,
		severityMapping:     defaultSeverityMapping(),
	}
	
	for _, opt := range opts {
		opt(b)
	}
	
	return b
}

// defaultSeverityMapping returns the default severity mapping
func defaultSeverityMapping() map[olog.Severity]core.LogEventLevel {
	return map[olog.Severity]core.LogEventLevel{
		olog.SeverityTrace:     core.VerboseLevel,
		olog.SeverityTrace2:    core.VerboseLevel,
		olog.SeverityTrace3:    core.VerboseLevel,
		olog.SeverityTrace4:    core.VerboseLevel,
		olog.SeverityDebug:     core.DebugLevel,
		olog.SeverityDebug2:    core.DebugLevel,
		olog.SeverityDebug3:    core.DebugLevel,
		olog.SeverityDebug4:    core.DebugLevel,
		olog.SeverityInfo:      core.InformationLevel,
		olog.SeverityInfo2:     core.InformationLevel,
		olog.SeverityInfo3:     core.InformationLevel,
		olog.SeverityInfo4:     core.InformationLevel,
		olog.SeverityWarn:      core.WarningLevel,
		olog.SeverityWarn2:     core.WarningLevel,
		olog.SeverityWarn3:     core.WarningLevel,
		olog.SeverityWarn4:     core.WarningLevel,
		olog.SeverityError:     core.ErrorLevel,
		olog.SeverityError2:    core.ErrorLevel,
		olog.SeverityError3:    core.ErrorLevel,
		olog.SeverityError4:    core.ErrorLevel,
		olog.SeverityFatal:     core.FatalLevel,
		olog.SeverityFatal2:    core.FatalLevel,
		olog.SeverityFatal3:    core.FatalLevel,
		olog.SeverityFatal4:    core.FatalLevel,
		olog.SeverityUndefined: core.InformationLevel,
	}
}

// Emit implements the otel log.Logger interface
func (b *Bridge) Emit(ctx context.Context, record olog.Record) {
	// Map severity to mtlog level
	level := b.mapSeverity(record.Severity())
	
	// Extract message template and properties
	messageTemplate := record.Body().AsString()
	properties := make(map[string]any)
	
	// Add trace context if available
	if b.includeTraceContext {
		// OTEL log.Record doesn't have TraceID/SpanID methods
		// We need to extract from context instead
		{
			if span := otrace.SpanFromContext(ctx); span != nil {
				spanCtx := span.SpanContext()
				if spanCtx.IsValid() {
					if spanCtx.HasTraceID() {
						properties["trace.id"] = spanCtx.TraceID().String()
					}
					if spanCtx.HasSpanID() {
						properties["span.id"] = spanCtx.SpanID().String()
					}
				}
			}
		}
	}
	
	// Convert OTEL attributes to properties
	record.WalkAttributes(func(kv olog.KeyValue) bool {
		properties[string(kv.Key)] = b.convertValue(kv.Value)
		return true
	})
	
	// Log with mtlog
	b.logWithLevel(level, messageTemplate, properties)
}

// convertValue converts an OTEL value to a Go value
func (b *Bridge) convertValue(v olog.Value) any {
	switch v.Kind() {
	case olog.KindBool:
		return v.AsBool()
	case olog.KindFloat64:
		return v.AsFloat64()
	case olog.KindInt64:
		return v.AsInt64()
	case olog.KindString:
		return v.AsString()
	case olog.KindBytes:
		return v.AsBytes()
	case olog.KindSlice:
		slice := v.AsSlice()
		result := make([]any, len(slice))
		for i, elem := range slice {
			result[i] = b.convertValue(elem)
		}
		return result
	case olog.KindMap:
		m := v.AsMap()
		result := make(map[string]any)
		for _, kv := range m {
			result[string(kv.Key)] = b.convertValue(kv.Value)
		}
		return result
	default:
		return v.AsString()
	}
}

// mapSeverity maps OTEL severity to mtlog level
func (b *Bridge) mapSeverity(severity olog.Severity) core.LogEventLevel {
	if level, ok := b.severityMapping[severity]; ok {
		return level
	}
	return core.InformationLevel
}

// logWithLevel logs a message at the specified level
func (b *Bridge) logWithLevel(level core.LogEventLevel, messageTemplate string, properties map[string]any) {
	// Try to get cached template first
	var template *parser.MessageTemplate
	var err error
	
	if cached, ok := b.templateCache.Load(messageTemplate); ok {
		template = cached.(*parser.MessageTemplate)
	} else {
		// Parse template to get property names in order
		template, err = parser.Parse(messageTemplate)
		if err != nil {
			// Fallback to logging without arguments
			switch level {
			case core.VerboseLevel:
				b.logger.Verbose(messageTemplate)
			case core.DebugLevel:
				b.logger.Debug(messageTemplate)
			case core.InformationLevel:
				b.logger.Information(messageTemplate)
			case core.WarningLevel:
				b.logger.Warning(messageTemplate)
			case core.ErrorLevel:
				b.logger.Error(messageTemplate)
			case core.FatalLevel:
				b.logger.Fatal(messageTemplate)
			}
			return
		}
		// Cache the parsed template
		b.templateCache.Store(messageTemplate, template)
	}
	
	// Extract args in template order
	args := make([]any, 0)
	for _, token := range template.Tokens {
		if prop, ok := token.(*parser.PropertyToken); ok {
			if val, ok := properties[prop.PropertyName]; ok {
				args = append(args, val)
			}
		}
	}
	
	switch level {
	case core.VerboseLevel:
		b.logger.Verbose(messageTemplate, args...)
	case core.DebugLevel:
		b.logger.Debug(messageTemplate, args...)
	case core.InformationLevel:
		b.logger.Information(messageTemplate, args...)
	case core.WarningLevel:
		b.logger.Warning(messageTemplate, args...)
	case core.ErrorLevel:
		b.logger.Error(messageTemplate, args...)
	case core.FatalLevel:
		b.logger.Fatal(messageTemplate, args...)
	}
}

// Enabled implements otel log.Logger
func (b *Bridge) Enabled(ctx context.Context, param olog.EnabledParameters) bool {
	level := b.mapSeverity(param.Severity)
	return b.logger.IsEnabled(level)
}

// LoggerProvider adapts mtlog to work as an OpenTelemetry LoggerProvider
type LoggerProvider struct {
	embedded.LoggerProvider
	logger core.Logger
	bridge *Bridge
}

// NewLoggerProvider creates a new LoggerProvider backed by mtlog
func NewLoggerProvider(logger core.Logger, opts ...BridgeOption) *LoggerProvider {
	return &LoggerProvider{
		logger: logger,
		bridge: NewBridge(logger, opts...),
	}
}

// Logger returns a Logger for the given instrumentation scope
func (p *LoggerProvider) Logger(name string, options ...olog.LoggerOption) olog.Logger {
	// Could create a child logger with the scope name as a property
	// For now, return the same bridge for all scopes
	return p.bridge
}

// SetAsGlobal sets this provider as the global OTEL logger provider
func (p *LoggerProvider) SetAsGlobal() {
	global.SetLoggerProvider(p)
}

// Handler implements an mtlog sink that forwards to OTEL
type Handler struct {
	logger    olog.Logger
	capturer  *capture.DefaultCapturer
	
	// Options
	includeMessageTemplate bool
	severityMapping        map[core.LogEventLevel]olog.Severity
}

// HandlerOption configures the handler
type HandlerOption func(*Handler)

// WithMessageTemplate includes the original message template as an attribute
func WithMessageTemplate(enabled bool) HandlerOption {
	return func(h *Handler) {
		h.includeMessageTemplate = enabled
	}
}

// WithLevelMapping sets custom level to severity mapping
func WithLevelMapping(mapping map[core.LogEventLevel]olog.Severity) HandlerOption {
	return func(h *Handler) {
		h.severityMapping = mapping
	}
}

// NewHandler creates a new OTEL handler that acts as an mtlog sink
func NewHandler(logger olog.Logger, opts ...HandlerOption) *Handler {
	h := &Handler{
		logger:                 logger,
		capturer:               capture.NewCapturer(3, 1000, 100),
		includeMessageTemplate: true,
		severityMapping:        defaultLevelMapping(),
	}
	
	for _, opt := range opts {
		opt(h)
	}
	
	return h
}

// defaultLevelMapping returns the default level to severity mapping
func defaultLevelMapping() map[core.LogEventLevel]olog.Severity {
	return map[core.LogEventLevel]olog.Severity{
		core.VerboseLevel:     olog.SeverityTrace,
		core.DebugLevel:       olog.SeverityDebug,
		core.InformationLevel: olog.SeverityInfo,
		core.WarningLevel:     olog.SeverityWarn,
		core.ErrorLevel:       olog.SeverityError,
		core.FatalLevel:       olog.SeverityFatal,
	}
}

// Emit implements the mtlog sink interface
func (h *Handler) Emit(event *core.LogEvent) {
	ctx := context.Background()
	
	// Create OTEL record
	var record olog.Record
	record.SetTimestamp(event.Timestamp)
	record.SetSeverity(h.mapLevel(event.Level))
	record.SetSeverityText(h.getLevelString(event.Level))
	
	// Use the message template as the body to preserve structure
	record.SetBody(olog.StringValue(event.MessageTemplate))
	
	// Always add message template as an attribute for aggregation
	record.AddAttributes(olog.String("message.template", event.MessageTemplate))
	
	// Convert properties to attributes
	attrs := make([]olog.KeyValue, 0, len(event.Properties))
	for k, v := range event.Properties {
		// Handle special properties
		switch k {
		case "trace.id":
			// Note: OTEL log.Record doesn't have SetTraceID method
			// Trace ID should be added as an attribute
			attrs = append(attrs, olog.String(k, fmt.Sprintf("%v", v)))
			continue
		case "span.id":
			// Note: OTEL log.Record doesn't have SetSpanID method  
			// Span ID should be added as an attribute
			attrs = append(attrs, olog.String(k, fmt.Sprintf("%v", v)))
			continue
		case "trace.flags":
			// Add trace flags as attribute
			attrs = append(attrs, olog.String(k, fmt.Sprintf("%v", v)))
			continue
		}
		
		// Add as attribute
		attrs = append(attrs, h.createAttribute(k, v))
	}
	record.AddAttributes(attrs...)
	
	// Add exception if present
	if event.Exception != nil {
		record.AddAttributes(
			olog.String("exception.type", fmt.Sprintf("%T", event.Exception)),
			olog.String("exception.message", event.Exception.Error()),
		)
	}
	
	// Emit to OTEL logger
	h.logger.Emit(ctx, record)
}

// mapLevel maps mtlog level to OTEL severity
func (h *Handler) mapLevel(level core.LogEventLevel) olog.Severity {
	if severity, ok := h.severityMapping[level]; ok {
		return severity
	}
	return olog.SeverityUndefined
}

// getLevelString converts level to string
func (h *Handler) getLevelString(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "Verbose"
	case core.DebugLevel:
		return "Debug"
	case core.InformationLevel:
		return "Information"
	case core.WarningLevel:
		return "Warning"
	case core.ErrorLevel:
		return "Error"
	case core.FatalLevel:
		return "Fatal"
	default:
		return "Information"
	}
}

// createAttribute creates an OTEL attribute from a property
func (h *Handler) createAttribute(key string, value any) olog.KeyValue {
	switch v := value.(type) {
	case string:
		return olog.String(key, v)
	case int:
		return olog.Int64(key, int64(v))
	case int64:
		return olog.Int64(key, v)
	case float64:
		return olog.Float64(key, v)
	case bool:
		return olog.Bool(key, v)
	case []string:
		values := make([]olog.Value, len(v))
		for i, s := range v {
			values[i] = olog.StringValue(s)
		}
		return olog.Slice(key, values...)
	case []int:
		values := make([]olog.Value, len(v))
		for i, n := range v {
			values[i] = olog.Int64Value(int64(n))
		}
		return olog.Slice(key, values...)
	case []int64:
		values := make([]olog.Value, len(v))
		for i, n := range v {
			values[i] = olog.Int64Value(n)
		}
		return olog.Slice(key, values...)
	case []float64:
		values := make([]olog.Value, len(v))
		for i, f := range v {
			values[i] = olog.Float64Value(f)
		}
		return olog.Slice(key, values...)
	case []bool:
		values := make([]olog.Value, len(v))
		for i, b := range v {
			values[i] = olog.BoolValue(b)
		}
		return olog.Slice(key, values...)
	case time.Time:
		return olog.String(key, v.Format(time.RFC3339Nano))
	case time.Duration:
		return olog.Int64(key, v.Nanoseconds())
	default:
		// Convert to string for unknown types
		return olog.String(key, fmt.Sprintf("%v", v))
	}
}

// Close implements the sink interface
func (h *Handler) Close() error {
	// OTEL loggers don't have a close method
	return nil
}

// Flush implements the sink interface
func (h *Handler) Flush() error {
	// OTEL loggers handle their own flushing
	return nil
}