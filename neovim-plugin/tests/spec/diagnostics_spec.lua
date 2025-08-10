-- Tests for the diagnostics module
local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

describe('mtlog.diagnostics', function()
  local test_bufnr
  local namespace
  
  before_each(function()
    -- Reset config
    config.setup({})
    
    -- Create a test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/test/file.go')
    vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
      'package main',
      '',
      'import "github.com/willibrandon/mtlog"',
      '',
      'func main() {',
      '    log := mtlog.New()',
      '    log.Information("User {UserId} logged in", 123)',
      '    log.Error("Failed to {Action}", "process", err)',
      '}',
    })
    
    -- Setup diagnostics
    diagnostics.setup()
    namespace = diagnostics.get_namespace()
  end)
  
  after_each(function()
    -- Clear all diagnostics
    diagnostics.clear_all()
    
    -- Delete test buffer
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  describe('setup()', function()
    it('should create namespace', function()
      assert.is_not_nil(namespace)
      assert.is_number(namespace)
    end)
    
    it('should not create duplicate namespaces', function()
      local ns1 = diagnostics.get_namespace()
      diagnostics.setup()
      local ns2 = diagnostics.get_namespace()
      assert.equals(ns1, ns2)
    end)
  end)
  
  describe('diagnostic conversion', function()
    it('should convert analyzer JSON to Neovim diagnostic', function()
      local analyzer_diag = {
        file = '/test/file.go',
        line = 7,
        column = 29,
        end_column = 35,
        message = 'Property name should be PascalCase: UserId → UserId',
        code = 'MTLOG002',
        severity = 'warning',
        suggested_fixes = {
          {
            description = 'Change to PascalCase',
            edits = {
              {
                range = {
                  start = { line = 7, column = 29 },
                  ['end'] = { line = 7, column = 35 },
                },
                newText = 'UserId',
              },
            },
          },
        },
      }
      
      -- Convert and set diagnostic
      local nvim_diag = {
        lnum = analyzer_diag.line - 1,
        col = analyzer_diag.column - 1,
        end_lnum = analyzer_diag.line - 1,
        end_col = analyzer_diag.end_column - 1,
        message = analyzer_diag.message,
        severity = vim.diagnostic.severity.WARN,
        source = 'mtlog-analyzer',
        code = analyzer_diag.code,
        user_data = {
          suggested_fixes = analyzer_diag.suggested_fixes,
        },
      }
      
      diagnostics.set(test_bufnr, { nvim_diag })
      
      local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(1, #diags)
      assert.equals(nvim_diag.message, diags[1].message)
      assert.equals(nvim_diag.code, diags[1].code)
      assert.equals(nvim_diag.severity, diags[1].severity)
    end)
    
    it('should map severity levels correctly', function()
      local severities = {
        { analyzer = 'error', nvim = vim.diagnostic.severity.ERROR },
        { analyzer = 'warning', nvim = vim.diagnostic.severity.WARN },
        { analyzer = 'info', nvim = vim.diagnostic.severity.INFO },
        { analyzer = 'hint', nvim = vim.diagnostic.severity.HINT },
      }
      
      for _, test in ipairs(severities) do
        local diag = {
          lnum = 0,
          col = 0,
          message = 'Test',
          severity = test.nvim,
          source = 'mtlog-analyzer',
        }
        
        diagnostics.set(test_bufnr, { diag })
        local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
        assert.equals(test.nvim, diags[1].severity)
        diagnostics.clear(test_bufnr)
      end
    end)
    
    it('should preserve user data for code actions', function()
      local suggested_fixes = {
        {
          description = 'Add missing argument',
          edits = {
            {
              range = {
                start = { line = 8, column = 45 },
                ['end'] = { line = 8, column = 45 },
              },
              newText = ', "default"',
            },
          },
        },
      }
      
      local diag = {
        lnum = 7,
        col = 0,
        message = 'Template expects 2 arguments but got 1',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG001',
        user_data = {
          suggested_fixes = suggested_fixes,
        },
      }
      
      diagnostics.set(test_bufnr, { diag })
      local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      
      assert.is_not_nil(diags[1].user_data)
      assert.is_not_nil(diags[1].user_data.suggested_fixes)
      assert.equals(1, #diags[1].user_data.suggested_fixes)
      assert.equals('Add missing argument', diags[1].user_data.suggested_fixes[1].description)
    end)
  end)
  
  describe('set()', function()
    it('should set diagnostics for buffer', function()
      local diags = {
        {
          lnum = 6,
          col = 29,
          message = 'Test diagnostic',
          severity = vim.diagnostic.severity.WARN,
        },
      }
      
      diagnostics.set(test_bufnr, diags)
      
      local result = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(1, #result)
      assert.equals('Test diagnostic', result[1].message)
    end)
    
    it('should replace existing diagnostics', function()
      -- Set initial diagnostics
      diagnostics.set(test_bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'First',
          severity = vim.diagnostic.severity.ERROR,
        },
      })
      
      -- Replace with new diagnostics
      diagnostics.set(test_bufnr, {
        {
          lnum = 1,
          col = 0,
          message = 'Second',
          severity = vim.diagnostic.severity.WARN,
        },
      })
      
      local result = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(1, #result)
      assert.equals('Second', result[1].message)
    end)
    
    it('should trigger user event', function()
      local event_triggered = false
      local event_data = nil
      
      vim.api.nvim_create_autocmd('User', {
        pattern = 'MtlogDiagnosticsChanged',
        callback = function(args)
          event_triggered = true
          event_data = args.data or {}
        end,
      })
      
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Test', severity = vim.diagnostic.severity.ERROR },
      })
      
      assert.is_true(event_triggered)
      -- Only check event data if it exists (0.9+ feature)
      if event_data and event_data.bufnr then
        assert.equals(test_bufnr, event_data.bufnr)
        assert.equals(1, event_data.count)
      end
    end)
  end)
  
  describe('clear()', function()
    it('should clear diagnostics for buffer', function()
      -- Set diagnostics
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Test', severity = vim.diagnostic.severity.ERROR },
      })
      
      -- Clear them
      diagnostics.clear(test_bufnr)
      
      local result = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(0, #result)
    end)
    
    it('should trigger user event on clear', function()
      local event_triggered = false
      
      vim.api.nvim_create_autocmd('User', {
        pattern = 'MtlogDiagnosticsChanged',
        callback = function(args)
          -- For older versions without data, just trigger on any event
          if not args.data or args.data.count == 0 then
            event_triggered = true
          end
        end,
      })
      
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Test', severity = vim.diagnostic.severity.ERROR },
      })
      diagnostics.clear(test_bufnr)
      
      assert.is_true(event_triggered)
    end)
  end)
  
  describe('clear_all()', function()
    it('should clear diagnostics for all buffers', function()
      local bufnr2 = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr2, '/test/clear_all_file2.go')
      
      -- Set diagnostics in both buffers
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Test1', severity = vim.diagnostic.severity.ERROR },
      })
      diagnostics.set(bufnr2, {
        { lnum = 0, col = 0, message = 'Test2', severity = vim.diagnostic.severity.WARN },
      })
      
      -- Clear all
      diagnostics.clear_all()
      
      assert.equals(0, #vim.diagnostic.get(test_bufnr, { namespace = namespace }))
      assert.equals(0, #vim.diagnostic.get(bufnr2, { namespace = namespace }))
      
      vim.api.nvim_buf_delete(bufnr2, { force = true })
    end)
  end)
  
  describe('get_counts()', function()
    it('should count diagnostics by severity', function()
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Error 1', severity = vim.diagnostic.severity.ERROR },
        { lnum = 1, col = 0, message = 'Error 2', severity = vim.diagnostic.severity.ERROR },
        { lnum = 2, col = 0, message = 'Warning', severity = vim.diagnostic.severity.WARN },
        { lnum = 3, col = 0, message = 'Info', severity = vim.diagnostic.severity.INFO },
        { lnum = 4, col = 0, message = 'Hint', severity = vim.diagnostic.severity.HINT },
      })
      
      local counts = diagnostics.get_counts(test_bufnr)
      assert.equals(5, counts.total)
      assert.equals(2, counts.errors)
      assert.equals(1, counts.warnings)
      assert.equals(1, counts.info)
      assert.equals(1, counts.hints)
    end)
    
    it('should aggregate counts for all buffers', function()
      local bufnr2 = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr2, '/test/aggregate_file2.go')
      
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'Error', severity = vim.diagnostic.severity.ERROR },
        { lnum = 1, col = 0, message = 'Warning', severity = vim.diagnostic.severity.WARN },
      })
      diagnostics.set(bufnr2, {
        { lnum = 0, col = 0, message = 'Error', severity = vim.diagnostic.severity.ERROR },
        { lnum = 1, col = 0, message = 'Info', severity = vim.diagnostic.severity.INFO },
      })
      
      local counts = diagnostics.get_counts() -- No buffer specified
      assert.equals(4, counts.total)
      assert.equals(2, counts.errors)
      assert.equals(1, counts.warnings)
      assert.equals(1, counts.info)
      
      vim.api.nvim_buf_delete(bufnr2, { force = true })
    end)
  end)
  
  describe('apply_suggested_fix()', function()
    it('should apply text edit from suggested fix', function()
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
        'log.Information("User {userid} logged in", 123)',
      })
      
      local diagnostic = {
        lnum = 0,
        col = 23,
        message = 'Property should be PascalCase',
        severity = vim.diagnostic.severity.WARN,
        user_data = {
          suggested_fixes = {
            {
              description = 'Change to PascalCase',
              edits = {
                {
                  range = {
                    start = { line = 1, column = 24 },
                    ['end'] = { line = 1, column = 30 },
                  },
                  newText = 'UserId',
                },
              },
            },
          },
        },
      }
      
      -- Switch to test buffer
      vim.api.nvim_set_current_buf(test_bufnr)
      
      local success = diagnostics.apply_suggested_fix(diagnostic)
      assert.is_true(success)
      
      local lines = vim.api.nvim_buf_get_lines(test_bufnr, 0, -1, false)
      assert.equals('log.Information("User {UserId} logged in", 123)', lines[1])
    end)
    
    it('should handle missing suggested fixes', function()
      local diagnostic = {
        lnum = 0,
        col = 0,
        message = 'Test',
        severity = vim.diagnostic.severity.ERROR,
      }
      
      local success = diagnostics.apply_suggested_fix(diagnostic)
      assert.is_false(success)
    end)
  end)
  
  describe('namespace handling', function()
    it('should isolate mtlog diagnostics in separate namespace', function()
      -- Set mtlog diagnostic
      diagnostics.set(test_bufnr, {
        { lnum = 0, col = 0, message = 'mtlog diagnostic', severity = vim.diagnostic.severity.ERROR },
      })
      
      -- Set diagnostic in different namespace
      local other_ns = vim.api.nvim_create_namespace('other')
      vim.diagnostic.set(other_ns, test_bufnr, {
        { lnum = 1, col = 0, message = 'other diagnostic', severity = vim.diagnostic.severity.WARN },
      })
      
      -- Check mtlog namespace only has mtlog diagnostics
      local mtlog_diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(1, #mtlog_diags)
      assert.equals('mtlog diagnostic', mtlog_diags[1].message)
      
      -- Check all diagnostics
      local all_diags = vim.diagnostic.get(test_bufnr)
      assert.equals(2, #all_diags)
    end)
  end)
  
  describe('diagnostic navigation', function()
    it('should navigate to next/prev diagnostic', function()
      vim.api.nvim_win_set_buf(0, test_bufnr)
      vim.api.nvim_win_set_cursor(0, { 1, 0 })
      
      diagnostics.set(test_bufnr, {
        { lnum = 2, col = 0, message = 'First', severity = vim.diagnostic.severity.ERROR },
        { lnum = 5, col = 0, message = 'Second', severity = vim.diagnostic.severity.WARN },
        { lnum = 7, col = 0, message = 'Third', severity = vim.diagnostic.severity.INFO },
      })
      
      -- Go to next
      diagnostics.goto_next()
      local pos = vim.api.nvim_win_get_cursor(0)
      assert.equals(3, pos[1]) -- Line 3 (1-indexed)
      
      -- Go to next again
      diagnostics.goto_next()
      pos = vim.api.nvim_win_get_cursor(0)
      assert.equals(6, pos[1]) -- Line 6
      
      -- Go to previous
      diagnostics.goto_prev()
      pos = vim.api.nvim_win_get_cursor(0)
      assert.equals(3, pos[1]) -- Back to line 3
    end)
  end)
end)