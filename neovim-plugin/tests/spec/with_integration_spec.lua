-- Integration test for With() diagnostics - simpler version
local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

describe('With() diagnostics integration', function()
  local test_bufnr
  
  before_each(function()
    -- Setup config
    config.setup({})
    
    -- Create test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/test/with.go')
    vim.api.nvim_set_current_buf(test_bufnr)
    
    -- Setup diagnostics
    diagnostics.setup()
  end)
  
  after_each(function()
    diagnostics.clear_all()
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  it('should handle MTLOG009 with pos/end format', function()
    -- Set buffer content
    vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
      'log.With("Key", 123, "Extra")',
    })
    
    -- Create diagnostic with new format
    local diag = {
      lnum = 0,
      col = 0,
      end_lnum = 0,
      end_col = 30,
      message = '[MTLOG009] Odd arguments',
      severity = vim.diagnostic.severity.ERROR,
      source = 'mtlog-analyzer',
      code = 'MTLOG009',
      user_data = {
        suggested_fixes = {
          {
            description = 'Add empty value',
            edits = {
              {
                pos = 'test.go:1:29',  -- After "Extra"
                ['end'] = 'test.go:1:29',
                newText = ', ""',
              },
            },
          },
        },
      },
    }
    
    -- Set and verify diagnostic
    diagnostics.set(test_bufnr, { diag })
    local diags = vim.diagnostic.get(test_bufnr)
    assert.equals(1, #diags)
    assert.equals('MTLOG009', diags[1].code)
    
    -- Apply fix
    vim.api.nvim_win_set_cursor(0, { 1, 5 })
    local success = diagnostics.apply_suggested_fix(diags[1], 1)
    assert.is_true(success)
    
    -- Verify fix applied
    local lines = vim.api.nvim_buf_get_lines(test_bufnr, 0, 1, false)
    assert.is_true(lines[1]:match(', ""') ~= nil, "Expected empty string added")
  end)
  
  it('should verify config has new severity levels', function()
    local cfg = config.get()
    assert.is_not_nil(cfg.severity_levels.MTLOG009)
    assert.is_not_nil(cfg.severity_levels.MTLOG010)
    assert.is_not_nil(cfg.severity_levels.MTLOG011)
    assert.is_not_nil(cfg.severity_levels.MTLOG012)
    assert.is_not_nil(cfg.severity_levels.MTLOG013)
    
    -- Check severity values
    assert.equals(vim.diagnostic.severity.ERROR, cfg.severity_levels.MTLOG009)
    assert.equals(vim.diagnostic.severity.WARN, cfg.severity_levels.MTLOG010)
    assert.equals(vim.diagnostic.severity.INFO, cfg.severity_levels.MTLOG011)
    assert.equals(vim.diagnostic.severity.WARN, cfg.severity_levels.MTLOG012)
    assert.equals(vim.diagnostic.severity.ERROR, cfg.severity_levels.MTLOG013)
  end)
  
  it('should verify help has new diagnostic explanations', function()
    local help = require('mtlog.help')
    
    -- Check all new codes have help
    local codes = {'MTLOG009', 'MTLOG010', 'MTLOG011', 'MTLOG012', 'MTLOG013'}
    for _, code in ipairs(codes) do
      local explanation = help.diagnostic_explanations[code]
      assert.is_not_nil(explanation, code .. " missing from help")
      assert.is_not_nil(explanation.name)
      assert.is_not_nil(explanation.description)
      assert.is_not_nil(explanation.example)
      assert.is_not_nil(explanation.fix)
    end
  end)
end)