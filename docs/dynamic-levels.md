# Dynamic Level Control

Dynamic level control allows you to change logging levels at runtime without restarting your application. This is essential for production troubleshooting and performance optimization.

## Overview

mtlog provides two approaches to dynamic level control:

1. **Manual Control**: Programmatically change levels via API or HTTP endpoints
2. **Centralized Control**: Automatically sync levels with external systems like Seq

## Manual Level Control

### Basic Usage

```go
// Create a level switch
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

logger := mtlog.New(
    mtlog.WithLevelSwitch(levelSwitch),
    mtlog.WithConsole(),
)

// Change level at runtime
levelSwitch.SetLevel(core.DebugLevel)

// Check current level
currentLevel := levelSwitch.Level()
```

### Fluent Interface

Level switches support a fluent interface for convenient level changes:

```go
// Chain level changes
levelSwitch.Debug().Information().Warning()

// Final level is Warning
fmt.Printf("Current level: %v\n", levelSwitch.Level())
```

### Level Checking

Use level checking to avoid expensive operations when they won't be logged:

```go
if levelSwitch.IsEnabled(core.VerboseLevel) {
    // Expensive serialization only when needed
    data := expensiveSerialize(complexObject)
    logger.Verbose("Complex data: {@Data}", data)
}
```

### Convenience Methods

mtlog provides convenience methods for common scenarios:

```go
// Create logger with controlled level
option, levelSwitch := mtlog.WithControlledLevel(core.InformationLevel)
logger := mtlog.New(option, mtlog.WithConsole())

// Dynamic level with existing switch
existingSwitch := mtlog.NewLoggingLevelSwitch(core.DebugLevel)
logger := mtlog.New(
    mtlog.WithDynamicLevel(existingSwitch),
    mtlog.WithConsole(),
)
```

## Thread Safety

Level switches are fully thread-safe using atomic operations:

```go
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

// Safe to call from multiple goroutines
go func() {
    levelSwitch.SetLevel(core.DebugLevel)
}()

go func() {
    if levelSwitch.IsEnabled(core.VerboseLevel) {
        logger.Verbose("Verbose message")
    }
}()
```

## HTTP API for Level Control

Create HTTP endpoints to control logging levels remotely:

```go
func setupLevelControlAPI(levelSwitch *mtlog.LoggingLevelSwitch) {
    http.HandleFunc("/admin/loglevel", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case "GET":
            // Get current level
            level := levelSwitch.Level()
            json.NewEncoder(w).Encode(map[string]string{
                "level": level.String(),
            })
            
        case "POST":
            // Set new level
            var req struct {
                Level string `json:"level"`
            }
            if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            
            newLevel, err := core.ParseLogEventLevel(req.Level)
            if err != nil {
                http.Error(w, "Invalid level", http.StatusBadRequest)
                return
            }
            
            oldLevel := levelSwitch.Level()
            levelSwitch.SetLevel(newLevel)
            
            logger.Information("Log level changed from {OldLevel} to {NewLevel} via HTTP",
                oldLevel, newLevel)
                
            w.WriteHeader(http.StatusOK)
        }
    })
}
```

Usage:

```bash
# Get current level
curl http://localhost:8080/admin/loglevel

# Set level to Debug
curl -X POST http://localhost:8080/admin/loglevel \
  -H "Content-Type: application/json" \
  -d '{"level": "Debug"}'
```

## Centralized Control with Seq

Seq provides centralized level management across multiple applications.

### Basic Setup

```go
options := mtlog.SeqLevelControllerOptions{
    CheckInterval: 30 * time.Second,  // Check every 30 seconds
    InitialCheck:  true,              // Check immediately on startup
}

loggerOption, levelSwitch, controller := mtlog.WithSeqLevelControl(
    "http://localhost:5341", options)
defer controller.Close()

logger := mtlog.New(loggerOption)
```

### Advanced Configuration

```go
options := mtlog.SeqLevelControllerOptions{
    CheckInterval: 15 * time.Second,
    InitialCheck:  true,
    OnError: func(err error) {
        fmt.Printf("Level sync error: %v\n", err)
    },
}

// With API key authentication
controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)
defer controller.Close()
```

### Builder Pattern

For more complex configurations:

```go
controller := mtlog.NewSeqLevelControllerBuilder("http://localhost:5341").
    WithCheckInterval(10 * time.Second).
    WithInitialCheck(true).
    WithSeqAPIKey("your-api-key").
    WithErrorHandler(func(err error) {
        log.Printf("Seq level sync error: %v", err)
    }).
    WithLevelSwitch(existingLevelSwitch).
    Build()
```

### Manual Synchronization

Force immediate level synchronization:

```go
// Force check now
err := controller.ForceCheck()
if err != nil {
    log.Printf("Force check failed: %v", err)
}

// Get current status
currentLevel := controller.GetCurrentLevel()
lastSeqLevel := controller.GetLastSeqLevel()
```

## Multiple Applications

Share level switches across multiple loggers or applications:

```go
// Shared level switch
sharedSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

// Multiple loggers using the same switch
webLogger := mtlog.New(
    mtlog.WithLevelSwitch(sharedSwitch),
    mtlog.WithProperty("Component", "Web"),
    mtlog.WithConsole(),
)

dbLogger := mtlog.New(
    mtlog.WithLevelSwitch(sharedSwitch),
    mtlog.WithProperty("Component", "Database"),
    mtlog.WithConsole(),
)

// Changing the switch affects all loggers
sharedSwitch.SetLevel(core.DebugLevel)
```

## Environment-based Configuration

Set initial levels based on environment:

```go
func createLevelSwitch() *mtlog.LoggingLevelSwitch {
    initialLevel := core.InformationLevel
    
    if env := os.Getenv("LOG_LEVEL"); env != "" {
        if level, err := core.ParseLogEventLevel(env); err == nil {
            initialLevel = level
        }
    }
    
    // Development environment gets debug by default
    if os.Getenv("APP_ENV") == "development" {
        initialLevel = core.DebugLevel
    }
    
    return mtlog.NewLoggingLevelSwitch(initialLevel)
}
```

## Performance Impact

Dynamic level control is designed for minimal performance impact:

### Benchmark Results

| Operation | Static Level | Dynamic Level | Overhead |
|-----------|--------------|---------------|----------|
| Level check | 1.4 ns/op | 1.4 ns/op | 0% |
| Simple log | 16.8 ns/op | 16.9 ns/op | <1% |
| Filtered log | 1.4 ns/op | 1.5 ns/op | 7% |

### Implementation Details

- **Atomic operations**: Level reads use `atomic.LoadInt32`
- **Cache-friendly**: Level checking is a single memory read
- **Lock-free**: No mutexes in the critical path
- **Minimal allocation**: Zero allocations for level checks

## Production Patterns

### Application Lifecycle

```go
func setupLogging() (*mtlog.Logger, *mtlog.LoggingLevelSwitch, func()) {
    levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)
    
    logger := mtlog.New(
        mtlog.WithLevelSwitch(levelSwitch),
        mtlog.WithConsole(),
        mtlog.WithSeq("http://seq:5341"),
    )
    
    // Setup Seq level controller
    controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)
    
    // Return cleanup function
    return logger, levelSwitch, func() {
        controller.Close()
        logger.Close()
    }
}
```

### Debugging Workflows

1. **Incident Response**: Increase logging level via Seq UI
2. **Performance Debugging**: Enable verbose logs for specific components
3. **Load Testing**: Reduce logging to minimize overhead
4. **Normal Operations**: Return to information level

### Monitoring

Monitor level changes for audit and troubleshooting:

```go
type LevelChangeMonitor struct {
    lastLevel core.LogEventLevel
    logger    mtlog.Logger
}

func (m *LevelChangeMonitor) checkLevelChange(switch *mtlog.LoggingLevelSwitch) {
    currentLevel := levelSwitch.Level()
    if currentLevel != m.lastLevel {
        m.logger.Information("Log level changed from {OldLevel} to {NewLevel}",
            m.lastLevel, currentLevel)
        m.lastLevel = currentLevel
    }
}
```

## Best Practices

1. **Default to Information**: Start with Information level for production
2. **Use Seq for centralized control**: Manage all applications from one place  
3. **Monitor level changes**: Log when levels change for audit trail
4. **Graceful degradation**: Handle controller failures gracefully
5. **Environment-specific defaults**: Set appropriate defaults per environment
6. **Document level usage**: Document what each level means in your application
7. **Test level switching**: Include level control in your testing strategy

## Troubleshooting

### Common Issues

**Level not changing:**
- Check if level switch is properly configured
- Verify Seq connectivity and API keys
- Check controller error handlers

**Performance impact:**
- Ensure you're using `IsEnabled()` for expensive operations
- Verify atomic operations are working (should see no locks in profiling)

**Seq synchronization:**
- Verify Seq server is accessible
- Check API key permissions
- Monitor error handlers for sync failures

### Debugging

Enable detailed logging for the level controller:

```go
options := mtlog.SeqLevelControllerOptions{
    CheckInterval: 30 * time.Second,
    OnError: func(err error) {
        fmt.Printf("[LEVEL_CONTROLLER] Error: %v\n", err)
    },
}
```