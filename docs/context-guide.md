# Context Guide

mtlog provides comprehensive context support including context-aware logging methods, scoped properties via LogContext, and automatic deadline detection.

## Context-Aware Logging

mtlog provides context-aware variants of all logging methods that accept a context.Context parameter:

```go
// Standard methods
logger.Info("User logged in")

// Context-aware methods  
logger.InfoContext(ctx, "User logged in")
logger.ErrorContext(ctx, "Operation failed: {Error}", err)
```

All levels supported: `VerboseContext`, `DebugContext`, `InfoContext`, `WarnContext`, `ErrorContext`, `FatalContext`

## LogContext - Scoped Properties

Attach properties to a context that automatically flow to all loggers:

```go
import "github.com/willibrandon/mtlog"

// Add properties to context
ctx = mtlog.PushProperty(ctx, "RequestId", "abc-123")
ctx = mtlog.PushProperty(ctx, "UserId", 456)

// All logs using this context include these properties
logger.InfoContext(ctx, "Processing request")
// Output includes: RequestId=abc-123, UserId=456
```

### Property Inheritance

Properties flow through nested contexts:

```go
func handleRequest(ctx context.Context) {
    ctx = mtlog.PushProperty(ctx, "RequestId", generateRequestId())
    processUser(ctx, userId)
}

func processUser(ctx context.Context, userId int) {
    ctx = mtlog.PushProperty(ctx, "UserId", userId)
    // Logs include both RequestId and UserId
    logger.InfoContext(ctx, "Processing user")
}
```

### Property Precedence

Higher priority overrides lower:

1. LogContext properties (lowest)
2. ForContext properties 
3. Event properties (highest)

```go
ctx = mtlog.PushProperty(ctx, "UserId", 123)
logger.ForContext("UserId", 456).InfoContext(ctx, "Test") // UserId=456
logger.InfoContext(ctx, "User {UserId}", 789) // UserId=789
```

## Context Deadline Awareness

Automatically detect and warn when operations approach context deadlines:

### Basic Configuration

```go
// Warn when within 100ms of deadline
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithContextDeadlineWarning(100*time.Millisecond),
)

ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

logger.InfoContext(ctx, "Starting operation")
time.Sleep(350 * time.Millisecond)
logger.InfoContext(ctx, "Still processing...") // WARNING: Deadline approaching!
```

### Percentage-Based Thresholds

```go
// Warn when 20% of time remains (clean API)
logger := mtlog.New(
    mtlog.WithDeadlinePercentageOnly(0.2),  // 20% threshold
)

// Or with both absolute and percentage thresholds
logger := mtlog.New(
    mtlog.WithDeadlinePercentageThreshold(
        10*time.Millisecond, // Min absolute threshold
        0.2,                 // 20% threshold
    ),
)
```

**Note**: Percentage-based thresholds require the context to be logged early in its lifetime for accurate calculations. If a context is first seen when already near its deadline, percentage calculations may be less accurate.

### Properties Added

When approaching deadline:
```json
{
    "deadline.approaching": true,
    "deadline.remaining_ms": 95,
    "deadline.at": "2024-01-15T10:30:45Z",
    "deadline.first_warning": true
}
```

When deadline exceeded:
```json
{
    "deadline.exceeded": true,
    "deadline.exceeded_by_ms": 150
}
```

### Advanced Options

```go
import "github.com/willibrandon/mtlog/internal/enrichers"

logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(50*time.Millisecond,
        // Custom handler for deadline events
        enrichers.WithDeadlineCustomHandler(func(event *core.LogEvent, remaining time.Duration) {
            metrics.RecordDeadlineApproaching(remaining)
            event.Properties["alert.team"] = "platform"
        }),
        
        // Configure cache
        enrichers.WithDeadlineCacheSize(1000),
        enrichers.WithDeadlineCacheTTL(5*time.Minute),
    ),
)
```

## HTTP Handler Example

```go
func timeoutMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add timeout to all requests
        ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
        defer cancel()
        
        // Add request properties
        ctx = mtlog.PushProperty(ctx, "request_id", generateRequestID())
        ctx = mtlog.PushProperty(ctx, "method", r.Method)
        ctx = mtlog.PushProperty(ctx, "path", r.URL.Path)
        
        logger.InfoContext(ctx, "Request started")
        
        next.ServeHTTP(w, r.WithContext(ctx))
        
        logger.InfoContext(ctx, "Request completed")
    })
}
```

## Performance

- **Context methods**: ~2ns overhead vs standard methods
- **LogContext**: No overhead when not used, efficient for <10 properties
- **Deadline awareness**: 2.7ns when no deadline, ~5ns with deadline check
- **Cache**: O(1) lookup, bounded memory (default 1000 contexts)

## Best Practices

1. **Use context methods when you have a context**
```go
// Good
func process(ctx context.Context) {
    logger.InfoContext(ctx, "Processing")
}

// Avoid
func process(ctx context.Context) {
    logger.Info("Processing") // Missing context benefits
}
```

2. **Configure deadline awareness at startup**
```go
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(100*time.Millisecond),
    // Other options...
)
```

3. **Use LogContext for cross-cutting concerns**
```go
// Add once at request boundary
ctx = mtlog.PushProperty(ctx, "request_id", requestID)
ctx = mtlog.PushProperty(ctx, "tenant_id", tenantID)

// Properties flow through all operations
```

4. **Set appropriate thresholds**
```go
// Fast APIs (100ms target)
mtlog.WithContextDeadlineWarning(20*time.Millisecond)

// Batch jobs (5 minute target)  
mtlog.WithContextDeadlineWarning(30*time.Second)

// Mixed workloads
mtlog.WithDeadlinePercentageThreshold(10*time.Millisecond, 0.1)
```

## Troubleshooting

### Percentage thresholds not working?

If percentage-based thresholds aren't triggering as expected:

1. **Ensure early context logging** - The context must be seen early in its lifetime for accurate percentage calculation:
```go
// Good - log immediately after creating context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
logger.InfoContext(ctx, "Starting operation") // Prime the cache

// Bad - first log near deadline
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
time.Sleep(4*time.Second)
logger.InfoContext(ctx, "Almost done") // Too late for percentage calculation
```

2. **Check cache statistics**:
```go
if stats := logger.DeadlineStats(); stats != nil {
    s := stats.(core.DeadlineStats)
    fmt.Printf("Cache hit rate: %.2f%%\n", 
        float64(s.CacheSize)/float64(s.CacheCapacity)*100)
}
```

### Too many/few deadline warnings?

Adjust thresholds based on your operation SLAs:

```go
// Fast APIs (100ms SLA) - warn at 20ms
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(20*time.Millisecond),
)

// Medium operations (1s SLA) - warn at 20% remaining  
logger := mtlog.New(
    mtlog.WithDeadlinePercentageOnly(0.2),
)

// Long batch jobs (5min SLA) - warn at 30s
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(30*time.Second),
)
```

### Debugging deadline misses

Enable deadline metrics to track patterns:
```go
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(50*time.Millisecond,
        enrichers.WithDeadlineMetrics(true), // Logs to selflog
    ),
)
```

## SLA-Based Configuration Guide

Different types of operations require different deadline warning strategies:

### Fast APIs (< 100ms SLA)
```go
// Warn at 20% of deadline or 20ms, whichever comes first
logger := mtlog.New(
    mtlog.WithDeadlinePercentageThreshold(20*time.Millisecond, 0.2),
)
```

### Standard Web APIs (100ms - 1s SLA)
```go
// Percentage-based for better scaling
logger := mtlog.New(
    mtlog.WithDeadlinePercentageOnly(0.15), // Warn at 15% remaining
)
```

### Database Operations (1s - 5s SLA)
```go
// Absolute threshold for predictable warnings
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(500*time.Millisecond),
)
```

### Batch Jobs (> 1min SLA)
```go
// Large absolute threshold
logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(30*time.Second),
)
```

### Mixed Workloads
```go
// Use ForContext and WithDeadlineWarning to create specialized loggers
baseLogger := mtlog.New(mtlog.WithConsole())

apiLogger := baseLogger.
    ForContext("component", "api").
    WithDeadlineWarning(50*time.Millisecond)

batchLogger := baseLogger.
    ForContext("component", "batch").
    WithDeadlineWarning(30*time.Second)

// Each logger has its own deadline configuration
// API requests warn at 50ms, batch jobs warn at 30s
```

## Examples

- [Context logging](../examples/context/main.go)
- [LogContext properties](../examples/logcontext/main.go)
- [Deadline awareness](../examples/deadline-awareness/main.go)