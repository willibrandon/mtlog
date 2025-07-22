# Message Template Logging for Go - Design Document
*A Serilog-inspired logging library bringing message templates and pipeline architecture to Go*

## Project: `mtlog` - Message Template Logging

### Attribution
This library is inspired by [Serilog](https://serilog.net/), the excellent structured logging library for .NET created by Nicholas Blumhardt and the Serilog Contributors. `mtlog` aims to bring Serilog's proven design patterns to the Go ecosystem.

## Table of Contents
1. [Core Principles](#core-principles)
2. [Architecture Overview](#architecture-overview)
3. [Message Templates](#message-templates)
4. [Pipeline Design](#pipeline-design)
5. [Seq Integration](#seq-integration)
6. [Performance Considerations](#performance-considerations)
7. [API Design](#api-design)
8. [Implementation Roadmap](#implementation-roadmap)

## Core Principles

### 1. Message Templates as First-Class Citizens
- Message templates are preserved throughout the pipeline
- Templates serve as both human-readable messages and event types
- Property extraction maintains semantic meaning
- Templates enable powerful grouping and analysis in log stores

### 2. Composable Pipeline Architecture
- Each logging operation flows through distinct stages
- Stages are independently configurable and extensible
- Pipeline stages: Enrichment → Filtering → Destructuring → Output
- Each stage can be modified without affecting others

### 3. Native Seq Integration
- CLEF (Compact Log Event Format) as a primary output format
- Built-in batching and buffering for Seq ingestion
- API key authentication and dynamic level control
- Automatic template and property mapping for Seq's features

### 4. Go Idiomatic Design
- Zero dependencies for core functionality
- Interface-based design for extensibility
- Proper context.Context support
- Efficient memory usage and minimal allocations

## Architecture Overview

```
┌─────────────────┐
│   Application   │
└────────┬────────┘
         │ log.Information("User {UserId} logged in", userId)
         ▼
┌─────────────────┐
│ Message Template│ ← Preserves template structure
│     Parser      │   Extracts properties
└────────┬────────┘
         │ LogEvent{Template, Properties, Timestamp, Level}
         ▼
┌─────────────────┐
│   Enrichment    │ ← Adds contextual properties
│     Stage       │   (MachineName, Environment, etc.)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Filtering     │ ← Applies level and predicate filters
│     Stage       │   
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Destructuring   │ ← Converts complex types to log-friendly
│     Stage       │   representations
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Sinks       │ ← Outputs to multiple destinations
│   (Parallel)    │   (Console, File, Seq, Elasticsearch)
└─────────────────┘
```

## Message Templates

### Template Syntax
```go
// Basic property extraction
log.Information("User {UserId} logged in from {IpAddress}", userId, ip)

// Typed formatting hints (future enhancement)
log.Information("Order {OrderId} total: {Total:C}", orderId, total)
log.Debug("Processing started at {StartTime:yyyy-MM-dd HH:mm:ss}", time.Now())

// Destructuring hints
log.Information("Processing {@User} with {$Exception}", user, err)
```

### Template Parsing Rules
1. Properties are enclosed in braces: `{PropertyName}`
2. Property names must be valid identifiers
3. Nested braces are escaped: `{{literal}}`
4. Properties are positionally matched to arguments
5. Extra arguments are added as positional properties
6. Missing arguments result in empty property values

### Implementation Details
```go
type MessageTemplate struct {
    Raw    string
    Tokens []MessageTemplateToken
}

type MessageTemplateToken interface {
    Render(properties map[string]interface{}) string
}

type TextToken struct {
    Text string
}

type PropertyToken struct {
    PropertyName   string
    Destructuring  DestructuringHint
    Format         string
    Alignment      int
}

type DestructuringHint int
const (
    Default        DestructuringHint = iota
    Stringify      // {PropertyName}
    Destructure    // {@PropertyName}
    AsScalar       // {$PropertyName}
)
```

## Pipeline Design

### Stage 1: Enrichment
Enrichers add ambient properties to all log events:

```go
type LogEventEnricher interface {
    Enrich(event *LogEvent, propertyFactory LogEventPropertyFactory)
}

// Built-in enrichers
- MachineNameEnricher
- EnvironmentEnricher  
- ProcessEnricher
- ThreadIdEnricher
- TimestampEnricher
- ContextEnricher (from context.Context)
- CorrelationIdEnricher
```

### Stage 2: Filtering
Filters determine which events proceed through the pipeline:

```go
type LogEventFilter interface {
    IsEnabled(event *LogEvent) bool
}

// Built-in filters
- LevelFilter
- PredicateFilter
- ExpressionFilter (property-based)
- SamplingFilter
```

### Stage 3: Destructuring
Destructurers convert complex types into log-appropriate representations:

```go
type Destructurer interface {
    TryDestructure(value interface{}, 
                   propertyFactory LogEventPropertyFactory) (*LogEventProperty, bool)
}

// Destructuring policies
- MaxDepth
- MaxStringLength  
- MaxCollectionCount
- ScalarTypes []reflect.Type
- IgnoredTypes []reflect.Type
```

### Stage 4: Output (Sinks)
Sinks write events to destinations:

```go
type LogEventSink interface {
    Emit(event *LogEvent)
    Close() error
}

// Core sinks
- ConsoleSink (with themes)
- FileSink
- RollingFileSink
- BatchingSink (wrapper)
- AsyncSink (wrapper)
```

## Seq Integration

### CLEF Format Implementation
```go
type CLEFFormatter struct {
    // Implements Seq's Compact Log Event Format
}

// Output format
{
    "@t": "2024-01-15T10:30:45.1234567Z",
    "@mt": "User {UserId} logged in from {IpAddress}",
    "@l": "Information",
    "UserId": 123,
    "IpAddress": "192.168.1.1",
    "MachineName": "PROD-WEB-01"
}
```

### Seq Sink Features
```go
type SeqSink struct {
    serverUrl              string
    apiKey                 string
    batchSizeLimit        int
    period                time.Duration
    queueLimit            int
    
    // Advanced features
    controlLevelSwitch    *LoggingLevelSwitch
    durableBuffer         Buffer
    healthCheck           HealthReporter
}

// Dynamic level control
levelSwitch := NewLoggingLevelSwitch(InfoLevel)
log := New().
    MinimumLevel().ControlledBy(levelSwitch).
    WriteTo().Seq("http://seq:5341", controlLevelSwitch: levelSwitch)
```

### Seq-Specific Optimizations
1. Batch API usage for efficient ingestion
2. Gzip compression for payloads
3. Automatic retry with exponential backoff
4. Durable buffering for reliability
5. Health endpoint monitoring

## Performance Considerations

### Zero-Allocation Goals
1. **Message Template Caching**
   ```go
   var templateCache = sync.Map{} // Template string → parsed template
   ```

2. **Property Bag Pooling**
   ```go
   var propertyPool = sync.Pool{
       New: func() interface{} {
           return make(map[string]interface{}, 16)
       },
   }
   ```

3. **Minimal Interface Conversions**
   - Use concrete types internally where possible
   - Avoid unnecessary boxing/unboxing

### Benchmarking Targets
```
BenchmarkSimpleLog-8              5000000    200 ns/op    0 B/op    0 allocs/op
BenchmarkWithProperties-8         2000000    800 ns/op   64 B/op    2 allocs/op  
BenchmarkStructuredObject-8       1000000   1500 ns/op  256 B/op    8 allocs/op
```

### Comparison with Existing Loggers
Must achieve performance within 20% of zap/zerolog while maintaining Serilog's features.

## API Design

### Logger Configuration
```go
// Fluent configuration matching Serilog's style
log := mtlog.New().
    MinimumLevel().Debug().
    MinimumLevel().Override("Microsoft", Warning).
    Enrich().WithMachineName().
    Enrich().WithProperty("Application", "MyApp").
    Enrich().FromContext().
    Filter().ByExcluding(MatchProperty("RequestPath", "/health")).
    WriteTo().Console(theme: mtlog.Themes.Code).
    WriteTo().File("logs/app-.log", rollingInterval: Daily).
    WriteTo().Seq("http://localhost:5341").
    CreateLogger()
```

### Logging API
```go
// Logger interface - matches Serilog's API
type Logger interface {
    // Structured logging methods
    Verbose(messageTemplate string, args ...interface{})
    Debug(messageTemplate string, args ...interface{})
    Information(messageTemplate string, args ...interface{})
    Warning(messageTemplate string, args ...interface{})
    Error(messageTemplate string, args ...interface{})
    Fatal(messageTemplate string, args ...interface{})
    
    // Generic write
    Write(level LogEventLevel, messageTemplate string, args ...interface{})
    
    // Context support
    ForContext(propertyName string, value interface{}) Logger
    WithContext(ctx context.Context) Logger
}
```

### Go-Specific Enhancements
```go
// Context integration
ctx := context.WithValue(ctx, "userId", 123)
log.WithContext(ctx).Information("Processing request")

// Error helper
if err := doSomething(); err != nil {
    log.Error("Operation failed: {Error}", err)
}

// Panic recovery
defer log.Recover("Panic in {Operation}", "main loop")

// Timing helper
defer log.TimeOperation("Database query")()
```

## Implementation Roadmap

### Phase 1: Core Foundation (Week 1-2)
- [ ] Basic logger interface and implementation
- [ ] Message template parser
- [ ] Property extraction
- [ ] Console sink with basic formatting
- [ ] File sink
- [ ] Unit tests for core functionality

### Phase 2: Pipeline Implementation (Week 3-4)
- [ ] Enrichment framework
- [ ] Basic enrichers (machine, process, timestamp)
- [ ] Filtering framework
- [ ] Level and predicate filters
- [ ] Basic destructuring
- [ ] Benchmarks vs existing loggers

### Phase 3: Seq Integration (Week 5-6)
- [ ] CLEF formatter
- [ ] Seq sink with batching
- [ ] API key authentication
- [ ] Dynamic level control
- [ ] Durable buffering
- [ ] Integration tests with Seq

### Phase 4: Advanced Features (Week 7-8)
- [ ] Console themes
- [ ] Rolling file sink
- [ ] Async sink wrapper
- [ ] Context enricher
- [ ] Correlation ID support
- [ ] Configuration from JSON/YAML

### Phase 5: Production Readiness (Week 9-10)
- [ ] Performance optimization
- [ ] Memory profiling
- [ ] Comprehensive documentation
- [ ] Example applications
- [ ] Elasticsearch sink
- [ ] Splunk sink

### Phase 6: Community Release
- [ ] API documentation
- [ ] Migration guide from other loggers
- [ ] Benchmarking suite
- [ ] CI/CD setup
- [ ] Contribution guidelines

## Success Metrics

1. **Performance**: Within 20% of zap/zerolog for common operations
2. **Memory**: Zero allocations for simple log operations
3. **Compatibility**: Can read Serilog JSON configuration
4. **Adoption**: Clear migration path from existing Go loggers
5. **Seq Integration**: Feature parity with Serilog's Seq sink

## Open Questions

1. **Module Path**: `github.com/[username]/mtlog` initially
2. **Backwards Compatibility**: Commit to stable API from v1.0
3. **Plugin System**: Support for third-party sinks/enrichers
4. **Generics**: Utilize Go 1.18+ generics where beneficial
5. **License**: Determine appropriate open source license

## References

- [Serilog Documentation](https://github.com/serilog/serilog/wiki)
- [CLEF Specification](https://docs.datalust.co/docs/posting-raw-events#compact-json-format)
- [Seq API Documentation](https://docs.datalust.co/docs/seq-http-api)
- [Go Performance Best Practices](https://github.com/dgryski/go-perfbook)

---

*This design document is a living document and will be updated as the implementation progresses.*