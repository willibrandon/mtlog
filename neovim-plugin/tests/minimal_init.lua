-- Minimal init.lua for running tests

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

-- Configure for testing
vim.g.mtlog_test_mode = true
-- Use environment variable for analyzer path, falling back to PATH lookup
vim.g.mtlog_analyzer_path = vim.env.MTLOG_ANALYZER_PATH or vim.fn.exepath('mtlog-analyzer') or 'mtlog-analyzer'

-- Disable any auto commands during testing
vim.opt.swapfile = false
vim.opt.backup = false
vim.opt.writebackup = false

-- Set a fast updatetime for testing diagnostic display
vim.opt.updatetime = 100