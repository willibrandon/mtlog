# Changelog

All notable changes to mtlog.nvim will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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