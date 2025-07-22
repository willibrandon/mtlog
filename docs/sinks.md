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