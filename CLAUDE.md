# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**mtlog** (Message Template Logging) is a high-performance, Serilog-inspired structured logging library for Go. The library brings message templates and pipeline architecture to the Go ecosystem, with native integration for Seq, Elasticsearch, and Splunk.

## Key Features

### 1. Message Templates
- Templates like `"User {UserId} logged in"` are preserved throughout the pipeline
- Properties are extracted from templates and matched positionally to arguments
- Templates serve as both human-readable messages and event types for grouping/analysis
- Support for format specifiers like `{Count:000}` and `{Price:F2}`
- OTEL-compatible dotted property names like `{http.method}`, `{service.name}`, `{db.system}`

### 2. Pipeline Architecture
The logging pipeline follows this flow:
```
Message Template Parser → Enrichment → Filtering → Destructuring → Sinks (Output)
```

### 3. Ecosystem Compatibility
- **slog**: Full compatibility with Go's standard `log/slog` package via `slog.Handler` adapter
- **logr**: Integration with Kubernetes ecosystem via `logr.LogSink` adapter
- **Short Methods**: Convenience methods like `V()`, `D()`, `I()`, `W()`, `E()`, `F()`

### 4. Core Interfaces
- `Logger` - Main logging interface with methods like `Information()`, `Error()`, etc.
- `LogEventEnricher` - Adds contextual properties to log events
- `LogEventFilter` - Determines which events proceed through pipeline
- `Destructurer` - Converts complex types to log-appropriate representations
- `LogEventSink` - Outputs events to destinations (Console, File, Seq, etc.)
- `LoggingLevelSwitch` - Dynamic level control for runtime configuration

### 5. SelfLog Diagnostics
- Internal diagnostic facility for debugging silent failures
- Zero-cost when disabled (0.37ns/op with guard check)
- Outputs to any `io.Writer` or custom function
- Environment variable support: `MTLOG_SELFLOG=stderr/stdout/file`
- Reports sink failures, template errors, panic recovery, and configuration issues

## Development Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run only integration tests (requires Docker)
go test -tags=integration ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -run TestSeqIntegration ./...

# Run fuzz tests
go test -fuzz=FuzzParseMessageTemplate -fuzztime=30s ./parser

# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run benchmarks with specific focus
go test -bench=BenchmarkSimpleString -benchmem -benchtime=10s .

# Run mtlog-analyzer tests
cd cmd/mtlog-analyzer && go test -v ./...

# Run mtlog-analyzer on the project
go vet -vettool=$(which mtlog-analyzer) ./...
```

## mtlog-analyzer

The project includes a static analysis tool that catches common mtlog mistakes at compile time:

### Installation
```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
```

### Features
- Template/argument mismatch detection
- Format specifier validation
- Property naming conventions (PascalCase suggestions)
- Duplicate property detection
- Destructuring hints for complex types
- Error logging pattern validation
- Context key constant suggestions

### Usage
```bash
# Run with go vet
go vet -vettool=$(which mtlog-analyzer) ./...

# Run standalone
mtlog-analyzer ./...

# With configuration flags
mtlog-analyzer -strict -common-keys=tenant_id,org_id ./...
```

### Configuration Flags
- `-strict` - Enable strict format specifier validation
- `-common-keys` - Additional context keys to suggest as constants
- `-disable` - Disable specific checks (template, naming, etc.)
- `-ignore-dynamic-templates` - Suppress warnings for non-literal templates
- `-strict-logger-types` - Only analyze exact mtlog types
- `-downgrade-errors` - Downgrade errors to warnings for CI migration

## Project Structure

```
mtlog/
├── core/              # Core interfaces and types
├── parser/            # Message template parsing with format specifiers
├── enrichers/         # Built-in enrichers (machine name, thread ID, etc.)
├── filters/           # Level, predicate, sampling, and rate limit filters
├── destructure/       # Type destructuring with LogValue support
├── selflog/           # Internal diagnostics for debugging
├── sinks/             # Output destinations
│   ├── async.go       # Async sink wrapper with batching
│   ├── console.go     # Console output with themes
│   ├── file.go        # File and rolling file sinks
│   ├── seq.go         # Seq integration with CLEF formatting
│   ├── elasticsearch.go # Elasticsearch sink with data streams
│   ├── splunk.go      # Splunk HEC integration
│   └── durable.go     # Durable buffering for reliability
├── handler/           # Ecosystem adapters
│   ├── slog_handler.go   # slog.Handler implementation
│   └── logr_sink.go      # logr.LogSink implementation
├── formatters/        # Log formatters (CLEF, JSON)
├── configuration/     # JSON/YAML configuration support
├── integration/       # Integration tests
├── examples/          # Usage examples
└── cmd/
    └── mtlog-analyzer/  # Static analysis tool for mtlog usage
```

## Performance Achievements

The library achieves **zero allocations** for simple logging through optimized implementations:
- Simple log: ~17.3ns/op, 0B/op, 0 allocs ✓
- With properties: ~209ns/op, 448B/op, 4 allocs
- Below minimum level: ~1.5ns/op, 0B/op, 0 allocs ✓
- Dynamic level filtering: ~0.2ns/op, 0B/op, 0 allocs ✓

Performance is comparable to or better than zap/zerolog for common scenarios.

## Testing Infrastructure

### Container-Based Testing
The project uses real infrastructure for integration tests:
- **Seq** - Real Seq instance on ports 5341 (ingestion) and 8080 (query)
- **Elasticsearch** - Real ES instance on port 9200
- **Splunk** - Real Splunk instance on ports 8088 (HEC) and 8089 (management)

### CI/CD Pipeline
GitHub Actions workflow includes:
- Multi-OS testing (Ubuntu, Windows, macOS)
- Multi-Go version testing (1.21, 1.22, 1.23)
- Integration tests with real services
- Fuzz testing
- Race condition testing
- Performance benchmarking
- Code coverage reporting

### Test Categories
1. **Unit Tests** (570+ tests) - Fast, focused tests using MemorySink
2. **Integration Tests** - Real service testing with Docker containers
3. **Benchmarks** - Performance and allocation tracking
4. **Fuzz Tests** - Parser robustness testing
5. **Race Tests** - Concurrency safety verification
6. **SelfLog Tests** - Internal diagnostics verification

## Key Features Implemented

### Core Features
- ✓ Message template parsing with format specifiers
- ✓ Property extraction and rendering
- ✓ Pipeline architecture
- ✓ Context propagation
- ✓ Structured destructuring
- ✓ LogValue protocol support

### Sinks
- ✓ Console sink with color themes
- ✓ File sink with atomic writes
- ✓ Rolling file sink with retention
- ✓ Seq sink with batching and CLEF
- ✓ Elasticsearch sink with data streams
- ✓ Splunk HEC sink
- ✓ Async sink with buffering
- ✓ Durable sink with persistence

### Advanced Features
- ✓ Dynamic level control
- ✓ Seq level controller
- ✓ Configuration from JSON/YAML
- ✓ Environment variable expansion
- ✓ slog.Handler adapter
- ✓ logr.LogSink adapter
- ✓ Generic logger interface
- ✓ Short method names
- ✓ Static analyzer (mtlog-analyzer)
- ✓ SelfLog diagnostics facility

### Enrichers & Filters
- ✓ Machine name enricher
- ✓ Thread ID enricher
- ✓ Callers enricher
- ✓ Environment enricher
- ✓ Level filtering
- ✓ Predicate filtering
- ✓ Sampling filters
- ✓ Rate limiting

## Usage Examples

### Basic Usage
```go
log := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithSeq("http://localhost:5341"),
)

log.Information("User {UserId} logged in", 123)
log.Warning("Disk usage at {Percentage:P1}", 0.85)
```

### With slog
```go
slogger := mtlog.NewSlogLogger(
    mtlog.WithConsole(),
    mtlog.WithMinimumLevel(core.DebugLevel),
)

slog.SetDefault(slogger)
```

### With logr
```go
import mtlogr "github.com/willibrandon/mtlog/adapters/logr"

logrLogger := mtlogr.NewLogger(
    mtlog.WithConsole(),
    mtlog.WithProperty("app", "myapp"),
)
```

### Dynamic Level Control
```go
levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)
log := mtlog.New(
    mtlog.WithConsole(),
    mtlog.WithLevelSwitch(levelSwitch),
)

// Change level at runtime
levelSwitch.SetLevel(core.DebugLevel)
```

### Debugging with SelfLog
```go
// Enable for troubleshooting
selflog.Enable(os.Stderr)
defer selflog.Disable()

// Or use environment variable
// export MTLOG_SELFLOG=stderr

// Custom sinks can use selflog
func (s *MySink) Emit(event *core.LogEvent) {
    if err := s.doEmit(event); err != nil {
        if selflog.IsEnabled() {
            selflog.Printf("[mysink] emit failed: %v", err)
        }
    }
}
```

## Important Notes

- The library is feature-complete and ready for production use
- All performance targets have been met or exceeded
- Integration with major logging ecosystems is complete
- Comprehensive test coverage ensures reliability
- CI/CD pipeline ensures quality across platforms