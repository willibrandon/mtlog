package main

import (
	"context"
	"fmt"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/otel"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Setup OpenTelemetry
	res := resource.NewWithAttributes(
		"",
		attribute.String("service.name", "mtlog-otel-example"),
	)
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	otelapi.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create tracer and start a span
	tracer := otelapi.Tracer("example")
	ctx, span := tracer.Start(context.Background(), "main-operation")
	defer span.End()

	// Create logger with OTEL trace enrichment
	logger := mtlog.New(
		otel.WithOTELEnricher(ctx),     // Adds trace.id, span.id
		mtlog.WithConsoleProperties(),   // Shows properties in output
	)

	// Log messages - trace.id and span.id are automatically included
	logger.Information("Starting application")
	logger.Debug("Processing request")
	logger.Warning("Low memory")
	
	fmt.Println("\nTrace ID:", span.SpanContext().TraceID())
	fmt.Println("Span ID:", span.SpanContext().SpanID())
}