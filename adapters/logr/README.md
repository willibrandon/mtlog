# mtlog logr Adapter

This module provides a [logr](https://github.com/go-logr/logr) adapter for [mtlog](https://github.com/willibrandon/mtlog), enabling Kubernetes controllers and operators to use mtlog's powerful structured logging features while maintaining full compatibility with the logr API.

## Installation

```bash
go get github.com/willibrandon/mtlog/adapters/logr
```

## Quick Start

```go
import (
    "github.com/willibrandon/mtlog"
    mtlogr "github.com/willibrandon/mtlog/adapters/logr"
    "github.com/willibrandon/mtlog/core"
)

// Create a logr logger backed by mtlog
logger := mtlogr.NewLogger(
    mtlog.WithConsole(),
    mtlog.WithMinimumLevel(core.DebugLevel),
)

// Use standard logr API
logger.Info("starting reconciliation", "namespace", "default", "name", "my-app")
logger.V(1).Info("detailed info", "replicas", 3)
logger.Error(err, "failed to update resource", "reason", "conflict")
```

## Features

- **Full logr Compatibility**: Drop-in replacement for any logr backend
- **Message Templates**: Leverage mtlog's powerful message template system
- **Rich Sinks**: Output to Console, Seq, Elasticsearch, Splunk, and more
- **Pipeline Architecture**: Add enrichers, filters, and destructurers
- **Performance**: Benefit from mtlog's optimized logging pipeline

## V-Level Mapping

logr V-levels are mapped to mtlog levels:

| logr V-level | mtlog Level    | Description |
|-------------|----------------|-------------|
| V(0)        | Information    | Standard info messages |
| V(1)        | Debug          | Debug-level details |
| V(2+)       | Verbose        | Highly verbose output |

## Advanced Usage

### Custom Configuration

```go
// Create mtlog logger with custom configuration
mtlogLogger := mtlog.New(
    mtlog.WithSeq("http://localhost:5341"),
    mtlog.WithProperty("service", "my-controller"),
    mtlog.WithEnricher(enrichers.WithMachineName()),
    mtlog.WithFilter(filters.ByIncludingOnly(func(e core.LogEvent) bool {
        return e.Level >= core.InformationLevel
    })),
)

// Create logr logger from custom mtlog instance
logrLogger := logr.New(mtlogr.NewLogrSink(mtlogLogger))
```

### With Kubernetes Controller Runtime

```go
import (
    "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
    // Set mtlog as the global logger for controller-runtime
    log.SetLogger(mtlogr.NewLogger(
        mtlog.WithConsole(),
        mtlog.WithProperty("component", "manager"),
    ))
    
    // Your controller setup...
}
```

### Structured Logging Best Practices

```go
logger := mtlogr.NewLogger(mtlog.WithConsole())

// Good: Use consistent key names
logger.Info("pod created", 
    "namespace", pod.Namespace,
    "name", pod.Name,
    "phase", pod.Status.Phase,
)

// Use WithValues for persistent context
reqLogger := logger.WithValues("request_id", requestID)
reqLogger.Info("handling request")
reqLogger.Info("request completed", "duration_ms", duration.Milliseconds())

// Use WithName for component hierarchy
controllerLog := logger.WithName("controller").WithName("deployment")
controllerLog.Info("reconciling", "generation", deployment.Generation)
```

## Performance

The adapter maintains mtlog's high performance while providing logr compatibility:

- Zero allocations for disabled log levels
- Efficient property handling
- Batched output to remote sinks
- Minimal overhead for the adapter layer

## License

MIT License - same as mtlog