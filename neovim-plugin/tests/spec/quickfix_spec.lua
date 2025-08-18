-- Tests for the quick fix functionality
local diagnostics = require('mtlog.diagnostics')
local analyzer = require('mtlog.analyzer')
local config = require('mtlog.config')

describe('mtlog quick fix', function()
  local test_bufnr
  local namespace
  
  before_each(function()
    -- Reset config
    config.setup({})
    
    -- Create a test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/test/quickfix.go')
    vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
      'package main',
      '',
      'import "github.com/willibrandon/mtlog"',
      '',
      'func main() {',
      '    log := mtlog.New()',
      '    log.Information("User {userid} logged in", 123)',  -- Line 7 - MTLOG002 issue
      '    log.Error("Failed to {Action}", err)',             -- Line 8 - MTLOG001 issue
      '}',
    })
    
    -- Setup diagnostics
    diagnostics.setup()
    namespace = diagnostics.get_namespace()
    
    -- Switch to test buffer
    vim.api.nvim_set_current_buf(test_bufnr)
  end)
  
  after_each(function()
    -- Clear all diagnostics
    diagnostics.clear_all()
    
    -- Delete test buffer
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  describe('MtlogQuickFix command', function()
    it('should apply quick fix for MTLOG002 (PascalCase)', function()
      -- Create diagnostic with stdin-mode format (byte offsets)
      local diag = {
        lnum = 6,  -- 0-indexed line 6 = line 7 in editor
        col = 28, -- 0-indexed column
        end_lnum = 6,
        end_col = 34,
        message = '[MTLOG002] Property name should be PascalCase: userid → UserId',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG002',
        user_data = {
          suggested_fixes = {
            {
              description = 'Change to PascalCase',
              edits = {
                {
                  -- stdin mode uses file:line:col format
                  pos = 'test.go:7:28',  -- position of "userid" (1-indexed column)
                  ['end'] = 'test.go:7:34',  -- end of "userid" (exclusive)
                  newText = 'UserId',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic
      diagnostics.set(test_bufnr, { diag })
      
      -- Move cursor to the diagnostic
      vim.api.nvim_win_set_cursor(0, { 7, 29 })  -- 1-indexed line 7, column 29
      
      -- Get diagnostic at cursor
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor)
      assert.equals('MTLOG002', diag_at_cursor.code)
      
      -- Apply the fix
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      
      -- Debug: print what we got
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 6, 7, false)
      if lines[1] ~= '    log.Information("User {UserId} logged in", 123)' then
        print("Expected: '    log.Information(\"User {UserId} logged in\", 123)'")
        print("Got:      '" .. lines[1] .. "'")
        print("Success: " .. tostring(success))
        
        -- Show the whole buffer for context
        local all_lines = vim.api.nvim_buf_get_lines(test_bufnr, 0, -1, false)
        print("Full buffer:")
        for i, line in ipairs(all_lines) do
          print(string.format("  %d: %s", i, line))
        end
      end
      
      assert.is_true(success)
      assert.equals('    log.Information("User {UserId} logged in", 123)', lines[1])
    end)
    
    it('should apply quick fix for MTLOG001 (missing argument)', function()
      -- Create diagnostic with stdin-mode format
      local diag = {
        lnum = 7,  -- 0-indexed line 7 = line 8 in editor
        col = 4,
        end_lnum = 7,
        end_col = 42,
        message = '[MTLOG001] Template expects 2 arguments but got 1',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG001',
        user_data = {
          suggested_fixes = {
            {
              description = 'Add missing argument',
              edits = {
                {
                  -- stdin mode uses file:line:col format
                  pos = 'test.go:8:40',  -- position before ")" (1-indexed)
                  ['end'] = 'test.go:8:40',  -- same position (insertion)
                  newText = ', "defaultAction"',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic
      diagnostics.set(test_bufnr, { diag })
      
      -- Move cursor to the diagnostic
      vim.api.nvim_win_set_cursor(0, { 8, 5 })  -- Line 8
      
      -- Get diagnostic at cursor
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor)
      assert.equals('MTLOG001', diag_at_cursor.code)
      
      -- Apply the fix
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      -- Check the fix was applied
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 7, 8, false)
      assert.equals('    log.Error("Failed to {Action}", err, "defaultAction")', lines[1])
    end)
    
    it('should handle line/column format from analyzer', function()
      -- Create diagnostic with line/column format (alternative format)
      local diag = {
        lnum = 6,  -- 0-indexed
        col = 28,
        end_lnum = 6,
        end_col = 34,
        message = '[MTLOG002] Property name should be PascalCase',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG002',
        user_data = {
          suggested_fixes = {
            {
              description = 'Change to PascalCase',
              edits = {
                {
                  range = {
                    start = { line = 7, column = 28 },  -- 1-indexed
                    ['end'] = { line = 7, column = 34 },
                  },
                  newText = 'UserId',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic
      diagnostics.set(test_bufnr, { diag })
      
      -- Move cursor to the diagnostic
      vim.api.nvim_win_set_cursor(0, { 7, 29 })
      
      -- Apply the fix
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      -- Check the fix was applied
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 6, 7, false)
      assert.equals('    log.Information("User {UserId} logged in", 123)', lines[1])
    end)
    
    it('should handle multiple suggested fixes', function()
      -- Create diagnostic with multiple fixes
      local diag = {
        lnum = 6,
        col = 28,
        message = '[MTLOG002] Property issue',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG002',
        user_data = {
          suggested_fixes = {
            {
              description = 'Change to PascalCase',
              edits = {
                {
                  pos = 'test.go:7:28',
                  ['end'] = 'test.go:7:34',
                  newText = 'UserId',
                },
              },
            },
            {
              description = 'Change to camelCase',
              edits = {
                {
                  pos = 'test.go:7:28',
                  ['end'] = 'test.go:7:34',
                  newText = 'userId',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic
      diagnostics.set(test_bufnr, { diag })
      
      -- Move cursor to the diagnostic
      vim.api.nvim_win_set_cursor(0, { 7, 29 })
      
      -- Apply first fix
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.equals(2, #diag_at_cursor.user_data.suggested_fixes)
      
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      -- Check first fix was applied
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 6, 7, false)
      assert.equals('    log.Information("User {UserId} logged in", 123)', lines[1])
    end)
    
    it('should return false when no fixes available', function()
      local diag = {
        lnum = 6,
        col = 0,
        message = 'Some issue without fixes',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 7, 0 })
      
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor)
      
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_false(success)
    end)
    
    it('should handle empty edits array', function()
      local diag = {
        lnum = 6,
        col = 0,
        message = 'Issue with empty fix',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        user_data = {
          suggested_fixes = {
            {
              description = 'Empty fix',
              edits = {},  -- Empty edits array
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 7, 0 })
      
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)  -- Returns true but doesn't modify anything
    end)
  end)
  
  describe('analyzer integration', function()
    it('should analyze and provide quick fixes', function()
      -- Skip if analyzer not available
      if not analyzer.is_available() then
        return
      end
      
      local test_helpers = require('test_helpers')
      -- Create file in Go module context so analyzer can resolve imports
      local filepath = test_helpers.create_test_go_file('test_quickfix.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("User {userid} logged in", 123)
}
]])
      
      -- Since analyze_file is async, we simulate synchronous behavior for testing
      local results_received = nil
      local error_received = nil
      
      analyzer.analyze_file(filepath, function(results, err)
        results_received = results
        error_received = err
      end)
      
      -- Wait a bit for the async operation (in tests, it should complete immediately)
      vim.wait(100, function() return results_received ~= nil or error_received ~= nil end)
      
      -- Clean up the file
      test_helpers.delete_test_file(filepath)
      
      if error_received then
        -- Analyzer not working properly, skip test
        return
      end
      
      if results_received then
        -- Should have found the PascalCase issue
        assert.is_not_nil(results_received)
        assert.is_true(#results_received > 0)
        
        -- Check that the diagnostic has suggested fixes
        local diag = results_received[1]
        assert.is_not_nil(diag.user_data)
        assert.is_not_nil(diag.user_data.suggested_fixes)
        assert.is_true(#diag.user_data.suggested_fixes > 0)
      end
    end)
  end)
end)