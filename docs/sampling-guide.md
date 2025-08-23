# Sampling Guide

## Overview

mtlog provides comprehensive per-message sampling capabilities to help manage log volume in production while preserving important events. This guide covers all sampling strategies and how to use them effectively.

## Table of Contents

- [Basic Sampling Strategies](#basic-sampling-strategies)
- [Advanced Sampling](#advanced-sampling)
- [Sampling Profiles](#sampling-profiles)
- [Adaptive Sampling](#adaptive-sampling)
- [Configuration API](#configuration-api)
- [Best Practices](#best-practices)

## Basic Sampling Strategies

### Counter-Based Sampling

Log every Nth message:

```go
// Log every 10th message
logger.Sample(10).Info("This logs 1 in 10 times")

// Using configuration API
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().Every(10).Build(),
)
```

### Rate-Based Sampling

Sample a percentage of messages:

```go
// Log 20% of messages (randomly selected)
logger.SampleRate(0.2).Info("This logs ~20% of the time")

// Using configuration API
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().Rate(0.2).Build(),
)
```

### Time-Based Sampling

Log at most once per time period:

```go
// Log at most once per second
logger.SampleDuration(time.Second).Info("Rate limited to 1/sec")

// Using configuration API
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().Duration(time.Second).Build(),
)
```

### First-N Sampling

Log only the first N occurrences:

```go
// Log only first 100 occurrences
logger.SampleFirst(100).Info("Only first 100 are logged")

// Using configuration API
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().First(100).Build(),
)
```

## Advanced Sampling

### Group-Based Sampling

Sample within named groups - useful for different event categories:

```go
// Each user gets sampled independently
for _, userID := range users {
    logger.SampleGroup(userID, 10).Info("User {UserID} action", userID)
}
```

### Conditional Sampling

Sample based on runtime conditions:

```go
// Sample more during business hours
logger.SampleWhen(func() bool {
    hour := time.Now().Hour()
    return hour >= 9 && hour <= 17
}, 100).Info("Business hours event")
```

### Exponential Backoff

Reduce sampling rate exponentially for repetitive events:

```go
// First error logs, then 2nd, 4th, 8th, 16th, etc.
logger.SampleBackoff("db-error", 2.0).Error("Database connection failed")
```

## Sampling Profiles

mtlog includes pre-configured sampling profiles optimized for common scenarios:

```go
// Development profile - verbose logging
logger.SampleProfile("Development").Info("Dev message")

// Production profile - balanced for production use
logger.SampleProfile("Production").Info("Prod message")

// HighVolume profile - aggressive sampling for very high traffic
logger.SampleProfile("HighVolume").Info("High volume message")
```

### Available Profiles

| Profile | Description | Use Case |
|---------|-------------|----------|
| Development | Minimal sampling, verbose output | Local development |
| Production | Balanced sampling for production | Standard production systems |
| HighVolume | Aggressive sampling | High-traffic services |
| Debug | No sampling when debugging | Troubleshooting |
| Performance | Optimized for performance testing | Load testing |

### Custom Profiles

Register your own profiles:

```go
// Register during initialization
mtlog.AddCustomProfile("MyProfile", "Custom profile for my service", func() core.LogEventFilter {
    return filters.NewCompositeSamplingFilter(
        filters.ModeAnd,
        filters.NewRateSamplingFilter(0.1),
        filters.NewDurationSamplingFilter(100*time.Millisecond),
    )
})

// Use the custom profile
logger.SampleProfile("MyProfile").Info("Using custom profile")
```

## Adaptive Sampling

Adaptive sampling automatically adjusts the sampling rate to maintain a target events-per-second rate:

```go
// Maintain ~100 events per second
logger.SampleAdaptive(100).Info("Auto-adjusting rate")

// With custom parameters
logger.SampleAdaptiveWithOptions(
    100,                    // Target events/second
    0.001,                  // Min rate (0.1%)
    1.0,                    // Max rate (100%)
    time.Second,            // Adjustment interval
).Info("Custom adaptive sampling")
```

### Adaptive Sampling with Hysteresis

Prevent oscillation in sampling rates:

```go
logger.SampleAdaptiveWithHysteresis(
    100,                    // Target events/second
    0.15,                   // 15% change threshold
    0.3,                    // 30% adjustment aggressiveness
).Info("Stable adaptive sampling")
```

### Dampening Presets

Use predefined dampening configurations:

```go
// Conservative - slow, stable adjustments
logger.SampleAdaptiveWithPreset(100, mtlog.DampeningConservative)

// Moderate - balanced adjustment speed (default)
logger.SampleAdaptiveWithPreset(100, mtlog.DampeningModerate)

// Aggressive - fast adjustments
logger.SampleAdaptiveWithPreset(100, mtlog.DampeningAggressive)

// UltraStable - very slow changes, high stability
logger.SampleAdaptiveWithPreset(100, mtlog.DampeningUltraStable)

// Responsive - quick reaction to load changes
logger.SampleAdaptiveWithPreset(100, mtlog.DampeningResponsive)
```

## Configuration API

### Fluent Builder Pattern

Chain multiple sampling strategies:

```go
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Rate(0.5).                    // 50% sampling
        Duration(100*time.Millisecond). // At most 10/second
        First(1000).                  // Only first 1000
        Build(),
)
```

### Combining Strategies

Strategies are combined with AND logic by default:

```go
// Must pass ALL filters (AND logic)
config := mtlog.Sampling().
    Rate(0.5).                    // 50% chance AND
    Duration(100*time.Millisecond). // time window passed AND
    Group("api", 10).             // every 10th in group
    Build()
```

For OR logic, use `BuildWithMode`:

```go
// Passes if ANY filter passes (OR logic)
config := mtlog.Sampling().
    Every(100).                   // Every 100th OR
    Duration(time.Minute).        // Once per minute OR
    First(10).                    // First 10
    BuildWithMode(filters.ModeOr)
```

### Complex Sampling Policies

Implement custom sampling logic:

```go
type CustomPolicy struct{}

func (p *CustomPolicy) ShouldSample(event *core.LogEvent) bool {
    // Custom logic here
    return event.Level >= core.ErrorLevel
}

logger := mtlog.New(
    mtlog.WithSink(sink),
    mtlog.WithSamplingPolicy(&CustomPolicy{}),
)
```

## Best Practices

### 1. Choose the Right Strategy

- **Development**: Use minimal or no sampling
- **Production**: Use rate-based or adaptive sampling
- **High Volume**: Use aggressive sampling with important event preservation
- **Debugging**: Temporarily disable sampling for specific loggers

### 2. Preserve Important Events

Always log critical events without sampling:

```go
// Critical events bypass sampling
if critical {
    logger.Error("Critical error: {Error}", err)
} else {
    logger.Sample(100).Info("Regular event")
}
```

### 3. Use Groups for Multi-Tenant Systems

```go
// Each tenant gets fair sampling
logger.SampleGroup(tenantID, 100).Info("Tenant {TenantID} event", tenantID)
```

### 4. Monitor Sampling Effectiveness

Enable sampling summaries to track effectiveness:

```go
// Emit sampling statistics every minute
logger.EnableSamplingSummary(time.Minute)

// Get current stats
sampled, skipped := logger.GetSamplingStats()
log.Printf("Sampled: %d, Skipped: %d", sampled, skipped)
```

### 5. Memory Management

Configure memory limits for sampling state:

```go
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithSamplingMemoryLimit(50000), // Max 50k unique keys
)
```

### 6. Reset Sampling State

Reset counters when needed:

```go
// Reset all sampling counters
logger.ResetSampling()

// Reset specific group
logger.ResetSamplingGroup("api-endpoint-1")
```

## Performance Considerations

Sampling overhead is minimal:
- Simple sampling: ~17ns per decision
- With properties: ~200ns per decision
- Zero allocations for simple sampling

### Cache Warmup

Pre-populate caches for better cold-start performance:

```go
// Warmup common groups
mtlog.WarmupSamplingGroups([]string{
    "user-api",
    "admin-api",
    "webhook",
})

// Warmup backoff keys
mtlog.WarmupBackoffKeys([]string{
    "db-error",
    "api-timeout",
    "rate-limit",
})
```

## Examples

### Example: API Endpoint Sampling

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Sample per endpoint
    logger := baseLogger.SampleGroup(r.URL.Path, 100)
    
    // Log request (sampled)
    logger.Info("Request {Method} {Path}", r.Method, r.URL.Path)
    
    // Always log errors
    if err := processRequest(r); err != nil {
        baseLogger.Error("Request failed: {Error}", err)
    }
}
```

### Example: Error Rate Limiting

```go
func processJob(job Job) {
    err := job.Execute()
    if err != nil {
        // Exponential backoff for error logs
        logger.SampleBackoff(fmt.Sprintf("job-%s", job.Type), 2.0).
            Error("Job failed: {JobID} {Error}", job.ID, err)
    } else {
        // Sample success logs more aggressively
        logger.Sample(1000).Info("Job completed: {JobID}", job.ID)
    }
}
```

### Example: Multi-Strategy Sampling

```go
logger := mtlog.New(
    mtlog.WithConsole(),
    mtlog.Sampling().
        Rate(0.1).                       // 10% of messages
        Duration(100*time.Millisecond).  // Rate limit to 10/sec
        When(func() bool {               // Only during business hours
            h := time.Now().Hour()
            return h >= 9 && h <= 17
        }, 1).
        Build(),
)
```

## Troubleshooting

### No Events Being Logged

1. Check sampling rate isn't too aggressive
2. Verify filters aren't too restrictive
3. Use `GetSamplingStats()` to see if events are being skipped
4. Temporarily disable sampling to verify logger works

### Inconsistent Sampling

1. Ensure consistent use of sampling groups
2. Check for race conditions in conditional sampling
3. Verify time-based sampling intervals

### Memory Issues

1. Set appropriate memory limits
2. Use cache warmup to prevent allocation spikes
3. Monitor cache statistics for efficiency

### Performance Impact

1. Use simpler sampling strategies when possible
2. Avoid complex conditional logic
3. Pre-compile regular expressions used in conditions
4. Use sampling profiles instead of creating filters per-log