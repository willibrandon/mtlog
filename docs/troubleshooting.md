# Troubleshooting Guide

This guide helps diagnose and resolve common issues with mtlog.

## Table of Contents

- [Debugging Silent Failures](#debugging-silent-failures)
- [Using SelfLog](#using-selflog)
- [Common Issues](#common-issues)
- [Performance Issues](#performance-issues)
- [Integration Issues](#integration-issues)

## Debugging Silent Failures

When logs aren't appearing as expected, mtlog's internal diagnostics (selflog) can help identify the issue.

### Using SelfLog

SelfLog is mtlog's internal diagnostic logging facility that reports errors and warnings that would otherwise be silently discarded.

#### Enable SelfLog Programmatically

```go
import (
    "os"
    "github.com/willibrandon/mtlog/selflog"
)

// Log to stderr
selflog.Enable(os.Stderr)
defer selflog.Disable()

// Log to a file
f, _ := os.Create("mtlog-debug.log")
defer f.Close()
selflog.Enable(selflog.Sync(f))

// Custom handler
selflog.EnableFunc(func(msg string) {
    syslog.Warning("mtlog: " + msg)
})
```

#### Enable via Environment Variable

Set `MTLOG_SELFLOG` to automatically enable on startup:

```bash
# Log to stderr
export MTLOG_SELFLOG=stderr

# Log to stdout
export MTLOG_SELFLOG=stdout

# Log to file
export MTLOG_SELFLOG=/path/to/mtlog-debug.log
```

#### SelfLog Message Format

Messages are formatted as:
```
2025-01-29T15:30:45Z [component] message details
```

Example output:
```
2025-01-29T15:30:45Z [console] write failed: broken pipe
2025-01-29T15:30:46Z [seq] batch send error: connection refused (url=http://localhost:5341)
2025-01-29T15:30:47Z [parser] template validation error: unclosed property at position 23
```

### What SelfLog Reports

1. **Sink Failures**
   - Write errors (file permissions, disk full)
   - Network errors (connection refused, timeouts)
   - HTTP errors (authentication, bad requests)

2. **Configuration Issues**
   - Unknown sink/enricher/filter types
   - Type mismatches in configuration
   - Parse failures for numeric values
   - Unknown console themes or log levels

3. **Template Problems**
   - Unclosed properties: `"User {Name logged in"`
   - Empty property names: `"Value {} found"`
   - Invalid property names: `"Count {123}"`

4. **Panic Recovery**
   - Capturing panics (infinite recursion, nil dereference)
   - LogValue implementation panics
   - Worker goroutine panics in async sinks

5. **Resource Issues**
   - Async sink buffer overflow
   - Durable sink buffer corruption
   - File handle exhaustion

## Common Issues

### Logs Not Appearing

1. **Check Minimum Level**
   ```go
   // Ensure your log level is high enough
   log := mtlog.New(
       mtlog.WithMinimumLevel(core.DebugLevel),
       mtlog.WithConsole(),
   )
   ```

2. **Verify Sink Configuration**
   ```go
   // Enable selflog to see sink errors
   selflog.Enable(os.Stderr)
   
   // Check if sink is reachable
   sink, err := sinks.NewSeqSink("http://localhost:5341")
   if err != nil {
       log.Printf("Seq sink error: %v", err)
   }
   ```

3. **Template/Argument Mismatch**
   ```go
   // Wrong: 2 properties but only 1 argument
   log.Information("User {Name} with ID {Id}", "Alice")
   
   // Right: Match property count
   log.Information("User {Name} with ID {Id}", "Alice", 123)
   ```

### Network Sink Issues

#### Seq Connection Problems
```go
// Enable selflog to see connection errors
selflog.Enable(os.Stderr)

// Test with curl first
// curl -X POST http://localhost:5341/api/events/raw -d '{}'

// Use longer timeouts for slow networks
sink, _ := sinks.NewSeqSink("http://localhost:5341",
    sinks.WithSeqTimeout(30*time.Second),
    sinks.WithSeqRetry(5, 2*time.Second),
)
```

#### Elasticsearch Issues
```go
// Check cluster health
// curl http://localhost:9200/_cluster/health

// Use proper index naming
sink, _ := sinks.NewElasticsearchSink("http://localhost:9200",
    sinks.WithElasticsearchIndex("logs-{2006.01.02}"),
    sinks.WithElasticsearchTimeout(30*time.Second),
)
```

### File Sink Problems

1. **Permission Denied**
   ```go
   // Check directory permissions
   // Enable selflog to see exact error
   selflog.Enable(os.Stderr)
   
   sink, err := sinks.NewFileSink("/var/log/app.log")
   if err != nil {
       // Try user-writable location
       sink, _ = sinks.NewFileSink("./app.log")
   }
   ```

2. **Disk Full**
   ```go
   // Use rolling files to manage space
   sink, _ := sinks.NewRollingFileSink(sinks.RollingFileOptions{
       PathFormat:      "logs/app-{Date}.log",
       RetainedFiles:   7,  // Keep only 7 files
       FileSizeLimitMB: 10, // 10MB per file
   })
   ```

### Memory and Performance

1. **High Memory Usage**
   ```go
   // Limit async sink buffer
   wrapped := sinks.NewConsoleSink()
   async := sinks.NewAsyncSink(wrapped,
       sinks.WithAsyncBufferSize(1000),    // Smaller buffer
       sinks.WithAsyncWorkers(2),          // Fewer workers
       sinks.WithAsyncDropOnFull(true),    // Drop instead of block
   )
   ```

2. **Slow Capturing**
   ```go
   // Limit capturing depth
   capturer := capture.NewCapturer(
       2,    // Max depth (default: 3)
       100,  // Max string length
       50,   // Max collection items
   )
   ```

## Performance Issues

### Identifying Bottlenecks

1. **Enable CPU Profiling**
   ```go
   import _ "net/http/pprof"
   go http.ListenAndServe("localhost:6060", nil)
   // go tool pprof http://localhost:6060/debug/pprof/profile
   ```

2. **Use Benchmarks**
   ```bash
   cd benchmarks
   go test -bench=. -benchmem -cpuprofile=cpu.prof
   go tool pprof cpu.prof
   ```

3. **Monitor Allocations**
   ```go
   // Use selflog to identify allocation sources
   selflog.EnableFunc(func(msg string) {
       if strings.Contains(msg, "dropped") {
           // Log allocation spike
           runtime.GC()
           var m runtime.MemStats
           runtime.ReadMemStats(&m)
           log.Printf("Alloc: %v MB", m.Alloc/1024/1024)
       }
   })
   ```

### Optimization Tips

1. **Use Simple Templates for High-Frequency Logs**
   ```go
   // Slow: Complex template with multiple properties
   log.Debug("Processing request {RequestId} for user {UserId} at {Timestamp}")
   
   // Fast: Simple template
   log.Debug("Processing request")
   ```

2. **Batch Operations**
   ```go
   // Use async sink with batching
   async := sinks.NewAsyncSink(sink,
       sinks.WithAsyncBatchSize(100),
       sinks.WithAsyncFlushInterval(1*time.Second),
   )
   ```

3. **Filter Early**
   ```go
   // Filter before expensive operations
   log := mtlog.New(
       mtlog.WithFilter(filters.ByLevelThreshold(core.InformationLevel)),
       mtlog.WithFilter(filters.ByPredicate(func(e *core.LogEvent) bool {
           // Skip health check logs
           return !strings.Contains(e.MessageTemplate, "health")
       })),
       mtlog.WithSink(expensiveSink),
   )
   ```

## Integration Issues

### Docker/Kubernetes

1. **Console Output Not Visible**
   ```go
   // Force unbuffered output
   sink := sinks.NewConsoleSink()
   sink.SetOutput(os.Stdout) // Ensure stdout
   
   // Or use environment detection
   if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
       // In Kubernetes, use JSON for better parsing
       sink := sinks.NewConsoleSink()
       sink.SetFormatter(formatters.NewJSONFormatter())
   }
   ```

2. **Time Zone Issues**
   ```go
   // Use UTC in containers
   sink := sinks.NewConsoleSink()
   sink.SetFormatter(formatters.NewOutputTemplateFormatter(
       "{Timestamp:2006-01-02T15:04:05.000Z07:00} [{Level:u3}] {Message}{NewLine}{Exception}",
   ))
   ```

### Library Conflicts

1. **Multiple Loggers**
   ```go
   // Bridge slog to mtlog
   slogger := mtlog.NewSlogLogger(
       mtlog.WithConsole(),
       mtlog.WithProperty("logger", "slog"),
   )
   slog.SetDefault(slogger)
   ```

2. **Context Propagation**
   ```go
   // Ensure context flows through middleware
   ctx := context.Background()
   ctx = mtlog.PushProperty(ctx, "RequestId", requestId)
   
   logger := baseLogger.WithContext(ctx)
   // All logs will include RequestId
   ```

## Getting Help

If you're still experiencing issues:

1. **Enable SelfLog** and check the output carefully
2. **Check the [examples](../examples)** directory for working code
3. **Run the [integration tests](../integration)** to verify your environment
4. **Search [existing issues](https://github.com/willibrandon/mtlog/issues)**
5. **Open a new issue** with:
   - mtlog version (`go list -m github.com/willibrandon/mtlog`)
   - Go version (`go version`)
   - Minimal reproducible example
   - SelfLog output
   - Expected vs actual behavior

## Using Custom Sinks

When implementing custom sinks, use selflog for diagnostics:

```go
type MySink struct {
    // ...
}

func (s *MySink) Emit(event *core.LogEvent) {
    if err := s.doEmit(event); err != nil {
        if selflog.IsEnabled() {
            selflog.Printf("[mysink] emit failed: %v", err)
        }
    }
}

func (s *MySink) Close() error {
    // Implement idempotent close
    var closeErr error
    s.closeOnce.Do(func() {
        if err := s.doClose(); err != nil {
            if selflog.IsEnabled() {
                selflog.Printf("[mysink] close failed: %v", err)
            }
            closeErr = err
        }
    })
    return closeErr
}
```
