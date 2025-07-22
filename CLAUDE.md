# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the **mtlog** (Message Template Logging) project - a Serilog-inspired structured logging library for Go. The project aims to bring message templates and pipeline architecture to the Go ecosystem, with native Seq integration.

## Key Architecture Concepts

### 1. Message Templates
- Templates like `"User {UserId} logged in"` are preserved throughout the pipeline
- Properties are extracted from templates and matched positionally to arguments
- Templates serve as both human-readable messages and event types for grouping/analysis

### 2. Pipeline Architecture
The logging pipeline follows this flow:
```
Message Template Parser → Enrichment → Filtering → Destructuring → Sinks (Output)
```

### 3. Core Interfaces
- `Logger` - Main logging interface with methods like `Information()`, `Error()`, etc.
- `LogEventEnricher` - Adds contextual properties to log events
- `LogEventFilter` - Determines which events proceed through pipeline
- `Destructurer` - Converts complex types to log-appropriate representations
- `LogEventSink` - Outputs events to destinations (Console, File, Seq, etc.)

## Development Commands

Since this is a new Go project without implementation yet, here are the expected commands once development begins:

```bash
# Initialize Go module
go mod init github.com/[username]/mtlog

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run only integration tests
go test -tags=integration ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -run TestSeqIntegration ./...

# Run tests in container (once docker-compose.test.yml is created)
docker-compose -f docker-compose.test.yml up --abort-on-container-exit

# Build the library
go build ./...

# Format code
go fmt ./...

# Run linter (once added)
golangci-lint run

# Generate mocks (if using mockgen)
go generate ./...
```

## Project Structure (Planned)

Based on the design document, the expected structure will be:
- `core/` - Core logging interfaces and types
- `parser/` - Message template parsing
- `enrichers/` - Built-in enrichers (machine name, process, etc.)
- `filters/` - Level and predicate filters
- `destructurers/` - Type destructuring logic
- `sinks/` - Output destinations (console, file, seq)
- `seq/` - Seq-specific integration and CLEF formatting

## Important Design Goals

1. **Zero Allocations** - Simple log operations should have zero allocations ✓
2. **Performance** - Target performance within 20% of zap/zerolog ✓
3. **Serilog Compatibility** - API should feel familiar to Serilog users
4. **Go Idiomatic** - Follow Go conventions and patterns

## Performance

The library achieves **zero allocations** for simple logging through a fast path implementation:
- Simple log: 13.6ns/op, 0B/op, 0 allocs
- With properties: 183ns/op, 448B/op, 4 allocs
- Below minimum level: 1.4ns/op, 0B/op, 0 allocs

See [PERFORMANCE.md](PERFORMANCE.md) for detailed benchmarks and optimization history.

## Testing Strategy

### Container-Based Testing
The project uses real infrastructure for integration tests via Docker Compose:
- **Seq** - Real Seq instance for testing log ingestion and querying
- **Elasticsearch** - Real Elasticsearch for testing the ES sink
- **No Mocks** - Integration tests use actual services, not mocks

### Test Types
1. **Unit Tests** - Use in-memory sinks (MemorySink) for fast feedback
2. **Integration Tests** - Test against real Seq/Elasticsearch in containers
3. **Benchmarks** - Track performance and allocations from day one
4. **Table-Driven Tests** - Go-style tests for comprehensive coverage

### Testing Philosophy
- Real dependencies over mocks
- Integration-first approach
- Continuous performance tracking
- Container-based infrastructure

## Implementation Status

According to the design document's roadmap:
- Phase 1: Core Foundation (not started)
- Phase 2: Pipeline Implementation
- Phase 3: Seq Integration
- Phase 4: Advanced Features
- Phase 5: Production Readiness
- Phase 6: Community Release

Currently, only the design document exists - no implementation has begun.

## Week-by-Week Development Plan

### Week 1: Core + Learning Go
- Days 1-2: Message template parser (string manipulation)
- Days 3-4: Basic logger and sinks (interfaces and methods)
- Days 5-7: File operations and basic enrichers (io package)

### Week 2: Pipeline + Seq
- Days 1-2: Pipeline architecture (composition patterns)
- Days 3-4: Seq sink with batching (goroutines and channels)
- Days 5-7: Container-based testing setup

### Week 3: Polish + Performance
- Days 1-2: Performance optimization (profiling)
- Days 3-4: Elasticsearch sink
- Days 5-7: Documentation and examples