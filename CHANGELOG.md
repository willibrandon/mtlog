# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **GoLand Plugin** - Real-time validation for mtlog message templates in JetBrains IDEs
  - Automatic template/argument validation with intelligent highlighting
  - Quick fixes for property naming (PascalCase) and argument count mismatches
  - Three severity levels with configurable mappings
  - Performance optimized with caching and debouncing
  - Configurable analyzer path and flags via settings
  - Support for Windows, macOS, and Linux
  - Published to JetBrains Marketplace under plugin ID "com.mtlog.analyzer"
  - Full integration tests with real mtlog-analyzer

- **VS Code Extension** - Real-time validation for mtlog message templates (#7)
  - Automatic diagnostics on save and file changes with 500ms debounce
  - Three severity levels: errors (red), warnings (yellow), suggestions (blue)
  - Status bar indicator showing analysis state and issue count
  - SHA-256 based caching to skip redundant analysis
  - CPU-based concurrency control (uses half of logical cores)
  - Automatic installation prompt when mtlog-analyzer not found
  - Configurable analyzer path and flags via settings
  - Published to VS Code Marketplace under publisher ID "mtlog"
  - Originally released with dual-tagging strategy (`ext/v*`), now uses unified release process

### Changed
- **Unified Release Process** - All components now release together
  - Single `v*` tag releases library, binaries, and both IDE extensions
  - Removed `ext/v*` dual-tagging strategy for simplicity
  - VS Code and GoLand extensions publish to their marketplaces automatically

## [0.6.0] - 2025-07-30

### Added
- **OTEL Compatibility** - Support for dots in property names (#4)
  - Enable OpenTelemetry-style property names like `{http.method}`, `{service.name}`, `{db.system}`
  - Validation prevents properties that are only dots (e.g., `{.}`, `{..}`)
  - Analyzer updated to skip PascalCase suggestions for dotted properties
  - Works with all features: format specifiers, capturing hints, Go template syntax
  - Example: `log.Information("HTTP {http.method} to {http.url} took {http.duration.ms:F2}ms", "GET", "/api", 123.45)`

### Changed
- **BREAKING: New Output Template Syntax** - `${...}` for built-in elements (#6)
  - Output templates now use `${...}` syntax for built-in elements to prevent ambiguity
  - Built-in elements: `${Timestamp}`, `${Level}`, `${Message}`, `${Exception}`, `${NewLine}`, `${Properties}`
  - User properties continue to use `{...}` syntax: `{UserId}`, `{RequestId}`, etc.
  - Prevents conflicts when logging properties with names like "Message" or "Level"
  - Example: `mtlog.WithConsoleTemplate("[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}")`
  - All format specifiers work with the new syntax: `${Timestamp:yyyy-MM-dd}`, `${Level:u3}`
  - Migration required: Update all output templates in your configuration and code

- **BREAKING: Renamed 'destructuring' to 'capturing'** (#5)
  - `Destructurer` interface → `Capturer` interface
  - `TryDestructure` method → `TryCapture` method
  - `WithDestructuring()` → `WithCapturing()`
  - `internal/destructure/` → `internal/capture/`
  - `examples/destructuring/` → `examples/capturing/`
  - All documentation and analyzer messages updated
  - This better reflects the operation: capturing structured data from objects
  - Migration required: Update all references in your code

## [0.5.0] - 2025-07-29

### Added
- **SelfLog** - Internal diagnostics facility for debugging silent failures
  - Zero-cost when disabled with single atomic pointer check
  - Flexible output to any `io.Writer` or custom function
  - Thread-safe with `Sync()` wrapper for concurrent writes
  - Environment variable support (`MTLOG_SELFLOG=stderr/stdout/file`)
  - Structured output format: `{timestamp} [{component}] {message}`
  - `IsEnabled()` guard to avoid formatting costs when disabled
  - Performance: 0.37ns/op when disabled (with guard), 148ns/op to io.Discard
  
- **Comprehensive SelfLog Instrumentation**
  - All sinks report write/emit failures with contextual information
  - Async sink: buffer overflow, worker panics, dropped event counts
  - Durable sink: persistence failures, buffer corruption
  - Network sinks (Seq, Elasticsearch, Splunk): connection errors, HTTP failures
  - File/Rolling sinks: permission errors, disk space issues
  - Template validation: unclosed properties, empty names, invalid syntax
  - Capturing: panic recovery with type information
  - Configuration: unknown types, parse failures, type mismatches
  
- **Idempotent Close Methods**
  - ElasticsearchSink, SeqSink, and SplunkSink now use `sync.Once` for safe multiple calls
  - Prevents double-close panics in complex shutdown scenarios
  
- **Template Validation**
  - Runtime validation via `parser.ValidateTemplate()` 
  - Detects unclosed properties, empty property names, spaces in names
  - Validation errors logged through selflog for debugging
  
### Documentation
- Added comprehensive troubleshooting guide (`docs/troubleshooting.md`)
- SelfLog usage examples for debugging common issues
- Custom sink implementation guidance with selflog integration
- Performance troubleshooting tips and profiling guidance

### Testing
- Added selflog tests for all instrumented components (241 total tests)
- Race condition tests for concurrent selflog usage
- Benchmarks confirming performance targets
- Cross-platform compatibility (Windows path handling)

## [0.4.0] - 2025-07-29

### Changed
- **Dependency Management**
  - Moved benchmarks to separate module (`benchmarks/`) to isolate zap and zerolog dependencies
  - Moved logr integration to separate module (`adapters/logr/`) to avoid forcing go-logr dependency
  - Users no longer need to download benchmark-only dependencies when using mtlog
  - Fixed zero-allocation benchmarks by implementing `EmitSimple` in benchmark discardSink
  - Main module now has zero external dependencies

### Breaking Changes
- **logr Integration**
  - `mtlog.NewLogrLogger()` has been moved to the separate `github.com/willibrandon/mtlog/adapters/logr` module
  - Import the adapter module and use `mtlogr.NewLogger()` instead
  - Example migration:
    ```go
    // Before:
    logrLogger := mtlog.NewLogrLogger(...)
    
    // After:
    import mtlogr "github.com/willibrandon/mtlog/adapters/logr"
    logrLogger := mtlogr.NewLogger(...)
    ```

### Documentation
- Added comprehensive Go documentation for the logr adapter module
- Updated README, website, and examples to reflect the new import path
- Added CI/CD support for testing adapter modules

## [0.3.0] - 2025-07-28

### Added
- **ForType** - Type-based logging with automatic SourceContext
  - `ForType[T]()` function for automatic SourceContext extraction from Go types using reflection
  - `TypeNameOptions` struct for customizable type name formatting (package inclusion, depth, prefix/suffix)
  - `ExtractTypeName[T]()` for custom logger creation with type-based naming
  - Thread-safe LRU cache with configurable size limits via `MTLOG_TYPE_NAME_CACHE_SIZE` environment variable (default: 10,000 entries)
  - Multi-tenant support with `ForTypeWithCacheKey()` and `ExtractTypeNameWithCacheKey()` for separate cache namespaces
  - Performance: ~213ns/op uncached, ~148ns/op cached with ~1.4x speedup from caching
  - Robust generic type handling including deeply nested generics, multiple type parameters, and complex built-in combinations
  - Cache statistics with hits, misses, evictions, hit ratio, current size, and max size monitoring
  - `SimplifyAnonymous` option for cleaner anonymous struct names
  - `WarnOnUnknown` option for debugging interface types and unresolvable types
  - Comprehensive test coverage (150+ tests) including edge cases, concurrency, and complex generics
  - Complete documentation with multi-tenant examples and cache configuration

- **LogContext** - Scoped property propagation through context
  - `PushProperty()` function for attaching properties to context that automatically flow through all log events
  - Thread-safe immutable implementation with property copying for concurrent access
  - Property inheritance through context hierarchy with proper precedence handling
  - Property precedence: event-specific > ForContext > LogContext > standard context values
  - `LogContextEnricher` integration with existing enrichment pipeline
  - Comprehensive test coverage including edge cases, inheritance, and integration scenarios
  - Performance benchmarks showing reasonable overhead (3.3x for single property, 4.2x for multiple properties)
  - Deep nesting support (10+ levels) with predictable performance characteristics
  - Complete documentation with examples and real-world usage patterns
  - Working example demonstrating web service request tracing

- **mtlog-analyzer** - Static analysis tool for mtlog usage
  - Detects template/argument mismatches
  - Validates format specifiers
  - Suggests property naming conventions (PascalCase)
  - Detects duplicate properties
  - Suggests capturing hints for complex types
  - Validates error logging patterns
  - Suggests constants for common context keys
  - Configuration flags: `-strict`, `-common-keys`, `-disable`, `-ignore-dynamic-templates`, `-strict-logger-types`, `-downgrade-errors`
  - IDE integration with automatic fixes
  - Can be used standalone or as a go vet tool
  - Comprehensive test coverage (88.6%)

## [0.2.1] - 2025-07-27

### Changed
- **Internal Organization**
  - Moved implementation packages (capture, enrichers, filters, formatters, parser, handler) to `internal/` directory
  - Public API remains unchanged - all user-facing packages and interfaces are unaffected
  
### Fixed
- **CI/CD**
  - Updated GitHub Actions fuzz test paths to use new internal package locations
  - Fixed Splunk integration tests by properly exposing management port 8089
  - Updated Splunk test configuration to use non-default password for remote login
  - Increased dynamic level filtering performance threshold to 150ns to account for OS variance

### Documentation
- Updated testing documentation to reflect actual Splunk requirements

## [0.2.0] - 2025-07-27

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

## [0.1.0] - 2025-7-23

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
  - Type-safe capturing with caching for performance
  - Dynamic level control with runtime adjustments
  - Configuration from JSON for flexible deployment

- **Performance**
  - 8.7x faster than zap for simple string logging
  - Zero allocations for trivial logging path
  - Comprehensive benchmarks against zap and zerolog

### Security
- Removed hardcoded test tokens from integration tests
- Added proper environment variable requirements for sensitive data

[0.6.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.6.0
[0.5.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.5.0
[0.4.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.4.0
[0.3.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.3.0
[0.2.1]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.1
[0.2.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.0
[0.1.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.1.0