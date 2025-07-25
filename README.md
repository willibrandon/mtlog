# mtlog - Message Template Logging for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/willibrandon/mtlog.svg)](https://pkg.go.dev/github.com/willibrandon/mtlog)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

mtlog is a high-performance structured logging library for Go, inspired by [Serilog](https://serilog.net/). It brings message templates and pipeline architecture to the Go ecosystem, achieving zero allocations for simple logging operations while providing powerful features for complex scenarios.

## Features

### Core Features
- **Zero-allocation logging** for simple messages (13.6 ns/op)
- **Message templates** with positional property extraction and format specifiers
- **Pipeline architecture** for clean separation of concerns
- **Type-safe generics** for better compile-time safety
- **LogValue interface** for safe logging of sensitive data
- **8.7x faster** than zap for simple string logging
- **Standard library compatibility** via slog.Handler adapter (Go 1.21+)
- **Kubernetes ecosystem** support via logr.LogSink adapter

### Sinks & Output
- **Console sink** with customizable themes (dark, light, ANSI colors)
- **File sink** with rolling policies (size, time-based)
- **Seq integration** with CLEF format and dynamic level control
- **Elasticsearch sink** for centralized log storage and search
- **Splunk sink** with HEC (HTTP Event Collector) support
- **Async sink wrapper** for high-throughput scenarios
- **Durable buffering** with persistent storage for reliability

### Pipeline Components
- **Rich enrichment** with built-in and custom enrichers
- **Advanced filtering** including rate limiting and sampling
- **Type-safe destructuring** with caching for performance
- **Dynamic level control** with runtime adjustments
- **Configuration from JSON** for flexible deployment

## Installation

```bash
go get github.com/willibrandon/mtlog
```

## Quick Start

```go
package main

import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/core"
)

func main() {
    // Create a logger with console output
    log := mtlog.New(
        mtlog.WithConsoleProperties(),
        mtlog.WithMinimumLevel(core.InformationLevel),
    )

    // Simple logging
    log.Information("Application started")
    
    // Message templates with properties
    userId := 123
    log.Information("User {UserId} logged in", userId)
    
    // Destructuring complex types
    order := Order{ID: 456, Total: 99.95}
    log.Information("Processing {@Order}", order)
}
```

## Message Templates

mtlog uses message templates that preserve structure throughout the logging pipeline:

```go
// Properties are extracted positionally
log.Information("User {UserId} logged in from {IP}", userId, ipAddress)

// Destructuring hints:
// @ - destructure complex types into properties
log.Information("Order {@Order} created", order)

// $ - force scalar rendering (stringify)
log.Information("Error occurred: {$Error}", err)

// Format specifiers (new feature)
log.Information("Price: {Amount:C} for {Quantity:N0} items", 99.95, 1000)
log.Information("Processing time: {Duration:F2}ms", 123.456)
```

## Pipeline Architecture

The logging pipeline processes events through distinct stages:

```
Message Template Parser → Enrichment → Filtering → Destructuring → Output
```

### Configuration with Functional Options

```go
log := mtlog.New(
    // Output configuration
    mtlog.WithConsoleTheme("dark"),     // Console with dark theme
    mtlog.WithRollingFile("app.log", 10*1024*1024), // Rolling file (10MB)
    mtlog.WithSeq("http://localhost:5341", "api-key"), // Seq integration
    
    // Enrichment
    mtlog.WithTimestamp(),              // Add timestamp
    mtlog.WithMachineName(),            // Add hostname
    mtlog.WithProcessInfo(),            // Add process ID/name
    mtlog.WithCallersInfo(),            // Add file/line info
    
    // Filtering & Level Control
    mtlog.WithMinimumLevel(core.DebugLevel),
    mtlog.WithDynamicLevel(levelSwitch), // Runtime level control
    mtlog.WithFilter(customFilter),
    
    // Destructuring
    mtlog.WithDestructuring(),          // Enable @ hints
    mtlog.WithDestructuringDepth(5),    // Max depth
)
```

## Enrichers

Enrichers add contextual information to all log events:

```go
// Built-in enrichers
log := mtlog.New(
    mtlog.WithTimestamp(),
    mtlog.WithMachineName(),
    mtlog.WithProcessInfo(),
    mtlog.WithEnvironmentVariables("APP_ENV", "VERSION"),
    mtlog.WithThreadId(),
    mtlog.WithCallersInfo(),
    mtlog.WithCorrelationId("RequestId"),
)

// Context-based enrichment
ctx := context.WithValue(context.Background(), "RequestId", "abc-123")
log.ForContext("UserId", userId).Information("Processing request")
```

## Filters

Control which events are logged with powerful filtering:

```go
// Level filtering
mtlog.WithMinimumLevel(core.WarningLevel)

// Custom predicate
mtlog.WithFilter(filters.NewPredicateFilter(func(e *core.LogEvent) bool {
    return !strings.Contains(e.MessageTemplate.Text, "health-check")
}))

// Rate limiting
mtlog.WithFilter(filters.NewRateLimitFilter(100, time.Minute))

// Statistical sampling
mtlog.WithFilter(filters.NewSamplingFilter(0.1)) // 10% of events

// Property-based filtering
mtlog.WithFilter(filters.NewExpressionFilter("UserId", 123))
```

## Sinks

mtlog supports multiple output destinations with advanced features:

### Console Sink with Themes

```go
// Dark theme (default)
mtlog.WithConsoleTheme("dark")

// Light theme
mtlog.WithConsoleTheme("light") 

// ANSI colors
mtlog.WithConsoleTheme("ansi")

// Plain text (no colors)
mtlog.WithConsole()
```

### File Sinks

```go
// Simple file output
mtlog.WithFileSink("app.log")

// Rolling file by size
mtlog.WithRollingFile("app.log", 10*1024*1024) // 10MB

// Rolling file by time
mtlog.WithRollingFileTime("app.log", time.Hour) // Every hour
```

### Seq Integration

```go
// Basic Seq integration
mtlog.WithSeq("http://localhost:5341")

// With API key
mtlog.WithSeq("http://localhost:5341", "your-api-key")

// Advanced configuration
mtlog.WithSeqAdvanced("http://localhost:5341",
    sinks.WithSeqBatchSize(100),
    sinks.WithSeqBatchTimeout(5*time.Second),
    sinks.WithSeqCompression(true),
)

// Dynamic level control via Seq
levelOption, levelSwitch, controller := mtlog.WithSeqLevelControl(
    "http://localhost:5341",
    mtlog.SeqLevelControllerOptions{
        CheckInterval: 30*time.Second,
        InitialCheck: true,
    },
)
```

### Elasticsearch Integration

```go
// Basic Elasticsearch
mtlog.WithElasticsearch("http://localhost:9200", "logs")

// With authentication
mtlog.WithElasticsearchAdvanced(
    []string{"http://localhost:9200"},
    elasticsearch.WithIndex("myapp-logs"),
    elasticsearch.WithAPIKey("api-key"),
    elasticsearch.WithBatchSize(100),
)
```

### Splunk Integration

```go
// Splunk HEC integration
mtlog.WithSplunk("http://localhost:8088", "your-hec-token")

// Advanced Splunk configuration
mtlog.WithSplunkAdvanced("http://localhost:8088",
    sinks.WithSplunkToken("hec-token"),
    sinks.WithSplunkIndex("main"),
    sinks.WithSplunkSource("myapp"),
)
```

### Async and Durable Sinks

```go
// Wrap any sink for async processing
mtlog.WithAsync(mtlog.WithFileSink("app.log"))

// Durable buffering (survives crashes)
mtlog.WithDurable(
    mtlog.WithSeq("http://localhost:5341"),
    sinks.WithDurableDirectory("./logs/buffer"),
    sinks.WithDurableMaxSize(100*1024*1024), // 100MB buffer
)
```

## Dynamic Level Control

Control logging levels at runtime without restarting your application:

### Manual Level Control

```go
// Create a level switch
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

logger := mtlog.New(
    mtlog.WithLevelSwitch(levelSwitch),
    mtlog.WithConsole(),
)

// Change level at runtime
levelSwitch.SetLevel(core.DebugLevel)

// Fluent interface
levelSwitch.Debug().Information().Warning()

// Check if level is enabled
if levelSwitch.IsEnabled(core.VerboseLevel) {
    // Expensive logging operation
}
```

### Centralized Level Control with Seq

```go
// Automatic level synchronization with Seq server
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

## Configuration from JSON

Configure loggers using JSON for flexible deployments:

```go
// Load from JSON file
config, err := configuration.LoadFromFile("logging.json")
if err != nil {
    log.Fatal(err)
}

logger := config.CreateLogger()
```

Example `logging.json`:
```json
{
    "minimumLevel": "Information",
    "sinks": [
        {
            "type": "Console",
            "theme": "dark"
        },
        {
            "type": "RollingFile",
            "path": "logs/app.log",
            "maxSize": "10MB"
        },
        {
            "type": "Seq",
            "serverUrl": "http://localhost:5341",
            "apiKey": "${SEQ_API_KEY}"
        }
    ],
    "enrichers": ["Timestamp", "MachineName", "ProcessInfo"]
}
```

## Safe Logging with LogValue

Protect sensitive data with the LogValue interface:

```go
type User struct {
    ID       int
    Username string
    Password string // Never logged
}

func (u User) LogValue() interface{} {
    return map[string]interface{}{
        "id":       u.ID,
        "username": u.Username,
        // Password intentionally omitted
    }
}

// Password won't appear in logs
user := User{ID: 1, Username: "alice", Password: "secret"}
log.Information("User logged in: {@User}", user)
```

## Performance

Benchmark results on AMD Ryzen 9 9950X:

| Operation | mtlog | zap | zerolog | Winner |
|-----------|-------|-----|---------|---------|
| Simple string | 16.82 ns | 146.6 ns | 36.46 ns | **mtlog** |
| Filtered out | 1.47 ns | 3.57 ns | 1.71 ns | **mtlog** |
| Two properties | 190.6 ns | 216.9 ns | 49.48 ns | zerolog |
| With context | 205.2 ns | 130.8 ns | 35.25 ns | zerolog |

## Examples

See the [examples](./examples) directory for complete examples:

- [Basic logging](./examples/basic/main.go)
- [Using enrichers](./examples/enrichers/main.go)
- [Context logging](./examples/context/main.go)
- [Advanced filtering](./examples/filtering/main.go)
- [Destructuring](./examples/destructuring/main.go)
- [LogValue interface](./examples/logvalue/main.go)
- [Console themes](./examples/themes/main.go)
- [Rolling files](./examples/rolling/main.go)
- [Seq integration](./examples/seq/main.go)
- [Elasticsearch](./examples/elasticsearch/main.go)
- [Splunk integration](./examples/splunk/main.go)
- [Async logging](./examples/async/main.go)
- [Durable buffering](./examples/durable/main.go)
- [Dynamic levels](./examples/dynamic-levels/main.go)
- [Configuration](./examples/configuration/main.go)
- [Generics usage](./examples/generics/main.go)

## Ecosystem Compatibility

### Standard Library (slog)

mtlog provides full compatibility with Go's standard `log/slog` package:

```go
// Use mtlog as a backend for slog
slogger := mtlog.NewSlogLogger(
    mtlog.WithSeq("http://localhost:5341"),
    mtlog.WithMinimumLevel(core.InformationLevel),
)

// Use standard slog API
slogger.Info("user logged in", "user_id", 123, "ip", "192.168.1.1")

// Or create a custom slog handler
logger := mtlog.New(mtlog.WithConsole())
slogger = slog.New(logger.AsSlogHandler())
```

### Kubernetes (logr)

mtlog integrates with the Kubernetes ecosystem via logr:

```go
// Use mtlog as a backend for logr
logrLogger := mtlog.NewLogrLogger(
    mtlog.WithConsole(),
    mtlog.WithMinimumLevel(core.DebugLevel),
)

// Use standard logr API
logrLogger.Info("reconciling", "namespace", "default", "name", "my-app")
logrLogger.Error(err, "failed to update resource")

// Or create a custom logr sink
logger := mtlog.New(mtlog.WithSeq("http://localhost:5341"))
logrLogger = logr.New(logger.AsLogrSink())
```

## Advanced Usage

### Custom Sinks

Implement the `core.LogEventSink` interface for custom outputs:

```go
type CustomSink struct{}

func (s *CustomSink) Emit(event *core.LogEvent) error {
    // Process the log event
    return nil
}

log := mtlog.New(
    mtlog.WithSink(&CustomSink{}),
)
```

### Custom Enrichers

Add custom properties to all events:

```go
type UserEnricher struct {
    userID int
}

func (e *UserEnricher) Enrich(event *core.LogEvent, factory core.LogEventPropertyFactory) {
    event.AddPropertyIfAbsent(factory.CreateProperty("UserId", e.userID))
}

log := mtlog.New(
    mtlog.WithEnricher(&UserEnricher{userID: 123}),
)
```

### Type Registration

Register types for special handling during destructuring:

```go
destructurer := destructure.NewDefaultDestructurer()
destructurer.RegisterScalarType(reflect.TypeOf(uuid.UUID{}))
```

## Documentation

For comprehensive guides and examples, see the [docs](./docs) directory:

- **[Quick Reference](./docs/quick-reference.md)** - Quick reference for all features
- **[Sinks Guide](./docs/sinks.md)** - Complete guide to all output destinations
- **[Dynamic Level Control](./docs/dynamic-levels.md)** - Runtime level management
- **[Type-Safe Generics](./docs/generics.md)** - Compile-time safe logging methods
- **[Configuration](./docs/configuration.md)** - JSON-based configuration
- **[Performance](./docs/performance.md)** - Benchmarks and optimization
- **[Testing](./docs/testing.md)** - Container-based integration testing

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests (requires Docker)
go test -tags=integration ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run integration tests with Seq
docker run -d --name seq-test -e ACCEPT_EULA=Y -e SEQ_FIRSTRUN_NOAUTHENTICATION=true -p 8080:80 -p 5342:5341 datalust/seq
go test -tags=integration ./...
docker stop seq-test && docker rm seq-test

# Run integration tests with Elasticsearch
docker run -d --name es-test -e "discovery.type=single-node" -e "xpack.security.enabled=false" -p 9200:9200 docker.elastic.co/elasticsearch/elasticsearch:8.11.1
# Wait for Elasticsearch to be ready
sleep 30
go test -tags=integration ./...
docker stop es-test && docker rm es-test

# Run integration tests with Splunk
docker run -d --name splunk-test -p 8000:8000 -p 8088:8088 -e SPLUNK_START_ARGS="--accept-license" -e SPLUNK_PASSWORD="changeme" -e SPLUNK_HEC_TOKEN="00000000-0000-0000-0000-000000000000" splunk/splunk:latest
# Wait for Splunk to be ready
sleep 60
go test -tags=integration ./...
docker stop splunk-test && docker rm splunk-test
```

See [testing.md](./docs/testing.md) for detailed integration test setup with Docker containers.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
