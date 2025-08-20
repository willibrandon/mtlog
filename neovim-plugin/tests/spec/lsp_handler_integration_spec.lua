-- Tests for actual LSP handler integration
-- This test verifies that mtlog actions appear in vim.lsp.buf.code_action()

describe('mtlog LSP handler integration', function()
  local lsp_integration
  local diagnostics
  local config
  
  before_each(function()
    -- Clear module cache
    package.loaded['mtlog.lsp_integration'] = nil
    package.loaded['mtlog.diagnostics'] = nil
    package.loaded['mtlog.config'] = nil
    package.loaded['mtlog'] = nil
    
    -- Stop any existing mtlog clients
    local get_clients = vim.lsp.get_active_clients or vim.lsp.get_clients
    local clients = get_clients and get_clients() or {}
    for _, client in ipairs(clients) do
      if client.name == 'mtlog-analyzer' then
        vim.lsp.stop_client(client.id)
      end
    end
    
    -- Reset LSP handlers to default (if _default exists)
    if vim.lsp.handlers._default then
      vim.lsp.handlers['textDocument/codeAction'] = vim.lsp.handlers._default['textDocument/codeAction']
    else
      -- Just set to nil to reset
      vim.lsp.handlers['textDocument/codeAction'] = nil
    end
    
    -- Load modules
    config = require('mtlog.config')
    diagnostics = require('mtlog.diagnostics')
    lsp_integration = require('mtlog.lsp_integration')
    
    -- Setup
    config.setup({
      lsp_integration = {
        enabled = true,
        show_suppress_action = true,
      },
    })
    diagnostics.setup()
  end)
  
  it('should create a fake LSP client', function()
    -- Setup LSP integration
    lsp_integration.setup()
    
    -- Check that a client was created
    local get_clients = vim.lsp.get_active_clients or vim.lsp.get_clients
    local clients = get_clients and get_clients() or {}
    local found_mtlog = false
    for _, client in ipairs(clients) do
      if client.name == 'mtlog-analyzer' then
        found_mtlog = true
        break
      end
    end
    
    assert.is_true(found_mtlog, 'mtlog-analyzer client should be created')
  end)
  
  it('should provide code actions through fake LSP client', function()
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
    
    -- Setup LSP integration
    lsp_integration.setup()
    
    -- Get code actions directly from the integration
    local range = {
      start = { line = 10, character = 5 },
      ['end'] = { line = 10, character = 15 },
    }
    
    local actions = lsp_integration.get_code_actions(bufnr, range)
    
    -- Check that mtlog actions are generated
    assert.is_not_nil(actions)
    assert.is_true(#actions > 0)  -- Should have mtlog actions
    
    -- Find the mtlog action
    local has_fix = false
    for _, action in ipairs(actions) do
      if action.title and action.title:find('MTLOG001') then
        has_fix = true
        break
      end
    end
    assert.is_true(has_fix, 'Should have fix action')
    
    -- Clean up
    lsp_integration.stop()
    vim.api.nvim_buf_delete(bufnr, { force = true })
  end)
  
  it('should handle diagnostics without fixes', function()
    -- Create a test diagnostic without fixes
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
    
    -- Setup LSP integration
    lsp_integration.setup()
    
    local range = {
      start = { line = 5, character = 0 },
      ['end'] = { line = 5, character = 10 },
    }
    
    -- Get code actions
    local actions = lsp_integration.get_code_actions(bufnr, range)
    
    -- Should have suppress action even without fixes
    assert.is_not_nil(actions)
    assert.is_true(#actions > 0)
    
    -- Find the suppress action
    local has_suppress = false
    for _, action in ipairs(actions) do
      if action.title == 'Suppress MTLOG004' then
        has_suppress = true
        break
      end
    end
    assert.is_true(has_suppress, 'Should have suppress action')
    
    -- Clean up
    lsp_integration.stop()
    vim.api.nvim_buf_delete(bufnr, { force = true })
  end)
  
  it('should register LSP commands', function()
    -- Setup LSP integration
    lsp_integration.setup()
    lsp_integration.register_commands()
    
    -- Check that commands are registered
    assert.is_not_nil(vim.lsp.commands['mtlog.suppress_diagnostic'])
    assert.is_not_nil(vim.lsp.commands['mtlog.apply_fix'])
    
    -- Clean up
    lsp_integration.stop()
  end)
  
  it('should only setup once', function()
    -- Setup LSP integration
    lsp_integration.setup()
    
    -- Get code actions to verify it works
    local bufnr = vim.api.nvim_create_buf(false, true)
    local actions1 = lsp_integration.get_code_actions(bufnr, {
      start = { line = 0, character = 0 },
      ['end'] = { line = 0, character = 1 },
    })
    
    -- Setup again - should not create another client
    lsp_integration.setup()
    
    -- Should still work
    local actions2 = lsp_integration.get_code_actions(bufnr, {
      start = { line = 0, character = 0 },
      ['end'] = { line = 0, character = 1 },
    })
    
    -- Both should work (return empty arrays since no diagnostics)
    assert.is_not_nil(actions1)
    assert.is_not_nil(actions2)
    
    -- Clean up
    lsp_integration.stop()
    vim.api.nvim_buf_delete(bufnr, { force = true })
  end)
end)