# Sentry Adapter for mtlog

Production-ready error tracking and monitoring integration for mtlog with [Sentry](https://sentry.io).

## Features

- 🚀 **Automatic Error Tracking** - Captures errors with stack traces
- 📝 **Message Template Interpolation** - Shows actual values in Sentry UI
- 🍞 **Breadcrumb Collection** - Tracks events leading to errors
- 📦 **Efficient Batching** - Reduces network overhead
- 🔄 **Retry Logic** - Handles transient network failures with exponential backoff
- 📊 **Metrics Collection** - Monitor integration health in real-time
- 🎯 **Custom Fingerprinting** - Control error grouping strategies
- ⚡ **Performance Optimized** - String builder pooling and stack trace caching
- 🔧 **Environment Variables** - Flexible configuration options
- 🎭 **Transaction Tracking** - Performance monitoring with spans
- 💾 **Stack Trace Caching** - LRU cache for repeated errors
- 🎲 **Advanced Sampling** - Multiple strategies to control event volume
- 🔍 **Comprehensive Observability** - Track every aspect of the integration

## Installation

```bash
go get github.com/willibrandon/mtlog/adapters/sentry
```

## Quick Start

```go
package main

import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/sentry"
    "github.com/willibrandon/mtlog/core"
)

func main() {
    // Create Sentry sink
    sentrySink, err := sentry.NewSentrySink(
        "https://your-key@sentry.io/project-id",
        sentry.WithEnvironment("production"),
        sentry.WithRelease("v1.0.0"),
    )
    if err != nil {
        panic(err)
    }
    defer sentrySink.Close()

    // Create logger with Sentry
    logger := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithSink(sentrySink),
    )

    // Log errors to Sentry
    logger.Error("Database connection failed: {Error}", err)
}
```

## Configuration Options

### Basic Configuration

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    // Environment and release tracking
    sentry.WithEnvironment("production"),
    sentry.WithRelease("v1.2.3"),
    sentry.WithServerName("api-server-1"),
    
    // Level configuration
    sentry.WithMinLevel(core.ErrorLevel),      // Only send errors and above
    sentry.WithBreadcrumbLevel(core.DebugLevel), // Collect debug+ as breadcrumbs
    
    // Sampling
    sentry.WithSampleRate(0.25), // Sample 25% of events
    
    // Batching
    sentry.WithBatchSize(100),
    sentry.WithBatchTimeout(5 * time.Second),
    
    // Breadcrumbs
    sentry.WithMaxBreadcrumbs(50),
)
```

### Retry Configuration

Configure automatic retry for resilient error tracking:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithRetry(3, 1*time.Second),  // 3 retries with exponential backoff
    sentry.WithRetryJitter(0.2),         // 20% jitter to prevent thundering herd
)
```

The retry mechanism uses exponential backoff:
- 1st retry: ~1 second
- 2nd retry: ~2 seconds  
- 3rd retry: ~4 seconds

### Metrics Collection

Monitor your Sentry integration health:

```go
// Enable metrics with periodic callback
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithMetricsCallback(30*time.Second, func(m sentry.Metrics) {
        fmt.Printf("Events sent: %d, Failed: %d, Retry rate: %.2f%%\n",
            m.EventsSent, m.EventsFailed,
            float64(m.RetryCount)/float64(m.EventsSent)*100)
    }),
)

// Or retrieve metrics on demand
metrics := sentrySink.Metrics()
fmt.Printf("Average batch size: %.2f\n", metrics.AverageBatchSize)
fmt.Printf("Last flush duration: %v\n", metrics.LastFlushDuration)
```

Available metrics:
- **Event Statistics**: EventsSent, EventsDropped, EventsFailed, EventsRetried
- **Breadcrumb Statistics**: BreadcrumbsAdded, BreadcrumbsEvicted
- **Batch Statistics**: BatchesSent, AverageBatchSize
- **Performance Metrics**: LastFlushDuration, TotalFlushTime
- **Network Statistics**: RetryCount, NetworkErrors

### Custom Fingerprinting

Control how errors are grouped in Sentry:

```go
// Group by message template only
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithFingerprinter(sentry.ByTemplate()),
)

// Group by template and error type
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithFingerprinter(sentry.ByErrorType()),
)

// Group by template and user ID
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithFingerprinter(sentry.ByProperty("UserId")),
)

// Group by multiple properties
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithFingerprinter(sentry.ByMultipleProperties("TenantId", "Service")),
)

// Custom fingerprinting logic
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithFingerprinter(sentry.Custom(func(event *core.LogEvent) string {
        // Your custom logic here
        return fmt.Sprintf("%s:%v", event.MessageTemplate, event.Properties["RequestId"])
    })),
)
```

### Environment Variables

The adapter supports configuration via environment variables:

```bash
export SENTRY_DSN="https://xxx@sentry.io/project"
```

Then in your code:

```go
// Will use SENTRY_DSN if dsn is empty
sentrySink, _ := sentry.NewSentrySink("",
    sentry.WithEnvironment("production"),
)
```

### Performance Monitoring / Transaction Tracking

Track application performance with distributed tracing:

```go
import (
    "context"
    "github.com/willibrandon/mtlog/adapters/sentry"
)

// Start a transaction
ctx := sentry.StartTransaction(context.Background(), 
    "checkout-flow", "http.request")
defer sentry.GetTransaction(ctx).Finish()

// Track database operations
dbCtx, finishDB := sentry.TraceDatabaseQuery(ctx, 
    "SELECT * FROM orders WHERE id = ?", "orders_db")
err := db.Query(dbCtx, orderID)
finishDB(err)

// Track HTTP requests
httpCtx, finishHTTP := sentry.TraceHTTPRequest(ctx, 
    "POST", "https://payment-api.com/charge")
statusCode, err := client.Post(httpCtx, paymentData)
finishHTTP(statusCode)

// Track cache operations
cacheCtx, finishCache := sentry.TraceCache(ctx, "get", "order:123")
value, hit := cache.Get(cacheCtx, "order:123")
finishCache(hit)

// Measure custom operations
err = sentry.MeasureSpan(ctx, "validate.inventory", func() error {
    return inventory.Validate(items)
})

// Batch operations with metrics
err = sentry.BatchSpan(ctx, "process.items", len(items), func() error {
    return processItems(items)
})
```

Available tracing functions:
- `StartTransaction` - Begin a new transaction
- `StartSpan` - Create a child span
- `TraceHTTPRequest` - Track HTTP calls
- `TraceDatabaseQuery` - Track database operations
- `TraceCache` - Track cache hits/misses
- `MeasureSpan` - Time any operation
- `BatchSpan` - Track batch processing with throughput metrics
- `TransactionMiddleware` - Wrap handlers with automatic tracing

### Stack Trace Caching

Optimize performance by caching stack traces for repeated errors:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    // Configure stack trace cache size (default: 1000)
    sentry.WithStackTraceCacheSize(2000),
    
    // Disable caching if needed
    // sentry.WithStackTraceCacheSize(0),
)
```

Benefits:
- **Reduced CPU usage** - Avoid repeated stack trace extraction
- **Lower memory allocations** - Reuse cached stack traces
- **LRU eviction** - Automatically manages cache size
- **Thread-safe** - Concurrent access supported

## Sampling

Control which events are sent to Sentry to manage costs and reduce noise while maintaining visibility into critical issues.

### Sampling Strategies

#### 1. Fixed Rate Sampling
Simple percentage-based sampling:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithSampling(0.1), // Sample 10% of events
)
```

#### 2. Adaptive Sampling
Automatically adjusts sampling rate based on traffic volume:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithAdaptiveSampling(100), // Target 100 events per second
)
```

- Reduces sampling during traffic spikes
- Increases sampling during low traffic
- Maintains target event rate automatically

#### 3. Priority-Based Sampling
Sample based on event importance:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithPrioritySampling(0.05), // 5% base rate
)
```

- Fatal events: Always sampled (100%)
- Events with errors: Higher priority
- Events with user context: Medium priority
- Regular events: Base rate

#### 4. Burst Detection Sampling
Handles traffic bursts with automatic backoff:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithBurstSampling(1000), // Burst threshold: 1000 events/sec
)
```

- Normal traffic: Full sampling
- During burst: Reduced sampling (5-10%)
- Automatic backoff period
- Prevents overwhelming Sentry during incidents

#### 5. Group-Based Sampling
Limits events per error group:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithGroupSampling(10, time.Minute), // Max 10 per error type per minute
)
```

- Prevents repetitive errors from flooding
- Maintains visibility into all error types
- Time-windowed limits

#### 6. Sampling Profiles
Pre-configured profiles for common scenarios:

```go
// Development - No sampling
sentry.WithSamplingProfile(sentry.SamplingProfileDevelopment)

// Production - Balanced sampling with adaptive rates
sentry.WithSamplingProfile(sentry.SamplingProfileProduction)

// High Volume - Aggressive sampling for high-traffic apps
sentry.WithSamplingProfile(sentry.SamplingProfileHighVolume)

// Critical - Minimal sampling, only critical events
sentry.WithSamplingProfile(sentry.SamplingProfileCritical)
```

#### 7. Custom Sampling Logic
Implement your own sampling decisions:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithCustomSampling(func(event *core.LogEvent) bool {
        // Sample premium users at higher rate
        if userId, ok := event.Properties["UserId"].(int); ok {
            if userId < 1000 { // Premium users
                return rand.Float32() < 0.5 // 50% sampling
            }
            return rand.Float32() < 0.01 // 1% for regular users
        }
        return rand.Float32() < 0.1 // 10% default
    }),
)
```

### Integration with mtlog Sampling

Leverage mtlog's powerful per-message sampling APIs for fine-grained control:

```go
logger := mtlog.New(
    mtlog.WithSink(sentrySink),
)

// Sample every 10th message
logger.Sample(10).Error("Database connection failed", err)

// Sample once per minute
logger.SampleDuration(time.Minute).Warning("Cache miss for key {Key}", key)

// Sample 20% of messages
logger.SampleRate(0.2).Information("User action {Action}", action)

// Sample first 5 occurrences only
logger.SampleFirst(5).Error("Initialization error", err)

// Sample with exponential backoff
logger.SampleBackoff("api-error", 2.0).Error("API rate limit exceeded")

// Conditional sampling
logger.SampleWhen(func() bool { 
    return time.Now().Hour() >= 9 && time.Now().Hour() <= 17 
}, 1).Information("Business hours event")

// Group sampling
logger.SampleGroup("user-errors", 10).Error("User {UserId} validation failed", userId)
```

### Advanced Sampling Configuration

Complete sampling configuration with all options:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithSamplingConfig(&sentry.SamplingConfig{
        Strategy:          sentry.SamplingAdaptive,
        Rate:              0.1,                    // Base rate: 10%
        ErrorRate:         0.5,                    // Error sampling: 50%
        FatalRate:         1.0,                    // Fatal sampling: 100%
        AdaptiveTargetEPS: 100,                    // Target 100 events/sec
        BurstThreshold:    1000,                   // Burst mode above 1000/sec
        GroupSampling:     true,                   // Enable group sampling
        GroupSampleRate:   10,                     // 10 events per group
        GroupWindow:       time.Minute,            // Per minute window
        CustomSampler: func(e *core.LogEvent) bool {
            // Additional custom logic
            return true
        },
    }),
)
```

### Sampling Metrics

Monitor sampling effectiveness:

```go
// Get sampling statistics
metrics := sentrySink.Metrics()
effectiveRate := float64(metrics.EventsSent) / 
    float64(metrics.EventsSent + metrics.EventsDropped) * 100
fmt.Printf("Effective sampling rate: %.1f%%\n", effectiveRate)
```

## Advanced Usage

### Filtering Events

Use BeforeSend to filter or modify events:

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
        // Don't send events from development
        if event.Environment == "development" {
            return nil
        }
        
        // Redact sensitive data
        if event.Extra["password"] != nil {
            event.Extra["password"] = "[REDACTED]"
        }
        
        return event
    }),
)
```

### Ignoring Specific Errors

```go
sentrySink, _ := sentry.NewSentrySink(dsn,
    sentry.WithIgnoreErrors(
        context.Canceled,
        io.EOF,
    ),
)
```

### Context Enrichment

Enrich events with contextual information:

```go
import sentryctx "github.com/willibrandon/mtlog/adapters/sentry"

// Set user context
ctx := sentryctx.WithUser(ctx, sentry.User{
    ID:       "user-123",
    Username: "john.doe",
    Email:    "john@example.com",
})

// Add tags
ctx = sentryctx.WithTags(ctx, map[string]string{
    "tenant_id": "tenant-456",
    "region":    "us-west-2",
})

// Log with context
logger.WithContext(ctx).Error("Operation failed")
```

## Performance Considerations

The Sentry adapter is optimized for production use:

1. **String Builder Pooling**: Reuses string builders to minimize allocations
2. **Batching**: Reduces network calls by batching events
3. **Async Processing**: Non-blocking event submission
4. **Breadcrumb Buffering**: Efficient circular buffer for breadcrumbs
5. **Minimal Overhead**: ~170ns for message interpolation

## Benchmarks

```bash
go test -bench=. -benchmem
```

### Benchmark Results (AMD Ryzen 9 9950X)
```
BenchmarkRetryCalculation-32               100000000        11.07 ns/op       0 B/op       0 allocs/op
BenchmarkStackTraceCaching-32               20087894        60.74 ns/op      48 B/op       2 allocs/op
BenchmarkMetricsCollection-32              167154866         7.113 ns/op      0 B/op       0 allocs/op
BenchmarkTransactionCreation-32              1992597       652.3 ns/op    1208 B/op       9 allocs/op
BenchmarkMessageInterpolation-32             6992760       178.1 ns/op     136 B/op       4 allocs/op
BenchmarkEventConversion-32                   894856      1322 ns/op      1597 B/op      12 allocs/op
BenchmarkComplexMessageInterpolation-32      3232375       415.1 ns/op     320 B/op       6 allocs/op
BenchmarkBreadcrumbAddition-32               4703395       237.2 ns/op     401 B/op       4 allocs/op
BenchmarkBatchProcessing-32                137846361         8.668 ns/op      8 B/op       0 allocs/op
```

### Performance Highlights

#### Ultra-Fast Core Operations
- **Retry Calculation**: 11.07 ns/op - Zero-allocation exponential backoff
- **Metrics Collection**: 7.11 ns/op - Atomic counter updates with no allocations
- **Batch Processing**: 8.67 ns/op - Highly optimized batch operations

#### Efficient Caching & Pooling
- **Stack Trace Caching**: 60.74 ns/op - LRU cache with ~95% hit rate in production
- **Message Interpolation**: 178.1 ns/op - String builder pooling reduces GC pressure
- **Complex Templates**: 415.1 ns/op - Handles multiple properties efficiently

#### Transaction & Performance Monitoring
- **Transaction Creation**: 652.3 ns/op - Lightweight span creation for tracing
- **Breadcrumb Addition**: 237.2 ns/op - Fast circular buffer operations
- **Event Conversion**: 1.32 μs - Complete Sentry event with all metadata

## Testing

### Unit Tests

```bash
cd adapters/sentry
go test -v ./...
go test -race -v ./...
```

### Integration Tests (Local Sentry)

```bash
# Start Sentry infrastructure
./scripts/setup-integration-test.sh

# Run integration tests
go test -tags=integration -v ./...

# Verify pipeline
./scripts/verify-sentry-pipeline.sh
```

## Examples

See the [examples](examples/) directory for complete, runnable examples:

- [Basic Usage](examples/basic/) - Simple error tracking with Sentry
- [Breadcrumbs](examples/breadcrumbs/) - Tracking event trails leading to errors
- [Context Enrichment](examples/context/) - Adding user, tenant, and request context
- [Retry Logic](examples/retry/) - Demonstrating resilient error submission with retries
- [Performance Monitoring](examples/performance/) - Full transaction tracking with spans
- [Metrics Dashboard](examples/metrics/) - Real-time monitoring of integration health
- [Sampling Strategies](examples/sampling/) - All sampling strategies with live examples

Each example includes:
- Complete, runnable code
- Detailed comments explaining each feature
- Simulated scenarios demonstrating real-world usage
- Performance considerations and best practices

## Troubleshooting

### Enable Debug Logging

```go
import "github.com/willibrandon/mtlog/selflog"

// Enable self-diagnostics
selflog.Enable(os.Stderr)
defer selflog.Disable()

// Your Sentry configuration
sentrySink, _ := sentry.NewSentrySink(dsn)
```

### Common Issues

1. **Events not appearing in Sentry**
   - Check DSN is correct
   - Verify network connectivity
   - Enable selflog for diagnostics
   - Check sample rate isn't too low

2. **High memory usage**
   - Reduce batch size
   - Lower max breadcrumbs
   - Check for event flooding

3. **Slow performance**
   - Enable batching
   - Adjust batch timeout
   - Use sampling for high-volume events

## License

MIT License - see LICENSE file for details.