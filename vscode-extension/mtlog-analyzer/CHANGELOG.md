# Change Log

All notable changes to the "mtlog-analyzer" extension will be documented in this file.

## [Unreleased]

## [0.9.0] - 2025-08-19

### Added
- Support for With() method diagnostics (MTLOG009-MTLOG013)
  - MTLOG009: Detects odd argument count in With() calls
  - MTLOG010: Warns about non-string keys in With()
  - MTLOG011: Identifies duplicate keys across multiple With() calls
  - MTLOG012: Warns about reserved property names
  - MTLOG013: Detects empty string keys
- Updated suppression manager to include new diagnostic codes
- Quick fixes for With() method issues provided by analyzer

## [0.8.1] - 2025-08-10

### Changed
- Version bump for v0.8.1 patch release
- No functional changes in VS Code extension

## [0.8.0] - 2025-08-10

### Changed
- Version bump for consistency with mtlog v0.8.0 release
- No functional changes in this release

## [0.7.7] - 2025-08-08

### Added
- Format specifier quick fixes for MTLOG002 diagnostics
  - Automatically corrects invalid format specifiers
  - Supports conversion from .NET-style format strings
  - Handles all mtlog format types (numeric, float, percentage, etc.)
- LogValue() method stub generation for MTLOG005 diagnostics
  - Generates complete LogValue() method implementation
  - Includes smart detection of sensitive fields
  - Adds TODO comments for fields requiring review

### Changed
- Modularized extension codebase for better maintainability
  - Separated analyzer, diagnostics, and code action logic
  - Improved error handling and logging
  - Updated all dependencies to latest versions

## [0.7.6] - 2025-08-07

### Added
- String-to-constant quick fix support for MTLOG007 diagnostics
  - Automatically extracts repeated context keys to constants
  - Replaces all occurrences of the string literal with the new constant

### Fixed
- Updated to handle new TODO comment format from analyzer

## [0.7.5] - 2025-08-06

### Fixed
- No changes to VS Code extension in this release

## [0.7.4] - 2025-08-06

### Added
- Smart analyzer detection and auto-install prompts
  - Auto-detection in standard Go locations (`$GOBIN`, `$GOPATH/bin`, `~/go/bin`)
  - One-click installation prompt when analyzer not found
  - Improved error messages with specific paths and solutions
  - Automatic path caching for performance

## [0.7.3] - 2025-08-05

### Changed
- Centralized all quick fixes in the analyzer
  - Extension now uses analyzer-provided suggested fixes exclusively
  - Transitioned from file-based to stdin-based communication
  - Removed ~500 lines of duplicate quick fix code
  - Improved performance with streaming analysis

### Added
- Support for MTLOG001 template argument quick fixes
- Support for MTLOG006 missing error parameter quick fixes

## [0.7.2] - 2025-08-04

### Added
- Initial release of mtlog-analyzer VS Code extension
- Real-time diagnostics from mtlog-analyzer
- Quick fixes for property naming (PascalCase)
- Support for Windows, macOS, and Linux
- Configurable analyzer path
- Automatic save and reanalysis after applying fixes