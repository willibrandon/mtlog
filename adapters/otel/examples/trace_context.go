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
		attribute.String("service.name", "example"),
	)
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	otelapi.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Create tracer and start a span
	tracer := otelapi.Tracer("example")
	ctx, span := tracer.Start(context.Background(), "operation")
	defer span.End()

	// Create logger with OTEL enricher as shown in issue #36
	logger := mtlog.New(
		otel.WithOTELEnricher(ctx),  // Auto-adds trace.id, span.id
		mtlog.WithConsoleProperties(),  // Console sink that shows properties
	)

	// Log message - trace.id and span.id should be visible in output
	logger.Information("Processing request")
	
	// Expected output format from issue #36:
	fmt.Println("\n// Expected output: [2024-01-15] INF Processing request {trace.id=\"abc...\" span.id=\"def...\"}")
	
	// Show actual IDs
	fmt.Printf("// Actual trace.id: %s\n", span.SpanContext().TraceID())
	fmt.Printf("// Actual span.id:  %s\n", span.SpanContext().SpanID())
}