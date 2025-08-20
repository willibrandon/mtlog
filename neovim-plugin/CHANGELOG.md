# Changelog

All notable changes to mtlog.nvim will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.9.0] - 2025-08-19

### Added
- **With() Method Diagnostics** - Support for new mtlog-analyzer With() method checks
  - MTLOG009: Detects odd argument count in With() calls (requires key-value pairs)
  - MTLOG010: Warns about non-string keys in With() calls
  - MTLOG011: Informational cross-call duplicate property detection
  - MTLOG012: Warns about reserved property names (Timestamp, Level, etc.)
  - MTLOG013: Detects empty string keys in With() calls

- **Comprehensive Real-World Testing** - Improved reliability and test coverage
  - Test file creation now works within Go module context
  - Added real-world testing framework
  - Direct disk write for analyzer integration tests
  
### Enhanced
- **Quick Fix Parser** - Updated to handle new position format
  - Now supports both legacy `range` objects and new `pos`/`end` string format
  - Handles "file:line:column" position strings from With() diagnostics
  - Backward compatible with existing diagnostics
  
- **Help System** - Added explanations for all new diagnostic codes
  - Interactive help for MTLOG009-MTLOG013
  - Examples and fix suggestions for each new diagnostic
  - Updated quick reference card

- **Quick Fix Descriptions** - Improved fix menu display
  - Shows proper fix descriptions in MtlogQuickFix menu
  - Better user experience with clear action descriptions

### Fixed
- **macOS Compatibility** - Resolved macOS-specific issues
  - Now uses only vim.health API to avoid legacy errors

- **Position Parsing** - Improved handling of different position formats
  - Correctly parses 1-indexed columns from analyzer
  - Handles both insertion and replacement operations
  - Works with exclusive end positions

## [0.8.1] - 2025-08-10

### Fixed
- **Plugin Loading** - Fixed commands not registering with lazy.nvim and other plugin managers
  - Renamed `plugin/mtlog.lua` to `plugin/mtlog.vim` with Lua heredoc syntax
  - Commands now load correctly regardless of plugin manager or lazy loading strategy
  - Ensures compatibility with all Neovim plugin managers (lazy.nvim, packer, vim-plug, etc.)

## [0.8.0] - 2025-08-10

### Features
- **Initial Release** - Comprehensive Neovim plugin for mtlog-analyzer
  - Real-time analysis on save with debouncing
  - Smart activation for Go projects
  - Quick fixes via code actions
  - Statusline integration with diagnostic counts
  - Performance optimized with caching and async operations
  - Highly configurable with extensive options

- **Core Features**
  - Automatic analysis with file watching
  - Diagnostic display with virtual text, signs, and underlines
  - Severity level mappings for all MTLOG codes
  - Rate limiting for large workspaces
  - Cache with TTL for performance

- **Commands**
  - `:MtlogAnalyze` - Analyze current buffer or file
  - `:MtlogAnalyzeWorkspace` - Analyze entire workspace
  - `:MtlogClear` - Clear diagnostics
  - `:MtlogEnable/Disable/Toggle` - Control analyzer state
  - `:MtlogStatus` - Show plugin status
  - `:MtlogCache` - Manage analysis cache
  - `:MtlogQuickFix` - Apply quick fix at cursor
  - `:MtlogSuppress/Unsuppress` - Manage diagnostic suppressions
  - `:MtlogToggleDiagnostics` - Global diagnostics kill switch
  - `:MtlogCodeAction` - Show code actions menu

- **Advanced Features**
  - **LSP Integration** - Native code actions through vim.lsp.buf.code_action()
  - **Queue Management** - Concurrent analysis with pause/resume
  - **Context Rules** - Auto-enable/disable based on file patterns
  - **Diagnostic Suppression** - Session and workspace-level suppression
  - **Help System** - Interactive help with `:MtlogHelp` and `:MtlogExplain`
  - **Telescope Integration** - Fuzzy finder for suppressions and diagnostics

- **Performance Optimizations**
  - Analysis queue with configurable concurrency
  - Smart caching with file modification time tracking
  - Rate limiting to prevent overwhelming the analyzer
  - Debouncing for file change events

- **Configuration**
  - Extensive customization options
  - Per-diagnostic severity levels
  - Virtual text, signs, and underline configuration
  - Custom analyzer flags support
  - File pattern ignoring
  - Quick fix auto-save

### Documentation
- Comprehensive Vim help documentation (`:help mtlog`)
- Interactive help system with diagnostic explanations
- Quick reference card for common operations
- Welcome message for first-time users

### Testing
- 30+ test files with comprehensive coverage
- Integration tests for all commands
- Queue management and concurrency tests
- Context rules and suppression tests
- CI/CD with matrix testing across Neovim versions

## Installation

The plugin is distributed as part of the main mtlog repository with `rtp` configuration:

```lua
-- lazy.nvim
{
  'willibrandon/mtlog',
  rtp = 'neovim-plugin',
  ft = 'go',
  config = function()
    require('mtlog').setup()
  end,
}
```

## Requirements

- Neovim >= 0.8.0 (0.7.0 support dropped due to missing "log" stdpath)
- Go >= 1.21
- mtlog-analyzer installed and in PATH