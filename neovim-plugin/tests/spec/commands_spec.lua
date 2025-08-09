-- Integration tests for all mtlog user commands
describe('mtlog commands', function()
  local mtlog
  local test_utils
  
  before_each(function()
    -- Clear any existing modules
    package.loaded['mtlog'] = nil
    package.loaded['mtlog.config'] = nil
    package.loaded['mtlog.analyzer'] = nil
    package.loaded['mtlog.diagnostics'] = nil
    package.loaded['mtlog.cache'] = nil
    package.loaded['mtlog.utils'] = nil
    package.loaded['test_utils'] = nil
    
    -- Set up test environment
    vim.g.mtlog_analyzer_path = vim.fn.exepath('echo')  -- Use echo as mock analyzer
    
    -- Load the plugin commands (normally loaded automatically by Neovim)
    vim.cmd('runtime plugin/mtlog.lua')
    
    -- Load the plugin
    mtlog = require('mtlog')
    
    -- Load test utils - it should be in the same directory as this file
    local current_file = debug.getinfo(1, "S").source:sub(2)
    local test_dir = vim.fn.fnamemodify(current_file, ':h:h')  -- Go up to tests/ dir
    package.path = test_dir .. '/?.lua;' .. package.path
    test_utils = require('test_utils')
    
    -- Setup with test configuration
    mtlog.setup({
      auto_enable = false,  -- Don't auto-enable
      auto_analyze = false, -- Don't auto-analyze
      show_errors = false,  -- Don't show error notifications in tests
      cache = {
        enabled = false,  -- Disable cache for tests
      },
    })
  end)
  
  after_each(function()
    -- Clean up any buffers
    for _, buf in ipairs(vim.api.nvim_list_bufs()) do
      if vim.api.nvim_buf_is_valid(buf) then
        vim.api.nvim_buf_delete(buf, { force = true })
      end
    end
  end)
  
  describe(':MtlogAnalyze', function()
    it('should analyze current buffer', function()
      -- Create a test Go file
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      vim.api.nvim_buf_set_name(bufnr, 'test.go')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, {
        'package main',
        'import "github.com/willibrandon/mtlog"',
        'func main() {',
        '  log := mtlog.New()',
        '  log.Info("Test {Property}")',
        '}',
      })
      
      -- Run the command
      vim.cmd('MtlogAnalyze')
      
      -- Wait for async operation
      vim.wait(100)
      
      -- Command should execute without errors
      assert.is_true(true)  -- If we get here, command didn't error
    end)
    
    it('should analyze specified file', function()
      -- Create a test file
      local test_file = vim.fn.tempname() .. '.go'
      local file = io.open(test_file, 'w')
      file:write([[
package main
func main() {}
]])
      file:close()
      
      -- Run the command with file argument
      vim.cmd('MtlogAnalyze ' .. test_file)
      
      -- Wait for async operation
      vim.wait(100)
      
      -- Clean up
      os.remove(test_file)
      
      -- Command should execute without errors
      assert.is_true(true)
    end)
    
    it('should handle non-Go files gracefully', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      vim.api.nvim_buf_set_name(bufnr, 'test.txt')
      
      -- Should return early for non-Go files
      vim.cmd('MtlogAnalyze')
      
      -- No error should occur
      assert.is_true(true)
    end)
  end)
  
  describe(':MtlogAnalyzeWorkspace', function()
    it('should analyze workspace', function()
      -- Mock get_go_files to return empty list
      local utils = require('mtlog.utils')
      utils.get_go_files = function()
        return {}
      end
      
      -- Run the command
      vim.cmd('MtlogAnalyzeWorkspace')
      
      -- Should handle empty workspace gracefully
      assert.is_true(true)
    end)
    
    it('should process multiple Go files', function()
      local utils = require('mtlog.utils')
      local analyzer = require('mtlog.analyzer')
      
      -- Create temp files
      local test_files = {
        vim.fn.tempname() .. '.go',
        vim.fn.tempname() .. '.go',
      }
      
      for _, file in ipairs(test_files) do
        local f = io.open(file, 'w')
        f:write('package main\nfunc main() {}')
        f:close()
      end
      
      -- Mock get_go_files
      utils.get_go_files = function()
        return test_files
      end
      
      -- Mock analyze_file to track calls
      local analyzed_files = {}
      analyzer.analyze_file = function(filepath, callback)
        table.insert(analyzed_files, filepath)
        callback({}, nil)  -- Return empty results
      end
      
      -- Run the command
      vim.cmd('MtlogAnalyzeWorkspace')
      
      -- Wait for async operations
      vim.wait(100)
      
      -- Check that files were analyzed
      assert.equals(#test_files, #analyzed_files)
      
      -- Clean up
      for _, file in ipairs(test_files) do
        os.remove(file)
      end
    end)
  end)
  
  describe(':MtlogClear', function()
    it('should clear diagnostics for current buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      
      -- Set some mock diagnostics
      local diagnostics = require('mtlog.diagnostics')
      diagnostics.set(bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Test diagnostic',
          severity = vim.diagnostic.severity.ERROR,
        }
      })
      
      -- Clear diagnostics
      vim.cmd('MtlogClear')
      
      -- Check that diagnostics were cleared
      local diags = vim.diagnostic.get(bufnr, { namespace = diagnostics.ns })
      assert.equals(0, #diags)
    end)
    
    it('should clear all diagnostics with bang', function()
      local diagnostics = require('mtlog.diagnostics')
      
      -- Create multiple buffers with diagnostics
      for i = 1, 3 do
        local bufnr = vim.api.nvim_create_buf(false, true)
        diagnostics.set(bufnr, {
          {
            lnum = 0,
            col = 0,
            message = 'Test diagnostic ' .. i,
            severity = vim.diagnostic.severity.ERROR,
          }
        })
      end
      
      -- Clear all diagnostics
      vim.cmd('MtlogClear!')
      
      -- Check that all diagnostics were cleared
      for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
        local diags = vim.diagnostic.get(bufnr, { namespace = diagnostics.ns })
        assert.equals(0, #diags)
      end
    end)
  end)
  
  describe(':MtlogEnable/:MtlogDisable/:MtlogToggle', function()
    it('should enable the analyzer', function()
      -- Ensure disabled first
      mtlog.disable()
      assert.is_false(mtlog.enabled())
      
      -- Enable
      vim.cmd('MtlogEnable')
      assert.is_true(mtlog.enabled())
    end)
    
    it('should disable the analyzer', function()
      -- Ensure enabled first
      mtlog.enable()
      assert.is_true(mtlog.enabled())
      
      -- Disable
      vim.cmd('MtlogDisable')
      assert.is_false(mtlog.enabled())
    end)
    
    it('should toggle the analyzer state', function()
      -- Start disabled
      mtlog.disable()
      assert.is_false(mtlog.enabled())
      
      -- Toggle to enabled
      vim.cmd('MtlogToggle')
      assert.is_true(mtlog.enabled())
      
      -- Toggle back to disabled
      vim.cmd('MtlogToggle')
      assert.is_false(mtlog.enabled())
    end)
    
    it('should handle multiple enable calls gracefully', function()
      vim.cmd('MtlogEnable')
      vim.cmd('MtlogEnable')  -- Should be idempotent
      assert.is_true(mtlog.enabled())
    end)
    
    it('should handle multiple disable calls gracefully', function()
      vim.cmd('MtlogDisable')
      vim.cmd('MtlogDisable')  -- Should be idempotent
      assert.is_false(mtlog.enabled())
    end)
  end)
  
  describe(':MtlogStatus', function()
    it('should display status window', function()
      -- Run the command
      vim.cmd('MtlogStatus')
      
      -- Check that a floating window was created
      local wins = vim.api.nvim_list_wins()
      local found_float = false
      for _, win in ipairs(wins) do
        local config = vim.api.nvim_win_get_config(win)
        if config.relative ~= '' then
          found_float = true
          
          -- Check buffer content
          local bufnr = vim.api.nvim_win_get_buf(win)
          local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
          
          -- Should contain status information
          local content = table.concat(lines, '\n')
          assert.is_true(content:match('mtlog%.nvim Status') ~= nil)
          
          -- Close the window
          vim.api.nvim_win_close(win, true)
          break
        end
      end
      
      assert.is_true(found_float, 'Status window should be created')
    end)
    
    it('should show plugin state correctly', function()
      -- Enable plugin
      mtlog.enable()
      
      -- Run status command
      vim.cmd('MtlogStatus')
      
      -- Find the floating window
      local wins = vim.api.nvim_list_wins()
      for _, win in ipairs(wins) do
        local config = vim.api.nvim_win_get_config(win)
        if config.relative ~= '' then
          local bufnr = vim.api.nvim_win_get_buf(win)
          local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
          local content = table.concat(lines, '\n')
          
          -- Should show enabled state
          assert.is_true(content:match('✓ Plugin enabled') ~= nil)
          
          -- Close the window
          vim.api.nvim_win_close(win, true)
          break
        end
      end
    end)
  end)
  
  describe(':MtlogCache', function()
    it('should clear cache', function()
      local cache = require('mtlog.cache')
      local config = require('mtlog.config')
      
      -- Temporarily enable cache for this test
      mtlog.setup({
        auto_enable = false,
        auto_analyze = false,
        show_errors = false,
        cache = { enabled = true },
      })
      
      -- Create a temp file and add to cache
      local temp_file = vim.fn.tempname() .. '.go'
      local f = io.open(temp_file, 'w')
      f:write('package main\n')
      f:close()
      
      cache.set(temp_file, { test = 'data' })
      
      -- Clear cache
      vim.cmd('MtlogCache clear')
      
      -- Check that cache was cleared
      assert.is_nil(cache.get(temp_file))
      
      -- Clean up
      os.remove(temp_file)
    end)
    
    it('should show cache stats', function()
      -- Note: before_each will have run and set cache.enabled = false
      -- We need to reconfigure after that
      
      local cache = require('mtlog.cache')
      local config = require('mtlog.config')
      
      -- Force enable cache by directly modifying config state
      -- This is a hack but necessary because of test isolation issues
      local config_internal = config.get()
      config_internal.cache = { enabled = true, ttl_seconds = 300 }
      
      -- Add some test data using actual file paths that would work with mtime
      local temp_file1 = vim.fn.tempname() .. '.go'
      local temp_file2 = vim.fn.tempname() .. '.go'
      
      -- Create temp files with content
      local f1 = io.open(temp_file1, 'w')
      f1:write('package main\n')
      f1:close()
      
      local f2 = io.open(temp_file2, 'w') 
      f2:write('package test\n')
      f2:close()
      
      -- Debug: verify cache is enabled
      local cache_config = config.get('cache')
      local cache_enabled = config.get('cache.enabled')
      if not cache_enabled then
        error(string.format('Cache should be enabled but is not: %s. Full cache config: %s', 
          tostring(cache_enabled), vim.inspect(cache_config)))
      end
      
      -- Debug: check utils.get_mtime works
      local utils = require('mtlog.utils')
      local mtime1 = utils.get_mtime(temp_file1)
      local mtime2 = utils.get_mtime(temp_file2)
      if not mtime1 or not mtime2 then
        error('mtime failed: ' .. tostring(mtime1) .. ', ' .. tostring(mtime2))
      end
      
      cache.set(temp_file1, { test = 'data1' })
      cache.set(temp_file2, { test = 'data2' })
      
      -- Verify cache entries were set
      local cached1 = cache.get(temp_file1)
      local cached2 = cache.get(temp_file2)
      if not cached1 then
        error('Failed to set cache for temp_file1: ' .. temp_file1)
      end
      if not cached2 then
        error('Failed to set cache for temp_file2: ' .. temp_file2)
      end
      
      -- Get stats (command will show notification)
      vim.cmd('MtlogCache stats')
      
      -- Check cache stats
      local stats = cache.stats()
      if stats.entries ~= 2 then
        error(string.format('Expected 2 entries but got %d. Full stats: %s', stats.entries, vim.inspect(stats)))
      end
      
      -- Clean up
      os.remove(temp_file1)
      os.remove(temp_file2)
      
      -- Restore original settings
      mtlog.setup({
        auto_enable = false,
        auto_analyze = false,
        show_errors = false,
        cache = { enabled = false },
      })
    end)
    
    it('should handle invalid cache command', function()
      -- Should show warning for invalid subcommand
      vim.cmd('MtlogCache invalid')
      
      -- No error should be thrown
      assert.is_true(true)
    end)
  end)
  
  describe(':MtlogQuickFix', function()
    it('should handle no diagnostic at cursor', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      vim.api.nvim_win_set_cursor(0, {1, 0})
      
      -- Should show warning notification
      vim.cmd('MtlogQuickFix')
      
      -- No error should occur
      assert.is_true(true)
    end)
    
    it('should handle diagnostic without fixes', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, {'test line'})
      
      -- Set diagnostic without fixes
      local diagnostics = require('mtlog.diagnostics')
      diagnostics.set(bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Test diagnostic',
          severity = vim.diagnostic.severity.ERROR,
        }
      })
      
      -- Position cursor on diagnostic
      vim.api.nvim_win_set_cursor(0, {1, 0})
      
      -- Should show info notification about no fixes
      vim.cmd('MtlogQuickFix')
      
      -- No error should occur
      assert.is_true(true)
    end)
    
    it('should apply single fix directly', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_set_current_buf(bufnr)
      vim.api.nvim_buf_set_name(bufnr, 'test.go')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, {
        'log.Info("Test {property}")'
      })
      
      -- Set diagnostic with fix
      local diagnostics = require('mtlog.diagnostics')
      diagnostics.set(bufnr, {
        {
          lnum = 0,
          col = 17,
          end_lnum = 0,
          end_col = 25,
          message = '[MTLOG004] Non-PascalCase property',
          severity = vim.diagnostic.severity.WARN,
          user_data = {
            suggested_fixes = {
              {
                description = 'Change to PascalCase',
                textEdits = {
                  {
                    pos = 'test.go:1:18',
                    ['end'] = 'test.go:1:26',
                    newText = 'Property',
                  }
                }
              }
            }
          }
        }
      })
      
      -- Position cursor on diagnostic
      vim.api.nvim_win_set_cursor(0, {1, 17})
      
      -- Mock the apply function to track if it was called
      local apply_called = false
      local original_apply = diagnostics.apply_suggested_fix
      diagnostics.apply_suggested_fix = function(diag, idx)
        apply_called = true
        -- Mock successful application
        return true
      end
      
      -- Mock vim.cmd('write') to avoid file operations
      local original_cmd = vim.cmd
      vim.cmd = function(cmd)
        if cmd == 'write' then
          -- Do nothing
        else
          return original_cmd(cmd)
        end
      end
      
      -- Apply the fix
      vim.cmd('MtlogQuickFix')
      
      -- Wait for deferred function
      vim.wait(600)
      
      -- Check that apply was called
      assert.is_true(apply_called)
      
      -- Restore original functions
      diagnostics.apply_suggested_fix = original_apply
      vim.cmd = original_cmd
    end)
  end)
  
  describe('command completions', function()
    it('should provide file completion for MtlogAnalyze', function()
      local cmd_info = vim.api.nvim_get_commands({})['MtlogAnalyze']
      assert.equals('file', cmd_info.complete)
    end)
    
    it('should provide custom completion for MtlogCache', function()
      local completions = vim.fn.getcompletion('MtlogCache ', 'cmdline')
      assert.is_true(vim.tbl_contains(completions, 'clear'))
      assert.is_true(vim.tbl_contains(completions, 'stats'))
    end)
  end)
  
  describe('command descriptions', function()
    it('should have descriptions for all commands', function()
      local commands = {
        'MtlogAnalyze',
        'MtlogAnalyzeWorkspace',
        'MtlogClear',
        'MtlogEnable',
        'MtlogDisable',
        'MtlogToggle',
        'MtlogStatus',
        'MtlogCache',
        'MtlogQuickFix',
      }
      
      local cmd_info = vim.api.nvim_get_commands({})
      
      for _, cmd in ipairs(commands) do
        assert.is_not_nil(cmd_info[cmd], cmd .. ' should be defined')
        assert.is_not_nil(cmd_info[cmd].definition, cmd .. ' should have a definition')
      end
    end)
  end)
end)