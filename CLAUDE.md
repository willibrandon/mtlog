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

# Run tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem ./...

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

1. **Zero Allocations** - Simple log operations should have zero allocations
2. **Performance** - Target performance within 20% of zap/zerolog
3. **Serilog Compatibility** - API should feel familiar to Serilog users
4. **Go Idiomatic** - Follow Go conventions and patterns

## Implementation Status

According to the design document's roadmap:
- Phase 1: Core Foundation (not started)
- Phase 2: Pipeline Implementation
- Phase 3: Seq Integration
- Phase 4: Advanced Features
- Phase 5: Production Readiness
- Phase 6: Community Release

Currently, only the design document exists - no implementation has begun.