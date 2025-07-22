# mtlog - Message Template Logging for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/willibrandon/mtlog.svg)](https://pkg.go.dev/github.com/willibrandon/mtlog)
[![Go Report Card](https://goreportcard.com/badge/github.com/willibrandon/mtlog)](https://goreportcard.com/report/github.com/willibrandon/mtlog)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

mtlog is a high-performance structured logging library for Go, inspired by [Serilog](https://serilog.net/). It brings message templates and pipeline architecture to the Go ecosystem, achieving zero allocations for simple logging operations while providing powerful features for complex scenarios.

## Features

- **Zero-allocation logging** for simple messages (16.82 ns/op)
- **Message templates** with positional property extraction
- **Pipeline architecture** for clean separation of concerns
- **Rich enrichment** with built-in and custom enrichers
- **Advanced filtering** including rate limiting and sampling
- **Type-safe destructuring** with caching for performance
- **LogValue interface** for safe logging of sensitive data
- **8.7x faster** than zap for simple string logging

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
    mtlog.WithConsoleProperties(),      // Console with properties
    mtlog.WithFileSink("app.log"),      // File output
    
    // Enrichment
    mtlog.WithTimestamp(),              // Add timestamp
    mtlog.WithMachineName(),            // Add hostname
    mtlog.WithProcessInfo(),            // Add process ID/name
    mtlog.WithCallersInfo(),            // Add file/line info
    
    // Filtering
    mtlog.WithMinimumLevel(core.DebugLevel),
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

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [Serilog](https://serilog.net/) for .NET
- Performance insights from [zap](https://github.com/uber-go/zap) and [zerolog](https://github.com/rs/zerolog)