# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **mtlog-analyzer** - Static analysis tool for mtlog usage
  - Detects template/argument mismatches
  - Validates format specifiers
  - Suggests property naming conventions (PascalCase)
  - Detects duplicate properties
  - Suggests destructuring hints for complex types
  - Validates error logging patterns
  - Suggests constants for common context keys
  - Configuration flags: `-strict`, `-common-keys`, `-disable`, `-ignore-dynamic-templates`, `-strict-logger-types`, `-downgrade-errors`
  - IDE integration with automatic fixes
  - Can be used standalone or as a go vet tool
  - Comprehensive test coverage (88.6%)

## [0.2.1] - 2025-01-27

### Changed
- **Internal Organization**
  - Moved implementation packages (destructure, enrichers, filters, formatters, parser, handler) to `internal/` directory
  - Public API remains unchanged - all user-facing packages and interfaces are unaffected
  
### Fixed
- **CI/CD**
  - Updated GitHub Actions fuzz test paths to use new internal package locations
  - Fixed Splunk integration tests by properly exposing management port 8089
  - Updated Splunk test configuration to use non-default password for remote login
  - Increased dynamic level filtering performance threshold to 150ns to account for OS variance

### Documentation
- Updated testing documentation to reflect actual Splunk requirements

## [0.2.0] - 2025-01-27

### Added
- **Output Templates**
  - Serilog-style output templates with customizable formatting
  - Support for timestamp, level, message, and property formatting
  - Format specifiers for timestamps (HH:mm:ss, yyyy-MM-dd), levels (u3, u, l), and numbers (F2, P1, 000)
  - Go template syntax support (`{{.Property}}`) alongside traditional `{Property}` syntax
  - Template analysis for performance optimization
  
- **Console Themes**
  - New Literate theme with ANSI 256-color support for beautiful, readable output
  - Enhanced color selection algorithms for better visual hierarchy
  - Automatic terminal capability detection with fallback modes
  - Environment variable support for color control (MTLOG_FORCE_COLOR)
  
- **Source Context Enrichment**
  - Automatic source context detection from call stack
  - LRU cache with configurable size via MTLOG_SOURCE_CTX_CACHE environment variable
  - Intelligent filtering of mtlog internal packages
  
- **API Improvements**
  - New `Build()` function for error-safe logger initialization
  - Better error propagation in template parsing
  - Simplified API by removing redundant helper functions
  
- **Testing & Quality**
  - Comprehensive fuzz tests for output template parser
  - Fuzz tests for message template parser enhancements
  - CI integration for automated fuzz testing
  
### Changed
- **Performance**
  - Updated benchmarks to reflect current performance (17.3 ns/op for simple logging)
  - Optimized Windows VT terminal processing to avoid redundant syscalls
  - Template parsing occurs once at initialization for better performance
  
- **Console Output**
  - Improved bracket and status code rendering in console themes
  - Better handling of dimmed text for readability
  - Enhanced Windows terminal support with VT100 processing
  
### Fixed
- Template parsing errors are now properly propagated instead of being silently ignored
- Windows console flicker eliminated by checking VT processing state before enabling
- Source context cache now properly bounded with LRU eviction
- All golangci-lint issues resolved for clean CI builds

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

[0.2.1]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.1
[0.2.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.0
[0.1.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.1.0