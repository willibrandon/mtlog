# Changelog

All notable changes to the mtlog-analyzer Zed extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of mtlog-analyzer extension for Zed editor
- Full Language Server Protocol (LSP) support via mtlog-lsp wrapper
- Real-time diagnostics for all MTLOG001-MTLOG013 issues:
  - Template/argument mismatches
  - Format specifier validation
  - Property naming conventions (PascalCase)
  - Duplicate property detection
  - Error logging pattern validation
  - Context key suggestions
- Code actions with quick fixes:
  - Convert property names to PascalCase
  - Add missing template arguments
  - Remove extra template arguments
- Automatic binary detection in standard Go paths:
  - `$GOBIN`
  - `$GOPATH/bin`
  - `$HOME/go/bin`
  - `/usr/local/bin`
  - System PATH
- Configurable analyzer settings via Zed's LSP configuration
- Support for custom analyzer flags:
  - `-strict` for strict validation
  - `-common-keys` for additional context keys
  - `-disable` to disable specific checks
  - `-ignore-dynamic-templates` for dynamic template handling
  - `-strict-logger-types` for type-specific analysis
  - `-downgrade-errors` for CI migration support
- Proper UTF-16 code unit handling for accurate text positioning
- Diagnostic and fix caching for improved performance
- WASM-based extension using Rust (wasm32-wasip2 target)

### Technical Details
- Implements LSP protocol with JSON-RPC communication
- Separate caching for diagnostics and code actions to avoid conflicts
- Optimized file path matching and comparison
- Comprehensive error handling and logging
- Zero-configuration with sensible defaults
- CI/CD integration with GitHub Actions