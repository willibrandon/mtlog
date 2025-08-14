# OpenTelemetry Adapter for mtlog

This adapter provides OpenTelemetry integration for mtlog, including automatic trace context enrichment and OTLP export capabilities.

## ⚠️ Important: Separate Module

**This adapter is a separate Go module** to maintain mtlog's zero-dependency principle. OpenTelemetry dependencies are only added to your project when you explicitly import this adapter module.

```
github.com/willibrandon/mtlog         # Main module - zero dependencies
github.com/willibrandon/mtlog/adapters/otel  # OTEL adapter - requires OTEL SDK
```

## Installation

```bash
go get github.com/willibrandon/mtlog/adapters/otel
```

## Features

- **Automatic Trace Enrichment**: Adds `trace.id`, `span.id`, and `trace.flags` to log events
- **OTLP Export**: Send logs to OpenTelemetry collectors via gRPC or HTTP
- **Bridge Support**: Bidirectional compatibility between mtlog and OpenTelemetry logging
- **Zero Dependencies**: Main mtlog module remains dependency-free
- **High Performance**: <5ns overhead when no span is present

## Quick Start

### Import Pattern

```go
import (
    "github.com/willibrandon/mtlog"
    otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
)

// Create logger with OTEL support
logger := mtlog.New(
    otelmtlog.WithOTELEnricher(ctx),
    otelmtlog.WithOTLPGRPC("localhost:4317"),
)
```

## Context Best Practices

### ✅ Correct: Use Request Context

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Use request context which contains the active span
    logger := mtlog.New(
        otelmtlog.WithOTELEnricher(r.Context()),
        mtlog.WithConsole(),
    )
    
    logger.Information("Processing request {Path}", r.URL.Path)
    // Output: [INF] Processing request /api/users {trace.id="abc123" span.id="def456" Path="/api/users"}
}
```

### ❌ Incorrect: Background Context

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // DON'T use background context - loses trace information
    logger := mtlog.New(
        otelmtlog.WithOTELEnricher(context.Background()), // No trace!
        mtlog.WithConsole(),
    )
    
    logger.Information("Processing request")
    // Output: [INF] Processing request (no trace.id or span.id)
}
```

### Per-Request Logger Pattern

```go
func middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Create per-request logger with trace context
        logger := mtlog.New(
            otelmtlog.WithOTELEnricher(r.Context()),
            mtlog.WithProperty("request_id", generateRequestID()),
        )
        
        // Add logger to context for downstream use
        ctx := context.WithValue(r.Context(), loggerKey, logger)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Usage Examples

### Basic Trace Enrichment

```go
import (
    "context"
    "github.com/willibrandon/mtlog"
    otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
    "go.opentelemetry.io/otel"
)

// Setup tracer
tracer := otel.Tracer("my-service")

// Create span
ctx, span := tracer.Start(context.Background(), "operation")
defer span.End()

// Create logger with OTEL trace enrichment
logger := mtlog.New(
    otelmtlog.WithOTELEnricher(ctx),  // Auto-adds trace.id, span.id
    mtlog.WithConsole(),
)

logger.Information("Processing request")
// Output includes: {trace.id="..." span.id="..."}
```

### OTLP Export

```go
// Export to OTEL collector via gRPC
logger := mtlog.New(
    otel.WithOTLPGRPC("localhost:4317"),
    mtlog.WithConsole(),
)

// Export via HTTP
logger := mtlog.New(
    otel.WithOTLPHTTP("localhost:4318"),
    mtlog.WithConsole(),
)
```

### Request-Scoped Logging

For best performance with request-scoped loggers:

```go
// Pre-extract trace context once per request
requestLogger := mtlog.New(
    otel.WithStaticOTELEnricher(ctx),  // Caches trace/span IDs
    mtlog.WithConsole(),
)
```

## Performance

- **FastOTELEnricher**: ~8.6ns overhead (default)
- **StaticOTELEnricher**: ~3.3ns overhead (best for request-scoped)
- **Zero allocations** when no span is present

## OpenTelemetry Bridge

Use mtlog as an OpenTelemetry LoggerProvider:

```go
bridge := otel.NewBridge(logger)
// Use bridge with OTEL instrumentation libraries
```

## Convenience Builders

### Quick Setup with Defaults

```go
import otelmtlog "github.com/willibrandon/mtlog/adapters/otel"

// Creates logger with OTEL defaults (reads OTEL_EXPORTER_OTLP_ENDPOINT)
logger := otelmtlog.NewOTELLogger(ctx)
```

### Per-Request Logger

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Optimized for request handling with static enricher
    logger := otelmtlog.NewRequestLogger(
        r.Context(),
        generateRequestID(),
    )
    
    logger.Information("Handling {Method} {Path}", r.Method, r.URL.Path)
}
```

### Custom Builder

```go
logger, err := otelmtlog.NewBuilder(ctx).
    WithEndpointFromEnv().        // Read from OTEL_EXPORTER_OTLP_ENDPOINT
    WithStaticEnricher().         // Use static enricher for performance
    WithHTTP().                   // Use HTTP transport
    WithBatching(200, 10*time.Second).
    WithConsole().                // Also output to console
    WithMinimumLevel(core.DebugLevel).
    Build()
```

### TLS Configuration

#### Insecure Connections (Development)

```go
sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("localhost:4317"),
    otelmtlog.WithOTLPInsecure(), // Disable TLS
)
```

#### Custom TLS Configuration

```go
tlsConfig := &tls.Config{
    ServerName: "otel-collector.company.com",
    MinVersion: tls.VersionTLS12,
}

sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("otel-collector.company.com:4317"),
    otelmtlog.WithOTLPTLSConfig(tlsConfig),
)
```

#### Mutual TLS (mTLS)

```go
sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("secure.company.com:4317"),
    otelmtlog.WithOTLPClientCert("client.crt", "client.key"),
    otelmtlog.WithOTLPCACert("company-ca.crt"),
)
```

#### Skip Certificate Verification (Insecure)

```go
// Only for development with self-signed certificates
sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("self-signed.local:4317"),
    otelmtlog.WithOTLPSkipVerify(), // INSECURE - development only
)
```

### Sampling for High-Volume Scenarios

```go
// Rate-based sampling (10% of events)
sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("localhost:4317"),
    otelmtlog.WithOTLPSampling(otelmtlog.NewRateSampler(0.1)),
)

// Adaptive sampling (maintain target rate)
samplingSink := otelmtlog.NewSamplingSink(
    otlpSink,
    otelmtlog.NewAdaptiveSampler(100), // 100 events/second
)

// Level-based sampling (Warning and above)
samplingSink := otelmtlog.NewSamplingSink(
    otlpSink,
    otelmtlog.NewLevelSampler(core.WarningLevel),
)
```

### Prometheus Metrics Export

```go
// OTLP sink with integrated Prometheus metrics
sink, err := otelmtlog.NewOTLPSink(
    otelmtlog.WithOTLPEndpoint("localhost:4317"),
    otelmtlog.WithPrometheusMetrics(9090), // Metrics on port 9090
)

// Standalone metrics exporter
metricsExporter, err := otelmtlog.NewMetricsExporter(
    otelmtlog.WithMetricsPort(9090),
    otelmtlog.WithMetricsPath("/metrics"),
)
```

### Environment Configuration

The builder and convenience functions respect standard OTEL environment variables:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_HEADERS="api-key=secret"
```

## Running with OpenTelemetry Collector

```bash
# Start OTEL collector
docker run -p 4317:4317 -p 4318:4318 otel/opentelemetry-collector:latest

# Run your application
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run main.go
```

## Dependencies

This adapter requires OpenTelemetry Go SDK v1.37.0+. Dependencies are only added when you import this package.