package otel_test

import (
	"context"
	"testing"

	"github.com/willibrandon/mtlog/core"
	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Benchmark the FastOTELEnricher with no span (target: <5ns)
func BenchmarkFastOTELEnricher_NoSpan(b *testing.B) {
	ctx := context.Background()
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		enricher.Enrich(event, factory)
		// Clear properties for next iteration
		for k := range event.Properties {
			delete(event.Properties, k)
		}
	}
}

// Benchmark just the span check without enrichment
func BenchmarkFastOTELEnricher_SpanCheckOnly(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// This is what happens inside the enricher
		span := trace.SpanFromContext(ctx)
		if span != nil {
			spanCtx := span.SpanContext()
			_ = spanCtx.IsValid()
		}
	}
}

// Benchmark the FastOTELEnricher with a valid span
func BenchmarkFastOTELEnricher_WithSpan(b *testing.B) {
	tracer := otel.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Properties: make(map[string]any),
		}
		enricher.Enrich(event, factory)
	}
}

// Benchmark the StaticOTELEnricher (pre-extracted values)
func BenchmarkStaticOTELEnricher_WithSpan(b *testing.B) {
	tracer := otel.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	enricher := mtlogotel.NewStaticOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Properties: make(map[string]any),
		}
		enricher.Enrich(event, factory)
	}
}

// Benchmark the StaticOTELEnricher with no span
func BenchmarkStaticOTELEnricher_NoSpan(b *testing.B) {
	ctx := context.Background()
	enricher := mtlogotel.NewStaticOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		enricher.Enrich(event, factory)
		// Clear properties for next iteration
		for k := range event.Properties {
			delete(event.Properties, k)
		}
	}
}

// Benchmark the caching OTELEnricher
func BenchmarkOTELEnricher_WithSpan_Cached(b *testing.B) {
	tracer := otel.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	enricher := mtlogotel.NewOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	
	// Warm up the cache
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	enricher.Enrich(event, factory)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Properties: make(map[string]any),
		}
		enricher.Enrich(event, factory)
	}
}

// Benchmark with no-op tracer (simulates disabled tracing)
func BenchmarkFastOTELEnricher_NoOpTracer(b *testing.B) {
	tracer := noop.NewTracerProvider().Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	factory := &mockPropertyFactory{}
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		enricher.Enrich(event, factory)
		// Clear properties for next iteration
		for k := range event.Properties {
			delete(event.Properties, k)
		}
	}
}

// Comparison benchmark: baseline with no enricher
func BenchmarkBaseline_NoEnricher(b *testing.B) {
	factory := &mockPropertyFactory{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		event := &core.LogEvent{
			Properties: make(map[string]any),
		}
		// Just create the event without enrichment
		_ = event
		_ = factory
	}
}

// Benchmark property creation overhead
func BenchmarkPropertyCreation(b *testing.B) {
	factory := &mockPropertyFactory{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		prop1 := factory.CreateProperty("trace.id", "abc123")
		prop2 := factory.CreateProperty("span.id", "def456")
		prop3 := factory.CreateProperty("trace.flags", "01")
		_ = prop1
		_ = prop2
		_ = prop3
	}
}

// Benchmark trace ID string conversion
func BenchmarkTraceIDConversion(b *testing.B) {
	tracer := otel.Tracer("bench")
	_, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	spanCtx := span.SpanContext()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		traceID := spanCtx.TraceID().String()
		spanID := spanCtx.SpanID().String()
		_ = traceID
		_ = spanID
	}
}