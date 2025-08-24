# Sinks Guide

This guide covers all available sinks in mtlog and how to configure them.

## Console Sink

The console sink outputs log events to stdout/stderr with optional theming.

### Basic Usage

```go
// Plain console output
log := mtlog.New(mtlog.WithConsole())

// Console with properties displayed
log := mtlog.New(mtlog.WithConsoleProperties())
```

### Themes

mtlog supports multiple console themes for better readability:

```go
// Dark theme (default) - optimized for dark terminals
log := mtlog.New(mtlog.WithConsoleTheme("dark"))

// Light theme - optimized for light terminals  
log := mtlog.New(mtlog.WithConsoleTheme("light"))

// ANSI colors - standard ANSI color codes
log := mtlog.New(mtlog.WithConsoleTheme("ansi"))
```

#### Theme Colors

- **Dark Theme**: Uses bright colors suitable for dark backgrounds
- **Light Theme**: Uses darker colors suitable for light backgrounds
- **ANSI Theme**: Uses standard ANSI color codes for maximum compatibility

## File Sinks

### Simple File Sink

```go
log := mtlog.New(mtlog.WithFileSink("app.log"))
```

### Rolling File Sink

Automatically rotate log files based on size or time.

#### Size-based Rolling

```go
// Roll when file reaches 10MB
log := mtlog.New(mtlog.WithRollingFile("app.log", 10*1024*1024))

// With custom configuration
log := mtlog.New(
    mtlog.WithRollingFileAdvanced("logs/app.log",
        sinks.WithMaxFileSize(50*1024*1024),    // 50MB
        sinks.WithMaxFiles(10),                  // Keep 10 files
        sinks.WithCompressionLevel(6),           // Compress old files
    ),
)
```

#### Time-based Rolling

```go
// Roll every hour
log := mtlog.New(mtlog.WithRollingFileTime("app.log", time.Hour))

// Roll daily at midnight
log := mtlog.New(mtlog.WithRollingFileTime("app.log", 24*time.Hour))
```

## Seq Integration

Seq is a centralized structured logging server with powerful querying capabilities.

### Basic Configuration

```go
// Without authentication
log := mtlog.New(mtlog.WithSeq("http://localhost:5341"))

// With API key
log := mtlog.New(mtlog.WithSeqAPIKey("http://localhost:5341", "your-api-key"))
```

### Advanced Configuration

```go
log := mtlog.New(
    mtlog.WithSeqAdvanced("http://localhost:5341",
        sinks.WithSeqAPIKey("your-api-key"),
        sinks.WithSeqBatchSize(100),             // Batch 100 events
        sinks.WithSeqBatchTimeout(5*time.Second), // Or every 5 seconds
        sinks.WithSeqCompression(true),          // Enable gzip compression
        sinks.WithSeqRetry(3, time.Second),      // 3 retries with 1s delay
    ),
)
```

### CLEF Format

mtlog automatically formats events using the Compact Log Event Format (CLEF) for Seq:

```json
{
  "@t": "2025-01-22T10:30:45.123Z",
  "@mt": "User {UserId} logged in from {IPAddress}",
  "@l": "Information",
  "UserId": 123,
  "IPAddress": "192.168.1.100"
}
```

### Dynamic Level Control

Synchronize log levels with Seq server configuration:

```go
options := mtlog.SeqLevelControllerOptions{
    CheckInterval: 30 * time.Second,
    InitialCheck:  true,
}

loggerOption, levelSwitch, controller := mtlog.WithSeqLevelControl(
    "http://localhost:5341", options)
defer controller.Close()

logger := mtlog.New(loggerOption)

// Level changes in Seq UI automatically update your application
```

## Elasticsearch Integration

Send logs to Elasticsearch for powerful search and analytics.

### Basic Configuration

```go
log := mtlog.New(mtlog.WithElasticsearch("http://localhost:9200", "logs-index"))
```

### Advanced Configuration

```go
log := mtlog.New(
    mtlog.WithElasticsearchAdvanced(
        []string{"http://node1:9200", "http://node2:9200"}, // Multiple nodes
        sinks.WithElasticsearchIndex("myapp-logs-%{+yyyy.MM.dd}"), // Date-based indices
        sinks.WithElasticsearchAPIKey("base64-encoded-key"),
        sinks.WithElasticsearchBatchSize(100),
        sinks.WithElasticsearchBatchTimeout(10*time.Second),
        sinks.WithElasticsearchPipeline("log-pipeline"),    // Ingest pipeline
    ),
)
```

### Index Templates

mtlog automatically creates appropriate mappings for log events:

```json
{
  "mappings": {
    "properties": {
      "@timestamp": {"type": "date"},
      "@level": {"type": "keyword"},
      "@message": {"type": "text"},
      "@messageTemplate": {"type": "keyword"},
      "properties": {"type": "object", "dynamic": true}
    }
  }
}
```

## Sentry Integration

Send error tracking and performance monitoring data to Sentry with intelligent sampling and retry logic.

### Basic Configuration

```go
import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/sentry"
)

// Basic error tracking
sink, _ := sentry.WithSentry("https://key@sentry.io/project")
log := mtlog.New(mtlog.WithSink(sink))

// With environment variable (recommended for production)
// export SENTRY_DSN="https://key@sentry.io/project"
sink, _ := sentry.WithSentry("")
log := mtlog.New(mtlog.WithSink(sink))
```

### Advanced Configuration

```go
sink, _ := sentry.WithSentry("https://key@sentry.io/project",
    sentry.WithEnvironment("production"),
    sentry.WithRelease("v1.2.3"),
    sentry.WithServerName("api-server-01"),
    sentry.WithDebug(true),
    sentry.WithAttachStacktrace(true),
    sentry.WithMaxBreadcrumbs(100),
    sentry.WithTracesSampleRate(0.2),      // 20% of transactions
    sentry.WithProfilesSampleRate(0.1),     // 10% profiling
)
log := mtlog.New(mtlog.WithSink(sink))
```

### Sampling Strategies

The Sentry adapter provides multiple sampling strategies to control data volume:

#### Fixed Sampling
```go
// Sample 10% of all events
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project",
    sentry.WithFixedSampling(0.1),
)
log := mtlog.New(mtlog.WithSink(sink))
```

#### Adaptive Sampling
Automatically adjusts sampling rate based on error volume:

```go
// Adaptive sampling from 1% to 50% based on error rate
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project",
    sentry.WithAdaptiveSampling(0.01, 0.5),
)
log := mtlog.New(mtlog.WithSink(sink))
```

#### Priority Sampling
Different rates for different error levels:

```go
// High sampling for errors, low for warnings
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project",
    sentry.WithPrioritySampling(map[core.LogEventLevel]float64{
        core.FatalLevel:   1.0,   // 100% for fatal
        core.ErrorLevel:   0.5,   // 50% for errors
        core.WarningLevel: 0.1,   // 10% for warnings
    }),
)
log := mtlog.New(mtlog.WithSink(sink))
```

#### Burst Sampling
Handle traffic spikes gracefully:

```go
// Allow bursts of 100 events/sec, then sample at 10%
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project",
    sentry.WithBurstSampling(100, 0.1),
)
log := mtlog.New(mtlog.WithSink(sink))
```

#### Group-Based Sampling
Sample based on error patterns:

```go
// Different rates for different error groups
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project",
    sentry.WithGroupSampling(func(event *core.LogEvent) string {
        if strings.Contains(event.RenderMessage(), "database") {
            return "database"
        }
        return "default"
    }, map[string]float64{
        "database": 0.5,  // 50% for database errors
        "default":  0.1,  // 10% for everything else
    }),
)
log := mtlog.New(mtlog.WithSink(sink))
```

#### Custom Sampling
Implement your own logic:

```go
sink, _ := sentry.WithSentry("https://key@sentry.io/project",
    sentry.WithCustomSampling(func(event *core.LogEvent) bool {
        // Sample all errors from production
        if event.Properties["Environment"] == "production" {
            return true
        }
        // Sample 10% from other environments
        return rand.Float64() < 0.1
    }),
)
log := mtlog.New(mtlog.WithSink(sink))
```

### Performance Monitoring

Track transactions and spans for distributed tracing:

```go
import "github.com/willibrandon/mtlog/adapters/sentry"

// Start a transaction
ctx := sentry.StartTransaction(context.Background(), "ProcessOrder", "order.process")
defer func() {
    if tx := sentry.GetTransaction(ctx); tx != nil {
        tx.Finish()
    }
}()

// Add spans for operations
span := sentry.StartSpan(ctx, "db.query", "SELECT * FROM orders")
// ... perform database query
span.Finish()

// Log within transaction context
log.Information("Order processed successfully")
```

### Retry and Reliability

Configure retry logic for network failures:

```go
sink, _ := sentry.WithSentry("https://key@sentry.io/project",
    sentry.WithRetryPolicy(3, time.Second),           // 3 retries, 1s initial delay
    sentry.WithRetryBackoff(2.0, 30*time.Second),     // 2x backoff, max 30s
    sentry.WithRetryJitter(0.1),                      // 10% jitter
)
log := mtlog.New(mtlog.WithSink(sink))
```

### Stack Trace Caching

Optimize performance with stack trace caching:

```go
sink, _ := sentry.WithSentry("https://key@sentry.io/project",
    sentry.WithStackTraceCache(1000),  // Cache up to 1000 stack traces
    sentry.WithStackTraceTTL(5*time.Minute),
)
log := mtlog.New(mtlog.WithSink(sink))
```

### Metrics Collection

Monitor Sentry sink performance:

```go
// Assuming you have a *SentrySink instance
sink, _ := sentry.NewSentrySink("https://key@sentry.io/project")
metrics := sink.Metrics()
fmt.Printf("Events sent: %d\n", metrics.EventsSent)
fmt.Printf("Events dropped: %d\n", metrics.EventsDropped)
fmt.Printf("Retry attempts: %d\n", metrics.RetryAttempts)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency())
```

### Event Format

Events are enriched with Sentry-specific fields:

```json
{
  "event_id": "fc6d8c0c43fc4630ad850ee518f1b9d0",
  "timestamp": "2025-01-22T10:30:45.123Z",
  "level": "error",
  "message": "User 123 failed to login from 192.168.1.100",
  "logger": "auth",
  "platform": "go",
  "environment": "production",
  "release": "v1.2.3",
  "server_name": "api-server-01",
  "tags": {
    "user_id": "123",
    "ip_address": "192.168.1.100"
  },
  "breadcrumbs": [
    {
      "timestamp": "2025-01-22T10:30:40.000Z",
      "message": "User authentication started",
      "category": "auth"
    }
  ],
  "exception": {
    "type": "AuthenticationError",
    "value": "Invalid credentials",
    "stacktrace": {
      "frames": [...]
    }
  }
}
```

### Best Practices

1. **Use environment variables** for DSN in production
2. **Configure sampling** appropriate to your error volume
3. **Enable stack trace caching** for high-throughput applications
4. **Use transactions** for tracing critical user journeys
5. **Monitor metrics** to ensure events are being sent successfully
6. **Configure retry policy** for network resilience
7. **Use breadcrumbs** to provide context for errors

## Splunk Integration

Send logs to Splunk using the HTTP Event Collector (HEC).

### Basic Configuration

```go
log := mtlog.New(mtlog.WithSplunk("http://localhost:8088", "your-hec-token"))
```

### Advanced Configuration

```go
log := mtlog.New(
    mtlog.WithSplunkAdvanced("http://localhost:8088",
        sinks.WithSplunkToken("your-hec-token"),
        sinks.WithSplunkIndex("main"),
        sinks.WithSplunkSource("myapp"),
        sinks.WithSplunkSourceType("json"),
        sinks.WithSplunkHost("web-server-01"),
        sinks.WithSplunkBatchSize(100),
    ),
)
```

### Event Format

Events are sent in Splunk's HEC format:

```json
{
  "time": 1642856645.123,
  "host": "web-server-01",
  "source": "myapp",
  "sourcetype": "json",
  "index": "main",
  "event": {
    "@timestamp": "2025-01-22T10:30:45.123Z",
    "@level": "Information",
    "@message": "User 123 logged in from 192.168.1.100",
    "@messageTemplate": "User {UserId} logged in from {IPAddress}",
    "UserId": 123,
    "IPAddress": "192.168.1.100"
  }
}
```

## Async Sink Wrapper

Wrap any sink for asynchronous processing to improve application performance.

### Basic Usage

```go
// Wrap file sink for async processing
log := mtlog.New(mtlog.WithAsync(mtlog.WithFileSink("app.log")))

// Wrap Seq sink for async processing
log := mtlog.New(mtlog.WithAsync(mtlog.WithSeq("http://localhost:5341")))
```

### Advanced Configuration

```go
log := mtlog.New(
    mtlog.WithAsyncAdvanced(
        mtlog.WithSeq("http://localhost:5341"),
        sinks.WithAsyncBufferSize(10000),        // Buffer 10k events
        sinks.WithAsyncWorkers(2),               // 2 worker goroutines
        sinks.WithAsyncFlushTimeout(time.Second), // Flush every second
    ),
)
```

### Benefits

- **Non-blocking**: Logging calls return immediately
- **High throughput**: Batch processing of events
- **Backpressure handling**: Configurable buffer overflow strategies

## Durable Buffering

Ensure log events survive application crashes and network failures.

### Basic Usage

```go
log := mtlog.New(
    mtlog.WithDurable(
        mtlog.WithSeq("http://localhost:5341"),
    ),
)
```

### Advanced Configuration

```go
log := mtlog.New(
    mtlog.WithDurableAdvanced(
        mtlog.WithSeq("http://localhost:5341"),
        sinks.WithDurableDirectory("./logs/buffer"),
        sinks.WithDurableMaxSize(100*1024*1024),     // 100MB buffer
        sinks.WithDurableFlushInterval(5*time.Second),
        sinks.WithDurableRetryPolicy(sinks.ExponentialBackoff{
            InitialDelay: time.Second,
            MaxDelay:     time.Minute,
            MaxAttempts:  10,
        }),
    ),
)
```

### Features

- **Persistent storage**: Events stored on disk during outages
- **Automatic retry**: Configurable retry policies for failed sends
- **Buffer management**: Automatic cleanup of old buffer files
- **Crash recovery**: Resume sending after application restart

## Event Routing Sinks

Route log events to different destinations based on their properties and levels.

### Conditional Sink

Filter events based on predicates with zero overhead for non-matching events.

#### Basic Usage

```go
// Route only errors to a dedicated error file
alertSink, _ := sinks.NewFileSink("alerts.log")
criticalAlertSink := sinks.NewConditionalSink(
    func(event *core.LogEvent) bool {
        return event.Level >= core.ErrorLevel && 
               event.Properties["Alert"] != nil
    },
    alertSink,
)

log := mtlog.New(
    mtlog.WithSink(sinks.NewConsoleSink()),     // All events
    mtlog.WithSink(criticalAlertSink),          // Only critical alerts
)
```

#### Built-in Predicates

```go
// Level-based filtering
errorSink := sinks.NewConditionalSink(
    sinks.LevelPredicate(core.ErrorLevel),
    targetSink,
)

// Property existence
auditSink := sinks.NewConditionalSink(
    sinks.PropertyPredicate("Audit"),
    auditFileSink,
)

// Property value matching
prodSink := sinks.NewConditionalSink(
    sinks.PropertyValuePredicate("Environment", "production"),
    productionSink,
)
```

#### Combining Predicates

```go
// AND logic - all conditions must match
complexFilter := sinks.NewConditionalSink(
    sinks.AndPredicate(
        sinks.LevelPredicate(core.ErrorLevel),
        sinks.PropertyPredicate("Critical"),
        sinks.PropertyValuePredicate("Environment", "production"),
    ),
    alertSink,
)

// OR logic - any condition matches
broadFilter := sinks.NewConditionalSink(
    sinks.OrPredicate(
        sinks.LevelPredicate(core.FatalLevel),
        sinks.PropertyPredicate("SecurityAlert"),
    ),
    securitySink,
)

// NOT logic - invert condition
excludeFilter := sinks.NewConditionalSink(
    sinks.NotPredicate(sinks.PropertyPredicate("SkipLogging")),
    targetSink,
)
```

#### Performance

Conditional sinks have near-zero overhead when predicates return false:
- **Predicate returns false**: ~3.7ns/op, 0 allocations
- **Predicate returns true**: ~164ns/op, 1 allocation

### Router Sink

Advanced routing with multiple destinations and configurable routing modes.

#### Routing Modes

```go
// FirstMatch: Stop at first matching route (exclusive routing)
router := sinks.NewRouterSink(sinks.FirstMatch,
    sinks.Route{
        Name:      "errors",
        Predicate: sinks.LevelPredicate(core.ErrorLevel),
        Sink:      errorSink,
    },
    sinks.Route{
        Name:      "warnings",
        Predicate: sinks.LevelPredicate(core.WarningLevel),
        Sink:      warningSink,
    },
)

// AllMatch: Send to all matching routes (broadcast routing)
router := sinks.NewRouterSink(sinks.AllMatch,
    sinks.MetricRoute("metrics", metricsSink),
    sinks.AuditRoute("audit", auditSink),
    sinks.ErrorRoute("errors", errorSink),
)
```

#### Default Sink

Handle non-matching events with a default sink:

```go
router := sinks.NewRouterSinkWithDefault(
    sinks.FirstMatch,
    defaultSink, // Receives events that don't match any route
    routes...,
)
```

#### Dynamic Route Management

Add and remove routes at runtime:

```go
// Add a route dynamically
router.AddRoute(sinks.Route{
    Name:      "debug",
    Predicate: func(e *core.LogEvent) bool {
        return e.Level <= core.DebugLevel
    },
    Sink:      debugSink,
})

// Remove a route by name
router.RemoveRoute("debug")
```

#### Fluent Route Builder

```go
// Build routes with fluent API
route := sinks.NewRoute("special-events").
    When(func(e *core.LogEvent) bool {
        category, _ := e.Properties["Category"].(string)
        return category == "Special"
    }).
    To(specialSink)

router.AddRoute(route)
```

#### Pre-built Routes

```go
// Common route patterns
errorRoute := sinks.ErrorRoute("errors", errorSink)
auditRoute := sinks.AuditRoute("audit", auditSink)
metricRoute := sinks.MetricRoute("metrics", metricsSink)
```

#### Performance Characteristics

- **FirstMatch mode**: ~136ns/op with 3 routes
- **AllMatch mode**: ~403ns/op with 3 routes (all matching)
- Thread-safe route management with minimal lock contention

## Multiple Sinks

Combine multiple sinks for comprehensive logging:

```go
log := mtlog.New(
    mtlog.WithConsoleTheme("dark"),             // Console for development
    mtlog.WithRollingFile("app.log", 10*1024*1024), // Local file backup
    mtlog.WithAsync(mtlog.WithSeq("http://localhost:5341")), // Async Seq
    mtlog.WithDurable(mtlog.WithElasticsearch("http://localhost:9200", "logs")), // Durable ES
)
```

## Custom Sinks

Implement the `core.LogEventSink` interface for custom destinations:

```go
type CustomSink struct {
    // Your custom state
}

func (s *CustomSink) Emit(event *core.LogEvent) error {
    // Process the log event
    fmt.Printf("[%s] %s\n", event.Level, event.RenderMessage())
    return nil
}

func (s *CustomSink) Close() error {
    // Cleanup resources
    return nil
}

// Use your custom sink
log := mtlog.New(mtlog.WithSink(&CustomSink{}))
```

## Sink Performance

### Recommendations

1. **Use async wrappers** for network sinks (Seq, Elasticsearch, Splunk)
2. **Enable batching** to reduce network overhead
3. **Use durable buffering** for critical logs that must not be lost
4. **Configure appropriate buffer sizes** based on your logging volume
5. **Monitor sink performance** in production environments

### Benchmarks

| Sink | Throughput | Latency | Memory |
|------|------------|---------|---------|
| Console | 1M events/s | 1μs | Low |
| File | 500K events/s | 2μs | Low |
| Async File | 2M events/s | 0.5μs | Medium |
| Seq (sync) | 10K events/s | 100ms | Low |
| Seq (async) | 100K events/s | 1ms | Medium |

These benchmarks are approximate and depend on your specific environment and configuration.