-- Tests for With() method diagnostics (MTLOG009-MTLOG013)
local diagnostics = require('mtlog.diagnostics')
local analyzer = require('mtlog.analyzer')
local config = require('mtlog.config')

describe('mtlog With() diagnostics', function()
  local test_bufnr
  local namespace
  
  before_each(function()
    -- Reset config with new severity levels
    config.setup({
      severity_levels = {
        MTLOG009 = vim.diagnostic.severity.ERROR,
        MTLOG010 = vim.diagnostic.severity.WARN,
        MTLOG011 = vim.diagnostic.severity.INFO,
        MTLOG012 = vim.diagnostic.severity.WARN,
        MTLOG013 = vim.diagnostic.severity.ERROR,
      }
    })
    
    -- Create a test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/test/with_test.go')
    
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
  
  describe('MTLOG009 - Odd argument count', function()
    it('should detect and fix odd number of arguments', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'import "github.com/willibrandon/mtlog"',
        '',
        'func main() {',
        '    log := mtlog.New()',
        '    log.With("UserId", 123, "Action")',  -- Line 7 - Missing value
        '}',
      })
      
      -- Create diagnostic with suggested fixes
      local diag = {
        lnum = 6,  -- 0-indexed
        col = 8,
        end_lnum = 6,
        end_col = 38,
        message = '[MTLOG009] With() requires an even number of arguments (key-value pairs), got 3',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG009',
        user_data = {
          suggested_fixes = {
            {
              description = 'Add empty string value for the last key',
              edits = {
                {
                  pos = 'test.go:7:37',  -- After "Action"
                  ['end'] = 'test.go:7:37',
                  newText = ', ""',
                },
              },
            },
            {
              description = 'Remove the dangling key',
              edits = {
                {
                  pos = 'test.go:7:28',  -- From comma before "Action"
                  ['end'] = 'test.go:7:37',  -- To end of "Action"
                  newText = '',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic
      diagnostics.set(test_bufnr, { diag })
      
      -- Move cursor to the diagnostic
      vim.api.nvim_win_set_cursor(0, { 7, 10 })
      
      -- Get diagnostic at cursor
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor)
      assert.equals('MTLOG009', diag_at_cursor.code)
      
      -- Apply the first fix (add empty string)
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      -- Check the result
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 6, 7, false)
      assert.equals('    log.With("UserId", 123, "Action", "")', lines[1])
    end)
    
    it('should handle removal of dangling key', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("UserId", 123, "Action")',
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 8,
        end_lnum = 3,
        end_col = 38,
        message = '[MTLOG009] With() requires an even number of arguments',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG009',
        user_data = {
          suggested_fixes = {
            {
              description = 'Remove the dangling key',
              edits = {
                {
                  pos = 'test.go:4:28',  -- From comma
                  ['end'] = 'test.go:4:37',  -- To end of "Action"
                  newText = '',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 30 })
      
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 3, 4, false)
      -- Check that "Action" is removed and the line matches the expected pattern
      assert.is_nil(string.find(lines[1], '"Action"'), 'Dangling key "Action" was not removed')
      assert.is_truthy(string.match(lines[1], '^%s*log%.With%("UserId",%s*123,?%)$'), 'Line does not match expected log.With pattern')
    end)
  end)
  
  describe('MTLOG010 - Non-string key', function()
    it('should detect non-string keys', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With(123, "value")',  -- Non-string key
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 13,
        end_lnum = 3,
        end_col = 16,
        message = '[MTLOG010] With() key must be a string, got int',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG010',
        user_data = {
          suggested_fixes = {
            {
              description = 'Convert to string',
              edits = {
                {
                  pos = 'test.go:4:14',
                  ['end'] = 'test.go:4:17',
                  newText = '"123"',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 15 })  -- Move cursor to diagnostic position
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor(test_bufnr)
      assert.is_not_nil(diag_at_cursor, "No diagnostic found at cursor")
      assert.equals('MTLOG010', diag_at_cursor.code)
    end)
  end)
  
  describe('MTLOG011 - Cross-call duplicate', function()
    it('should detect duplicates across With() calls', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("UserId", 1).With("UserId", 2)',  -- Duplicate UserId
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 30,
        end_lnum = 3,
        end_col = 38,
        message = '[MTLOG011] Property "UserId" already set in previous With() call',
        severity = vim.diagnostic.severity.INFO,
        source = 'mtlog-analyzer',
        code = 'MTLOG011',
      }
      
      diagnostics.set(test_bufnr, { diag })
      local counts = diagnostics.get_counts(test_bufnr)
      assert.equals(1, counts.info)
    end)
  end)
  
  describe('MTLOG012 - Reserved property', function()
    it('should warn about reserved property names', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("Timestamp", time.Now())',  -- Reserved name
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 13,
        end_lnum = 3,
        end_col = 24,
        message = '[MTLOG012] "Timestamp" is a reserved property name that may cause confusion',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG012',
        user_data = {
          suggested_fixes = {
            {
              description = 'Use RequestTimestamp instead',
              edits = {
                {
                  pos = 'test.go:4:14',
                  ['end'] = 'test.go:4:25',
                  newText = '"RequestTimestamp"',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 15 })
      
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.equals('MTLOG012', diag_at_cursor.code)
      
      -- Apply the fix
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 3, 4, false)
      assert.equals('    log.With("RequestTimestamp", time.Now())', lines[1])
    end)
  end)
  
  describe('MTLOG013 - Empty key', function()
    it('should detect empty string keys', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("", "value")',  -- Empty key
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 13,
        end_lnum = 3,
        end_col = 15,
        message = '[MTLOG013] With() key cannot be an empty string',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG013',
        user_data = {
          suggested_fixes = {
            {
              description = 'Remove this key-value pair',
              edits = {
                {
                  pos = 'test.go:4:13',  -- Start of empty string
                  ['end'] = 'test.go:4:24',  -- End of "value"
                  newText = '',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 14 })  -- Move cursor to empty string position
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor(test_bufnr)
      assert.is_not_nil(diag_at_cursor, "No diagnostic found at cursor")
      assert.equals('MTLOG013', diag_at_cursor.code)
      assert.equals(vim.diagnostic.severity.ERROR, diag_at_cursor.severity)
    end)
  end)
  
  describe('Quick fix position parsing', function()
    it('should correctly parse file:line:column format', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("userid", 123)',  -- Line 4, needs PascalCase
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 13,
        end_lnum = 3,
        end_col = 21,
        message = '[MTLOG004] Property should be PascalCase',
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = 'MTLOG004',
        user_data = {
          suggested_fixes = {
            {
              description = 'Change to PascalCase',
              edits = {
                {
                  -- New format used by With() diagnostics
                  pos = 'test.go:4:15',  -- Position after opening quote (1-indexed)
                  ['end'] = 'test.go:4:21',  -- End position before closing quote (exclusive)
                  newText = 'UserId',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 15 })
      
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor)
      
      -- Apply the fix
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success)
      
      -- Verify the fix was applied correctly
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 3, 4, false)
      -- The property name inside quotes should be changed to UserId
      assert.is_true(lines[1]:match('"UserId"') ~= nil, "Expected UserId in: " .. lines[1])
    end)
    
    it('should handle insertion (same start and end position)', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("Key")',  -- Missing value
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 8,
        end_lnum = 3,
        end_col = 22,
        message = '[MTLOG009] Odd number of arguments',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG009',
        user_data = {
          suggested_fixes = {
            {
              description = 'Add value',
              edits = {
                {
                  pos = 'test.go:4:19',  -- After "Key"
                  ['end'] = 'test.go:4:19',  -- Same position for insertion
                  newText = ', "value"',
                },
              },
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      vim.api.nvim_win_set_cursor(0, { 4, 10 })  -- Move cursor to diagnostic
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor, "No diagnostic found at cursor")
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 1)
      assert.is_true(success, "Failed to apply fix")
      
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 3, 4, false)
      assert.equals('    log.With("Key", "value")', lines[1])
    end)
  end)
  
  describe('Multiple fixes in one diagnostic', function()
    it('should offer multiple fix options', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'package main',
        '',
        'func main() {',
        '    log.With("UserId", 123, "Action")',
        '}',
      })
      
      local diag = {
        lnum = 3,
        col = 8,
        end_lnum = 3,
        end_col = 38,
        message = '[MTLOG009] Odd arguments',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG009',
        user_data = {
          suggested_fixes = {
            {
              description = 'Add empty value',
              edits = {{
                pos = 'test.go:4:37',
                ['end'] = 'test.go:4:37',
                newText = ', ""',
              }},
            },
            {
              description = 'Remove dangling key',
              edits = {{
                pos = 'test.go:4:28',
                ['end'] = 'test.go:4:37',
                newText = '',
              }},
            },
          },
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      
      -- Apply second fix (remove)
      vim.api.nvim_win_set_cursor(0, { 4, 10 })  -- Move cursor to diagnostic
      local diag_at_cursor = diagnostics.get_diagnostic_at_cursor()
      assert.is_not_nil(diag_at_cursor, "No diagnostic found at cursor")
      local success = diagnostics.apply_suggested_fix(diag_at_cursor, 2)
      assert.is_true(success, "Failed to apply second fix")
      
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 3, 4, false)
      -- The fix should remove the dangling key
      local expected1 = '    log.With("UserId", 123)'
      local expected2 = '    log.With("UserId", 123,)'
      assert.is_true(lines[1] == expected1 or lines[1] == expected2)
    end)
  end)
  
  describe('Help system integration', function()
    it('should provide explanations for new diagnostics', function()
      local help = require('mtlog.help')
      
      -- Check that all new diagnostics have explanations
      assert.is_not_nil(help.diagnostic_explanations.MTLOG009)
      assert.is_not_nil(help.diagnostic_explanations.MTLOG010)
      assert.is_not_nil(help.diagnostic_explanations.MTLOG011)
      assert.is_not_nil(help.diagnostic_explanations.MTLOG012)
      assert.is_not_nil(help.diagnostic_explanations.MTLOG013)
      
      -- Verify explanations have required fields
      for code, explanation in pairs({
        MTLOG009 = help.diagnostic_explanations.MTLOG009,
        MTLOG010 = help.diagnostic_explanations.MTLOG010,
        MTLOG011 = help.diagnostic_explanations.MTLOG011,
        MTLOG012 = help.diagnostic_explanations.MTLOG012,
        MTLOG013 = help.diagnostic_explanations.MTLOG013,
      }) do
        assert.is_not_nil(explanation.name, code .. " missing name")
        assert.is_not_nil(explanation.description, code .. " missing description")
        assert.is_not_nil(explanation.example, code .. " missing example")
        assert.is_not_nil(explanation.fix, code .. " missing fix")
      end
    end)
  end)
end)