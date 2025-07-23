# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-23

### Added
- **Core Features**
  - Zero-allocation logging for simple messages (13.6 ns/op)
  - Message templates with positional property extraction and format specifiers
  - Pipeline architecture for clean separation of concerns
  - Type-safe generics for better compile-time safety
  - LogValue interface for safe logging of sensitive data
  - Short method names (Info/Warn) alongside full names for idiomatic Go usage
  - slog.Handler adapter for Go 1.21+ compatibility
  - logr.LogSink adapter for Kubernetes ecosystem compatibility
  - Fuzz testing for message template parser
  - Race condition tests for async and durable sinks
  - Comprehensive benchmarks for typical real-world scenarios

- **Sinks & Output**
  - Console sink with customizable themes (dark, light, ANSI colors)
  - File sink with rolling policies (size, time-based)
  - Seq integration with CLEF format and dynamic level control
  - Elasticsearch sink for centralized log storage and search
  - Splunk sink with HEC (HTTP Event Collector) support
  - Async sink wrapper for high-throughput scenarios
  - Durable buffering with persistent storage for reliability (now with configurable channel buffer size)

- **Pipeline Components**
  - Rich enrichment with built-in and custom enrichers
  - Advanced filtering including rate limiting and sampling
  - Type-safe destructuring with caching for performance
  - Dynamic level control with runtime adjustments
  - Configuration from JSON for flexible deployment

- **Performance**
  - 8.7x faster than zap for simple string logging
  - Zero allocations for trivial logging path
  - Comprehensive benchmarks against zap and zerolog

### Security
- Removed hardcoded test tokens from integration tests
- Added proper environment variable requirements for sensitive data

[0.1.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.1.0