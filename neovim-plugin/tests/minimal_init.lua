-- Minimal init.lua for running tests
-- REQUIRES: mtlog-analyzer binary installed and accessible
-- REQUIRES: Go environment with mtlog dependencies

-- Get the directory containing this file (tests directory)
local test_dir = debug.getinfo(1, "S").source:sub(2):match("(.*/)") or "./"
-- Get the plugin root directory (parent of tests)
local plugin_dir = test_dir:gsub("/tests/$", "") or "."

-- Add plugin directory to runtime path (absolute path)
vim.opt.rtp:append(plugin_dir)

-- Bootstrap Plenary if not available
local plenary_path = vim.fn.stdpath('data') .. '/site/pack/test/start/plenary.nvim'
if vim.fn.isdirectory(plenary_path) == 0 then
  print('Installing plenary.nvim for testing...')
  vim.fn.system({
    'git', 'clone', '--depth=1',
    'https://github.com/nvim-lua/plenary.nvim',
    plenary_path
  })
end
vim.opt.rtp:append(plenary_path)

-- Set up package path for the plugin (use absolute paths)
package.path = package.path .. ';' .. plugin_dir .. '/lua/?.lua;' .. plugin_dir .. '/lua/?/init.lua'

-- Add tests directory to path for test utilities
package.path = package.path .. ';' .. test_dir .. '/?.lua'

-- CRITICAL: Verify mtlog-analyzer is available
local analyzer_path = vim.env.MTLOG_ANALYZER_PATH or vim.fn.exepath('mtlog-analyzer')
if not analyzer_path or analyzer_path == '' then
  error("FATAL: mtlog-analyzer is REQUIRED but not found. Install it or set MTLOG_ANALYZER_PATH")
end

-- Verify analyzer is executable (just check if it runs, don't check version flag)
local test_result = vim.fn.system(analyzer_path .. ' -json /dev/null 2>&1')
-- The analyzer should run even with /dev/null as input
if vim.v.shell_error > 1 then -- Exit code 1 is ok (no Go files), >1 means real error
  -- Fallback: just check if help text works
  local help_result = vim.fn.system(analyzer_path .. ' 2>&1')
  if not help_result:match("mtlog") then
    error("FATAL: mtlog-analyzer exists but is not executable")
  end
end

-- Verify Go environment
local go_version = vim.fn.system('go version')
if vim.v.shell_error ~= 0 then
  error("FATAL: Go is REQUIRED but not found in PATH")
end

-- Configure for testing
vim.g.mtlog_test_mode = true
vim.g.mtlog_analyzer_path = analyzer_path

-- Set test project directory
vim.env.MTLOG_TEST_PROJECT_DIR = vim.env.MTLOG_TEST_PROJECT_DIR or '/tmp/mtlog-test-project-' .. os.time()

-- Disable any auto commands during testing
vim.opt.swapfile = false
vim.opt.backup = false
vim.opt.writebackup = false

-- Set a fast updatetime for testing diagnostic display
vim.opt.updatetime = 100

-- Initialize test helpers and set up Go project
local test_helpers = require('test_helpers')

-- Set up the Go project before running any tests
local ok, err = pcall(function()
  test_helpers.verify_go_environment()
  test_helpers.setup_go_project()
end)

if not ok then
  error("FATAL: Failed to set up test environment: " .. tostring(err))
end

-- Initialize mtlog plugin for health checks
pcall(function()
  require('mtlog').setup({
    auto_enable = false,
    auto_analyze = false,
  })
end)

-- Only print in verbose mode or when debugging
if vim.env.MTLOG_TEST_DEBUG then
  print("Test environment ready. Analyzer: " .. analyzer_path)
  print("Test project: " .. vim.env.MTLOG_TEST_PROJECT_DIR)
end