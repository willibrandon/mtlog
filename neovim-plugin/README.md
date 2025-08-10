# mtlog.nvim

A Neovim plugin for [mtlog-analyzer](https://github.com/willibrandon/mtlog), providing real-time static analysis for Go code using the mtlog structured logging library.

## Features

- 🔍 **Real-time Analysis** - Automatic analysis on save with debouncing
- 🎯 **Smart Activation** - Automatically enables for Go projects
- 💡 **Quick Fixes** - Apply suggested fixes with code actions
- 📊 **Statusline Integration** - Display diagnostic counts in your statusline
- ⚡ **Performance Optimized** - Caching, rate limiting, and async operations
- 🛠️ **Highly Configurable** - Customize every aspect of the plugin

## Requirements

- Neovim >= 0.7.0
- Go >= 1.21
- [mtlog-analyzer](https://github.com/willibrandon/mtlog/cmd/mtlog-analyzer) installed

## Installation

### Using [lazy.nvim](https://github.com/folke/lazy.nvim)

```lua
{
  'willibrandon/mtlog',
  rtp = 'neovim-plugin',
  ft = 'go',
  config = function()
    require('mtlog').setup({
      -- your configuration
    })
  end,
}
```

### Using [packer.nvim](https://github.com/wbthomason/packer.nvim)

```lua
use {
  'willibrandon/mtlog',
  rtp = 'neovim-plugin',
  ft = { 'go' },
  config = function()
    require('mtlog').setup({
      -- your configuration
    })
  end,
}
```

### Using [vim-plug](https://github.com/junegunn/vim-plug)

```vim
Plug 'willibrandon/mtlog', { 'rtp': 'neovim-plugin', 'for': 'go' }

" In your init.vim or init.lua
lua require('mtlog').setup()
```

### Install mtlog-analyzer

```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
```

## Configuration

### Default Configuration

```lua
require('mtlog').setup({
  -- Path to mtlog-analyzer executable
  analyzer_path = 'mtlog-analyzer',
  
  -- Automatically enable for Go projects
  auto_enable = true,
  
  -- Automatically analyze on save/change
  auto_analyze = true,
  
  -- Debounce time in milliseconds
  debounce_ms = 500,
  
  -- Virtual text configuration
  virtual_text = {
    enabled = true,
    prefix = '■ ',
    spacing = 2,
    severity_limit = vim.diagnostic.severity.HINT,
  },
  
  -- Sign column configuration
  signs = {
    enabled = true,
    priority = 10,
    text = {
      [vim.diagnostic.severity.ERROR] = '✗',
      [vim.diagnostic.severity.WARN] = '⚠',
      [vim.diagnostic.severity.INFO] = 'ℹ',
      [vim.diagnostic.severity.HINT] = '💡',
    },
  },
  
  -- Underline configuration
  underline = {
    enabled = true,
    severity_limit = vim.diagnostic.severity.WARN,
  },
  
  -- Severity mappings for diagnostic codes
  severity_levels = {
    MTLOG001 = vim.diagnostic.severity.ERROR,  -- Template/argument mismatch
    MTLOG002 = vim.diagnostic.severity.ERROR,  -- Invalid format specifier
    MTLOG003 = vim.diagnostic.severity.ERROR,  -- Missing error in Error/Fatal
    MTLOG004 = vim.diagnostic.severity.WARN,   -- Non-PascalCase property
    MTLOG005 = vim.diagnostic.severity.WARN,   -- Complex type needs LogValue
    MTLOG006 = vim.diagnostic.severity.WARN,   -- Duplicate property
    MTLOG007 = vim.diagnostic.severity.HINT,   -- String context key suggestion
  },
  
  -- Rate limiting
  rate_limit = {
    enabled = true,
    max_files_per_second = 10,
  },
  
  -- Cache configuration
  cache = {
    enabled = true,
    ttl_seconds = 300,  -- 5 minutes
  },
  
  -- Show error notifications
  show_errors = true,
  
  -- Custom analyzer flags
  analyzer_flags = {},
  
  -- File patterns to ignore
  ignore_patterns = {
    'vendor/',
    '%.pb%.go$',
    '_test%.go$',
  },
})
```

### Minimal Configuration

```lua
require('mtlog').setup({
  -- Disable virtual text, keep only signs
  virtual_text = false,
  
  -- Simpler signs
  signs = {
    text = {
      [vim.diagnostic.severity.ERROR] = 'E',
      [vim.diagnostic.severity.WARN] = 'W',
      [vim.diagnostic.severity.INFO] = 'I',
      [vim.diagnostic.severity.HINT] = 'H',
    },
  },
})
```

## Commands

| Command | Description |
|---------|-------------|
| `:MtlogAnalyze [file]` | Analyze current buffer or specified file |
| `:MtlogAnalyzeWorkspace` | Analyze entire workspace |
| `:MtlogClear[!]` | Clear diagnostics (! for all buffers) |
| `:MtlogEnable` | Enable analyzer |
| `:MtlogDisable` | Disable analyzer |
| `:MtlogToggle` | Toggle analyzer |
| `:MtlogStatus` | Show plugin status |
| `:MtlogCache {clear\|stats}` | Manage cache |
| `:MtlogQueue {show\|stats\|clear\|pause\|resume}` | Manage analysis queue |
| `:MtlogQuickFix` | Apply quick fix at cursor |
| `:MtlogToggleDiagnostics` | Toggle global diagnostics kill switch |
| `:MtlogSuppress [id]` | Suppress a diagnostic ID |
| `:MtlogUnsuppress [id]` | Unsuppress a diagnostic ID |
| `:MtlogShowSuppressions` | Show suppressed diagnostics |

## Keybindings

Example keybindings you can add to your configuration:

```lua
vim.keymap.set('n', '<leader>ma', ':MtlogAnalyze<CR>', { desc = 'Analyze current file' })
vim.keymap.set('n', '<leader>mw', ':MtlogAnalyzeWorkspace<CR>', { desc = 'Analyze workspace' })
vim.keymap.set('n', '<leader>mf', ':MtlogQuickFix<CR>', { desc = 'Apply quick fix' })
vim.keymap.set('n', '<leader>mc', ':MtlogClear<CR>', { desc = 'Clear diagnostics' })
vim.keymap.set('n', ']m', function() require('mtlog.diagnostics').goto_next() end, { desc = 'Next mtlog diagnostic' })
vim.keymap.set('n', '[m', function() require('mtlog.diagnostics').goto_prev() end, { desc = 'Previous mtlog diagnostic' })
```

## Statusline Integration

### Native Vim Statusline

```lua
-- Add to your statusline
vim.o.statusline = vim.o.statusline .. ' %{luaeval("require(\'mtlog.statusline\').get_component()")}'
```

### Lualine

```lua
require('lualine').setup({
  sections = {
    lualine_c = {
      require('mtlog.statusline').lualine_component(),
    },
  },
})
```

### Custom Integration

```lua
-- Get diagnostic counts
local counts = require('mtlog').get_counts()
print(string.format('Errors: %d, Warnings: %d', counts.errors, counts.warnings))

-- Get formatted component
local component = require('mtlog.statusline').get_component({
  nerd_fonts = true,
  format = 'short',  -- 'minimal', 'short', or 'long'
  show_zero = false,
})
```

## API

### Main Module (`require('mtlog')`)

```lua
local mtlog = require('mtlog')

-- Setup with configuration
mtlog.setup(opts)

-- Enable/disable analyzer
mtlog.enable()
mtlog.disable()
mtlog.toggle()

-- Analyze files
mtlog.analyze_buffer(bufnr)
mtlog.analyze_workspace()

-- Clear diagnostics
mtlog.clear(bufnr)
mtlog.clear_all()

-- Get diagnostic counts
local counts = mtlog.get_counts()

-- Check analyzer availability
local available = mtlog.is_available()
local version = mtlog.get_version()
```

### Diagnostics Module

```lua
local diagnostics = require('mtlog.diagnostics')

-- Navigate diagnostics
diagnostics.goto_next()
diagnostics.goto_prev()

-- Get diagnostic at cursor
local diag = diagnostics.get_diagnostic_at_cursor()

-- Apply suggested fix
diagnostics.apply_suggested_fix(diag, fix_index)

-- Show diagnostic float
diagnostics.show_float()

-- Set to quickfix/location list
diagnostics.setqflist()
diagnostics.setloclist()
```

## Health Check

Run `:checkhealth mtlog` to verify your installation:

- Neovim version compatibility
- Go installation
- mtlog-analyzer availability
- Current configuration
- Plugin status

## Troubleshooting

### Analyzer not found

Install mtlog-analyzer:
```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
```

Make sure `$GOPATH/bin` is in your PATH.

### No diagnostics appearing

1. Check if the plugin is enabled: `:MtlogStatus`
2. Verify analyzer is working: `mtlog-analyzer your-file.go`
3. Check for errors: `:messages`
4. Clear cache: `:MtlogCache clear`

### Performance issues

Adjust debouncing and rate limiting:
```lua
require('mtlog').setup({
  debounce_ms = 1000,  -- Increase debounce time
  rate_limit = {
    max_files_per_second = 5,  -- Reduce rate limit
  },
})
```

## Integration with LSP

The plugin integrates with Neovim's built-in LSP client to provide code actions. When you have an LSP client attached, mtlog quick fixes will appear in the code actions menu (usually triggered with `<leader>ca` or `:lua vim.lsp.buf.code_action()`).

## Screenshots

<!-- Add screenshots here -->

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This plugin is part of the mtlog project and follows the same license.