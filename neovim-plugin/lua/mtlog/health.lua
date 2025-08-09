-- Health check module for mtlog.nvim

local M = {}

local health = vim.health or require('health')
local utils = require('mtlog.utils')
local config = require('mtlog.config')
local analyzer = require('mtlog.analyzer')

-- Start health check report
local function start(msg)
  if health.start then
    health.start(msg)
  else
    health.report_start(msg)
  end
end

-- Report success
local function ok(msg)
  if health.ok then
    health.ok(msg)
  else
    health.report_ok(msg)
  end
end

-- Report warning
local function warn(msg, advice)
  if health.warn then
    health.warn(msg, advice)
  else
    health.report_warn(msg, advice)
  end
end

-- Report error
local function error(msg, advice)
  if health.error then
    health.error(msg, advice)
  else
    health.report_error(msg, advice)
  end
end

-- Report info
local function info(msg)
  if health.info then
    health.info(msg)
  else
    health.report_info(msg)
  end
end

-- Check Neovim version
local function check_neovim_version()
  start('Neovim version')
  
  local required = '0.7.0'
  local has_version, err_msg = utils.check_neovim_version(required)
  
  if has_version then
    ok(string.format('Neovim version: %s (>= %s required)', vim.version(), required))
  else
    error(err_msg or string.format('Neovim %s or higher required', required),
      'Please update Neovim to the latest version')
  end
  
  -- Check for optional features
  if vim.system then
    ok('vim.system API available (better async support)')
  else
    info('vim.system not available, using jobstart fallback')
  end
  
  if vim.diagnostic then
    ok('vim.diagnostic API available')
  else
    error('vim.diagnostic API not available',
      'This plugin requires Neovim 0.6+ with diagnostic support')
  end
end

-- Check Go installation
local function check_go_installation()
  start('Go installation')
  
  local go_version = vim.fn.system('go version 2>/dev/null')
  
  if vim.v.shell_error == 0 then
    local version = go_version:match('go(%d+%.%d+%.?%d*)')
    if version then
      ok(string.format('Go version: %s', version))
      
      -- Check minimum version (1.21+)
      local major, minor = version:match('(%d+)%.(%d+)')
      if major and minor then
        major = tonumber(major)
        minor = tonumber(minor)
        if major < 1 or (major == 1 and minor < 21) then
          warn('Go 1.21+ recommended for best compatibility',
            'Consider upgrading Go to version 1.21 or higher')
        end
      end
    else
      ok('Go is installed')
    end
  else
    error('Go not found in PATH',
      'Install Go from https://golang.org/dl/')
  end
  
  -- Check GOPATH/GOBIN
  local gopath = vim.fn.system('go env GOPATH 2>/dev/null'):gsub('%s+$', '')
  if gopath and gopath ~= '' then
    info(string.format('GOPATH: %s', gopath))
    
    local gobin = vim.fn.system('go env GOBIN 2>/dev/null'):gsub('%s+$', '')
    if gobin and gobin ~= '' then
      info(string.format('GOBIN: %s', gobin))
    else
      info(string.format('GOBIN: %s/bin (default)', gopath))
    end
  end
end

-- Check mtlog-analyzer installation
local function check_analyzer()
  start('mtlog-analyzer')
  
  local analyzer_path = config.get('analyzer_path')
  info(string.format('Configured path: %s', analyzer_path))
  
  -- Check if analyzer is available
  if analyzer.is_available() then
    local version = analyzer.get_version()
    if version then
      ok(string.format('mtlog-analyzer version: %s', version))
    else
      ok('mtlog-analyzer is installed')
    end
    
    -- Test analyzer on a simple case
    local test_file = vim.fn.tempname() .. '.go'
    local test_content = [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Test {Property}", "value")
}
]]
    
    if utils.write_file(test_file, test_content) then
      local result = vim.fn.system(analyzer_path .. ' -json ' .. test_file .. ' 2>/dev/null')
      vim.fn.delete(test_file)
      
      if vim.v.shell_error == 0 then
        ok('mtlog-analyzer test passed')
      else
        warn('mtlog-analyzer test had issues',
          'Check analyzer configuration and flags')
      end
    end
  else
    error('mtlog-analyzer not found',
      table.concat({
        'Install with: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest',
        'Make sure $GOPATH/bin or $GOBIN is in your PATH',
        'Or configure analyzer_path in setup()',
      }, '\n'))
  end
  
  -- Check analyzer flags
  local flags = config.get('analyzer_flags')
  if flags and #flags > 0 then
    info(string.format('Custom flags: %s', table.concat(flags, ' ')))
  end
end

-- Check Go project detection
local function check_go_project()
  start('Go project detection')
  
  local cwd = vim.fn.getcwd()
  info(string.format('Current directory: %s', cwd))
  
  if utils.is_go_project() then
    ok('Go project detected')
    
    -- Check for go.mod
    if vim.fn.filereadable('go.mod') == 1 then
      ok('go.mod found')
      
      -- Check if it uses mtlog
      local go_mod = utils.read_file('go.mod')
      if go_mod and go_mod:match('github.com/willibrandon/mtlog') then
        ok('Project uses mtlog')
      else
        info('Project does not appear to use mtlog')
      end
    else
      info('No go.mod found (using .go file detection)')
    end
    
    -- Count Go files
    local go_files = utils.get_go_files()
    info(string.format('Found %d Go files', #go_files))
  else
    warn('Not in a Go project',
      'Navigate to a Go project directory for automatic activation')
  end
  
  -- Check Git integration
  local git_root = utils.get_git_root()
  if git_root then
    info(string.format('Git root: %s', git_root))
  else
    info('Not in a Git repository')
  end
end

-- Check configuration
local function check_configuration()
  start('Configuration')
  
  local cfg = config.get()
  
  -- Check auto-enable
  if cfg.auto_enable then
    ok('Auto-enable is ON')
  else
    info('Auto-enable is OFF (manual activation required)')
  end
  
  -- Check auto-analyze
  if cfg.auto_analyze then
    ok('Auto-analyze is ON')
  else
    info('Auto-analyze is OFF (manual analysis required)')
  end
  
  -- Check debounce
  info(string.format('Debounce: %dms', cfg.debounce_ms))
  
  -- Check virtual text
  if cfg.virtual_text.enabled then
    ok('Virtual text is enabled')
  else
    info('Virtual text is disabled')
  end
  
  -- Check signs
  if cfg.signs.enabled then
    ok('Sign column is enabled')
  else
    info('Sign column is disabled')
  end
  
  -- Check cache
  if cfg.cache.enabled then
    ok(string.format('Cache is enabled (TTL: %ds)', cfg.cache.ttl_seconds))
    
    -- Check cache stats
    local cache = require('mtlog.cache')
    local stats = cache.stats()
    info(string.format('Cache entries: %d', stats.entries))
  else
    info('Cache is disabled')
  end
  
  -- Check rate limiting
  if cfg.rate_limit.enabled then
    ok(string.format('Rate limiting is enabled (%d files/sec)', 
      cfg.rate_limit.max_files_per_second))
  else
    info('Rate limiting is disabled')
  end
end

-- Check plugin status
local function check_plugin_status()
  start('Plugin status')
  
  local mtlog = require('mtlog')
  
  if mtlog.initialized() then
    ok('Plugin is initialized')
    
    if mtlog.enabled() then
      ok('Plugin is enabled')
      
      -- Check diagnostic counts
      local counts = mtlog.get_counts()
      info(string.format('Diagnostics: %d total (%d errors, %d warnings, %d info, %d hints)',
        counts.total, counts.errors, counts.warnings, counts.info, counts.hints))
    else
      info('Plugin is disabled')
    end
  else
    warn('Plugin is not initialized',
      'Call require("mtlog").setup() in your init.lua')
  end
end

-- Main health check
function M.check()
  start('mtlog.nvim')
  
  check_neovim_version()
  check_go_installation()
  check_analyzer()
  check_go_project()
  check_configuration()
  check_plugin_status()
  
  -- Final summary
  start('Summary')
  
  if analyzer.is_available() and utils.is_go_project() then
    ok('Ready to analyze mtlog usage!')
  elseif not analyzer.is_available() then
    error('Install mtlog-analyzer to use this plugin',
      'Run: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest')
  elseif not utils.is_go_project() then
    info('Navigate to a Go project to start using mtlog.nvim')
  end
end

return M