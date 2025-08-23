# mtlog - Message Template Logging for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/willibrandon/mtlog.svg)](https://pkg.go.dev/github.com/willibrandon/mtlog)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

mtlog is a high-performance structured logging library for Go, inspired by [Serilog](https://serilog.net/). It brings message templates and pipeline architecture to the Go ecosystem, achieving zero allocations for simple logging operations while providing powerful features for complex scenarios.

## Features

### Core Features
- **Zero-allocation logging** for simple messages (17.3 ns/op)
- **Message templates** with positional property extraction and format specifiers
- **Go template syntax** support (`{{.Property}}`) alongside traditional syntax
- **OpenTelemetry compatibility** with support for dotted property names (`{http.method}`, `{service.name}`)
- **Structured fields** via `With()` method for slog-style key-value pairs
- **Output templates** for customizable log formatting
- **ForType logging** with automatic SourceContext from Go types and intelligent caching
- **LogContext scoped properties** that flow through operation contexts
- **Source context enrichment** with intelligent caching for automatic logger categorization
- **Pipeline architecture** for clean separation of concerns
- **Type-safe generics** for better compile-time safety
- **LogValue interface** for safe logging of sensitive data
- **SelfLog diagnostics** for debugging silent failures and configuration issues
- **Standard library compatibility** via slog.Handler adapter (Go 1.21+)
- **Kubernetes ecosystem** support via logr.LogSink adapter

### HTTP Middleware
- **Multi-framework support** for net/http, Gin, Echo, Fiber, and Chi
- **High-performance request/response logging** with minimal overhead (~2.3μs per request)
- **Object pooling** for zero-allocation paths in high-throughput scenarios
- **Advanced sampling strategies** (rate, adaptive, path-based) for log volume control
- **Request body logging** with automatic sanitization of sensitive data
- **Distributed tracing** with W3C Trace Context, B3, and X-Ray format support
- **Health check handlers** with configurable metrics and liveness/readiness probes
- **Panic recovery** with detailed stack traces and custom error handling

### Sinks & Output
- **Console sink** with customizable themes (dark, light, ANSI, Literate)
- **File sink** with rolling policies (size, time-based)
- **Seq integration** with CLEF format and dynamic level control
- **Elasticsearch sink** for centralized log storage and search
- **Splunk sink** with HEC (HTTP Event Collector) support
- **OpenTelemetry (OTLP) sink** with gRPC/HTTP transport, batching, and trace correlation
- **Conditional sink** for predicate-based routing with zero overhead
- **Router sink** for multi-destination routing with FirstMatch/AllMatch modes
- **Async sink wrapper** for high-throughput scenarios
- **Durable buffering** with persistent storage for reliability

### Pipeline Components
- **Rich enrichment** with built-in and custom enrichers
- **Advanced filtering** including rate limiting and sampling
- **Minimum level overrides** by source context patterns
- **Type-safe capturing** with caching for performance
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
        mtlog.WithConsole(),
        mtlog.WithMinimumLevel(core.InformationLevel),
    )

    // Simple logging
    log.Info("Application started")
    
    // Message templates with properties
    userId := 123
    log.Info("User {UserId} logged in", userId)
    
    // Capturing complex types
    order := Order{ID: 456, Total: 99.95}
    log.Info("Processing {@Order}", order)
}

// For libraries that need error handling:
func NewLibraryLogger() (*mtlog.Logger, error) {
    return mtlog.Build(
        mtlog.WithConsoleTemplate("[${Timestamp:HH:mm:ss} ${Level:u3}] ${Message}"),
        mtlog.WithMinimumLevel(core.DebugLevel),
    )
}
```

## Visual Example

![mtlog with Literate theme](assets/literate-theme.png)

## Message Templates

mtlog uses message templates that preserve structure throughout the logging pipeline:

```go
// Properties are extracted positionally
log.Information("User {UserId} logged in from {IP}", userId, ipAddress)

// Go template syntax is also supported
log.Information("User {{.UserId}} logged in from {{.IP}}", userId, ipAddress)

// OTEL-style dotted properties for compatibility with OpenTelemetry conventions
log.Information("HTTP {http.method} to {http.url} returned {http.status_code}", "GET", "/api", 200)
log.Information("Service {service.name} in {service.namespace}", "api", "production")

// Mix both syntaxes as needed
log.Information("User {UserId} ({{.Username}}) from {IP}", userId, username, ipAddress)

// Capturing hints:
// @ - capture complex types into properties
log.Information("Order {@Order} created", order)

// $ - force scalar rendering (stringify)
log.Information("Error occurred: {$Error}", err)

// Format specifiers
log.Information("Processing time: {Duration:F2}ms", 123.456)
log.Information("Disk usage at {Percentage:P1}", 0.85)  // 85.0%
log.Information("Order {OrderId:000} total: ${Amount:F2}", 42, 99.95)

// String formatting - default is no quotes (Go-idiomatic)
log.Information("User {Name} logged in", "Alice")  // User Alice logged in
log.Information("User {Name:q} logged in", "Alice")  // User "Alice" logged in (explicit quotes)
log.Information("User {Name:l} logged in", "Alice")  // User Alice logged in (same as default)

// JSON formatting - outputs any value as JSON
log.Information("Config: {Settings:j}", map[string]any{"debug": true, "port": 8080})
// Config: {"debug":true,"port":8080}

// Numeric indexing (like string.Format in .NET)
log.Information("Processing {0} of {1} items", 5, 10)
log.Information("The {0} {1} {2} jumped over the {3} {4}", 
    "quick", "brown", "fox", "lazy", "dog")
```

### Numeric Indexing

mtlog supports numeric indexing similar to .NET's `string.Format` and Serilog:

```go
// Pure numeric indexing uses index values (like string.Format)
log.Information("Processing {0} of {1}", 5, 10)  // Processing 5 of 10
log.Information("Result: {1} before {0}", "first", "second")  // Result: second before first

// Mixed named and numeric uses positional matching (left-to-right)
log.Information("User {UserId} processed {0} of {1}", 123, 50, 100)
// UserId=123 (1st arg), 0=50 (2nd arg), 1=100 (3rd arg)

// Note: Avoid mixing named and numeric properties for clarity
```

## Output Templates

Control how log events are formatted for output with customizable templates. Output templates use `${...}` syntax for built-in elements to distinguish them from message template properties:

```go
// Console with custom output template and theme
log := mtlog.New(
    mtlog.WithConsoleTemplateAndTheme(
        "[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}",
        sinks.LiterateTheme(),
    ),
)

// File with detailed template
log := mtlog.New(
    mtlog.WithFileTemplate("app.log", 
        "[${Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} ${Level:u3}] {SourceContext}: ${Message}${NewLine}${Exception}"),
)
```

### Template Properties
- `${Timestamp}` - Event timestamp with optional format
- `${Level}` - Log level with format options (u3, u, l)
- `${Message}` - Rendered message from template
- `{SourceContext}` - Logger context/category
- `${Exception}` - Exception details if present
- `${NewLine}` - Platform-specific line separator
- Custom properties by name: `{RequestId}`, `{UserId}`, etc.

### Format Specifiers
- **Timestamps**: `HH:mm:ss`, `yyyy-MM-dd`, `HH:mm:ss.fff`
- **Levels**: 
  - `u3` or `w3` - Three-letter uppercase (INF, WRN, ERR)
  - `u` - Full uppercase (INFORMATION, WARNING, ERROR)
  - `w` - Full lowercase (information, warning, error)
  - `l` - Lowercase three-letter (inf, wrn, err) [deprecated, use w3]
- **Numbers**: `000` (zero-pad), `F2` (2 decimals), `P1` (percentage)
- **Strings**: `:l` removes quotes (literal format)

### Design Note: Why `${...}` for Built-ins?

Unlike Serilog which uses `{...}` for both built-in elements and properties in output templates, mtlog uses `${...}` for built-ins. This design choice prevents ambiguity when user properties have the same names as built-in elements (e.g., a property named "Message" would conflict with the built-in {Message}).

The `${...}` syntax provides clear disambiguation:
- `${Message}`, `${Timestamp}`, `${Level}` - Built-in template elements
- `{UserId}`, `{OrderId}`, `{Message}` - User properties from your log events

This means you can safely log a property called "Message" without conflicts:
```go
log.Information("Processing {Message} from {Queue}", userMessage, queueName)
// Output template: "[${Timestamp}] ${Level}: ${Message}"
// Result: "[2024-01-15] INF: Processing Hello World from orders"
```

## Pipeline Architecture

The logging pipeline processes events through distinct stages:

```
Message Template Parser → Enrichment → Filtering → Capturing → Output
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
    
    // Capturing
    mtlog.WithCapturing(),          // Enable @ hints
    mtlog.WithCapturingDepth(5),    // Max depth
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
    mtlog.WithSourceContext(), // Auto-detect logger context
)

// Structured fields with With() - slog-style
log.With("service", "auth", "version", "1.0").Info("Service started")
log.With("user_id", 123, "request_id", "abc-123").Info("Processing request")

// Context-based enrichment
ctx := context.WithValue(context.Background(), "RequestId", "abc-123")
log.ForContext("UserId", userId).Information("Processing request")

// Source context for sub-loggers
serviceLog := log.ForSourceContext("MyApp.Services.UserService")
serviceLog.Information("User service started")

// Type-based loggers with automatic SourceContext
userLogger := mtlog.ForType[User](log)
userLogger.Information("User operation") // SourceContext: "User"

orderLogger := mtlog.ForType[OrderService](log)
orderLogger.Information("Processing order") // SourceContext: "OrderService"
```

## Per-Message Sampling

mtlog provides fine-grained per-message sampling to manage log volume for high-frequency events while preserving important logs. This feature allows different sampling strategies for different log statements, giving you precise control over what gets logged in production.

### Basic Sampling Methods

#### Sample Every Nth Message
```go
// Log every 10th message
sampledLogger := logger.Sample(10)
for i := 1; i <= 100; i++ {
    sampledLogger.Info("Processing item {Number}", i)
}
// Logs messages: 1, 11, 21, 31, 41, 51, 61, 71, 81, 91
```

#### Time-Based Sampling
```go
// Log at most once per second
sampledLogger := logger.SampleDuration(time.Second)
for i := 0; i < 1000; i++ {
    sampledLogger.Info("Rapid event {Number}", i)
    time.Sleep(10 * time.Millisecond)
}
// Logs first message, then one message per second
```

#### Rate-Based Sampling
```go
// Log 10% of messages
sampledLogger := logger.SampleRate(0.1)
for i := 1; i <= 1000; i++ {
    sampledLogger.Info("High volume event {EventId}", i)
}
// Logs approximately 100 messages (10% of 1000)
```

#### First N Occurrences
```go
// Log only the first 5 occurrences
sampledLogger := logger.SampleFirst(5)
for i := 1; i <= 100; i++ {
    sampledLogger.Warning("Initialization warning {Step}", i)
}
// Logs only messages 1, 2, 3, 4, 5
```

### Advanced Sampling

#### Group Sampling
Share sampling counters across multiple loggers:

```go
// Multiple loggers share the same sampling counter
dbLogger := logger.SampleGroup("database", 10)
cacheLogger := logger.SampleGroup("database", 10)

// Both loggers contribute to the same counter
for i := 1; i <= 20; i++ {
    dbLogger.Info("DB query {Id}", i)      // Messages 1, 11
    cacheLogger.Info("Cache hit {Id}", i)  // Messages 6, 16
}
```

#### Conditional Sampling
Sample only when specific conditions are met:

```go
var highLoad atomic.Bool

// Sample every 5th message only during high load
sampledLogger := logger.SampleWhen(func() bool {
    return highLoad.Load()
}, 5)

// Normal load - no messages logged
sampledLogger.Info("Normal operation")

// High load - sampling active
highLoad.Store(true)
for i := 1; i <= 20; i++ {
    sampledLogger.Info("High load event {Number}", i)
}
// Logs messages: 1, 6, 11, 16 (every 5th during high load)
```

#### Exponential Backoff Sampling
Reduce frequency of repeated messages exponentially:

```go
// Log with exponential backoff (factor 2.0)
errorLogger := logger.SampleBackoff("connection-error", 2.0)

for i := 1; i <= 100; i++ {
    errorLogger.Error("Connection failed, attempt {Attempt}", i)
}
// Logs attempts: 1, 2, 4, 8, 16, 32, 64
```

### Production Example

Different sampling strategies for different scenarios:

```go
baseLogger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithMinimumLevel(core.InformationLevel),
)

// Health check endpoint - heavily rate limited
healthLogger := baseLogger.
    ForContext("Endpoint", "/health").
    SampleDuration(10 * time.Second) // At most once per 10 seconds

// High-traffic read endpoint - sample percentage
apiLogger := baseLogger.
    ForContext("Endpoint", "/api/users").
    SampleRate(0.01) // 1% of requests

// Debug logs - conditional sampling
debugLogger := baseLogger.
    SampleWhen(func() bool {
        return os.Getenv("DEBUG_SAMPLING") == "true"
    }, 100) // Every 100th when debugging enabled

// Error logging with backoff
errorLogger := baseLogger.
    SampleBackoff("api-error", 2.0) // Exponential backoff
```

### Configuration Options

#### Global Default Sampling
Set default sampling for all messages:

```go
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithDefaultSampling(100), // Sample every 100th message by default
)
```

#### Reset Sampling Counters
Reset sampling state when needed:

```go
// Reset all sampling counters for a logger
sampledLogger.ResetSampling()

// Reset specific group counter
logger.ResetSamplingGroup("database")
```

#### Sampling Statistics & Summaries
Monitor sampling effectiveness:

```go
// Enable periodic summary events
sampledLogger := logger.Sample(10).EnableSamplingSummary(5 * time.Minute)
// Emits: "Sampling summary for last 5m: 1000 messages logged, 9000 messages skipped"

// Get current statistics
sampled, skipped := sampledLogger.GetSamplingStats()
fmt.Printf("Sampled: %d, Skipped: %d\n", sampled, skipped)
```

#### Cache Warmup
Pre-populate caches to avoid cold-start spikes:

```go
// Warmup common group names at startup
mtlog.WarmupSamplingGroups([]string{"database", "cache", "api", "auth"})

// Warmup common error keys
mtlog.WarmupSamplingBackoff([]string{"connection-error", "timeout", "rate-limit"}, 2.0)
```

### Performance

Per-message sampling adds minimal overhead:
- Counter sampling: ~4ns per decision, 0 allocations
- Rate sampling: ~4ns per decision, 0 allocations (per-filter random source)
- Duration sampling: ~35ns per decision, 0 allocations
- All sampling decisions use lock-free atomic operations
- LRU cache prevents unbounded memory growth (default 10,000 keys)
- Cache warmup available to prevent cold-start allocation spikes

### Use Cases

- **Database Query Logging**: Sample queries in production without overwhelming logs
- **HTTP Request Logging**: Log percentage of high-traffic endpoints
- **Heartbeat/Health Checks**: Time-based sampling to reduce noise
- **Error Logging**: Exponential backoff for repeated errors
- **Debug Logging**: Conditional sampling based on environment or load
- **Initialization**: Log first N occurrences during startup

### Advanced Sampling Configuration

#### Fluent Sampling Configuration Builder

For complex scenarios, use the fluent sampling configuration API to combine multiple strategies:

```go
// Pipeline-style sampling (filters applied in sequence)
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Every(10).           // First: sample every 10th message
        Rate(0.5).           // Then: 50% of those that pass
        First(100).          // Finally: only first 100 that make it through
        Build(),             // Apply as sequential pipeline
)

// Composite AND sampling (all conditions must match)
logger := mtlog.New(
    mtlog.WithConsole(), 
    mtlog.Sampling().
        Every(2).            // Must be every 2nd message
        First(10).           // Must be within first 10 evaluations
        CombineAND(),        // Both conditions must be true
)

// Composite OR sampling (any condition can match)
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Every(5).            // Either every 5th message
        First(3).            // Or first 3 messages  
        CombineOR(),         // Either condition allows logging
)
```

#### Custom Sampling Policies

Implement the `SamplingPolicy` interface for complex custom sampling logic:

```go
// Custom policy that samples based on user properties
type UserBasedSamplingPolicy struct {
    adminRate   float32 // Always log admin users
    premiumRate float32 // Higher rate for premium users
    basicRate   float32 // Lower rate for basic users
}

func (p *UserBasedSamplingPolicy) ShouldSample(event *core.LogEvent) bool {
    userTier, _ := event.Properties["UserTier"].(string)
    switch userTier {
    case "admin":
        return true // Always log admin
    case "premium": 
        return rand.Float32() < p.premiumRate
    case "basic":
        return rand.Float32() < p.basicRate
    default:
        return false
    }
}

func (p *UserBasedSamplingPolicy) Reset() { /* reset counters */ }
func (p *UserBasedSamplingPolicy) Stats() core.SamplingStats { /* return stats */ }

// Use the custom policy
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithSamplingPolicy(&UserBasedSamplingPolicy{
        adminRate:   1.0,   // 100% for admins
        premiumRate: 0.5,   // 50% for premium
        basicRate:   0.1,   // 10% for basic
    }),
)
```

#### Pipeline vs Composite Behavior

Understanding the difference between `Build()` and `CombineAND()`/`CombineOR()`:

- **Pipeline (`Build()`)**: Filters are applied sequentially. Each filter only sees events that passed the previous filter.
- **Composite (`CombineAND()`/`CombineOR()`)**: Each filter evaluates all events independently, then the results are combined with logical AND/OR.

```go
// Pipeline: Every(2) → First(5) → Result
// First(5) only sees odd events from Every(2)
mtlog.Sampling().Every(2).First(5).Build()

// Composite: Every(2) AND First(5) → Result  
// Both filters evaluate all events, results combined with AND logic
mtlog.Sampling().Every(2).First(5).CombineAND()
```

### Predefined Sampling Profiles

For common scenarios, use predefined sampling profiles:

```go
// High-traffic API endpoints (1% sampling)
apiLogger := logger.SampleProfile("HighTrafficAPI")

// Background workers (every 10th message)
workerLogger := logger.SampleProfile("BackgroundWorker") 

// Development debug mode (first 100 then 10% sampling)
debugLogger := logger.SampleProfile("DebugVerbose")

// Production error logging (exponential backoff)
errorLogger := logger.SampleProfile("ProductionErrors")

// Health checks (time-based, once per minute)
healthLogger := logger.SampleProfile("HealthChecks")

// Critical alerts (first 50 then exponential backoff)
alertLogger := logger.SampleProfile("CriticalAlerts")

// Get available profiles
profiles := mtlog.GetAvailableProfiles()
fmt.Printf("Available profiles: %v\n", profiles)

// Get profile description
if desc, ok := mtlog.GetProfileDescription("HighTrafficAPI"); ok {
    fmt.Printf("Profile description: %s\n", desc)
}
```

### Adaptive Sampling

Automatically adjust sampling rates based on target events per second:

```go
// Target 100 events per second - automatically adjusts sampling rate
adaptiveLogger := logger.SampleAdaptive(100)

// Custom adaptive sampling with bounds and adjustment interval
adaptiveLogger := logger.SampleAdaptiveWithOptions(
    50,           // target events/second
    0.001,        // minimum rate (0.1%)
    0.5,          // maximum rate (50%)
    2*time.Second, // adjustment interval
)

// Advanced adaptive sampling with hysteresis for stability
hysteresisLogger := logger.SampleAdaptiveWithHysteresis(
    100,        // target events/second
    0.001,      // minimum rate (0.1%)
    0.3,        // maximum rate (30%)
    2*time.Second, // adjustment interval
    0.1,        // hysteresis threshold (10%) - prevents oscillation
    0.8,        // aggressiveness factor (0.0-1.0) - controls adjustment speed
)

// Ultimate adaptive sampling with dampening for extreme load variations
dampenedLogger := logger.SampleAdaptiveWithDampening(
    100,        // target events/second
    0.001,      // minimum rate (0.1%)
    0.3,        // maximum rate (30%)
    2*time.Second, // adjustment interval
    0.1,        // hysteresis threshold (10%) - prevents oscillation
    0.8,        // aggressiveness factor (0.0-1.0) - controls adjustment speed
    0.4,        // dampening factor (0.1-1.0) - reduces oscillation under extreme load
)

// Get current adaptive sampling statistics
stats := adaptiveLogger.GetAdaptiveStats()
fmt.Printf("Current rate: %.2f%%, Target: %d events/sec\n", 
    stats.CurrentRate*100, stats.TargetEventsPerSecond)
```

### Adaptive Sampling with Dampening Presets

For simplified configuration, use predefined dampening presets optimized for different environments:

```go
// Conservative: Heavy dampening for stable production (25% hysteresis, 15% aggressiveness)
conservativeLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningConservative)

// Moderate: Balanced for general production use (15% hysteresis, 30% aggressiveness)
moderateLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningModerate)

// Aggressive: Light dampening for dynamic environments (8% hysteresis, 60% aggressiveness)
aggressiveLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningAggressive)

// Ultra Stable: Maximum stability for critical systems (40% hysteresis, 5% aggressiveness)
ultraStableLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningUltraStable)

// Responsive: Minimal dampening for development (5% hysteresis, 80% aggressiveness)
responsiveLogger := logger.SampleAdaptiveWithPreset(100, mtlog.DampeningResponsive)

// Custom rate limits with presets
customLogger := logger.SampleAdaptiveWithPresetCustom(
    150,                      // target events/second
    mtlog.DampeningAggressive, // preset configuration
    0.05,                     // minimum rate (5%)
    0.8,                      // maximum rate (80%)
)
```

**Available Presets:**
- **Conservative**: For stable, predictable production environments (3s intervals, heavy dampening)
- **Moderate**: Balanced configuration suitable for most production environments (1s intervals)
- **Aggressive**: Quick response for dynamic environments (500ms intervals, light dampening)
- **Ultra Stable**: Maximum stability for critical systems (5s intervals, maximum dampening)
- **Responsive**: For development or testing environments (200ms intervals, minimal dampening)

### Custom Sampling Profiles

Add your own custom profiles for reuse across your application:

```go
// Add a custom profile
mtlog.AddCustomProfile("DatabaseQueries", 
    "Heavy sampling for database query logs",
    func() core.LogEventFilter {
        return filters.NewRateSamplingFilter(0.05) // 5% sampling
    })

// Bulk register multiple custom profiles
customProfiles := map[string]mtlog.SamplingProfile{
    "CacheOperations": {
        Name:        "Cache Operations",
        Description: "Lightweight sampling for cache operations",
        Factory: func() core.LogEventFilter {
            return filters.NewCounterSamplingFilter(50) // Every 50th operation
        },
    },
    "SecurityAudits": {
        Name:        "Security Audits", 
        Description: "Complete logging for security-related events",
        Factory: func() core.LogEventFilter {
            return filters.NewRateSamplingFilter(1.0) // 100% sampling
        },
    },
}
mtlog.RegisterCustomProfiles(customProfiles)

// Freeze profile registry to prevent further modifications (recommended for production)
mtlog.FreezeProfiles()

// Use the custom profile
dbLogger := logger.SampleProfile("DatabaseQueries")

// Add versioned profiles for backward compatibility
mtlog.AddCustomProfileWithVersion("APIGateway", 
    "API Gateway sampling with improved algorithm", 
    "2.1",          // version
    false,          // not deprecated
    "",             // no replacement needed
    func() core.LogEventFilter {
        return filters.NewRateSamplingFilter(0.02) // 2% sampling
    })

// Mark old version as deprecated
mtlog.AddCustomProfileWithVersion("APIGateway", 
    "Legacy API Gateway sampling", 
    "1.0",          // old version
    true,           // deprecated
    "2.1",          // suggest replacement
    func() core.LogEventFilter {
        return filters.NewCounterSamplingFilter(50) // Every 50th message
    })

// Use specific version of a profile
legacyLogger := logger.SampleProfileWithVersion("APIGateway", "1.0")
modernLogger := logger.SampleProfileWithVersion("APIGateway", "2.1")

// Check profile versioning information
version, exists := mtlog.GetProfileVersion("APIGateway")
versions := mtlog.GetProfileVersions("APIGateway")
isDeprecated, replacement := mtlog.IsProfileDeprecated("APIGateway")
```

### Profile Version Auto-Migration

When using versioned profiles, mtlog can automatically migrate to compatible versions when the requested version is not available:

```go
// Configure migration policy
mtlog.SetMigrationPolicy(mtlog.MigrationPolicy{
    Consent:            mtlog.MigrationAuto,  // Auto-migrate without prompting
    PreferStable:       true,                // Skip deprecated versions
    MaxVersionDistance: 1,                   // Allow migration within 1 major version
})

// Request a specific version that might not exist
// If version "1.5" doesn't exist, auto-migrates to best compatible version
profile, actualVersion, found := mtlog.GetProfileWithMigration("APIGateway", "1.5")
if found {
    fmt.Printf("Using version %s (requested %s)\n", actualVersion, "1.5")
}

// Use with logger - automatically handles migration
logger := mtlog.New(mtlog.WithConsole())
migratedLogger := logger.SampleProfileWithVersion("APIGateway", "1.3")
// Logs warning if migration occurs: "Auto-migrated profile 'APIGateway' from version '1.3' to version '1.5'"
```

**Migration Consent Modes:**
- **`MigrationDeny`**: Strict mode - fail if exact version not found
- **`MigrationPrompt`**: Log warning and migrate to best available version (default)
- **`MigrationAuto`**: Silent automatic migration to compatible versions

**Migration Selection:**
- Prefers **stable** (non-deprecated) versions when `PreferStable: true`
- Respects **version distance** constraints to avoid major breaking changes
- Selects **newest compatible** version within constraints
- Falls back to **latest available** version if no constraints are met

```go
// Different migration policies for different environments

// Production: Conservative migration with warnings
mtlog.SetMigrationPolicy(mtlog.MigrationPolicy{
    Consent:            mtlog.MigrationPrompt, // Log warnings
    PreferStable:       true,                 // Avoid deprecated versions
    MaxVersionDistance: 0,                    // Same major version only
})

// Development: Flexible migration
mtlog.SetMigrationPolicy(mtlog.MigrationPolicy{
    Consent:            mtlog.MigrationAuto,  // Silent migration
    PreferStable:       false,                // Allow deprecated for testing
    MaxVersionDistance: 2,                    // Allow broader migration
})

// Critical systems: No migration
mtlog.SetMigrationPolicy(mtlog.MigrationPolicy{
    Consent:            mtlog.MigrationDeny,  // Fail on version mismatch
})
```

## Structured Fields with With()

The `With()` method provides a convenient way to add structured fields to log events, following the slog convention of accepting variadic key-value pairs:

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

// Reuse the base logger for multiple operations
apiLogger.Info("Handling request")
apiLogger.With("endpoint", "/users").Info("GET /users")
apiLogger.With("endpoint", "/products", "method", "POST").Info("POST /products")

// Request-scoped logging
requestLogger := apiLogger.With(
    "request_id", "abc-123",
    "user_id", 456,
)
requestLogger.Info("Request started")
requestLogger.With("duration_ms", 42).Info("Request completed")

// Combine With() and ForContext()
logger.
    With("service", "payment").
    ForContext("transaction_id", "tx-789").
    With("amount", 99.99, "currency", "USD").
    Info("Payment processed")
```

### With() vs ForContext()

- **With()**: Accepts variadic key-value pairs (slog-style), convenient for multiple fields
- **ForContext()**: Takes a single property name and value, returns a new logger
- Both methods create a new logger instance with the combined properties
- Both are safe for concurrent use

### Property Precedence

When combining With() and ForContext(), properties follow a precedence order:
- Properties passed directly to log methods take highest precedence
- ForContext() properties override With() properties
- Later With() calls override earlier With() calls in a chain

Example:
```go
logger.With("user", "alice").              // user=alice
    ForContext("user", "bob").             // user=bob (ForContext overrides)
    With("user", "charlie").               // user=charlie (later With overrides)
    Info("User {user} logged in", "david") // user=david (event property overrides all)
```

## LogContext - Scoped Properties

LogContext provides a way to attach properties to a context that will be automatically included in all log events created from loggers using that context. Properties follow a precedence order: event-specific properties (passed directly to log methods) override ForContext properties, which override LogContext properties (set via PushProperty).

```go
// Add properties to context that flow through all operations
ctx := context.Background()
ctx = mtlog.PushProperty(ctx, "RequestId", "req-12345")
ctx = mtlog.PushProperty(ctx, "UserId", userId)
ctx = mtlog.PushProperty(ctx, "TenantId", "acme-corp")

// Create a logger that includes context properties
log := logger.WithContext(ctx)
log.Information("Processing request") // Includes all pushed properties

// Properties are inherited - child contexts get parent properties
func processOrder(ctx context.Context, orderId string) {
    // Add operation-specific properties
    ctx = mtlog.PushProperty(ctx, "OrderId", orderId)
    ctx = mtlog.PushProperty(ctx, "Operation", "ProcessOrder")
    
    log := logger.WithContext(ctx)
    log.Information("Order processing started") // Includes all parent + new properties
}

// Property precedence example
ctx = mtlog.PushProperty(ctx, "UserId", 123)
logger.WithContext(ctx).Information("Test")                          // UserId=123
logger.WithContext(ctx).ForContext("UserId", 456).Information("Test") // UserId=456 (ForContext overrides)
logger.WithContext(ctx).Information("User {UserId}", 789)            // UserId=789 (event property overrides all)
```

This is particularly useful for:
- Request tracing in web applications
- Maintaining context through async operations
- Multi-tenant applications
- Batch processing with job-specific context

## ForType - Type-Based Logging

ForType provides automatic SourceContext from Go types, making it easy to categorize logs by the types they relate to without manual string constants:

```go
// Automatic SourceContext from type names
userLogger := mtlog.ForType[User](logger)
userLogger.Information("User created") // SourceContext: "User"

productLogger := mtlog.ForType[Product](logger)
productLogger.Information("Product updated") // SourceContext: "Product"

// Works with pointers (automatically dereferenced)
mtlog.ForType[*User](logger).Information("User updated") // SourceContext: "User"

// Service-based logging
type UserService struct {
    logger core.Logger
}

func NewUserService(baseLogger core.Logger) *UserService {
    return &UserService{
        logger: mtlog.ForType[UserService](baseLogger),
    }
}

func (s *UserService) CreateUser(name string) {
    s.logger.Information("Creating user {Name}", name)
    // All logs from this service have SourceContext: "UserService"
}
```

### Advanced Type Naming

For more control over type names, use `ExtractTypeName` with `TypeNameOptions`:

```go
// Include package for disambiguation
opts := mtlog.TypeNameOptions{
    IncludePackage: true,
    PackageDepth:   1, // Only immediate package
}
name := mtlog.ExtractTypeName[User](opts) // Result: "myapp.User"
logger := baseLogger.ForContext("SourceContext", name)

// Add prefixes for microservice identification
opts = mtlog.TypeNameOptions{Prefix: "UserAPI."}
name = mtlog.ExtractTypeName[User](opts) // Result: "UserAPI.User"

// Simplify anonymous structs
opts = mtlog.TypeNameOptions{SimplifyAnonymous: true}
name = mtlog.ExtractTypeName[struct{ Name string }](opts) // Result: "AnonymousStruct"

// Disable warnings for production
opts = mtlog.TypeNameOptions{WarnOnUnknown: false}
name = mtlog.ExtractTypeName[interface{}](opts) // Result: "Unknown" (no warning logged)

// Combine multiple options
opts = mtlog.TypeNameOptions{
    IncludePackage:    true,
    PackageDepth:      1,
    Prefix:            "MyApp.",
    Suffix:            ".Handler",
    SimplifyAnonymous: true,
}
```

### Performance & Caching

ForType uses reflection with intelligent caching for optimal performance:

- **~7% overhead** vs manual `ForSourceContext` (uncached)
- **~1% overhead** with caching enabled (subsequent calls)
- **Thread-safe** caching with `sync.Map`
- **Zero allocations** for cached type names

```go
// Performance comparison
ForType[User](logger).Information("User operation")           // ~7% slower than manual
logger.ForSourceContext("User").Information("User operation") // Baseline performance

// But subsequent ForType calls are nearly free due to caching

// Cache statistics for monitoring
stats := mtlog.GetTypeNameCacheStats()
fmt.Printf("Cache hits: %d, misses: %d, evictions: %d, hit ratio: %.1f%%, size: %d/%d", 
    stats.Hits, stats.Misses, stats.Evictions, stats.HitRatio, stats.Size, stats.MaxSize)

// For testing scenarios requiring cache isolation
mtlog.ResetTypeNameCache() // Clears cache and statistics
```

### Multi-Tenant Support

For applications serving multiple tenants, ForType supports tenant-specific cache namespaces:

```go
// Multi-tenant type-based logging with separate cache namespaces
func CreateTenantLogger(baseLogger core.Logger, tenantID string) core.Logger {
    tenantPrefix := fmt.Sprintf("tenant:%s", tenantID)
    return mtlog.ForTypeWithCacheKey[UserService](baseLogger, tenantPrefix)
}

// Each tenant gets separate cache entries
acmeLogger := CreateTenantLogger(logger, "acme-corp")    // Cache key: tenant:acme-corp + UserService
globexLogger := CreateTenantLogger(logger, "globex-inc") // Cache key: tenant:globex-inc + UserService

acmeLogger.Information("Processing acme user")   // SourceContext: "UserService" (acme cache)
globexLogger.Information("Processing globex user") // SourceContext: "UserService" (globex cache)

// Custom type naming per tenant
opts := mtlog.TypeNameOptions{Prefix: "AcmeCorp."}
acmeName := mtlog.ExtractTypeNameWithCacheKey[User](opts, "tenant:acme")
// Result: "AcmeCorp.User" (cached separately per tenant)
```

### Type Name Cache Configuration

The type name cache can be configured via environment variables:

```bash
# Set cache size limit (default: 10,000 entries)
export MTLOG_TYPE_NAME_CACHE_SIZE=50000  # For large applications
export MTLOG_TYPE_NAME_CACHE_SIZE=1000   # For memory-constrained environments
export MTLOG_TYPE_NAME_CACHE_SIZE=0      # Disable caching entirely
```

The cache uses LRU (Least Recently Used) eviction when the size limit is exceeded, ensuring memory usage stays bounded while keeping frequently used type names cached.

This is particularly useful for:
- Large applications with many service types
- Type-safe logger categorization
- Automatic SourceContext without string constants
- Service-oriented architectures
- Multi-tenant applications requiring cache isolation

## Filters

Control which events are logged with powerful filtering:

```go
// Level filtering
mtlog.WithMinimumLevel(core.WarningLevel)

// Minimum level overrides by source context
mtlog.WithMinimumLevelOverrides(map[string]core.LogEventLevel{
    "github.com/gin-gonic/gin":       core.WarningLevel,    // Suppress Gin info logs
    "github.com/go-redis/redis":      core.ErrorLevel,      // Only Redis errors
    "myapp/internal/services":        core.DebugLevel,      // Debug for internal services
    "myapp/internal/services/auth":   core.VerboseLevel,    // Verbose for auth debugging
})

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
// Literate theme - beautiful, easy on the eyes
mtlog.WithConsoleTheme(sinks.LiterateTheme())

// Dark theme (default)
mtlog.WithConsoleTheme(sinks.DarkTheme())

// Light theme
mtlog.WithConsoleTheme(sinks.LightTheme()) 

// Plain text (no colors)
mtlog.WithConsoleTheme(sinks.NoColorTheme())
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

### Event Routing with Conditional and Router Sinks

Route log events to different destinations based on their properties:

#### Conditional Sink

Filter events based on predicates with zero overhead for non-matching events:

```go
// Create a conditional sink for critical alerts
alertSink, _ := sinks.NewFileSink("alerts.log")
criticalAlertSink := sinks.NewConditionalSink(
    func(event *core.LogEvent) bool {
        return event.Level >= core.ErrorLevel && 
               event.Properties["Alert"] != nil
    },
    alertSink,
)

// Use built-in predicates
auditSink := sinks.NewConditionalSink(
    sinks.PropertyPredicate("Audit"),
    auditFileSink,
)

// Combine predicates
complexFilter := sinks.NewConditionalSink(
    sinks.AndPredicate(
        sinks.LevelPredicate(core.ErrorLevel),
        sinks.PropertyPredicate("Critical"),
        sinks.PropertyValuePredicate("Environment", "production"),
    ),
    targetSink,
)

logger := mtlog.New(
    mtlog.WithSink(sinks.NewConsoleSink()),
    mtlog.WithSink(criticalAlertSink),
    mtlog.WithSink(auditSink),
)

// Only critical errors with Alert property go to alerts.log
logger.With("Alert", true).Error("Database connection lost")
```

#### Router Sink

Advanced routing with multiple destinations and routing modes:

```go
// FirstMatch mode - exclusive routing (stops at first match)
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

// AllMatch mode - broadcast to all matching routes
router := sinks.NewRouterSink(sinks.AllMatch,
    sinks.MetricRoute("metrics", metricsSink),
    sinks.AuditRoute("audit", auditSink),
    sinks.ErrorRoute("errors", errorSink),
)

// With default sink for non-matching events
router := sinks.NewRouterSinkWithDefault(
    sinks.FirstMatch,
    defaultSink,
    routes...,
)

// Dynamic route management at runtime
router.AddRoute(sinks.Route{
    Name:      "debug",
    Predicate: func(e *core.LogEvent) bool { 
        return e.Level <= core.DebugLevel 
    },
    Sink:      debugSink,
})
router.RemoveRoute("debug")

// Fluent route builder API
route := sinks.NewRoute("special-events").
    When(func(e *core.LogEvent) bool {
        category, _ := e.Properties["Category"].(string)
        return category == "Special"
    }).
    To(specialSink)

logger := mtlog.New(
    mtlog.WithSink(router),
    mtlog.WithSink(sinks.NewConsoleSink()),
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

See the [examples](./examples) directory and [OTEL examples](./adapters/otel/examples) for complete examples:

- [Basic logging](./examples/basic/main.go)
- [Using enrichers](./examples/enrichers/main.go)
- [Context logging](./examples/context/main.go)
- [Type-based logging](./examples/fortype/main.go)
- [LogContext scoped properties](./examples/logcontext/main.go)
- [Advanced filtering](./examples/filtering/main.go)
- [Capturing](./examples/capturing/main.go)
- [LogValue interface](./examples/logvalue/main.go)
- [Console themes](./examples/themes/main.go)
- [Output templates](./examples/output-templates/main.go)
- [Go template syntax](./examples/go-templates/main.go)
- [Rolling files](./examples/rolling/main.go)
- [Seq integration](./examples/seq/main.go)
- [Elasticsearch](./examples/elasticsearch/main.go)
- [Splunk integration](./examples/splunk/main.go)
- [OpenTelemetry basics](./adapters/otel/examples/simple/main.go)
- [OTEL with metrics](./adapters/otel/examples/metrics/main.go)
- [OTEL with sampling](./adapters/otel/examples/sampling/main.go)
- [OTEL with TLS](./adapters/otel/examples/tls/main.go)
- [Async logging](./examples/async/main.go)
- [Durable buffering](./examples/durable/main.go)
- [Dynamic levels](./examples/dynamic-levels/main.go)
- [Configuration](./examples/configuration/main.go)
- [Generics usage](./examples/generics/main.go)
- [HTTP middleware](./adapters/middleware/examples/) - net/http, Gin, Echo, Fiber, Chi

## Ecosystem Compatibility

### HTTP Middleware

mtlog provides HTTP middleware adapters for popular Go web frameworks:

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
router := gin.New()
router.Use(middleware.Gin(logger))

// Echo
e := echo.New()
e.Use(middleware.Echo(logger))

// Fiber
app := fiber.New()
app.Use(middleware.Fiber(logger))

// Chi
r := chi.NewRouter()
r.Use(middleware.Chi(logger))
```

Features include request/response logging, body capture with sanitization, distributed tracing, health checks, and object pooling for high-performance scenarios. See the [HTTP Middleware Guide](./adapters/middleware/README.md) for detailed documentation.

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
import mtlogr "github.com/willibrandon/mtlog/adapters/logr"

logrLogger := mtlogr.NewLogger(
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

### OpenTelemetry (OTEL)

mtlog provides comprehensive OpenTelemetry integration with bidirectional support - use mtlog as an OTEL logger or send mtlog events to OTEL collectors:

```go
import "github.com/willibrandon/mtlog/adapters/otel"

// Basic OTLP sink with automatic trace correlation
logger := otel.NewOTELLogger(
    otel.WithOTLPEndpoint("localhost:4317"),
    otel.WithOTLPInsecure(), // For non-TLS connections
)

// Advanced configuration with batching and TLS
logger := mtlog.New(
    otel.WithOTLPSink(
        otel.WithOTLPEndpoint("otel-collector:4317"),
        otel.WithOTLPTransport(otel.OTLPTransportGRPC), // or OTLPTransportHTTP
        otel.WithOTLPBatching(100, 5*time.Second),
        otel.WithOTLPCompression("gzip"),
        otel.WithOTLPClientCert("client.crt", "client.key"),
    ),
)

// Automatic trace context enrichment in HTTP handlers
func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := otel.NewRequestLogger(ctx,
        otel.WithOTLPEndpoint("localhost:4317"),
        otel.WithOTLPInsecure(),
    )
    
    // Logs automatically include trace.id, span.id, trace.flags
    logger.Information("Processing request for {Path}", r.URL.Path)
}

// Sampling strategies for high-volume scenarios
logger := mtlog.New(
    otel.WithOTLPSink(
        otel.WithOTLPEndpoint("localhost:4317"),
        otel.WithOTLPSampling(otel.NewRateSampler(0.1)), // Sample 10% of events
        // or: otel.NewLevelSampler(core.WarningLevel)    // Only warnings and above
        // or: otel.NewAdaptiveSampler(1000)              // Target 1000 events/sec
    ),
)

// Prometheus metrics export for monitoring
exporter, _ := otel.NewMetricsExporter(
    otel.WithMetricsPort(9090),
    otel.WithMetricsPath("/metrics"),
)
defer exporter.Close()

// Use mtlog as an OTEL Bridge (mtlog -> OTEL)
otelLogger := otel.NewBridge(logger)
otelLogger.Emit(ctx, record) // Use OTEL log.Logger interface

// Use OTEL as an mtlog sink (OTEL -> mtlog)
handler := otel.NewHandler(otelLogger)
logger := mtlog.New(mtlog.WithSink(handler))
```

For complete OpenTelemetry integration documentation, see the [OTEL adapter README](./adapters/otel/README.md).

## Environment Variables

mtlog respects several environment variables for runtime configuration:

### Color Control

```bash
# Force specific color mode (overrides terminal detection)
export MTLOG_FORCE_COLOR=none     # Disable all colors
export MTLOG_FORCE_COLOR=8        # Force 8-color mode (basic ANSI)
export MTLOG_FORCE_COLOR=256      # Force 256-color mode

# Standard NO_COLOR variable is also respected
export NO_COLOR=1                 # Disable colors (follows no-color.org)
```

### Performance Tuning

```bash
# Adjust source context cache size (default: 10000)
export MTLOG_SOURCE_CTX_CACHE=50000  # Increase for large applications
export MTLOG_SOURCE_CTX_CACHE=1000   # Decrease for memory-constrained environments

# Adjust type name cache size (default: 10000)
export MTLOG_TYPE_NAME_CACHE_SIZE=50000  # For applications with many types
export MTLOG_TYPE_NAME_CACHE_SIZE=1000   # For memory-constrained environments
export MTLOG_TYPE_NAME_CACHE_SIZE=0      # Disable type name caching
```

## Tools

### mtlog-analyzer

A static analysis tool that catches common mtlog mistakes at compile time:

```bash
# Install the analyzer
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest

# Run with go vet
go vet -vettool=$(which mtlog-analyzer) ./...
```

The analyzer detects:
- Template/argument count mismatches
- Invalid property names (spaces, starting with numbers)
- Duplicate properties in templates and With() calls
- Missing capturing hints for complex types
- Error logging without error values
- With() method issues (odd arguments, non-string keys, empty keys)
- Cross-call duplicate detection for property overrides
- Reserved property name shadowing (opt-in)

Example catches:
```go
// ❌ Template has 2 properties but 1 argument provided
log.Information("User {UserId} logged in from {IP}", userId)

// ❌ Duplicate property 'UserId'
log.Information("User {UserId} did {Action} as {UserId}", id, "login", id)

// ❌ With() requires even number of arguments (MTLOG009)
log.With("key1", "value1", "key2")  // Missing value

// ❌ With() key must be a string (MTLOG010)
log.With(123, "value")

// ❌ Duplicate keys in With() (MTLOG003)
log.With("id", 1, "name", "test", "id", 2)

// ⚠️ Cross-call property override (MTLOG011)
logger := log.With("service", "api")
logger.With("service", "auth")  // Overrides previous 'service'

// ✅ Correct usage
log.Information("User {@User} has {Count} items", user, count)
log.With("userId", 123, "requestId", "abc").Info("Request processed")
```

See [mtlog-analyzer README](./cmd/mtlog-analyzer/README.md) for detailed documentation and CI integration.

### mtlog-lsp

A Language Server Protocol implementation that bundles mtlog-analyzer for editor integrations:

```bash
# Install the LSP server
go install github.com/willibrandon/mtlog/cmd/mtlog-lsp@latest
```

mtlog-lsp provides:
- Zero-subprocess overhead with bundled analyzer
- Real-time diagnostics for all MTLOG001-MTLOG013 issues
- Code actions and quick fixes
- Workspace configuration support
- Performance optimized with package caching

Primarily used by the [Zed extension](./zed-extension/mtlog/README.md). See [mtlog-lsp README](./cmd/mtlog-lsp/README.md) for detailed documentation.

### IDE Extensions

#### VS Code Extension

For real-time validation in Visual Studio Code, install the [mtlog-analyzer extension](./vscode-extension/mtlog-analyzer/README.md):

1. Install mtlog-analyzer: `go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`
2. Install the extension from VS Code Marketplace (search for "mtlog-analyzer")
3. Get instant feedback on template errors as you type

The extension provides:
- 🔍 Real-time diagnostics with squiggly underlines
- 🎯 Precise error locations - click to jump to issues
- 📊 Three severity levels: errors, warnings, and suggestions
- 🔧 Quick fixes for common issues (Ctrl+. for PascalCase conversion, argument count fixes)
- ⚙️ Configurable analyzer path and flags

#### GoLand Plugin

For real-time validation in GoLand and other JetBrains IDEs with Go support, install the [mtlog-analyzer plugin](./goland-plugin/README.md):

1. Install mtlog-analyzer: `go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`
2. Install the plugin from JetBrains Marketplace (search for "mtlog-analyzer")
3. Get instant feedback on template errors as you type

The plugin provides:
- 🔍 Real-time template validation as you type
- 🎯 Intelligent highlighting (template errors highlight the full string, property warnings highlight just the property)
- 🔧 Quick fixes for common issues (Alt+Enter for PascalCase conversion, argument count fixes)
- ⚙️ Configurable analyzer path, flags, and severity levels
- 🚀 Performance optimized with caching and debouncing

#### Neovim Plugin

For Neovim users, a comprehensive plugin is included in the repository at [neovim-plugin/](./neovim-plugin/):

```lua
-- Install with lazy.nvim
{
  'willibrandon/mtlog',
  lazy = false,  -- Load immediately to ensure commands are available
  config = function(plugin)
    -- Handle the plugin's subdirectory structure
    vim.opt.rtp:append(plugin.dir .. "/neovim-plugin")
    vim.cmd("runtime plugin/mtlog.vim")
    
    require('mtlog').setup()
  end,
  ft = 'go',
}
```

The plugin provides:
- 🔍 Real-time analysis on save with debouncing
- 🎯 LSP integration for code actions
- 🔧 Quick fixes and diagnostic suppression
- 📊 Statusline integration with diagnostic counts
- ⚡ Advanced features: queue management, context rules, help system
- 🚀 Performance optimized with caching and async operations

See the [plugin README](./neovim-plugin/README.md) for detailed configuration and usage.

#### Zed Extension

For real-time validation in Zed editor, install the [mtlog-analyzer extension](./zed-extension/mtlog/README.md):

1. Install mtlog-lsp (includes bundled analyzer):
   ```bash
   go install github.com/willibrandon/mtlog/cmd/mtlog-lsp@latest
   ```
2. Install the extension from Zed's extension manager (search for "mtlog-analyzer")
3. Get instant feedback on template errors as you type

The extension provides:
- 🔍 Real-time diagnostics for all MTLOG001-MTLOG013 issues
- 🔧 Quick fixes via code actions for common issues
- 🚀 Automatic binary detection in standard Go paths
- ⚙️ Configurable analyzer flags and custom paths

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

Register types for special handling during capturing:

```go
capturer := capture.NewDefaultCapturer()
capturer.RegisterScalarType(reflect.TypeOf(uuid.UUID{}))
```

## Documentation

For comprehensive guides and examples, see the [docs](./docs) directory:

- **[Quick Reference](./docs/quick-reference.md)** - Quick reference for all features
- **[Template Syntax](./docs/template-syntax.md)** - Guide to message template syntaxes
- **[Sinks Guide](./docs/sinks.md)** - Complete guide to all output destinations
- **[Routing Patterns](./docs/routing-patterns.md)** - Advanced event routing patterns and best practices
- **[Dynamic Level Control](./docs/dynamic-levels.md)** - Runtime level management
- **[Type-Safe Generics](./docs/generics.md)** - Compile-time safe logging methods
- **[Configuration](./docs/configuration.md)** - JSON-based configuration
- **[Performance](./docs/performance.md)** - Benchmarks and optimization
- **[Testing](./docs/testing.md)** - Container-based integration testing
- **[Troubleshooting](./docs/troubleshooting.md)** - Debugging guide with selflog

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests with Docker Compose
docker-compose -f docker/docker-compose.test.yml up -d
go test -tags=integration ./...
docker-compose -f docker/docker-compose.test.yml down

# Run benchmarks (in benchmarks/ folder)
cd benchmarks && go test -bench=. -benchmem
```

See [testing.md](./docs/testing.md) for detailed testing guide and manual container setup.

## Contributing

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
