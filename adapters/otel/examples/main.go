//go:build otel
// +build otel

// Package main demonstrates OpenTelemetry integration with mtlog.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// Enable selflog for debugging
	selflog.Enable(selflog.Sync(log.Writer()))
	defer selflog.Disable()

	// Initialize OpenTelemetry
	ctx := context.Background()
	shutdown, err := initOTEL(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize OTEL (running examples without tracing): %v", err)
	} else {
		defer shutdown(ctx)
	}

	// Example 1: Basic OTEL enrichment
	basicExample(ctx)

	// Example 2: OTLP export configuration
	otlpConfigExample(ctx)

	// Example 3: Distributed tracing
	distributedTracingExample(ctx)

	// Example 4: Request-scoped logging
	requestScopedExample(ctx)

	// Example 5: Performance comparison
	performanceExample(ctx)

	fmt.Println("\nAll examples completed successfully!")
}

// initOTEL initializes OpenTelemetry tracing
func initOTEL(ctx context.Context) (func(context.Context) error, error) {
	// Create OTLP exporter
	exporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint("localhost:4317"),
			otlptracegrpc.WithInsecure(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource without schema conflicts
	// Use Empty() to avoid auto/sdk default resource with conflicting schema
	res := resource.NewWithAttributes(
		"", // No schema URL to avoid conflicts
		attribute.String("service.name", "mtlog-otel-example"),
		attribute.String("service.version", "1.0.0"),
		attribute.String("environment", "development"),
	)
	// No error possible with NewWithAttributes

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set as global
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return shutdown function
	return tp.Shutdown, nil
}

// Example 1: Basic OTEL enrichment
func basicExample(ctx context.Context) {
	fmt.Println("\n=== Example 1: Basic OTEL Enrichment ===")

	// Start a span
	tracer := otel.Tracer("example")
	ctx, span := tracer.Start(ctx, "basic-operation")
	defer span.End()

	// Create a logger with OTEL enrichment
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTELEnricher(ctx), // Automatically adds trace.id and span.id
		mtlog.WithMinimumLevel(core.DebugLevel),
	)

	// Log with automatic trace context
	logger.Information("Processing user request for {UserId}", 12345)
	logger.Debug("Cache lookup for key {CacheKey}", "user:12345")

	// Add span attributes
	span.SetAttributes(
		attribute.String("user.id", "12345"),
		attribute.String("operation", "cache-lookup"),
	)

	logger.Information("Request completed successfully")
}

// Example 2: OTLP export configuration
func otlpConfigExample(ctx context.Context) {
	fmt.Println("\n=== Example 2: OTLP Export Configuration ===")

	// Method 1: Use environment variables (recommended for production)
	// Set OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
	logger1 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTLP(), // Uses environment variables
	)

	// Method 2: Explicit gRPC configuration
	logger2 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTLPGRPC("localhost:4317"),
	)

	// Method 3: Explicit HTTP configuration
	logger3 := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTLPHTTP("http://localhost:4318/v1/logs"),
	)

	// Method 4: Full configuration with builder
	otlpConfig := mtlog.NewOTLPConfig().
		WithEndpoint("localhost:4317").
		WithGRPC().
		WithCompression("gzip").
		WithBatching(100, 5*time.Second).
		WithRetry(1*time.Second, 30*time.Second).
		WithHeaders(map[string]string{
			"api-key": "your-api-key",
		})

	logger4 := mtlog.New(
		mtlog.WithConsole(),
		otlpConfig.Build(),
	)

	// Start a span for correlation
	tracer := otel.Tracer("example")
	ctx, span := tracer.Start(ctx, "otlp-export-demo")
	defer span.End()

	// Create logger with span context
	spanLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithStaticOTELEnricher(ctx), // Use static enricher for efficiency
	)

	// Log some messages
	spanLogger.Information("Testing OTLP export")
	logger1.Information("Logger 1: Environment-based config")
	logger2.Information("Logger 2: gRPC config")
	logger3.Information("Logger 3: HTTP config")
	logger4.Information("Logger 4: Full custom config")
}

// Example 3: Distributed tracing
func distributedTracingExample(ctx context.Context) {
	fmt.Println("\n=== Example 3: Distributed Tracing ===")

	// Simulate receiving a request with trace context
	tracer := otel.Tracer("api-server")

	// Start a parent span
	ctx, parentSpan := tracer.Start(ctx, "handle-request",
		trace.WithAttributes(
			attribute.String("http.method", "POST"),
			attribute.String("http.path", "/api/orders"),
			attribute.String("http.host", "api.example.com"),
		),
	)
	defer parentSpan.End()

	// Create a logger for this request
	requestLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithStaticOTELEnricher(ctx), // Static since context won't change
		mtlog.WithProperty("request.id", "req-123"),
		mtlog.WithProperty("service.name", "api-server"),
	)

	requestLogger.Information("Received order request")

	// Simulate calling downstream services
	processOrder(ctx, requestLogger, tracer)
	updateInventory(ctx, requestLogger, tracer)

	requestLogger.Information("Order processing completed")
}

func processOrder(ctx context.Context, logger core.Logger, tracer trace.Tracer) {
	// Start a child span
	ctx, span := tracer.Start(ctx, "process-order",
		trace.WithAttributes(
			attribute.String("service.name", "order-processor"),
		),
	)
	defer span.End()

	// Create a logger for this operation
	opLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithStaticOTELEnricher(ctx),
		mtlog.WithProperty("service.name", "order-processor"),
	)

	opLogger.Debug("Validating order")
	time.Sleep(10 * time.Millisecond)

	opLogger.Debug("Calculating pricing")
	time.Sleep(20 * time.Millisecond)

	opLogger.Information("Order processed successfully")
}

func updateInventory(ctx context.Context, logger core.Logger, tracer trace.Tracer) {
	// Start another child span
	ctx, span := tracer.Start(ctx, "update-inventory",
		trace.WithAttributes(
			attribute.String("service.name", "inventory-service"),
		),
	)
	defer span.End()

	// Create a logger for this operation
	invLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithStaticOTELEnricher(ctx),
		mtlog.WithProperty("service.name", "inventory-service"),
	)

	invLogger.Debug("Checking stock levels")
	time.Sleep(15 * time.Millisecond)

	invLogger.Debug("Updating inventory counts")
	time.Sleep(10 * time.Millisecond)

	invLogger.Information("Inventory updated")
}

// Example 4: Request-scoped logging
func requestScopedExample(ctx context.Context) {
	fmt.Println("\n=== Example 4: Request-Scoped Logging ===")

	// Simulate an HTTP handler with trace propagation
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from headers (W3C Trace Context)
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start a span for this request
		tracer := otel.Tracer("http-server")
		ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
				attribute.String("http.host", r.Host),
				attribute.String("http.scheme", "http"),
				attribute.String("http.user_agent", r.UserAgent()),
			),
		)
		defer span.End()

		// Create a request-scoped logger with static enricher
		// This logger will have the same trace/span IDs for the entire request
		logger := mtlog.New(
			mtlog.WithConsole(),
			mtlog.WithStaticOTELEnricher(ctx), // Static for the request lifetime
			mtlog.WithProperty("http.method", r.Method),
			mtlog.WithProperty("http.path", r.URL.Path),
			mtlog.WithProperty("http.remote_addr", r.RemoteAddr),
			mtlog.WithProperty("request.id", "req-"+generateID()),
		)

		// Use the logger throughout request processing
		logger.Information("Request started")

		// Simulate processing with potential error
		if err := processHTTPRequest(ctx, logger); err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.Int("http.status_code", 500))
			logger.Error("Request failed: {Error}", err)
			if w != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		span.SetAttributes(attribute.Int("http.status_code", 200))
		logger.Information("Request completed successfully")
		if w != nil {
			w.WriteHeader(http.StatusOK)
		}
	}

	// Simulate a request with W3C Trace Context header
	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("User-Agent", "mtlog-example/1.0")
	req.Host = "api.example.com"
	req.RemoteAddr = "192.168.1.100:54321"
	
	handler(nil, req)
}

func processHTTPRequest(ctx context.Context, logger core.Logger) error {
	logger.Debug("Authenticating request")
	time.Sleep(5 * time.Millisecond)
	
	logger.Debug("Loading user data")
	time.Sleep(10 * time.Millisecond)
	
	logger.Debug("Applying business logic")
	time.Sleep(15 * time.Millisecond)
	
	return nil
}

// Example 5: Performance comparison
func performanceExample(ctx context.Context) {
	fmt.Println("\n=== Example 5: Performance Comparison ===")

	// Start a span for testing
	tracer := otel.Tracer("perf-test")
	ctx, span := tracer.Start(ctx, "performance-test")
	defer span.End()

	// Test 1: FastOTELEnricher (re-extracts each time, good for changing contexts)
	fastLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTELEnricher(ctx),
	)

	// Test 2: StaticOTELEnricher (extracts once, best for request-scoped loggers)
	staticLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithStaticOTELEnricher(ctx),
	)

	// Test 3: No OTEL enrichment (baseline)
	plainLogger := mtlog.New(
		mtlog.WithConsole(),
	)

	// Measure performance
	const iterations = 1000

	// Fast enricher
	start := time.Now()
	for i := 0; i < iterations; i++ {
		fastLogger.Debug("Fast enricher log {Iteration}", i)
	}
	fastDuration := time.Since(start)

	// Static enricher
	start = time.Now()
	for i := 0; i < iterations; i++ {
		staticLogger.Debug("Static enricher log {Iteration}", i)
	}
	staticDuration := time.Since(start)

	// Plain logger
	start = time.Now()
	for i := 0; i < iterations; i++ {
		plainLogger.Debug("Plain logger log {Iteration}", i)
	}
	plainDuration := time.Since(start)

	fmt.Printf("\nPerformance Results (%d iterations):\n", iterations)
	fmt.Printf("  Plain logger:   %v (%.2f ns/op)\n", plainDuration, float64(plainDuration.Nanoseconds())/float64(iterations))
	fmt.Printf("  Fast enricher:  %v (%.2f ns/op)\n", fastDuration, float64(fastDuration.Nanoseconds())/float64(iterations))
	fmt.Printf("  Static enricher: %v (%.2f ns/op)\n", staticDuration, float64(staticDuration.Nanoseconds())/float64(iterations))
	
	// Test without span (should be very fast)
	ctxNoSpan := context.Background()
	noSpanLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithOTELEnricher(ctxNoSpan),
	)
	
	start = time.Now()
	for i := 0; i < iterations; i++ {
		noSpanLogger.Debug("No span log {Iteration}", i)
	}
	noSpanDuration := time.Since(start)
	
	fmt.Printf("  No span enricher: %v (%.2f ns/op)\n", noSpanDuration, float64(noSpanDuration.Nanoseconds())/float64(iterations))
	fmt.Println("\nNote: The <5ns overhead target applies to the enricher itself, not the full logging pipeline")
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}