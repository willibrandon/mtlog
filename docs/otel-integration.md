# OpenTelemetry Integration

mtlog provides comprehensive OpenTelemetry (OTEL) integration, enabling distributed tracing correlation, OTLP export, and compatibility with the OTEL logging ecosystem.

## Features

### 1. Automatic Trace Context Extraction

mtlog can automatically extract and include OpenTelemetry trace context in all log events:

- `trace.id` - W3C trace ID (32 hex characters)
- `span.id` - Span ID (16 hex characters)  
- `trace.flags` - Trace flags (sampled/not sampled)

### 2. OTLP Export

Export logs directly to any OpenTelemetry collector via:
- OTLP/gRPC (port 4317)
- OTLP/HTTP (port 4318)

### 3. Bridge Adapters

- Use mtlog as an OTEL `LoggerProvider`
- Use OTEL loggers as mtlog sinks

### 4. Zero-Cost When Disabled

OTEL features are behind a build tag (`-tags=otel`) ensuring:
- Zero dependencies when not needed
- Zero runtime overhead when disabled
- <5ns overhead when enabled but no span present

## Installation

### Basic Installation (without OTEL)

```bash
go get github.com/willibrandon/mtlog
```

### With OTEL Support

```bash
# Add OTEL dependencies
./scripts/add-otel-deps.sh

# Or manually
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/trace
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc
# ... (see scripts/add-otel-deps.sh for full list)
```

## Building

### Without OTEL (default)

```bash
go build ./...
```

### With OTEL Support

```bash
go build -tags=otel ./...
```

## Usage

### Basic Trace Context Enrichment

```go
import (
    "context"
    "github.com/willibrandon/mtlog"
    "go.opentelemetry.io/otel"
)

func main() {
    // Start a span
    tracer := otel.Tracer("my-service")
    ctx, span := tracer.Start(context.Background(), "operation")
    defer span.End()
    
    // Create logger with OTEL enrichment
    logger := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithOTELEnricher(ctx), // Automatically adds trace.id, span.id
    )
    
    // All logs will include trace context
    logger.Information("Processing request {RequestId}", "req-123")
    // Output: 2025-01-29 10:15:23 [INF] Processing request req-123 (trace.id: abc123..., span.id: def456...)
}
```

### Request-Scoped Logging

For HTTP handlers or request processing, use the static enricher for better performance:

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from headers
    ctx := otel.GetTextMapPropagator().Extract(
        r.Context(), 
        propagation.HeaderCarrier(r.Header),
    )
    
    // Start a span
    ctx, span := tracer.Start(ctx, "handle-request")
    defer span.End()
    
    // Create request-scoped logger with static enricher
    // Static enricher extracts trace IDs once and reuses them
    logger := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithStaticOTELEnricher(ctx), // More efficient for request scope
        mtlog.WithProperty("request.id", generateRequestID()),
    )
    
    // Use logger throughout request
    logger.Information("Request started")
    processRequest(ctx, logger)
    logger.Information("Request completed")
}
```

### OTLP Export

#### Using Environment Variables (Recommended)

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_HEADERS=api-key=secret
export OTEL_EXPORTER_OTLP_COMPRESSION=gzip
```

```go
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithOTLP(), // Uses environment variables
)
```

#### Explicit Configuration

```go
// gRPC transport
logger := mtlog.New(
    mtlog.WithOTLPGRPC("localhost:4317"),
)

// HTTP transport
logger := mtlog.New(
    mtlog.WithOTLPHTTP("http://localhost:4318/v1/logs"),
)

// Full configuration
config := mtlog.NewOTLPConfig().
    WithEndpoint("localhost:4317").
    WithGRPC().
    WithCompression("gzip").
    WithBatching(100, 5*time.Second).
    WithRetry(1*time.Second, 30*time.Second).
    WithHeaders(map[string]string{
        "api-key": "your-api-key",
    })

logger := mtlog.New(
    mtlog.WithConsole(),
    config.Build(),
)
```

### OTEL Bridge

Use mtlog as an OTEL logger provider:

```go
// Create mtlog logger
mtlogger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithMinimumLevel(core.DebugLevel),
)

// Create OTEL provider backed by mtlog
provider := mtlog.WithOTELBridge(mtlogger)
provider.SetAsGlobal()

// Now OTEL-instrumented libraries will use mtlog
otelLogger := provider.Logger("my-scope")
```

## Performance

### Enricher Performance

Three enricher variants optimize for different use cases:

| Enricher | No Span | With Span | Use Case |
|----------|---------|-----------|----------|
| FastOTELEnricher | <5ns | ~50ns | Default, changing contexts |
| StaticOTELEnricher | 0ns | ~10ns | Request-scoped, fixed context |
| OTELEnricher | <5ns | ~30ns (cached) | Long-lived loggers |

### Benchmark Results

```
BenchmarkFastOTELEnricher_NoSpan-10         500000000    3.82 ns/op    0 B/op    0 allocs/op
BenchmarkFastOTELEnricher_WithSpan-10        20000000   52.3 ns/op    48 B/op    3 allocs/op
BenchmarkStaticOTELEnricher_WithSpan-10     100000000   10.2 ns/op     0 B/op    0 allocs/op
BenchmarkStaticOTELEnricher_NoSpan-10      1000000000    0.31 ns/op    0 B/op    0 allocs/op
```

## Configuration

### Environment Variables

Standard OTEL environment variables are supported:

- `OTEL_EXPORTER_OTLP_ENDPOINT` - Collector endpoint
- `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` - Logs-specific endpoint
- `OTEL_EXPORTER_OTLP_HEADERS` - Headers (comma-separated key=value)
- `OTEL_EXPORTER_OTLP_COMPRESSION` - Compression (gzip, none)
- `OTEL_EXPORTER_OTLP_TIMEOUT` - Timeout in milliseconds

### Semantic Conventions

mtlog follows OTEL semantic conventions for property names:

```go
// Standard OTEL properties
logger.Information("Request processed", 
    "trace.id", traceID,
    "span.id", spanID,
    "service.name", "api-gateway",
    "service.version", "1.2.3",
    "http.method", "GET",
    "http.status_code", 200,
    "http.url", "/api/users",
    "db.system", "postgresql",
    "db.name", "users",
)
```

## Testing

### Run Tests with OTEL

```bash
# Run all tests with OTEL support
go test -tags=otel ./...

# Run OTEL-specific tests
go test -tags=otel ./internal/enrichers
go test -tags=otel ./sinks
go test -tags=otel ./otel

# Run benchmarks
go test -tags=otel -bench=OTEL -benchmem ./internal/enrichers
```

### Integration Testing

Start a local OTEL collector for integration testing:

```bash
# Using Jaeger (includes OTLP receiver)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Run integration example
go run -tags=otel ./examples/otel_integration
```

## Troubleshooting

### Enable Self-Logging

```go
import "github.com/willibrandon/mtlog/selflog"

selflog.Enable(os.Stderr)
defer selflog.Disable()
```

Or via environment:

```bash
MTLOG_SELFLOG=stderr go run -tags=otel main.go
```

### Common Issues

1. **"OTLP sink requires building with -tags=otel"**
   - Build with: `go build -tags=otel`

2. **Missing dependencies**
   - Run: `./scripts/add-otel-deps.sh`

3. **Connection refused to collector**
   - Ensure collector is running on correct ports
   - Check firewall settings

4. **No trace IDs in logs**
   - Ensure span is active in context
   - Use `WithOTELEnricher(ctx)` not `WithOTELEnricher(context.Background())`

## Examples

See `/examples/otel_integration/` for complete working examples:

- Basic trace enrichment
- OTLP export configuration
- Distributed tracing
- Request-scoped logging
- Performance comparisons

## Design Principles

1. **Zero-cost abstraction**: No overhead when not used
2. **Build-time selection**: OTEL features only included when needed
3. **Performance first**: Multiple enricher variants for different use cases
4. **Standards compliant**: Follows OTEL specifications and conventions
5. **Production ready**: Batching, retry, compression, and proper error handling