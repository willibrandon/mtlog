package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func initTracer() func() {
	ctx := context.Background()

	// Create OTLP exporter
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	))
	if err != nil {
		panic(err)
	}

	// Create resource
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("builder-example"),
			semconv.ServiceVersion("1.0.0"),
		),
	)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("builder-example")

	return func() {
		_ = tp.Shutdown(context.Background())
	}
}

// Example 1: Using the convenience function
func example1() {
	fmt.Println("\n=== Example 1: Quick Setup ===")
	
	ctx, span := tracer.Start(context.Background(), "example1")
	defer span.End()

	// Simplest setup - reads OTEL_EXPORTER_OTLP_ENDPOINT from environment
	logger := otelmtlog.NewOTELLogger(ctx)
	
	logger.Information("Quick setup example {Feature}", "builder")
	logger.Warning("This is so easy!")
}

// Example 2: Using the builder for customization
func example2() {
	fmt.Println("\n=== Example 2: Custom Builder ===")
	
	ctx, span := tracer.Start(context.Background(), "example2")
	defer span.End()

	logger, err := otelmtlog.NewBuilder(ctx).
		WithEndpointFromEnv().              // Read from environment
		WithStaticEnricher().               // Use static enricher for performance
		WithHTTP().                         // Use HTTP transport
		WithBatching(200, 10*time.Second). // Custom batching
		WithConsole().                      // Also output to console
		WithMinimumLevel(core.DebugLevel). // Enable debug logging
		Build()

	if err != nil {
		panic(err)
	}

	logger.Debug("Debug message visible {Detail}", "custom-builder")
	logger.Information("Custom builder example")
	logger.Warning("With custom batching!")
}

// Example 3: HTTP server with per-request loggers
func example3() {
	fmt.Println("\n=== Example 3: HTTP Server ===")

	mux := http.NewServeMux()
	
	// Middleware that creates per-request logger
	middleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Start span for request
			ctx, span := tracer.Start(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
			defer span.End()

			// Create per-request logger with trace context
			logger := otelmtlog.NewRequestLogger(ctx, generateRequestID())
			
			// Log request details
			logger.Information("Request started {Method} {Path} {RemoteAddr}",
				r.Method, r.URL.Path, r.RemoteAddr)

			// Add logger to context for downstream handlers
			ctx = context.WithValue(ctx, "logger", logger)
			
			// Call next handler
			next(w, r.WithContext(ctx))

			logger.Information("Request completed")
		}
	}

	// Example endpoint
	mux.HandleFunc("/api/hello", middleware(func(w http.ResponseWriter, r *http.Request) {
		// Get logger from context
		logger := r.Context().Value("logger").(core.Logger)
		
		logger.Information("Processing hello endpoint")
		
		// Simulate some work
		time.Sleep(100 * time.Millisecond)
		
		w.Write([]byte("Hello from OTEL-instrumented service!\n"))
	}))

	fmt.Println("Starting server on :8080")
	fmt.Println("Try: curl http://localhost:8080/api/hello")
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Let it run for a bit
	time.Sleep(30 * time.Second)
	server.Shutdown(context.Background())
}

// Example 4: Different enricher strategies
func example4() {
	fmt.Println("\n=== Example 4: Enricher Strategies ===")

	ctx, span := tracer.Start(context.Background(), "example4")
	defer span.End()

	// Fast enricher - default, good for most cases
	fastLogger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithConsole(),
		mtlog.WithProperty("enricher", "fast"),
	)
	fastLogger.Information("Using fast enricher")

	// Static enricher - best for request-scoped loggers
	staticLogger := mtlog.New(
		mtlog.WithEnricher(otelmtlog.NewStaticOTELEnricher(ctx)),
		mtlog.WithConsole(),
		mtlog.WithProperty("enricher", "static"),
	)
	staticLogger.Information("Using static enricher")

	// Caching enricher - good for long-lived loggers with same context
	cachingLogger := mtlog.New(
		mtlog.WithEnricher(otelmtlog.NewOTELEnricher(ctx)),
		mtlog.WithConsole(),
		mtlog.WithProperty("enricher", "caching"),
	)
	cachingLogger.Information("Using caching enricher")
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

func main() {
	// Initialize tracer
	cleanup := initTracer()
	defer cleanup()

	// Set OTEL endpoint if not already set
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	}

	fmt.Println("OpenTelemetry Builder Examples")
	fmt.Println("==============================")
	fmt.Println("Make sure OTEL collector is running on localhost:4317")

	// Run examples
	example1()
	time.Sleep(1 * time.Second)

	example2()
	time.Sleep(1 * time.Second)

	example4()
	time.Sleep(1 * time.Second)

	// Uncomment to run HTTP server example
	// example3()

	fmt.Println("\n✅ Examples completed!")
}