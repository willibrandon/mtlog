-- Tests for LSP integration - NO MOCKS, real LSP operations
local test_helpers = require('test_helpers')
local analyzer = require('mtlog.analyzer')

describe('mtlog LSP integration', function()
  local lsp_integration
  local diagnostics
  local config
  local test_files = {}
  
  before_each(function()
    -- Ensure analyzer is available
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
    
    -- Clear module cache
    package.loaded['mtlog.lsp_integration'] = nil
    package.loaded['mtlog.diagnostics'] = nil
    package.loaded['mtlog.config'] = nil
    
    -- Load modules
    lsp_integration = require('mtlog.lsp_integration')
    diagnostics = require('mtlog.diagnostics')
    config = require('mtlog.config')
    
    -- Setup
    config.setup({
      lsp_integration = {
        enabled = true,
        show_suppress_action = true,
      },
    })
    diagnostics.setup()
    
    -- Clear test files
    test_files = {}
  end)
  
  after_each(function()
    -- Clean up test files
    for _, filepath in ipairs(test_files) do
      test_helpers.delete_test_file(filepath)
    end
    
    -- Clean up buffers
    for _, buf in ipairs(vim.api.nvim_list_bufs()) do
      if vim.api.nvim_buf_is_valid(buf) then
        pcall(vim.api.nvim_buf_delete, buf, { force = true })
      end
    end
  end)
  
  describe('code actions with real diagnostics', function()
    it('should generate code actions from real analyzer diagnostics', function(done)
      local done_fn = done
      -- Create a real test file with issues
      local test_file = test_helpers.create_test_go_file('lsp_actions.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("User {user_id} logged in", 123)  // MTLOG004 - needs PascalCase
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      -- Run real analyzer
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Set real diagnostics
          diagnostics.set(bufnr, results)
          
          -- Find MTLOG004 diagnostic
          local mtlog004_diag = nil
          for _, diag in ipairs(results) do
            if diag.code == 'MTLOG004' then
              mtlog004_diag = diag
              break
            end
          end
          
          if mtlog004_diag then
            -- Get code actions for the diagnostic range
            local range = {
              start = { line = mtlog004_diag.lnum, character = mtlog004_diag.col },
              ['end'] = { line = mtlog004_diag.end_lnum or mtlog004_diag.lnum, 
                        character = mtlog004_diag.end_col or mtlog004_diag.col },
            }
            
            local actions = lsp_integration.get_code_actions(bufnr, range)
            
            assert.is_not_nil(actions)
            assert.is_true(#actions > 0, "Should have at least one action")
            
            -- Should have fix action if analyzer provided it
            local has_fix = false
            local has_suppress = false
            
            for _, action in ipairs(actions) do
              if action.title:match('PascalCase') or action.title:match('Convert') then
                has_fix = true
              end
              if action.title:match('Suppress MTLOG004') then
                has_suppress = true
              end
            end
            
            if mtlog004_diag.user_data and mtlog004_diag.user_data.suggested_fixes then
              assert.is_true(has_fix, "Should have fix action")
            end
            assert.is_true(has_suppress, "Should have suppress action")
          end
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should handle real diagnostics without fixes', function(done)
      local done_fn = done
      -- Create a file that will generate a warning without fix
      local test_file = test_helpers.create_test_go_file('lsp_no_fix.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    template := "Dynamic " + "template"
    log.Information(template, 123)  // MTLOG008 - dynamic template warning
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          
          if results and #results > 0 then
            diagnostics.set(bufnr, results)
            
            local first_diag = results[1]
            local range = {
              start = { line = first_diag.lnum, character = first_diag.col },
              ['end'] = { line = first_diag.end_lnum or first_diag.lnum, 
                        character = first_diag.end_col or first_diag.col },
            }
            
            local actions = lsp_integration.get_code_actions(bufnr, range)
            assert.is_not_nil(actions)
            
            -- Should have suppress action only if no fixes
            if not (first_diag.user_data and first_diag.user_data.suggested_fixes) then
              local suppress_found = false
              for _, action in ipairs(actions) do
                if action.title:match('Suppress') then
                  suppress_found = true
                  break
                end
              end
              assert.is_true(suppress_found or #actions == 0)
            end
          end
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should not show suppress action when disabled', function(done)
      local done_fn = done
      -- Disable suppress action
      config.setup({
        lsp_integration = {
          enabled = true,
          show_suppress_action = false,
        },
      })
      
      local test_file = test_helpers.create_test_go_file('lsp_no_suppress.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Error("Error {Code}")  // Missing argument
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          
          if results and #results > 0 then
            diagnostics.set(bufnr, results)
            
            local first_diag = results[1]
            local range = {
              start = { line = first_diag.lnum, character = first_diag.col },
              ['end'] = { line = first_diag.end_lnum or first_diag.lnum, 
                        character = first_diag.end_col or first_diag.col },
            }
            
            local actions = lsp_integration.get_code_actions(bufnr, range)
            assert.is_not_nil(actions)
            
            -- Should not have suppress action
            for _, action in ipairs(actions) do
              assert.is_false(action.title:match('Suppress'), 
                            "Should not have suppress action when disabled")
            end
          end
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should not show suppress action for already suppressed diagnostics', function()
      -- Set up with suppressed diagnostic
      config.setup({
        suppressed_diagnostics = { 'MTLOG001' },
        lsp_integration = {
          enabled = true,
          show_suppress_action = true,
        },
      })
      
      -- Create a real diagnostic
      local test_diag = {
        lnum = 5,
        col = 0,
        severity = vim.diagnostic.severity.ERROR,
        message = '[MTLOG001] Already suppressed',
        code = 'MTLOG001',
        source = 'mtlog-analyzer',
      }
      
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.diagnostic.set(diagnostics.get_namespace(), bufnr, { test_diag })
      
      local range = {
        start = { line = 5, character = 0 },
        ['end'] = { line = 5, character = 10 },
      }
      
      local actions = lsp_integration.get_code_actions(bufnr, range)
      
      assert.is_not_nil(actions)
      
      -- Should not have suppress action for MTLOG001
      for _, action in ipairs(actions) do
        assert.is_false(action.title == 'Suppress MTLOG001',
                       "Should not show suppress for already suppressed")
      end
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('workspace edit creation with real data', function()
    it('should create valid LSP workspace edits from real fixes', function(done)
      local done_fn = done
      local test_file = test_helpers.create_test_go_file('lsp_edit.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("User {userId} logged in", 123)  // camelCase property
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          
          -- Find diagnostic with fixes
          local diag_with_fix = nil
          for _, diag in ipairs(results or {}) do
            if diag.user_data and diag.user_data.suggested_fixes then
              diag_with_fix = diag
              break
            end
          end
          
          if diag_with_fix and diag_with_fix.user_data.suggested_fixes[1] then
            local fix = diag_with_fix.user_data.suggested_fixes[1]
            if fix.edits then
              local workspace_edit = lsp_integration._create_workspace_edit(bufnr, fix.edits)
              
              assert.is_not_nil(workspace_edit)
              assert.is_not_nil(workspace_edit.changes)
              
              local uri = vim.uri_from_bufnr(bufnr)
              assert.is_not_nil(workspace_edit.changes[uri])
              
              local text_edits = workspace_edit.changes[uri]
              assert.is_true(#text_edits > 0, "Should have text edits")
            end
          end
          
          done_fn()
        end)
      end)
    end, 10000)
  end)
  
  describe('diagnostic conversion', function()
    it('should convert real Neovim diagnostic to LSP format', function(done)
      local done_fn = done
      local test_file = test_helpers.create_test_go_file('lsp_convert.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Error("Error {Code}")  // Missing argument
}
]])
      table.insert(test_files, test_file)
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          
          if results and #results > 0 then
            local nvim_diag = results[1]
            local lsp_diag = lsp_integration._diagnostic_to_lsp(nvim_diag)
            
            assert.is_not_nil(lsp_diag)
            assert.is_number(lsp_diag.range.start.line)
            assert.is_number(lsp_diag.range.start.character)
            assert.is_number(lsp_diag.range['end'].line)
            assert.is_number(lsp_diag.range['end'].character)
            assert.is_number(lsp_diag.severity)
            assert.is_string(lsp_diag.code)
            assert.equals('mtlog-analyzer', lsp_diag.source)
            assert.is_string(lsp_diag.message)
          end
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should convert severity levels correctly', function()
      -- Test severity conversion
      assert.equals(1, lsp_integration._severity_to_lsp(vim.diagnostic.severity.ERROR))
      assert.equals(2, lsp_integration._severity_to_lsp(vim.diagnostic.severity.WARN))
      assert.equals(3, lsp_integration._severity_to_lsp(vim.diagnostic.severity.INFO))
      assert.equals(4, lsp_integration._severity_to_lsp(vim.diagnostic.severity.HINT))
    end)
  end)
  
  describe('commands', function()
    it('should register real LSP commands', function()
      lsp_integration.register_commands()
      
      -- Check that commands are registered
      assert.is_not_nil(vim.lsp.commands['mtlog.suppress_diagnostic'])
      assert.is_not_nil(vim.lsp.commands['mtlog.apply_fix'])
    end)
  end)
  
  describe('buffer attachment', function()
    it('should create real buffer-local command when attached', function()
      local test_file = test_helpers.create_test_go_file('lsp_attach.go', [[
package main

func main() {
    println("test")
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      lsp_integration.attach(bufnr)
      
      -- Check that buffer command exists
      local commands = vim.api.nvim_buf_get_commands(bufnr, {})
      assert.is_not_nil(commands.MtlogCodeAction)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('code actions at cursor with real diagnostics', function()
    it('should get code actions at real cursor position', function(done)
      local done_fn = done
      local test_file = test_helpers.create_test_go_file('lsp_cursor.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // Cursor will be on next line
    log.Information("User {UserId} logged in")  // Missing argument
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      vim.api.nvim_set_current_buf(bufnr)
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          
          if results and #results > 0 then
            diagnostics.set(bufnr, results)
            
            -- Find line with diagnostic
            local diag = results[1]
            
            -- Set cursor to diagnostic position (convert 0-based to 1-based)
            vim.api.nvim_win_set_cursor(0, { diag.lnum + 1, diag.col })
            
            local actions = lsp_integration.get_code_actions_at_cursor()
            
            assert.is_not_nil(actions)
            -- Should have actions if there's a diagnostic at cursor
            assert.is_true(#actions > 0, "Should have actions at diagnostic position")
          end
          
          done_fn()
        end)
      end)
    end, 10000)
  end)
end)