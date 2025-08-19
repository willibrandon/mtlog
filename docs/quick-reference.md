# Quick Reference

A quick reference for all mtlog features and common usage patterns.

## Basic Setup

```go
import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/core"
)

// Simple logger
logger := mtlog.New(mtlog.WithConsole())

// Production logger
logger := mtlog.New(
    mtlog.WithConsoleTheme("dark"),
    mtlog.WithSeq("http://localhost:5341", "api-key"),
    mtlog.WithMinimumLevel(core.InformationLevel),
)
```

## Logging Methods

### Traditional Methods
```go
logger.Verbose("Verbose message")
logger.Debug("Debug: {Value}", value)
logger.Information("Info: {User} {Action}", user, action)
logger.Warning("Warning: {Count} items", count)
logger.Error("Error: {Error}", err)
logger.Fatal("Fatal: {Reason}", reason)
```

### Generic Methods (Type-Safe)
```go
logger.VerboseT("Verbose message")
logger.DebugT("Debug: {Value}", value)
logger.InformationT("Info: {User} {Action}", user, action)
logger.WarningT("Warning: {Count} items", count)
logger.ErrorT("Error: {Error}", err)
logger.FatalT("Fatal: {Reason}", reason)
```

## Message Templates

### Template Syntaxes
```go
// Traditional syntax
logger.Information("User {UserId} logged in", userId)

// Go template syntax
logger.Information("User {{.UserId}} logged in", userId)

// Mix both syntaxes
logger.Information("User {UserId} ({{.Username}}) logged in", userId, username)
```

### Capturing Hints
```go
// @ - capture complex types
logger.Information("User: {@User}", user)

// $ - force scalar/string rendering
logger.Information("Error: {$Error}", complexError)
```

### Format Specifiers
```go
// Numbers
logger.Information("Order {Id:000} total: ${Amount:F2}", 42, 99.95)
logger.Information("Progress: {Percent:P1}", 0.755)    // 75.5%
logger.Information("Speed: {Value:F2} MB/s", 123.456)  // 123.46 MB/s
logger.Information("CPU Usage: {Usage:P0}", 0.65)      // 65%

// Timestamps (in output templates)
// ${Timestamp:HH:mm:ss} -> 15:04:05
// ${Timestamp:yyyy-MM-dd} -> 2024-01-02
// ${Timestamp:yyyy-MM-dd HH:mm:ss.fff} -> 2024-01-02 15:04:05.123

// Levels (in output templates)
// ${Level:u3} -> INF, WRN, ERR
// ${Level:u} -> INFORMATION, WARNING, ERROR
// ${Level:l} -> information, warning, error
```

## Sinks

### Console
```go
mtlog.WithConsole()                    // Plain console
mtlog.WithConsoleProperties()          // Console with properties
mtlog.WithConsoleTheme(sinks.LiterateTheme())   // Literate theme (beautiful!)
mtlog.WithConsoleTheme(sinks.DarkTheme())       // Dark theme
mtlog.WithConsoleTheme(sinks.LightTheme())      // Light theme
mtlog.WithConsoleTheme(sinks.NoColorTheme())    // No colors

// With output template
mtlog.WithConsoleTemplate("[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}")
```

### File
```go
mtlog.WithFileSink("app.log")                           // Simple file
mtlog.WithRollingFile("app.log", 10*1024*1024)         // Size-based rolling (10MB)
mtlog.WithRollingFileTime("app.log", time.Hour)        // Time-based rolling

// With output template
mtlog.WithFileTemplate("app.log", 
    "[${Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} ${Level:u3}] {SourceContext}: ${Message}${NewLine}${Exception}")
```

### Seq
```go
mtlog.WithSeq("http://localhost:5341")                 // Basic
mtlog.WithSeqAPIKey("http://localhost:5341", "key")    // With API key
mtlog.WithSeqAdvanced("http://localhost:5341",         // Advanced config
    sinks.WithSeqBatchSize(100),
    sinks.WithSeqBatchTimeout(5*time.Second),
)
```

### Elasticsearch
```go
mtlog.WithElasticsearch("http://localhost:9200", "logs")
mtlog.WithElasticsearchAdvanced(
    []string{"http://node1:9200", "http://node2:9200"},
    sinks.WithElasticsearchIndex("logs-%{+yyyy.MM.dd}"),
    sinks.WithElasticsearchAPIKey("key"),
)
```

### Splunk
```go
mtlog.WithSplunk("http://localhost:8088", "hec-token")
mtlog.WithSplunkAdvanced("http://localhost:8088",
    sinks.WithSplunkToken("token"),
    sinks.WithSplunkIndex("main"),
)
```

### OpenTelemetry (OTEL)
```go
import "github.com/willibrandon/mtlog/adapters/otel"

// Basic OTLP sink
logger := otel.NewOTELLogger(
    otel.WithOTLPEndpoint("localhost:4317"),
    otel.WithOTLPInsecure(),
)

// Advanced with batching and compression
logger := mtlog.New(
    otel.WithOTLPSink(
        otel.WithOTLPEndpoint("otel-collector:4317"),
        otel.WithOTLPTransport(otel.OTLPTransportGRPC),
        otel.WithOTLPBatching(100, 5*time.Second),
        otel.WithOTLPCompression("gzip"),
    ),
)

// With trace context enrichment
logger := otel.NewRequestLogger(ctx,
    otel.WithOTLPEndpoint("localhost:4317"),
    otel.WithOTLPInsecure(),
)

// With sampling
logger := mtlog.New(
    otel.WithOTLPSink(
        otel.WithOTLPEndpoint("localhost:4317"),
        otel.WithOTLPSampling(otel.NewRateSampler(0.1)), // 10% sampling
    ),
)
```

### Async & Durable
```go
mtlog.WithAsync(mtlog.WithFileSink("app.log"))         // Async wrapper
mtlog.WithDurable(mtlog.WithSeq("http://localhost:5341"))  // Durable buffering
```

## Enrichers

```go
mtlog.WithTimestamp()                          // Add @timestamp
mtlog.WithMachineName()                        // Add MachineName
mtlog.WithProcessInfo()                        // Add ProcessId, ProcessName
mtlog.WithCallersInfo()                        // Add file, line, method
mtlog.WithEnvironmentVariables("APP_ENV")     // Add env vars
mtlog.WithThreadId()                           // Add ThreadId
mtlog.WithCorrelationId("RequestId")          // Add correlation ID
mtlog.WithProperty("Version", "1.0.0")        // Static property
mtlog.WithSourceContext()                      // Auto-detect source context (cached)
mtlog.WithSourceContext("MyApp.Services")      // Explicit source context
```

## Filters

```go
mtlog.WithMinimumLevel(core.WarningLevel)     // Level filter

// Minimum level overrides by source context
mtlog.WithMinimumLevelOverrides(map[string]core.LogEventLevel{
    "github.com/gin-gonic/gin":     core.WarningLevel,  // Only warnings from Gin
    "myapp/internal/services":      core.DebugLevel,    // Debug for services
    "myapp/internal/services/auth": core.VerboseLevel,  // Verbose for auth
})

mtlog.WithFilter(filters.NewPredicateFilter(func(e *core.LogEvent) bool {
    return !strings.Contains(e.MessageTemplate.Text, "health")
}))
mtlog.WithFilter(filters.NewRateLimitFilter(100, time.Minute))    // Rate limiting
mtlog.WithFilter(filters.NewSamplingFilter(0.1))                 // 10% sampling
```

## Dynamic Level Control

### Manual Control
```go
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)
logger := mtlog.New(mtlog.WithLevelSwitch(levelSwitch))

levelSwitch.SetLevel(core.DebugLevel)          // Change level
level := levelSwitch.Level()                   // Get current level
enabled := levelSwitch.IsEnabled(core.DebugLevel)  // Check if enabled
```

### Seq Integration
```go
options := mtlog.SeqLevelControllerOptions{
    CheckInterval: 30 * time.Second,
    InitialCheck: true,
}
loggerOption, levelSwitch, controller := mtlog.WithSeqLevelControl(
    "http://localhost:5341", options)
defer controller.Close()
```

## Context Logging

### With() Method (Structured Fields)

```go
// Basic usage with key-value pairs
logger.With("service", "api", "version", "1.0").Info("Service started")

// Chaining With() calls
logger.
    With("environment", "production").
    With("region", "us-west-2").
    Info("Deployment complete")

// Create a base logger with common fields
apiLogger := logger.With(
    "component", "api",
    "host", "api-server-01",
)

// Reuse the base logger
apiLogger.Info("Handling request")
apiLogger.With("endpoint", "/users").Info("GET /users")

// Request-scoped logging
requestLogger := apiLogger.With(
    "request_id", "abc-123",
    "user_id", 456,
)
requestLogger.Info("Request started")
requestLogger.With("duration_ms", 42).Info("Request completed")
```

### ForContext() Method

```go
// Add single context property
contextLogger := logger.ForContext("RequestId", "abc-123")
contextLogger.Information("Processing request")

// Multiple properties (variadic)
contextLogger := logger.ForContext("UserId", 123, "SessionId", "xyz")

// Source context for sub-loggers
serviceLogger := logger.ForSourceContext("MyApp.Services.UserService")
serviceLogger.Information("User service initialized")
```

### With() vs ForContext()

- **With()**: Accepts variadic key-value pairs (slog-style), convenient for multiple fields
- **ForContext()**: Takes property name and value(s), returns a new logger
- Both methods create a new logger instance with the combined properties

## Configuration from JSON

```go
config, err := configuration.LoadFromFile("logging.json")
logger := config.CreateLogger()
```

Example config:
```json
{
    "minimumLevel": "Information",
    "sinks": [
        {"type": "Console", "theme": "dark"},
        {"type": "Seq", "serverUrl": "http://localhost:5341"}
    ],
    "enrichers": ["Timestamp", "MachineName"]
}
```

## LogValue Interface

```go
type User struct {
    ID       int
    Username string
    Password string // sensitive
}

func (u User) LogValue() interface{} {
    return map[string]interface{}{
        "id":       u.ID,
        "username": u.Username,
        // Password omitted for security
    }
}
```

## Performance Tips

1. **Use IsEnabled() for expensive operations:**
```go
if logger.IsEnabled(core.VerboseLevel) {
    data := expensiveSerialize(object)
    logger.Verbose("Data: {@Data}", data)
}
```

2. **Use async sinks for network destinations:**
```go
mtlog.WithAsync(mtlog.WithSeq("http://localhost:5341"))
```

3. **Enable durable buffering for critical logs:**
```go
mtlog.WithDurable(mtlog.WithElasticsearch("http://localhost:9200", "logs"))
```

4. **Use appropriate batch sizes:**
```go
mtlog.WithSeqAdvanced("http://localhost:5341",
    sinks.WithSeqBatchSize(100),           // Good for most cases
    sinks.WithSeqBatchTimeout(5*time.Second),
)
```

## Common Patterns

### Web Application
```go
logger := mtlog.New(
    mtlog.WithConsoleTheme("dark"),               // Development console
    mtlog.WithSeq("http://seq:5341", apiKey),     // Centralized logging
    mtlog.WithTimestamp(),                        // Always include time
    mtlog.WithMachineName(),                      // Identify server
    mtlog.WithMinimumLevel(core.InformationLevel),
)

// In handlers
func handleRequest(w http.ResponseWriter, r *http.Request) {
    reqLogger := logger.ForContext("RequestId", generateID())
    reqLogger.Information("Processing {Method} {Path}", r.Method, r.URL.Path)
    // ... handle request
}
```

### Microservice
```go
logger := mtlog.New(
    mtlog.WithAsync(mtlog.WithSeq("http://seq:5341")),    // Async for performance
    mtlog.WithDurable(mtlog.WithFileSink("service.log")), // Durable backup
    mtlog.WithProperty("Service", "payment-service"),      // Service identity
    mtlog.WithProperty("Version", version),                // Version tracking
    mtlog.WithTimestamp(),
    mtlog.WithMachineName(),
)
```

### Development
```go
logger := mtlog.New(
    mtlog.WithConsoleTheme("dark"),
    mtlog.WithMinimumLevel(core.VerboseLevel),     // See everything
    mtlog.WithCallersInfo(),                       // File/line info
)
```

### Production
```go
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)
logger := mtlog.New(
    mtlog.WithLevelSwitch(levelSwitch),            // Runtime control
    mtlog.WithAsync(mtlog.WithSeq("http://seq:5341")),
    mtlog.WithDurable(mtlog.WithFileSink("app.log")),
    mtlog.WithTimestamp(),
    mtlog.WithMachineName(),
    mtlog.WithProcessInfo(),
)

// Setup level controller for runtime adjustment
controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)
defer controller.Close()
```

## Error Handling

```go
// Log errors with context
func processOrder(orderID string) error {
    logger.Information("Processing order {OrderId}", orderID)
    
    order, err := repository.GetOrder(orderID)
    if err != nil {
        logger.Error("Failed to retrieve order {OrderId}: {Error}", orderID, err)
        return fmt.Errorf("order retrieval failed: %w", err)
    }
    
    // Process order...
    logger.Information("Order {OrderId} processed successfully", orderID)
    return nil
}
```

## Testing

```go
func TestLogging(t *testing.T) {
    // Use memory sink for testing
    memorySink := sinks.NewMemorySink()
    logger := mtlog.New(mtlog.WithSink(memorySink))
    
    logger.Information("Test message")
    
    events := memorySink.Events()
    if len(events) != 1 {
        t.Errorf("Expected 1 event, got %d", len(events))
    }
    
    if events[0].MessageTemplate != "Test message" {
        t.Errorf("Unexpected message: %s", events[0].MessageTemplate)
    }
}
```

## Static Analysis

### mtlog-analyzer

Static analysis tool that catches common mistakes at compile time:

```bash
# Install
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest

# Run with go vet
go vet -vettool=$(which mtlog-analyzer) ./...
```

### Common Diagnostics

```go
// MTLOG001: Template/argument mismatch
log.Info("User {Id} from {IP}", userId)  // ❌ Missing IP argument

// MTLOG003: Duplicate properties
log.Info("{Id} and {Id}", 1, 2)  // ❌ Duplicate 'Id'
log.With("id", 1, "id", 2)       // ❌ Duplicate key in With()

// MTLOG009: With() odd arguments
log.With("key1", "val1", "key2")  // ❌ Missing value

// MTLOG010: With() non-string key
log.With(123, "value")  // ❌ Key must be string

// MTLOG011: Cross-call duplicate
logger := log.With("service", "api")
logger.With("service", "auth")  // ⚠️ Overrides 'service'

// MTLOG013: Empty key
log.With("", "value")  // ❌ Empty key ignored
```

### IDE Integration

- **VS Code**: Install [mtlog-analyzer extension](https://marketplace.visualstudio.com/items?itemName=mtlog.mtlog-analyzer)
- **GoLand**: Install [mtlog-analyzer plugin](https://plugins.jetbrains.com/plugin/24877-mtlog-analyzer)
- **Neovim**: Use [mtlog.nvim plugin](https://github.com/willibrandon/mtlog/tree/main/neovim-plugin)