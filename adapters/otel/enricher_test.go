package otel_test

import (
	"context"
	"testing"

	"github.com/willibrandon/mtlog/core"
	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"go.opentelemetry.io/otel"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockPropertyFactory implements core.LogEventPropertyFactory for testing
type mockPropertyFactory struct{}

func (f *mockPropertyFactory) CreateProperty(name string, value any) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

func TestOTELEnricher_WithSpan(t *testing.T) {
	// Create a tracer provider
	tp := sdk.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("test")
	
	// Start a span
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()
	
	// Get span context
	spanCtx := span.SpanContext()
	
	// Create enricher
	enricher := mtlogotel.NewOTELEnricher(ctx)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify trace ID was added
	if traceID, ok := event.Properties["trace.id"]; !ok {
		t.Error("Expected trace.id property")
	} else if traceID != spanCtx.TraceID().String() {
		t.Errorf("Expected trace.id %s, got %s", spanCtx.TraceID().String(), traceID)
	}
	
	// Verify span ID was added
	if spanID, ok := event.Properties["span.id"]; !ok {
		t.Error("Expected span.id property")
	} else if spanID != spanCtx.SpanID().String() {
		t.Errorf("Expected span.id %s, got %s", spanCtx.SpanID().String(), spanID)
	}
	
	// Verify trace flags were added
	if _, ok := event.Properties["trace.flags"]; !ok {
		t.Error("Expected trace.flags property")
	}
}

func TestOTELEnricher_WithoutSpan(t *testing.T) {
	// Create enricher with context without span
	ctx := context.Background()
	enricher := mtlogotel.NewOTELEnricher(ctx)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify no trace properties were added
	if _, ok := event.Properties["trace.id"]; ok {
		t.Error("Did not expect trace.id property")
	}
	if _, ok := event.Properties["span.id"]; ok {
		t.Error("Did not expect span.id property")
	}
	if _, ok := event.Properties["trace.flags"]; ok {
		t.Error("Did not expect trace.flags property")
	}
}

func TestOTELEnricher_NilContext(t *testing.T) {
	// Create enricher with nil context
	enricher := mtlogotel.NewOTELEnricher(nil)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich - should not panic
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify no properties were added
	if len(event.Properties) != 0 {
		t.Error("Expected no properties to be added")
	}
}

func TestStaticOTELEnricher_WithSpan(t *testing.T) {
	// Create a tracer
	tracer := otel.Tracer("test")
	
	// Start a span
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()
	
	// Get span context
	spanCtx := span.SpanContext()
	
	// Create static enricher
	enricher := mtlogotel.NewStaticOTELEnricher(ctx)
	
	// Create multiple events
	for i := 0; i < 3; i++ {
		event := &core.LogEvent{
			Properties: make(map[string]any),
		}
		
		// Enrich
		factory := &mockPropertyFactory{}
		enricher.Enrich(event, factory)
		
		// Verify trace ID
		if traceID, ok := event.Properties["trace.id"]; !ok {
			t.Error("Expected trace.id property")
		} else if traceID != spanCtx.TraceID().String() {
			t.Errorf("Expected trace.id %s, got %s", spanCtx.TraceID().String(), traceID)
		}
		
		// Verify span ID
		if spanID, ok := event.Properties["span.id"]; !ok {
			t.Error("Expected span.id property")
		} else if spanID != spanCtx.SpanID().String() {
			t.Errorf("Expected span.id %s, got %s", spanCtx.SpanID().String(), spanID)
		}
	}
}

func TestFastOTELEnricher_WithSpan(t *testing.T) {
	// Create a tracer
	tracer := otel.Tracer("test")
	
	// Start a span
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()
	
	// Get span context
	spanCtx := span.SpanContext()
	
	// Create fast enricher
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify trace ID
	if traceID, ok := event.Properties["trace.id"]; !ok {
		t.Error("Expected trace.id property")
	} else if traceID != spanCtx.TraceID().String() {
		t.Errorf("Expected trace.id %s, got %s", spanCtx.TraceID().String(), traceID)
	}
	
	// Verify span ID
	if spanID, ok := event.Properties["span.id"]; !ok {
		t.Error("Expected span.id property")
	} else if spanID != spanCtx.SpanID().String() {
		t.Errorf("Expected span.id %s, got %s", spanCtx.SpanID().String(), spanID)
	}
}

func TestFastOTELEnricher_WithoutSpan(t *testing.T) {
	// Create fast enricher with context without span
	ctx := context.Background()
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify no trace properties were added
	if _, ok := event.Properties["trace.id"]; ok {
		t.Error("Did not expect trace.id property")
	}
	if _, ok := event.Properties["span.id"]; ok {
		t.Error("Did not expect span.id property")
	}
}

func TestFastOTELEnricher_NilContext(t *testing.T) {
	// Create fast enricher with nil context
	enricher := mtlogotel.NewFastOTELEnricher(nil)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich - should not panic
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// Verify no properties were added
	if len(event.Properties) != 0 {
		t.Error("Expected no properties to be added")
	}
}

func TestOTELEnricher_SampledFlag(t *testing.T) {
	// Create a no-op tracer with sampled span
	tracer := noop.NewTracerProvider().Tracer("test")
	
	// Start a span (no-op spans are not sampled by default)
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()
	
	// Create enricher
	enricher := mtlogotel.NewFastOTELEnricher(ctx)
	
	// Create event
	event := &core.LogEvent{
		Properties: make(map[string]any),
	}
	
	// Enrich
	factory := &mockPropertyFactory{}
	enricher.Enrich(event, factory)
	
	// With no-op tracer, no properties should be added
	if len(event.Properties) != 0 {
		t.Error("Expected no properties with no-op tracer")
	}
}

// Benchmark tests

func BenchmarkOTELEnricher_WithSpan(b *testing.B) {
	tracer := otel.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()
	
	enricher := mtlogotel.NewOTELEnricher(ctx)
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

func BenchmarkOTELEnricher_WithoutSpan(b *testing.B) {
	ctx := context.Background()
	enricher := mtlogotel.NewOTELEnricher(ctx)
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

// Benchmark moved to otel_bench_test.go

// Benchmark moved to otel_bench_test.go

func BenchmarkFastOTELEnricher_WithoutSpan(b *testing.B) {
	ctx := context.Background()
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