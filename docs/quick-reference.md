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

### Event Routing
```go
// Conditional sink - zero overhead for non-matching events
alertSink := sinks.NewConditionalSink(
    func(e *core.LogEvent) bool { 
        return e.Level >= core.ErrorLevel && e.Properties["Alert"] != nil 
    },
    sinks.NewFileSink("alerts.log"),
)

// Built-in predicates
sinks.LevelPredicate(core.ErrorLevel)                    // Level filtering
sinks.PropertyPredicate("Audit")                         // Property exists
sinks.PropertyValuePredicate("Environment", "production") // Property value
sinks.AndPredicate(pred1, pred2, pred3)                  // All must match
sinks.OrPredicate(pred1, pred2)                          // Any matches
sinks.NotPredicate(pred)                                 // Invert predicate

// Router sink - FirstMatch mode (exclusive routing)
router := sinks.NewRouterSink(sinks.FirstMatch,
    sinks.ErrorRoute("errors", errorSink),
    sinks.AuditRoute("audit", auditSink),
)

// Router sink - AllMatch mode (broadcast routing)  
router := sinks.NewRouterSink(sinks.AllMatch,
    sinks.MetricRoute("metrics", metricsSink),
    sinks.AuditRoute("audit", auditSink),
)

// Dynamic route management
router.AddRoute(sinks.Route{
    Name:      "debug",
    Predicate: func(e *core.LogEvent) bool { return e.Level <= core.DebugLevel },
    Sink:      debugSink,
})
router.RemoveRoute("debug")

// Fluent route builder
route := sinks.NewRoute("special").
    When(func(e *core.LogEvent) bool { return e.Properties["Special"] != nil }).
    To(specialSink)
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

## Per-Message Sampling

### Basic Sampling
```go
// Sample every Nth message
sampledLogger := logger.Sample(10)  // Every 10th message

// Time-based sampling
sampledLogger := logger.SampleDuration(time.Second)  // At most once per second

// Rate-based sampling (percentage)
sampledLogger := logger.SampleRate(0.1)  // 10% of messages

// First N occurrences
sampledLogger := logger.SampleFirst(100)  // First 100 messages only
```

### Advanced Sampling
```go
// Group sampling - share counter across loggers
dbLogger := logger.SampleGroup("database", 10)
cacheLogger := logger.SampleGroup("database", 10)  // Same counter

// Conditional sampling
var highLoad atomic.Bool
sampledLogger := logger.SampleWhen(func() bool {
    return highLoad.Load()
}, 5)  // Every 5th when condition true

// Exponential backoff
errorLogger := logger.SampleBackoff("connection-error", 2.0)
// Logs at: 1st, 2nd, 4th, 8th, 16th, 32nd...
```

### Configuration
```go
// Default sampling for all messages
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithDefaultSampling(100),  // Every 100th by default
)

// Reset sampling counters
sampledLogger.ResetSampling()
logger.ResetSamplingGroup("database")

// Sampling statistics
sampledLogger.EnableSamplingSummary(5 * time.Minute)
sampled, skipped := sampledLogger.GetSamplingStats()

// Cache warmup (at startup)
mtlog.WarmupSamplingGroups([]string{"database", "api"})
mtlog.WarmupSamplingBackoff([]string{"error", "timeout"}, 2.0)
```

### Advanced Sampling Configuration

#### Predefined Sampling Profiles

Ready-to-use sampling profiles for common production scenarios:

```go
// High-traffic API endpoints (1% sampling)
apiLogger := logger.SampleProfile("HighTrafficAPI")

// Background workers (10% sampling)
workerLogger := logger.SampleProfile("BackgroundWorker")

// Error logging with exponential backoff
errorLogger := logger.SampleProfile("ErrorReporting")

// Debug mode with higher sampling (25%)
debugLogger := logger.SampleProfile("DebugVerbose")

// Interactive user actions (50% sampling)
userLogger := logger.SampleProfile("UserInteractive")

// Database operations (every 5th message)
dbLogger := logger.SampleProfile("DatabaseOps")

// Analytics events (5% sampling)
analyticsLogger := logger.SampleProfile("Analytics")

// System health monitoring (time-based, once per second)
healthLogger := logger.SampleProfile("SystemHealth")
```

#### Adaptive Sampling

Automatically adjusts sampling rates to maintain target throughput:

```go
// Target 100 events per second - automatically adjusts sampling rate
adaptiveLogger := logger.SampleAdaptive(100)

// Advanced adaptive sampling with bounds
adaptiveLogger := logger.SampleAdaptiveWithOptions(
    250,                    // Target: 250 events/second
    0.01,                   // Minimum rate: 1%
    1.0,                    // Maximum rate: 100%
    30*time.Second,         // Check interval
)

// Advanced adaptive sampling with hysteresis for stability
hysteresisLogger := logger.SampleAdaptiveWithHysteresis(
    200,                    // Target: 200 events/second
    0.005,                  // Minimum rate: 0.5%
    0.8,                    // Maximum rate: 80%
    15*time.Second,         // Check interval
    0.15,                   // Hysteresis: 15% (prevents oscillation)
    0.7,                    // Aggressiveness: 70% (smoother adjustments)
)

// Ultimate adaptive sampling with dampening for extreme load
dampenedLogger := logger.SampleAdaptiveWithDampening(
    200,                    // Target: 200 events/second
    0.005,                  // Minimum rate: 0.5%
    0.8,                    // Maximum rate: 80%
    15*time.Second,         // Check interval
    0.15,                   // Hysteresis: 15% (prevents oscillation)
    0.7,                    // Aggressiveness: 70% (smoother adjustments)
    0.4,                    // Dampening: 40% (reduces oscillation)
)

// Simplified adaptive sampling with dampening presets
conservativeLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningConservative)
moderateLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningModerate)
aggressiveLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningAggressive)
ultraStableLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningUltraStable)
responsiveLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningResponsive)

// Custom rate limits with presets
customLogger := logger.SampleAdaptiveWithPresetCustom(150, mtlog.DampeningAggressive, 0.05, 0.8)

// Adaptive sampling automatically:
// - Measures actual event rate
// - Increases sampling when below target
// - Decreases sampling when above target
// - Uses hysteresis to prevent rate oscillation
// - Applies exponential smoothing for stability
// - Stays within configured min/max bounds
```

#### Fluent Sampling Configuration Builder

For complex scenarios, combine multiple sampling strategies:

```go
// Pipeline-style sampling (filters applied in sequence)
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Every(10).       // First: sample every 10th message
        Rate(0.5).       // Then: 50% of those that pass
        First(100).      // Finally: only first 100 that make it through
        Build(),         // Apply as sequential pipeline
)

// Composite AND sampling (all conditions must match)
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Every(2).        // Must be every 2nd message
        First(10).       // Must be within first 10 evaluations
        CombineAND(),    // Both conditions must be true
)

// Composite OR sampling (any condition can match)
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Every(5).        // Either every 5th message
        First(3).        // Or first 3 messages
        CombineOR(),     // Either condition allows logging
)
```

#### Custom Sampling Profiles

Create application-specific sampling profiles:

```go
// Define custom profiles for your application
customProfiles := map[string]mtlog.SamplingProfile{
    "PaymentProcessing": {
        Description: "Critical payment operations - log all errors, sample others",
        Config: func() mtlog.Option {
            return mtlog.Sampling().
                When(func() bool { return getCurrentErrorRate() > 0.01 }, 1). // All errors
                Rate(0.1).                                                     // 10% normal ops
                CombineOR()
        },
    },
    "UserAnalytics": {
        Description: "User behavior tracking",
        Config: func() mtlog.Option {
            return mtlog.Sampling().
                First(1000).     // First 1000 events per user
                Rate(0.05).      // Then 5% sampling
                Build()
        },
    },
}

// Register and use custom profiles
mtlog.RegisterSamplingProfiles(customProfiles)

// Bulk register multiple profiles with error handling
if err := mtlog.RegisterCustomProfiles(customProfiles); err != nil {
    log.Fatal("Failed to register sampling profiles:", err)
}

// Freeze profile registry after registration (recommended for production)
mtlog.FreezeProfiles()

// Use custom profiles
paymentLogger := logger.SampleProfile("PaymentProcessing")

// Profile versioning for backward compatibility
mtlog.AddCustomProfileWithVersion("PaymentV2", "Enhanced payment processing", "2.0", false, "", 
    func() core.LogEventFilter { return mtlog.Sampling().Rate(0.05).Build() })

// Use specific version
legacyPayment := logger.SampleProfileWithVersion("PaymentV2", "1.0")
modernPayment := logger.SampleProfileWithVersion("PaymentV2", "2.0")

// Version management
versions := mtlog.GetProfileVersions("PaymentV2")
isDeprecated, replacement := mtlog.IsProfileDeprecated("PaymentV2")

// Profile version auto-migration
mtlog.SetMigrationPolicy(mtlog.MigrationPolicy{
    Consent:            mtlog.MigrationAuto,  // Auto-migrate without prompting
    PreferStable:       true,                // Skip deprecated versions
    MaxVersionDistance: 1,                   // Allow migration within 1 major version
})

// Request version that might not exist - auto-migrates to compatible version
profile, actualVersion, found := mtlog.GetProfileWithMigration("PaymentV2", "1.5")
migratedLogger := logger.SampleProfileWithVersion("PaymentV2", "1.3") // Auto-migrates if needed
```

#### Custom Sampling Policies

```go
// Implement SamplingPolicy interface for complex logic
type UserBasedSamplingPolicy struct {
    adminRate   float32
    premiumRate float32  
    basicRate   float32
}

func (p *UserBasedSamplingPolicy) ShouldSample(event *core.LogEvent) bool {
    userTier, _ := event.Properties["UserTier"].(string)
    switch userTier {
    case "admin":   return true
    case "premium": return rand.Float32() < p.premiumRate
    case "basic":   return rand.Float32() < p.basicRate
    default:        return false
    }
}

// Use the custom policy
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithSamplingPolicy(&UserBasedSamplingPolicy{
        adminRate: 1.0, premiumRate: 0.5, basicRate: 0.1,
    }),
)
```

#### Pipeline vs Composite Behavior

```go
// Pipeline (Build): Filters applied sequentially
// Each filter only sees events that passed the previous filter
mtlog.Sampling().Every(2).First(5).Build()

// Composite (CombineAND): Each filter evaluates all events independently
// Results combined with logical AND/OR
mtlog.Sampling().Every(2).First(5).CombineAND()
```

### Production Pattern
```go
// Different sampling for different endpoints
healthLogger := logger.
    ForContext("Endpoint", "/health").
    SampleDuration(10 * time.Second)  // Once per 10 seconds

apiLogger := logger.
    ForContext("Endpoint", "/api/users").
    SampleRate(0.01)  // 1% sampling

errorLogger := logger.
    SampleBackoff("api-error", 2.0)  // Exponential backoff
```

## Context Logging

### Context-Aware Methods

```go
// All logging methods have context-aware variants
logger.VerboseContext(ctx, "Verbose message")
logger.DebugContext(ctx, "Debug: {Value}", value)
logger.InfoContext(ctx, "Info: {User} {Action}", user, action)
logger.WarnContext(ctx, "Warning: {Count} items", count)
logger.ErrorContext(ctx, "Error: {Error}", err)
logger.FatalContext(ctx, "Fatal: {Reason}", reason)
```

### Context Deadline Awareness

```go
// Basic configuration - warn when within 100ms of deadline
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithContextDeadlineWarning(100*time.Millisecond),
)

// Percentage-based threshold - warn when 20% of time remains
logger := mtlog.New(
    mtlog.WithDeadlinePercentageThreshold(
        1*time.Millisecond,  // Min absolute threshold
        0.2,                 // 20% threshold
    ),
)

// Usage with timeout context
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

logger.InfoContext(ctx, "Starting operation")
time.Sleep(350 * time.Millisecond)
logger.InfoContext(ctx, "Still processing...") // WARNING: Deadline approaching!

// Advanced options
import "github.com/willibrandon/mtlog/internal/enrichers"

logger := mtlog.New(
    mtlog.WithContextDeadlineWarning(50*time.Millisecond,
        enrichers.WithDeadlineCustomHandler(func(event *core.LogEvent, remaining time.Duration) {
            // Custom logic when deadline approaches
            metrics.RecordDeadlineApproaching(remaining)
        }),
        enrichers.WithDeadlineCacheSize(1000),
        enrichers.WithDeadlineCacheTTL(5*time.Minute),
    ),
)

// Properties added when approaching deadline:
// - deadline.approaching: true
// - deadline.remaining_ms: 95
// - deadline.at: "2024-01-15T10:30:45Z"
// - deadline.first_warning: true

// Properties added when deadline exceeded:
// - deadline.exceeded: true
// - deadline.exceeded_by_ms: 150
```

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

## HTTP Middleware

### Basic Setup

```go
import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/adapters/middleware"
)

logger := mtlog.New(mtlog.WithConsole())

// net/http
mw := middleware.Middleware(middleware.DefaultOptions(logger))
handler := mw(yourHandler)

// Gin
router.Use(middleware.Gin(logger))

// Echo
e.Use(middleware.Echo(logger))

// Fiber
app.Use(middleware.Fiber(logger))

// Chi
r.Use(middleware.Chi(logger))
```

### Configuration Options

```go
options := &middleware.Options{
    Logger:            logger,
    GenerateRequestID: true,
    RequestIDHeader:   "X-Request-ID",
    SkipPaths:         []string{"/health", "/metrics"},
    RequestFields:     []string{"method", "path", "ip", "user_agent"},
    LatencyField:      "duration_ms",
    LatencyUnit:      "ms",
    
    // Body logging
    LogRequestBody:   true,
    LogResponseBody:  true,
    MaxBodySize:      4096,
    BodySanitizer:    middleware.DefaultBodySanitizer,
    
    // Sampling
    Sampler: middleware.NewPathSamplerBuilder().
        Never("/health").
        Sometimes("/api/status", 0.1).
        Always("*").
        Build(),
        
    // Custom fields
    CustomFields: []middleware.FieldExtractor{
        middleware.UserIDFromHeader,
        middleware.TraceIDFromContext,
    },
    
    // Metrics
    MetricsRecorder: myMetricsRecorder,
}

mw := middleware.Middleware(options)
```

### Sampling Strategies

```go
// Rate-based sampling (10% of requests)
sampler := middleware.NewRateSampler(0.1)

// Adaptive sampling (target 100 logs/second)
sampler := middleware.NewAdaptiveSampler(100)

// Path-based sampling with patterns
sampler := middleware.NewPathSamplerBuilder().
    Never("/health*").
    Sometimes("/api/status", 0.1).
    Always("/api/*/debug").
    Sometimes("*", 0.5).
    Build()

// Composite sampling (AND/OR logic)
sampler := middleware.NewCompositeSampler(
    middleware.CompositeAND,
    middleware.NewRateSampler(0.5),
    middleware.NewPathSampler(rules),
)
```

### Body Sanitization

```go
// Default sanitizer (redacts passwords, tokens, etc.)
options.BodySanitizer = middleware.DefaultBodySanitizer

// Custom regex sanitizer
options.BodySanitizer = middleware.RegexBodySanitizer(
    regexp.MustCompile(`"credit_card":\s*"[^"]+"`),
    regexp.MustCompile(`"ssn":\s*"[^"]+"`),
)

// Function-based sanitizer
options.BodySanitizer = func(body []byte, contentType string) []byte {
    // Custom sanitization logic
    return sanitizedBody
}
```

### Request Logger Helper

```go
func handler(w http.ResponseWriter, r *http.Request) {
    reqLogger := middleware.GetRequestLogger(r).
        WithUser("user-123").
        WithOperation("CreateOrder").
        WithResource("Order", "ord-456")
    
    reqLogger.Information("Processing order creation")
    
    if err := processOrder(); err != nil {
        reqLogger.WithError(err).Error("Order creation failed")
    }
}
```

### Context Helpers

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

### Health Check Handlers

```go
// Basic health check handler
healthHandler := middleware.NewHealthCheckHandler(logger).
    WithVersion("1.0.0").
    WithEnvironment("production").
    WithMetrics(true)

// Add custom checks
healthHandler.AddCheck("database", func() middleware.Check {
    if err := db.Ping(); err != nil {
        return middleware.Check{
            Status: "unhealthy",
            Error:  err.Error(),
        }
    }
    return middleware.Check{Status: "healthy"}
})

// Use as HTTP handler
http.Handle("/health", healthHandler)

// Simple liveness/readiness handlers
http.HandleFunc("/liveness", middleware.LivenessHandler())
http.HandleFunc("/readiness", middleware.ReadinessHandler(
    middleware.DatabaseHealthChecker("postgres", db.Ping),
    middleware.HTTPHealthChecker("api", "http://api:8080/health", 5*time.Second),
))
```

### Performance with Object Pooling

```go
// Pooling is enabled by default, can be controlled globally
middleware.EnablePooling = true

// Get pool statistics
stats := middleware.GetPoolStats()
fmt.Printf("Error pool hits: %d\n", stats.ErrorPoolHits)

// Reset statistics
middleware.ResetPoolStats()

// Batch metrics for high-throughput
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