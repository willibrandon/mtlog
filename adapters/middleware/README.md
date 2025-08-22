# mtlog HTTP Middleware

HTTP request/response logging middleware for Go web frameworks using mtlog structured logging.

## Features

- 🚀 Support for multiple frameworks: net/http, Gin, Echo, Fiber, Chi
- 🔍 Automatic request ID generation and propagation
- ⏱️ Request latency tracking with configurable units
- 📊 Customizable log levels based on HTTP status codes
- 🎯 Selective path skipping (e.g., health checks)
- 🔗 Context injection for nested logging
- 📝 Configurable request field logging
- 🛡️ Request/response body logging with sanitization
- 📈 Advanced sampling strategies (rate, adaptive, path-based)
- 🎛️ Custom field extractors for dynamic context
- 💥 Panic recovery with stack traces
- 📊 Metrics integration (Prometheus/OpenTelemetry ready)
- 🔧 Request logger helper for fluent API
- ✅ Options validation for fail-fast configuration

## Installation

```bash
go get github.com/willibrandon/mtlog/adapters/middleware
```

## Quick Start

### net/http

```go
import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())
mw := middleware.Middleware(middleware.DefaultOptions(logger))

handler := mw(yourHandler)
http.ListenAndServe(":8080", handler)
```

### Gin

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())
router := gin.New()
router.Use(middleware.Gin(logger))
```

### Echo

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())
e := echo.New()
e.Use(middleware.Echo(logger))
```

### Fiber

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())
app := fiber.New()
app.Use(middleware.Fiber(logger))
```

### Chi

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())
r := chi.NewRouter()
r.Use(middleware.Chi(logger))
```

## Performance Characteristics

The middleware is highly optimized for production use with minimal overhead:

- **Skip paths**: ~98ns, 4 allocations (near-zero overhead)
- **Sampled out**: ~333ns, 37 allocations (when request is not logged)
- **Full logging**: ~2.4μs, 56 allocations (complete request logging)
- **Memory per request**: ~4.4KB average
- **Raw handler baseline**: ~102ns (for comparison)

The middleware adds approximately 2.3μs of overhead to each logged request, which is negligible for most HTTP services. Skip paths and sampling can further reduce this overhead for high-traffic endpoints.

### Allocation Breakdown
- ResponseWriter wrapper: 6 allocations
- UUID generation: 2 allocations
- Context operations: 4 allocations
- Logger.With() calls: ~8 allocations per field
- Template args slice: 1 allocation

## Configuration Options

```go
options := &middleware.Options{
    Logger:            logger,           // mtlog logger instance
    GenerateRequestID: true,             // Auto-generate request IDs
    RequestIDHeader:   "X-Request-ID",   // Header for request ID
    SkipPaths:         []string{         // Paths to skip logging
        "/health",
        "/metrics",
    },
    RequestFields: []string{             // Fields to include in logs
        "method",
        "path", 
        "ip",
        "user_agent",
        "referer",
        "proto",
        "host",
    },
    LatencyField: "duration_ms",         // Field name for latency
    LatencyUnit:  "ms",                  // Unit: ms, us, ns, s
    CustomLevelFunc: func(statusCode int) core.LogEventLevel {
        switch {
        case statusCode >= 500:
            return core.ErrorLevel
        case statusCode >= 400:
            return core.WarningLevel
        default:
            return core.InformationLevel
        }
    },
    
    // Advanced features
    LogRequestBody:  true,                // Log request bodies
    LogResponseBody: true,                // Log response bodies
    MaxBodySize:     4096,                // Max body size to log
    BodySanitizer:   middleware.DefaultBodySanitizer, // Sanitize sensitive fields
    
    // Sampling
    Sampler: middleware.NewPathSamplerBuilder().
        Never("/health").
        Never("/metrics").
        Sometimes("/api/status", 0.1).
        Always("*").
        Build(),
    
    // Custom field extraction
    CustomFields: []middleware.FieldExtractor{
        middleware.UserIDFromHeader,
        middleware.TraceIDFromContext,
        middleware.TenantIDFromSubdomain,
    },
    
    // Metrics recording
    MetricsRecorder: myMetricsRecorder,
    
    // Panic handling
    PanicHandler: func(w http.ResponseWriter, r *http.Request, err any) {
        // Custom panic response
    },
}
```

## Advanced Features

### Body Logging and Sanitization

Log request and response bodies with automatic sanitization of sensitive fields:

```go
options := &middleware.Options{
    LogRequestBody:  true,
    LogResponseBody: true,
    MaxBodySize:     2048,
    BodySanitizer:   middleware.DefaultBodySanitizer, // Redacts passwords, tokens, etc.
}
```

The default sanitizer automatically redacts common sensitive fields in JSON payloads. You can also create custom sanitizers:

```go
options.BodySanitizer = middleware.RegexBodySanitizer(
    regexp.MustCompile(`"credit_card":\s*"[^"]+"`),
    regexp.MustCompile(`"ssn":\s*"[^"]+"`),
)
```

### Sampling Strategies

Control which requests get logged to manage log volume:

```go
// Rate-based sampling (log 10% of requests)
options.Sampler = middleware.NewRateSampler(0.1)

// Adaptive sampling (target 100 logs per second)
options.Sampler = middleware.NewAdaptiveSampler(100)

// Path-based sampling with glob patterns
options.Sampler = middleware.NewPathSamplerBuilder().
    CaseInsensitive().
    Never("/health*").           // Never log health checks
    Sometimes("/api/status", 0.1). // Log 10% of status checks
    Always("/api/*/debug").       // Always log debug endpoints
    Sometimes("*", 0.5).          // Log 50% of everything else
    Build()

// Composite sampling (AND/OR logic)
options.Sampler = middleware.NewCompositeSampler(
    middleware.CompositeAND,
    middleware.NewRateSampler(0.5),
    middleware.NewPathSampler(rules),
)
```

### Custom Field Extractors

Extract dynamic fields from requests:

```go
options.CustomFields = []middleware.FieldExtractor{
    // Pre-defined extractors
    middleware.UserIDFromHeader,
    middleware.SessionIDFromCookie,
    middleware.TraceIDFromContext,
    middleware.TenantIDFromSubdomain,
    middleware.APIVersionFromPath,
    middleware.GeoLocationFromHeaders,
    middleware.DeviceTypeFromUserAgent,
    
    // Custom extractor
    {
        Name: "AccountId",
        Extract: func(r *http.Request) any {
            // Your custom logic
            return r.Header.Get("X-Account-ID")
        },
    },
}
```

### Request Logger Helper

Use the fluent API for request-scoped logging:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    logger := middleware.GetRequestLogger(r).
        WithUser("user-123").
        WithOperation("CreateOrder").
        WithResource("Order", "ord-456")
    
    logger.Information("Processing order creation")
    
    if err := processOrder(); err != nil {
        logger.WithError(err).Error("Order creation failed")
    }
}
```

### Context Helpers

Simplified logging with context:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Simple logging
    middleware.InfoContext(ctx, "Processing request")
    middleware.ErrorContext(ctx, "Failed to process: {Error}", err)
    
    // Add fields to context logger
    ctx = middleware.WithFieldsContext(ctx, map[string]any{
        "UserId": "user-123",
        "Action": "UpdateProfile",
    })
    
    middleware.InfoContext(ctx, "User action completed")
}
```

### Metrics Integration

Record HTTP metrics for monitoring:

```go
// Implement the MetricsRecorder interface
type MyMetrics struct {
    // Your metrics implementation
}

func (m *MyMetrics) RecordRequest(method, path string, statusCode int, duration time.Duration) {
    // Record to Prometheus, StatsD, etc.
}

options.MetricsRecorder = &MyMetrics{}
```

The middleware provides a `SimpleMetricsRecorder` for testing and basic in-memory metrics.

## Performance Optimization with Object Pooling

The middleware includes object pooling to reduce allocations in high-throughput scenarios. Object pools are enabled by default but can be controlled via the `EnablePooling` variable.

### Pooled Objects

The following objects are automatically pooled:

- **MiddlewareError**: Structured error objects
- **responseWriter**: HTTP response wrapper structs  
- **RequestMetric**: Metrics recording structs
- **bytes.Buffer**: Buffers used for body capture

### Pooling Configuration

```go
import "github.com/willibrandon/mtlog/adapters/middleware"

// Enable/disable pooling globally (default: true)
middleware.EnablePooling = true

// Get pool statistics
stats := middleware.GetPoolStats()
fmt.Printf("Error pool hits: %d\n", stats.ErrorPoolHits)

// Reset pool statistics
middleware.ResetPoolStats()
```

### Performance Benefits

Benchmarks show significant allocation reduction with pooling enabled:

```
BenchmarkMiddleware/WithoutPooling-8    1000000   1523 ns/op   512 B/op   8 allocs/op
BenchmarkMiddleware/WithPooling-8       2000000    758 ns/op   128 B/op   2 allocs/op
```

### Best Practices for High-Throughput

1. **Keep pooling enabled** for production workloads
2. **Use batch metrics recording** for high request volumes:
   ```go
   batchRecorder := middleware.NewBatchMetricsRecorder(
       func(metrics []middleware.RequestMetric) {
           // Flush to your metrics backend
       },
       5*time.Second, // Flush interval
       1000,          // Batch size
   )
   defer batchRecorder.Close()
   
   options.MetricsRecorder = batchRecorder
   ```

3. **Configure appropriate sampling** for verbose endpoints:
   ```go
   sampler := middleware.NewDynamicPathSampler([]middleware.PathSamplingRule{
       {Pattern: "/health", Rate: 0.0},     // Skip health checks
       {Pattern: "/metrics", Rate: 0.0},    // Skip metrics
       {Pattern: "/api/v1/*", Rate: 1.0},   // Log all API calls
       {Pattern: "*", Rate: 0.1},           // Sample 10% of others
   })
   options.Sampler = sampler
   ```

## Context Integration

The middleware injects the logger and request ID into the request context, allowing for nested logging within handlers:

### net/http & Chi

```go
func handler(w http.ResponseWriter, r *http.Request) {
    logger := middleware.FromContext(r.Context())
    requestID := middleware.RequestIDFromContext(r.Context())
    
    if logger != nil {
        logger.Information("Processing request", "RequestId", requestID)
    }
}
```

### Gin

```go
func handler(c *gin.Context) {
    logger := middleware.LoggerFromGinContext(c)
    requestID := middleware.RequestIDFromGinContext(c)
    
    if logger != nil {
        logger.Information("Processing request", "RequestId", requestID)
    }
}
```

### Echo

```go
func handler(c echo.Context) error {
    logger := middleware.LoggerFromEchoContext(c)
    requestID := middleware.RequestIDFromEchoContext(c)
    
    if logger != nil {
        logger.Information("Processing request", "RequestId", requestID)
    }
    return nil
}
```

### Fiber

```go
func handler(c *fiber.Ctx) error {
    logger := middleware.LoggerFromFiberContext(c)
    requestID := middleware.RequestIDFromFiberContext(c)
    
    if logger != nil {
        logger.Information("Processing request", "RequestId", requestID)
    }
    return nil
}
```

## Log Output Example

```
[2025-01-21 10:15:23] INF HTTP GET /api/users responded 200 in 15ms Method=GET Path=/api/users StatusCode=200 duration_ms=15 Size=256 IP=192.168.1.1 RequestId=550e8400-e29b-41d4-a716-446655440000
[2025-01-21 10:15:24] WRN HTTP POST /api/users responded 400 in 5ms Method=POST Path=/api/users StatusCode=400 duration_ms=5 Size=45 Error="invalid email format"
[2025-01-21 10:15:25] ERR HTTP GET /error responded 500 in 2ms Method=GET Path=/error StatusCode=500 duration_ms=2 Size=21 Error="database connection failed"
```

## Running Examples

The package includes complete examples for each supported framework:

```bash
# net/http example
go run examples/middleware/nethttp/main.go

# Gin example
go run examples/middleware/gin/main.go

# Echo example
go run examples/middleware/echo/main.go

# Fiber example
go run examples/middleware/fiber/main.go

# Chi example
go run examples/middleware/chi/main.go
```

Each example runs on port 8080 and includes:
- `/` - Home endpoint
- `/api/users` - List users (GET)
- `/api/users` - Create user (POST)
- `/api/users/{id}` - Get/Update/Delete user
- `/error` - Simulated error endpoint
- `/health` - Health check (skipped by middleware)

## Testing

Run the test suite:

```bash
go test -v ./adapters/middleware/...
```

## License

MIT License - See the main mtlog repository for details.