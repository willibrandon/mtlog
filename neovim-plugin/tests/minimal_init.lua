-- Minimal init.lua for running tests

-- Add current directory to runtime path
vim.opt.rtp:append('.')

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

-- Set up package path for the plugin
package.path = package.path .. ';./lua/?.lua;./lua/?/init.lua'

-- Add tests directory to path for test utilities
package.path = package.path .. ';./tests/?.lua'

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