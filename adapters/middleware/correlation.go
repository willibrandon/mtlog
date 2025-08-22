package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// Context keys for correlation
type correlationKey string

const (
	// TraceIDContextKey is the context key for trace ID
	TraceIDContextKey correlationKey = "trace-id"
	
	// SpanIDContextKey is the context key for span ID
	SpanIDContextKey correlationKey = "span-id"
	
	// ParentSpanIDContextKey is the context key for parent span ID
	ParentSpanIDContextKey correlationKey = "parent-span-id"
	
	// CorrelationIDContextKey is the context key for correlation ID
	CorrelationIDContextKey correlationKey = "correlation-id"
	
	// TraceContextKey is the context key for the full TraceContext
	TraceContextKey correlationKey = "trace-context"
)

// Common trace headers
const (
	// W3C Trace Context headers
	HeaderTraceParent = "Traceparent"
	HeaderTraceState  = "Tracestate"
	
	// X-Ray headers
	HeaderXRayTraceID = "X-Amzn-Trace-Id"
	
	// B3 headers (Zipkin)
	HeaderB3TraceID      = "X-B3-TraceId"
	HeaderB3SpanID       = "X-B3-SpanId"
	HeaderB3ParentSpanID = "X-B3-ParentSpanId"
	HeaderB3Sampled      = "X-B3-Sampled"
	HeaderB3             = "B3"
	
	// Custom headers
	HeaderTraceID       = "X-Trace-ID"
	HeaderSpanID        = "X-Span-ID"
	HeaderParentSpanID  = "X-Parent-Span-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderRequestID     = "X-Request-ID"
)

// TraceContext holds distributed tracing information
type TraceContext struct {
	TraceID       string `json:"trace_id"`
	SpanID        string `json:"span_id"`
	ParentSpanID  string `json:"parent_span_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Sampled       bool   `json:"sampled"`
	Baggage       map[string]string `json:"baggage,omitempty"`
}

// CorrelationOptions configures correlation middleware
type CorrelationOptions struct {
	// GenerateTraceID generates a new trace ID if none exists
	GenerateTraceID bool
	
	// GenerateSpanID generates a new span ID for each request
	GenerateSpanID bool
	
	// PropagateDownstream propagates trace context to downstream services
	PropagateDownstream bool
	
	// HeaderFormat specifies which header format to use
	HeaderFormat TraceHeaderFormat
	
	// Logger for correlation events
	Logger core.Logger
	
	// CustomTraceIDGenerator allows custom trace ID generation
	CustomTraceIDGenerator func() string
	
	// CustomSpanIDGenerator allows custom span ID generation
	CustomSpanIDGenerator func() string
	
	// ExtractBaggage extracts additional context from headers
	ExtractBaggage bool
	
	// BaggagePrefix is the header prefix for baggage items
	BaggagePrefix string
}

// TraceHeaderFormat specifies the trace header format
type TraceHeaderFormat int

const (
	// FormatCustom uses simple X-Trace-ID headers
	FormatCustom TraceHeaderFormat = iota
	
	// FormatW3C uses W3C Trace Context format
	FormatW3C
	
	// FormatB3 uses Zipkin B3 format
	FormatB3
	
	// FormatB3Single uses Zipkin B3 single header format
	FormatB3Single
	
	// FormatXRay uses AWS X-Ray format
	FormatXRay
)

// PropagateTraceContext creates middleware for trace context propagation
func PropagateTraceContext(next http.Handler, opts ...CorrelationOptions) http.Handler {
	options := CorrelationOptions{
		GenerateTraceID:     true,
		GenerateSpanID:      true,
		PropagateDownstream: true,
		HeaderFormat:        FormatCustom,
		BaggagePrefix:       "X-Baggage-",
	}
	
	if len(opts) > 0 {
		options = opts[0]
	}
	
	// Set default generators if not provided
	if options.CustomTraceIDGenerator == nil {
		options.CustomTraceIDGenerator = generateTraceID
	}
	if options.CustomSpanIDGenerator == nil {
		options.CustomSpanIDGenerator = generateSpanID
	}
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract or generate trace context
		traceCtx := extractTraceContext(r, options.HeaderFormat)
		
		// Generate IDs if needed
		if traceCtx.TraceID == "" && options.GenerateTraceID {
			traceCtx.TraceID = options.CustomTraceIDGenerator()
		}
		
		if options.GenerateSpanID {
			// Save current span as parent
			if traceCtx.SpanID != "" {
				traceCtx.ParentSpanID = traceCtx.SpanID
			}
			// Generate new span ID
			traceCtx.SpanID = options.CustomSpanIDGenerator()
		}
		
		// Extract correlation ID
		if traceCtx.CorrelationID == "" {
			traceCtx.CorrelationID = r.Header.Get(HeaderCorrelationID)
			if traceCtx.CorrelationID == "" {
				traceCtx.CorrelationID = r.Header.Get(HeaderRequestID)
			}
		}
		
		// Extract baggage if configured
		if options.ExtractBaggage && options.BaggagePrefix != "" {
			traceCtx.Baggage = extractBaggage(r, options.BaggagePrefix)
		}
		
		// Add to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, TraceIDContextKey, traceCtx.TraceID)
		ctx = context.WithValue(ctx, SpanIDContextKey, traceCtx.SpanID)
		if traceCtx.ParentSpanID != "" {
			ctx = context.WithValue(ctx, ParentSpanIDContextKey, traceCtx.ParentSpanID)
		}
		if traceCtx.CorrelationID != "" {
			ctx = context.WithValue(ctx, CorrelationIDContextKey, traceCtx.CorrelationID)
		}
		// Store the full TraceContext for access to baggage
		ctx = context.WithValue(ctx, TraceContextKey, traceCtx)
		
		// Add to logger if provided
		if options.Logger != nil {
			logger := options.Logger.
				With("TraceId", traceCtx.TraceID).
				With("SpanId", traceCtx.SpanID)
			
			if traceCtx.ParentSpanID != "" {
				logger = logger.With("ParentSpanId", traceCtx.ParentSpanID)
			}
			if traceCtx.CorrelationID != "" {
				logger = logger.With("CorrelationId", traceCtx.CorrelationID)
			}
			
			// Add baggage to logger
			for k, v := range traceCtx.Baggage {
				logger = logger.With("Baggage."+k, v)
			}
			
			ctx = context.WithValue(ctx, LoggerContextKey, logger)
		}
		
		// Propagate downstream if configured
		if options.PropagateDownstream {
			propagateTraceContext(w, traceCtx, options.HeaderFormat)
			
			// Propagate baggage
			for k, v := range traceCtx.Baggage {
				w.Header().Set(options.BaggagePrefix+k, v)
			}
		}
		
		// Continue with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractTraceContext extracts trace context from request headers
func extractTraceContext(r *http.Request, format TraceHeaderFormat) *TraceContext {
	ctx := &TraceContext{
		Sampled: true,
		Baggage: make(map[string]string),
	}
	
	switch format {
	case FormatW3C:
		extractW3CContext(r, ctx)
	case FormatB3:
		extractB3Context(r, ctx)
	case FormatB3Single:
		extractB3SingleContext(r, ctx)
	case FormatXRay:
		extractXRayContext(r, ctx)
	default:
		extractCustomContext(r, ctx)
	}
	
	return ctx
}

// extractCustomContext extracts custom trace headers
func extractCustomContext(r *http.Request, ctx *TraceContext) {
	ctx.TraceID = r.Header.Get(HeaderTraceID)
	ctx.SpanID = r.Header.Get(HeaderSpanID)
	ctx.ParentSpanID = r.Header.Get(HeaderParentSpanID)
	ctx.CorrelationID = r.Header.Get(HeaderCorrelationID)
}

// extractW3CContext extracts W3C Trace Context
func extractW3CContext(r *http.Request, ctx *TraceContext) {
	traceparent := r.Header.Get(HeaderTraceParent)
	if traceparent == "" {
		return
	}
	
	// Parse traceparent: version-trace_id-parent_id-trace_flags
	parts := strings.Split(traceparent, "-")
	if len(parts) >= 4 {
		ctx.TraceID = parts[1]
		ctx.SpanID = parts[2]
		if parts[3] == "01" {
			ctx.Sampled = true
		}
	}
}

// extractB3Context extracts B3 multi-header format
func extractB3Context(r *http.Request, ctx *TraceContext) {
	ctx.TraceID = r.Header.Get(HeaderB3TraceID)
	ctx.SpanID = r.Header.Get(HeaderB3SpanID)
	ctx.ParentSpanID = r.Header.Get(HeaderB3ParentSpanID)
	
	sampled := r.Header.Get(HeaderB3Sampled)
	ctx.Sampled = sampled == "1" || sampled == "true"
}

// extractB3SingleContext extracts B3 single header format
func extractB3SingleContext(r *http.Request, ctx *TraceContext) {
	b3 := r.Header.Get(HeaderB3)
	if b3 == "" {
		return
	}
	
	// Parse b3: trace_id-span_id-sampled-parent_span_id
	parts := strings.Split(b3, "-")
	if len(parts) >= 2 {
		ctx.TraceID = parts[0]
		ctx.SpanID = parts[1]
	}
	if len(parts) >= 3 {
		ctx.Sampled = parts[2] == "1"
	}
	if len(parts) >= 4 {
		ctx.ParentSpanID = parts[3]
	}
}

// extractXRayContext extracts AWS X-Ray trace header
func extractXRayContext(r *http.Request, ctx *TraceContext) {
	xray := r.Header.Get(HeaderXRayTraceID)
	if xray == "" {
		return
	}
	
	// Parse X-Ray: Root=1-trace_id;Parent=span_id;Sampled=1
	parts := strings.Split(xray, ";")
	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			switch kv[0] {
			case "Root":
				ctx.TraceID = strings.TrimPrefix(kv[1], "1-")
			case "Parent":
				ctx.ParentSpanID = kv[1]
			case "Sampled":
				ctx.Sampled = kv[1] == "1"
			}
		}
	}
}

// propagateTraceContext adds trace context to response headers
func propagateTraceContext(w http.ResponseWriter, ctx *TraceContext, format TraceHeaderFormat) {
	switch format {
	case FormatW3C:
		propagateW3CContext(w, ctx)
	case FormatB3:
		propagateB3Context(w, ctx)
	case FormatB3Single:
		propagateB3SingleContext(w, ctx)
	case FormatXRay:
		propagateXRayContext(w, ctx)
	default:
		propagateCustomContext(w, ctx)
	}
}

// propagateCustomContext adds custom trace headers
func propagateCustomContext(w http.ResponseWriter, ctx *TraceContext) {
	if ctx.TraceID != "" {
		w.Header().Set(HeaderTraceID, ctx.TraceID)
	}
	if ctx.SpanID != "" {
		w.Header().Set(HeaderSpanID, ctx.SpanID)
	}
	if ctx.ParentSpanID != "" {
		w.Header().Set(HeaderParentSpanID, ctx.ParentSpanID)
	}
	if ctx.CorrelationID != "" {
		w.Header().Set(HeaderCorrelationID, ctx.CorrelationID)
	}
}

// propagateW3CContext adds W3C trace headers
func propagateW3CContext(w http.ResponseWriter, ctx *TraceContext) {
	if ctx.TraceID != "" && ctx.SpanID != "" {
		sampled := "00"
		if ctx.Sampled {
			sampled = "01"
		}
		traceparent := fmt.Sprintf("00-%s-%s-%s", ctx.TraceID, ctx.SpanID, sampled)
		w.Header().Set(HeaderTraceParent, traceparent)
	}
}

// propagateB3Context adds B3 multi-header format
func propagateB3Context(w http.ResponseWriter, ctx *TraceContext) {
	if ctx.TraceID != "" {
		w.Header().Set(HeaderB3TraceID, ctx.TraceID)
	}
	if ctx.SpanID != "" {
		w.Header().Set(HeaderB3SpanID, ctx.SpanID)
	}
	if ctx.ParentSpanID != "" {
		w.Header().Set(HeaderB3ParentSpanID, ctx.ParentSpanID)
	}
	if ctx.Sampled {
		w.Header().Set(HeaderB3Sampled, "1")
	}
}

// propagateB3SingleContext adds B3 single header
func propagateB3SingleContext(w http.ResponseWriter, ctx *TraceContext) {
	if ctx.TraceID != "" && ctx.SpanID != "" {
		sampled := "0"
		if ctx.Sampled {
			sampled = "1"
		}
		b3 := fmt.Sprintf("%s-%s-%s", ctx.TraceID, ctx.SpanID, sampled)
		if ctx.ParentSpanID != "" {
			b3 = fmt.Sprintf("%s-%s", b3, ctx.ParentSpanID)
		}
		w.Header().Set(HeaderB3, b3)
	}
}

// propagateXRayContext adds X-Ray trace header
func propagateXRayContext(w http.ResponseWriter, ctx *TraceContext) {
	if ctx.TraceID != "" {
		xray := fmt.Sprintf("Root=1-%s", ctx.TraceID)
		if ctx.ParentSpanID != "" {
			xray = fmt.Sprintf("%s;Parent=%s", xray, ctx.ParentSpanID)
		}
		if ctx.Sampled {
			xray = fmt.Sprintf("%s;Sampled=1", xray)
		}
		w.Header().Set(HeaderXRayTraceID, xray)
	}
}

// extractBaggage extracts baggage items from headers
func extractBaggage(r *http.Request, prefix string) map[string]string {
	baggage := make(map[string]string)
	for name, values := range r.Header {
		if strings.HasPrefix(name, prefix) && len(values) > 0 {
			key := strings.TrimPrefix(name, prefix)
			baggage[key] = values[0]
		}
	}
	return baggage
}

// generateTraceID generates a new trace ID
func generateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateSpanID generates a new span ID
func generateSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetTraceContext retrieves the trace context from the request context
func GetTraceContext(ctx context.Context) *TraceContext {
	// First check if we have the full TraceContext stored
	if tc, ok := ctx.Value(TraceContextKey).(*TraceContext); ok {
		return tc
	}
	
	// Otherwise, reconstruct from individual values (for backward compatibility)
	tc := &TraceContext{
		Baggage: make(map[string]string),
	}
	
	if traceID, ok := ctx.Value(TraceIDContextKey).(string); ok {
		tc.TraceID = traceID
	}
	if spanID, ok := ctx.Value(SpanIDContextKey).(string); ok {
		tc.SpanID = spanID
	}
	if parentSpanID, ok := ctx.Value(ParentSpanIDContextKey).(string); ok {
		tc.ParentSpanID = parentSpanID
	}
	if correlationID, ok := ctx.Value(CorrelationIDContextKey).(string); ok {
		tc.CorrelationID = correlationID
	}
	
	return tc
}

// TraceRoundTripper adds trace context to outgoing HTTP requests
type TraceRoundTripper struct {
	Transport http.RoundTripper
	Format    TraceHeaderFormat
}

// RoundTrip implements http.RoundTripper
func (t *TraceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Get trace context from request context
	traceCtx := GetTraceContext(req.Context())
	
	// Clone request to avoid modifying the original
	req2 := req.Clone(req.Context())
	
	// Add trace headers
	switch t.Format {
	case FormatW3C:
		if traceCtx.TraceID != "" && traceCtx.SpanID != "" {
			sampled := "00"
			if traceCtx.Sampled {
				sampled = "01"
			}
			req2.Header.Set(HeaderTraceParent, fmt.Sprintf("00-%s-%s-%s", traceCtx.TraceID, traceCtx.SpanID, sampled))
		}
	case FormatB3:
		if traceCtx.TraceID != "" {
			req2.Header.Set(HeaderB3TraceID, traceCtx.TraceID)
		}
		if traceCtx.SpanID != "" {
			req2.Header.Set(HeaderB3SpanID, traceCtx.SpanID)
		}
	default:
		if traceCtx.TraceID != "" {
			req2.Header.Set(HeaderTraceID, traceCtx.TraceID)
		}
		if traceCtx.SpanID != "" {
			req2.Header.Set(HeaderSpanID, traceCtx.SpanID)
		}
	}
	
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	
	return transport.RoundTrip(req2)
}

// NewTracingClient creates an HTTP client that propagates trace context
func NewTracingClient(format TraceHeaderFormat) *http.Client {
	return &http.Client{
		Transport: &TraceRoundTripper{
			Transport: http.DefaultTransport,
			Format:    format,
		},
		Timeout: 30 * time.Second,
	}
}