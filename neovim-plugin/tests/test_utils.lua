-- Test utilities for mtlog.nvim tests
local M = {}

-- Create a mock analyzer that returns predefined results
---@param results table The results to return
---@return function Mock analyzer function
function M.mock_analyzer(results)
  return function(filepath, callback)
    vim.defer_fn(function()
      callback(results or {}, nil)
    end, 10)
  end
end

-- Create a test buffer with Go content
---@param content string[] Lines of content
---@return number Buffer number
function M.create_go_buffer(content)
  local bufnr = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_name(bufnr, 'test.go')
  vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, content or {
    'package main',
    'import "github.com/willibrandon/mtlog"',
    'func main() {}',
  })
  vim.bo[bufnr].filetype = 'go'
  return bufnr
end

-- Wait for async operations with timeout
---@param condition function Condition to check
---@param timeout number? Timeout in milliseconds (default 1000)
---@return boolean Success
function M.wait_for(condition, timeout)
  timeout = timeout or 1000
  local start = vim.loop.now()
  
  while vim.loop.now() - start < timeout do
    if condition() then
      return true
    end
    vim.wait(10)
  end
  
  return false
end

-- Get all diagnostics for a buffer
---@param bufnr number Buffer number
---@return table Diagnostics
function M.get_diagnostics(bufnr)
  local diagnostics = require('mtlog.diagnostics')
  return vim.diagnostic.get(bufnr, { namespace = diagnostics.ns })
end

-- Create a temporary Go file
---@param content string File content
---@return string filepath Path to the created file
function M.create_temp_go_file(content)
  local filepath = vim.fn.tempname() .. '.go'
  local file = io.open(filepath, 'w')
  file:write(content or 'package main\nfunc main() {}')
  file:close()
  return filepath
end

-- Clean up test buffers
function M.cleanup_buffers()
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_valid(buf) then
      pcall(vim.api.nvim_buf_delete, buf, { force = true })
    end
  end
end

-- Mock vim.notify to capture notifications
---@return table Captured notifications
function M.mock_notify()
  local notifications = {}
  local original_notify = vim.notify
  
  vim.notify = function(msg, level, opts)
    table.insert(notifications, {
      message = msg,
      level = level,
      opts = opts,
    })
  end
  
  return {
    notifications = notifications,
    restore = function()
      vim.notify = original_notify
    end,
  }
end

-- Create sample diagnostic data
---@param code string? Diagnostic code (e.g., "MTLOG001")
---@param line number? Line number (0-indexed)
---@param col number? Column number (0-indexed)
---@return table Diagnostic
function M.create_diagnostic(code, line, col)
  return {
    lnum = line or 0,
    col = col or 0,
    end_lnum = line or 0,
    end_col = (col or 0) + 10,
    message = string.format('[%s] Test diagnostic', code or 'MTLOG001'),
    severity = vim.diagnostic.severity.ERROR,
    source = 'mtlog-analyzer',
    code = code or 'MTLOG001',
  }
end

-- Create diagnostic with suggested fix
---@param code string? Diagnostic code
---@param fix table? Suggested fix data
---@return table Diagnostic with fix
function M.create_diagnostic_with_fix(code, fix)
  local diag = M.create_diagnostic(code)
  diag.user_data = {
    suggested_fixes = {
      fix or {
        description = 'Test fix',
        textEdits = {
          {
            pos = 'test.go:1:1',
            ['end'] = 'test.go:1:5',
            newText = 'fixed',
          }
        }
      }
    }
  }
  return diag
end

-- Assert that a buffer contains specific text
---@param bufnr number Buffer number
---@param expected string Expected text (can be partial)
function M.assert_buffer_contains(bufnr, expected)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  local content = table.concat(lines, '\n')
  assert.is_true(
    content:find(expected, 1, true) ~= nil,
    string.format('Buffer should contain "%s" but got:\n%s', expected, content)
  )
end

-- Assert that a buffer matches exact content
---@param bufnr number Buffer number
---@param expected string[] Expected lines
function M.assert_buffer_lines(bufnr, expected)
  local actual = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  assert.are.same(expected, actual)
end

-- Get the current floating window if any
---@return number? Window ID or nil
function M.get_floating_window()
  for _, win in ipairs(vim.api.nvim_list_wins()) do
    local config = vim.api.nvim_win_get_config(win)
    if config.relative ~= '' then
      return win
    end
  end
  return nil
end

-- Run a command and capture any errors
---@param cmd string Command to run
---@return boolean success, string? error
function M.run_command_safe(cmd)
  local ok, err = pcall(vim.cmd, cmd)
  if not ok then
    return false, err
  end
  return true, nil
end

return M