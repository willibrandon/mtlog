-- Tests for LSP integration

describe('mtlog LSP integration', function()
  local lsp_integration
  local diagnostics
  local config
  
  before_each(function()
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
  end)
  
  describe('code actions', function()
    it('should generate code actions from diagnostics', function()
      -- Create a test diagnostic with suggested fixes
      local test_diag = {
        lnum = 10,
        col = 5,
        end_lnum = 10,
        end_col = 15,
        severity = vim.diagnostic.severity.ERROR,
        message = 'Template argument mismatch',
        code = 'MTLOG001',
        source = 'mtlog-analyzer',
        user_data = {
          suggested_fixes = {
            {
              title = 'Add missing argument',
              edits = {
                {
                  start_line = 11,
                  start_col = 10,
                  end_line = 11,
                  end_col = 10,
                  new_text = ', userId',
                },
              },
            },
          },
        },
      }
      
      -- Set diagnostic in namespace
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.diagnostic.set(diagnostics.get_namespace(), bufnr, { test_diag })
      
      -- Get code actions for the diagnostic range
      local range = {
        start = { line = 10, character = 5 },
        ['end'] = { line = 10, character = 15 },
      }
      
      local actions = lsp_integration.get_code_actions(bufnr, range)
      
      assert.is_not_nil(actions)
      assert.are.equal(2, #actions)  -- Fix action + suppress action
      
      -- Check fix action
      assert.are.equal('Add missing argument', actions[1].title)
      assert.are.equal('quickfix', actions[1].kind)
      assert.is_true(actions[1].isPreferred)
      
      -- Check suppress action
      assert.are.equal('Suppress MTLOG001', actions[2].title)
      assert.are.equal('refactor', actions[2].kind)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should handle diagnostics without fixes', function()
      -- Create a diagnostic without suggested fixes
      local test_diag = {
        lnum = 5,
        col = 0,
        severity = vim.diagnostic.severity.WARN,
        message = 'Warning without fix',
        code = 'MTLOG004',
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
      assert.are.equal(1, #actions)  -- Only suppress action
      assert.are.equal('Suppress MTLOG004', actions[1].title)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should not show suppress action when disabled', function()
      -- Disable suppress action
      config.setup({
        lsp_integration = {
          enabled = true,
          show_suppress_action = false,
        },
      })
      
      local test_diag = {
        lnum = 5,
        col = 0,
        severity = vim.diagnostic.severity.WARN,
        message = 'Warning',
        code = 'MTLOG004',
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
      assert.are.equal(0, #actions)  -- No actions
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should not show suppress action for already suppressed diagnostics', function()
      -- Set up with suppressed diagnostic
      config.setup({
        suppressed_diagnostics = { 'MTLOG001' },
        lsp_integration = {
          enabled = true,
          show_suppress_action = true,
        },
      })
      
      local test_diag = {
        lnum = 5,
        col = 0,
        severity = vim.diagnostic.severity.ERROR,
        message = 'Already suppressed',
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
      assert.are.equal(0, #actions)  -- No suppress action since already suppressed
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('workspace edit creation', function()
    it('should create valid LSP workspace edits', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      
      local edits = {
        {
          start_line = 10,
          start_col = 5,
          end_line = 10,
          end_col = 10,
          new_text = 'replacement',
        },
      }
      
      local workspace_edit = lsp_integration._create_workspace_edit(bufnr, edits)
      
      assert.is_not_nil(workspace_edit)
      assert.is_not_nil(workspace_edit.changes)
      
      local uri = vim.uri_from_bufnr(bufnr)
      assert.is_not_nil(workspace_edit.changes[uri])
      
      local text_edits = workspace_edit.changes[uri]
      assert.are.equal(1, #text_edits)
      
      -- Check LSP coordinates (0-based)
      assert.are.equal(9, text_edits[1].range.start.line)
      assert.are.equal(4, text_edits[1].range.start.character)
      assert.are.equal(9, text_edits[1].range['end'].line)
      assert.are.equal(9, text_edits[1].range['end'].character)
      assert.are.equal('replacement', text_edits[1].newText)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('diagnostic conversion', function()
    it('should convert Neovim diagnostic to LSP format', function()
      local nvim_diag = {
        lnum = 5,
        col = 10,
        end_lnum = 5,
        end_col = 20,
        severity = vim.diagnostic.severity.ERROR,
        code = 'MTLOG001',
        source = 'mtlog-analyzer',
        message = 'Test error',
      }
      
      local lsp_diag = lsp_integration._diagnostic_to_lsp(nvim_diag)
      
      assert.is_not_nil(lsp_diag)
      assert.are.equal(5, lsp_diag.range.start.line)
      assert.are.equal(10, lsp_diag.range.start.character)
      assert.are.equal(5, lsp_diag.range['end'].line)
      assert.are.equal(20, lsp_diag.range['end'].character)
      assert.are.equal(1, lsp_diag.severity)  -- ERROR = 1 in LSP
      assert.are.equal('MTLOG001', lsp_diag.code)
      assert.are.equal('mtlog-analyzer', lsp_diag.source)
      assert.are.equal('Test error', lsp_diag.message)
    end)
    
    it('should convert severity levels correctly', function()
      assert.are.equal(1, lsp_integration._severity_to_lsp(vim.diagnostic.severity.ERROR))
      assert.are.equal(2, lsp_integration._severity_to_lsp(vim.diagnostic.severity.WARN))
      assert.are.equal(3, lsp_integration._severity_to_lsp(vim.diagnostic.severity.INFO))
      assert.are.equal(4, lsp_integration._severity_to_lsp(vim.diagnostic.severity.HINT))
    end)
  end)
  
  describe('commands', function()
    it('should register LSP commands', function()
      lsp_integration.register_commands()
      
      -- Check that commands are registered
      assert.is_not_nil(vim.lsp.commands['mtlog.suppress_diagnostic'])
      assert.is_not_nil(vim.lsp.commands['mtlog.apply_fix'])
    end)
  end)
  
  describe('buffer attachment', function()
    it('should create buffer-local command when attached', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      
      lsp_integration.attach(bufnr)
      
      -- Check that buffer command exists
      local commands = vim.api.nvim_buf_get_commands(bufnr, {})
      assert.is_not_nil(commands.MtlogCodeAction)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('code actions at cursor', function()
    it('should get code actions at cursor position', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      
      -- Add some content to the buffer first
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, {
        'package main',
        '',
        'import "fmt"',
        '',
        'func main() {',
        '    fmt.Println("Hello")',
        '    fmt.Println("World")',
        '    fmt.Println("Test")',
        '    fmt.Println("Line 9")',
        '    fmt.Println("Line 10")',
        '    fmt.Println("Line 11")',  -- Line 11 where we'll set cursor
        '    fmt.Println("Line 12")',
        '}',
      })
      
      vim.api.nvim_set_current_buf(bufnr)
      
      -- Set cursor position
      vim.api.nvim_win_set_cursor(0, { 11, 5 })  -- Line 11, column 5
      
      -- Add diagnostic at cursor position
      local test_diag = {
        lnum = 10,  -- 0-based in diagnostic
        col = 5,
        severity = vim.diagnostic.severity.ERROR,
        message = 'Error at cursor',
        code = 'MTLOG001',
        source = 'mtlog-analyzer',
        user_data = {
          suggested_fixes = {
            {
              title = 'Fix at cursor',
              edits = {},
            },
          },
        },
      }
      
      vim.diagnostic.set(diagnostics.get_namespace(), bufnr, { test_diag })
      
      local actions = lsp_integration.get_code_actions_at_cursor()
      
      assert.is_not_nil(actions)
      assert.is_true(#actions > 0)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
end)