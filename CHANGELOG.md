# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **RenderMessage() Method** - Public API for rendering message templates (#64)
  - New `RenderMessage()` method on `core.LogEvent` for custom sinks
  - Properly handles capturing operators (`{@Property}`), scalar hints (`{$Property}`), and format specifiers (`{Property:format}`)
  - Enables custom sinks to render message templates without accessing internal parser
  - Returns the original template as fallback if parsing fails
  - Example: `message := event.RenderMessage()` renders the template with all properties

- **Zed Extension** - Full editor support via Language Server Protocol (#56)
  - New `mtlog-lsp` command providing LSP wrapper for mtlog-analyzer
  - Native Zed extension using Rust/WASM (wasm32-wasip2 target)
  - Real-time diagnostics for all MTLOG001-MTLOG013 issues
  - Code actions with quick fixes for common problems
  - Automatic binary detection in standard Go paths
  - Configurable analyzer flags and custom paths
  - Proper UTF-16 code unit handling for accurate text positioning
  - Comprehensive CI/CD integration with GitHub Actions

## [0.9.0] - 2025-08-19

### Added
- **Core Library**
  - `With()` method for structured field logging (#42, #55)
    - Adds key-value pairs to log events: `logger.With("service", "api", "version", "1.0")`
    - Chainable for building context: `logger.With("env", "prod").With("region", "us-west")`
    - Performance optimized: 0 allocations when no fields, 2 allocations for ≤64 fields
    - Maintains structured properties for Serilog compatibility
    - Comprehensive analyzer diagnostics (MTLOG009-MTLOG013) for common mistakes
  
  - **OpenTelemetry (OTEL) Adapter** - Full integration with OpenTelemetry ecosystem
    - OTLP exporter with gRPC and HTTP transports
    - Adaptive sampling based on trace decisions
    - Automatic trace/span context correlation
    - Resource detection (process, OS, runtime)
    - Configurable batching and retry logic
    - TLS support with automatic detection
    - Comprehensive integration tests with real OTEL collector
  
  - **Modern Serilog Alignment** - Enhanced formatting capabilities (#37)
    - New `:j` format specifier for JSON output: `{Data:j}` renders any value as JSON
    - New `:q` format specifier to explicitly quote strings: `{Name:q}` → `"Alice"`
    - New `:l` format specifier for literal string output (no escaping)
    - Numeric property indexing: `{0}`, `{1}` for positional arguments (like .NET string.Format)
    - Structs render in Go style: `{Field1:value1 Field2:value2}`
    - Improved nil handling with `Null{}` sentinel type
    - Smart byte slice handling (UTF-8 string vs byte array)
  
  - **Template Cache Security Fix** - LRU cache with bounded size (#39)
    - Prevents memory exhaustion from unbounded template generation
    - Configurable cache size (default: 10,000 entries)
    - Sharded design for concurrent access (up to 64 shards)
    - O(1) operations with proper LRU eviction
    - Optional TTL support for time-based expiration

- **mtlog-analyzer Static Analysis Tool**
  - **With() Method Diagnostics** (MTLOG009-MTLOG013)
    - MTLOG009: Empty With() calls
    - MTLOG010: Duplicate properties in With()
    - MTLOG011: With() called on nil logger
    - MTLOG012: Invalid property count (odd number of arguments)
    - MTLOG013: Non-string property names
    - Suppression support via comments for all With() diagnostics
  
- **IDE Extensions**
  - **VS Code Extension** - Support for With() method diagnostics
    - Real-time validation of With() method usage
    - Quick fixes for common With() issues
    - Updated to handle all new MTLOG009-MTLOG013 diagnostics
  
  - **GoLand Plugin** - Support for With() method diagnostics
    - Full integration of With() method analysis
    - Quick fixes and intention actions for With() issues
    - Comprehensive test coverage for new diagnostics
  
  - **Neovim Plugin** - Enhanced reliability and With() diagnostics
    - Support for MTLOG009-MTLOG013 diagnostics
    - Improved test file creation in Go module context
    - Better fix descriptions in MtlogQuickFix menu
    - Comprehensive real-world testing framework

### Documentation
- Added comprehensive With() method examples and documentation
- Created OpenTelemetry adapter documentation with integration examples
- Updated README with new format specifiers and numeric indexing examples
- Added static analysis section covering With() diagnostics

## [0.8.1] - 2025-08-10

### Fixed
- **Neovim Plugin** - Fixed commands not loading with lazy.nvim and other plugin managers
  - Renamed `plugin/mtlog.lua` to `plugin/mtlog.vim` and wrapped Lua code in vim heredoc
  - This ensures proper command registration regardless of plugin manager or loading strategy
  - Commands are now available immediately after plugin installation without manual setup

## [0.8.0] - 2025-08-10

### Added
- **Neovim Plugin** - Comprehensive plugin for mtlog-analyzer integration (#31)
  - Real-time analysis with debouncing and smart activation
  - LSP integration providing code actions through standard interface
  - Queue management for concurrent analysis with pause/resume
  - Context rules for auto-enable/disable based on patterns
  - Diagnostic suppression at session and workspace levels
  - Interactive help system with `:MtlogHelp` and `:MtlogExplain`
  - Telescope integration for fuzzy finding
  - Statusline component with diagnostic counts
  - 30+ test files with comprehensive coverage
  - Requires Neovim >= 0.8.0

## [0.7.7] - 2025-08-08

### Added
- **Analyzer** - MTLOG002 quick fixes for invalid format specifiers (#29)
  - Automatically suggests valid format specifiers based on common mistakes
  - Handles .NET-style format strings (e.g., "d3" → "000", "f2" → "F2")
  - Supports all mtlog format types: numeric, float, percentage, exponential, hexadecimal
  - Available in strict mode for comprehensive format validation
  
- **Analyzer** - MTLOG005 enhanced capturing hints with LogValue() stub generation (#30)
  - Dual quick fixes: add @ prefix OR generate LogValue() method stub
  - Smart detection of sensitive fields (passwords, tokens, API keys, etc.)
  - Generated stubs include TODO comments for sensitive fields
  - Helps implement safe logging for complex types with sensitive data
  
- **VS Code Extension** - Format specifier and LogValue() quick fixes
  - Full support for MTLOG002 format specifier corrections
  - LogValue() method stub generation for complex types
  - Seamless integration with analyzer's suggested fixes
  
- **GoLand Plugin** - Format specifier and LogValue() quick fixes
  - Comprehensive support for MTLOG002 suggested fixes
  - LogValue() stub generation with sensitive field detection
  - Full test coverage for all new quick fix scenarios

### Changed
- **Analyzer** - Refactored codebase into focused, single-responsibility modules (#28)
  - Separated type checking, string conversion, and context key logic
  - Improved maintainability with clearer separation of concerns
  - Enhanced testability through modular design
  
- **VS Code Extension** - Modularized codebase and updated dependencies
  - Cleaner separation of analyzer, diagnostics, and code action modules
  - Updated all dependencies to latest versions
  - Improved error handling and logging

### Fixed
- **Analyzer** - Improved error handling in test flag restoration
  - Proper handling of Set() error returns in test cleanup
  - More robust test framework flag management
  
- **Documentation** - Enhanced IDE extension badges for better visibility
  - Improved contrast for both light and dark mode compatibility
  - Clearer visual indicators in marketplace listings

## [0.7.6] - 2025-08-07

### Added
- **Analyzer** - MTLOG007 quick fix for extracting repeated string literals to constants
  - Detects when context keys (e.g., "user_id", "request_id") are used multiple times
  - Generates appropriately named constants following Go naming conventions
  - Intelligently handles acronyms (ID, URL, API, etc.) in constant names
  - Finds optimal insertion position for const declarations
  
- **VS Code Extension** - String-to-constant quick fix support
  - Automatically applies MTLOG007 suggested fixes from analyzer
  - Replaces all occurrences of the string literal with the new constant
  
- **GoLand Plugin** - String-to-constant quick fix support  
  - Full support for MTLOG007 suggested fixes
  - Comprehensive test coverage with 10 different scenarios

### Fixed
- **Analyzer** - MTLOG001 TODO comment placement for existing inline comments
  - Now places TODO on next line when there's already a comment on the same line
  - Prevents double comments and maintains better code readability
  
- **CI/CD** - GoLand plugin tests now properly fail the CI pipeline
  - Removed `|| true` that was hiding test failures
  - Builds analyzer from source instead of using @latest for accurate testing

## [0.7.5] - 2025-08-06

### Fixed
- **GoLand Plugin** - Fixed @ApiStatus.OverrideOnly violation flagged by JetBrains verification
  - Extracted installation logic to static companion method to avoid direct actionPerformed invocation
  - Maintains full functionality while complying with IntelliJ Platform API requirements

## [0.7.4] - 2025-08-06

### Added
- **IDE Installation UX** - Smart analyzer detection and auto-install prompts (#22)
  - **VS Code Extension**
    - Auto-detection in standard Go locations (`$GOBIN`, `$GOPATH/bin`, `~/go/bin`)
    - One-click installation prompt when analyzer not found
    - Improved error messages with specific paths and solutions
    - Automatic path caching for performance
  
  - **GoLand Plugin**
    - Smart path detection following Go's installation precedence
    - Notification with Install/Settings actions when analyzer not found
    - Support for platform-specific locations (Windows Apps, /usr/local/go/bin)
    - findAnalyzerPath() made internal for testing

### Fixed
- **Build** - Fixed invalid Go version format in go.mod (changed from `1.23.0` to `1.23`)
- **VS Code Extension** - Added test binaries to .vscodeignore to reduce package size

## [0.7.3] - 2025-08-05

### Changed
- **Analyzer Architecture** - Centralized all quick fixes in the analyzer with stdin support (#21)
  - IDE extensions now use analyzer-provided suggested fixes exclusively
  - Transitioned from file-based to stdin-based communication for real-time analysis
  - Removed ~1000 lines of duplicate quick fix code from IDE extensions
  - Replaced os.ReadFile with AST-based analysis for better performance

### Added
- **Analyzer Quick Fixes** - Suggested fixes for MTLOG001 (template/argument mismatch) and MTLOG006 (missing error parameter)
  - MTLOG006 intelligently detects error variables in scope
  - Adds appropriate error variable or `nil` with TODO comment

### Fixed
- **Performance** - Eliminated repeated file I/O operations in both IDE extensions
- **Stability** - Fixed potential issues with stdin mode where files may not exist on disk
- **Code Quality** - Fixed analyzer tautological conditions and redundant control flow
- **GoLand Plugin** - Fixed IntelliJ IDEA Ultimate compatibility issue where plugin showed as "binary incompatible"

## [0.7.2] - 2025-08-04

### Added
- **Diagnostic Kill Switch** - Quick disable/enable for all mtlog diagnostics in IDE extensions (#19)
  - **VS Code Extension**
    - Command palette commands: "Toggle mtlog Diagnostics"
    - Clickable status bar item showing analyzer state and diagnostic counts
    - Diagnostic suppression with persistent workspace settings
    - Quick action to suppress specific diagnostic types
    - Keyboard shortcut: Ctrl+Alt+M (Cmd+Alt+M on Mac)
    - Suppression manager for managing suppressed diagnostics
  
  - **GoLand Plugin**
    - Status bar widget with visual state indicator (play/pause icons)
    - Notification bar on startup with Disable/Settings actions
    - Diagnostic suppression with immediate UI updates
    - Intention actions and quick fixes for suppressing diagnostics
    - Manage suppressed diagnostics dialog
    - Persistent state across IDE restarts

### Fixed
- **GoLand Plugin** - Fixed critical deadlock in template argument quick fix during preview generation
- **Both Extensions** - Suppressed diagnostics now disappear immediately without requiring file edits

### Changed
- **GoLand Plugin** - Inspection now enabled by default for better user experience

## [0.7.1] - 2025-08-03

### Fixed
- **VS Code Extension** - Prevent stale diagnostics from persisting after code fixes (#15)
  - Removed broken cache mechanism causing outdated error messages
  - Added version tracking to prevent race conditions
  - Diagnostics now properly clear when issues are resolved
  
- **GoLand Plugin** - Replace deprecated ProcessAdapter with ProcessListener for IntelliJ Platform 2024.2+ compatibility (#17)

## [0.7.0] - 2025-08-03

### Added
- **VS Code Quick Fixes** - Enhanced VS Code extension with quick fixes for mtlog diagnostics (#11)
  - PascalCase property name conversion (e.g., `user_id` → `UserId`)
  - Template/argument mismatch resolution (add/remove arguments)
  - Automatic save and reanalysis after applying fixes
  - Fixed parseDiagnostic for modern go vet JSON output formats
  - Quick fixes now work identically to the GoLand plugin implementation

- **GoLand Plugin** - Real-time validation for mtlog message templates in JetBrains IDEs (#9)
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

[0.8.1]: https://github.com/willibrandon/mtlog/releases/tag/v0.8.1
[0.8.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.8.0
[0.7.7]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.7
[0.7.6]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.6
[0.7.5]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.5
[0.7.4]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.4
[0.7.3]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.3
[0.7.2]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.2
[0.7.1]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.1
[0.7.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.7.0
[0.6.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.6.0
[0.5.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.5.0
[0.4.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.4.0
[0.3.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.3.0
[0.2.1]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.1
[0.2.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.2.0
[0.1.0]: https://github.com/willibrandon/mtlog/releases/tag/v0.1.0