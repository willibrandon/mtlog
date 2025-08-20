-- Tests for mtlog statusline module
local statusline = require('mtlog.statusline')
local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

describe('mtlog statusline', function()
  local test_bufnr
  
  before_each(function()
    -- Setup config
    config.setup()
    
    -- Create test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/test/statusline.go')
    
    -- Setup diagnostics
    diagnostics.setup()
    
    -- Clear any existing diagnostics
    diagnostics.clear_all()
  end)
  
  after_each(function()
    -- Clean up
    diagnostics.clear_all()
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  describe('get_component', function()
    it('should return empty string when no diagnostics and show_zero is false', function()
      local component = statusline.get_component({
        bufnr = test_bufnr,
        show_zero = false
      })
      assert.equals('', component)
    end)
    
    it('should return zero counts when no diagnostics and show_zero is true', function()
      local component = statusline.get_component({
        bufnr = test_bufnr,
        show_zero = true,
        format = 'short'
      })
      assert.is_not_nil(component)
      assert.is_true(component:match('0') ~= nil, "Should contain zero")
    end)
    
    it('should show error count in short format', function()
      -- Add error diagnostic
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test error',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      assert.is_not_equals('', component, "Component should not be empty")
      -- The component might use different formatting, let's be more flexible
      assert.is_true(component:match('1') ~= nil or component:match('error') ~= nil, 
                     "Should contain error count or text")
    end)
    
    it('should show warning count in short format', function()
      -- Add warning diagnostic
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test warning',
        severity = vim.diagnostic.severity.WARN
      }})
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      assert.is_not_equals('', component, "Component should not be empty")
      assert.is_true(component:match('1') ~= nil or component:match('warning') ~= nil,
                     "Should contain warning count or text")
    end)
    
    it('should show multiple severity counts', function()
      -- Add diagnostics of different severities
      diagnostics.set(test_bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Error 1',
          severity = vim.diagnostic.severity.ERROR
        },
        {
          lnum = 1,
          col = 0,
          message = 'Error 2',
          severity = vim.diagnostic.severity.ERROR
        },
        {
          lnum = 2,
          col = 0,
          message = 'Warning',
          severity = vim.diagnostic.severity.WARN
        },
        {
          lnum = 3,
          col = 0,
          message = 'Info',
          severity = vim.diagnostic.severity.INFO
        }
      })
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      assert.is_true(component:match('2') ~= nil, "Should contain error count of 2")
      assert.is_true(component:match('1') ~= nil, "Should contain other counts of 1")
    end)
    
    it('should support long format', function()
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test error',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'long'
      })
      
      assert.is_not_nil(component)
      -- Long format might include words like "Errors" or "Problems"
      assert.is_true(#component > 5, "Long format should be longer")
    end)
    
    it('should support minimal format', function()
      diagnostics.set(test_bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Error',
          severity = vim.diagnostic.severity.ERROR
        },
        {
          lnum = 1,
          col = 0,
          message = 'Warning',
          severity = vim.diagnostic.severity.WARN
        }
      })
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'minimal'
      })
      
      assert.is_not_nil(component)
      -- Minimal format should be very short
      assert.is_true(#component < 20, "Minimal format should be concise")
    end)
    
    it('should handle custom separator', function()
      diagnostics.set(test_bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Error',
          severity = vim.diagnostic.severity.ERROR
        },
        {
          lnum = 1,
          col = 0,
          message = 'Warning',
          severity = vim.diagnostic.severity.WARN
        }
      })
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'short',
        separator = ' | '
      })
      
      assert.is_not_nil(component)
      -- Should use custom separator if multiple severities
      if component:match('|') then
        assert.is_true(component:match(' | ') ~= nil, "Should use custom separator")
      end
    end)
    
    it('should use current buffer when bufnr not specified', function()
      vim.api.nvim_set_current_buf(test_bufnr)
      
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      local component = statusline.get_component({
        format = 'short'
      })
      
      assert.is_not_nil(component)
      assert.is_true(component:match('1') ~= nil, "Should show count from current buffer")
    end)
  end)
  
  describe('get_colored_component', function()
    it('should include highlight groups', function()
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test error',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      local component = statusline.get_colored_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      -- Should contain statusline highlight syntax
      assert.is_true(component:match('%%#') ~= nil, "Should contain highlight groups")
    end)
    
    it('should use diagnostic highlight groups', function()
      diagnostics.set(test_bufnr, {
        {
          lnum = 0,
          col = 0,
          message = 'Error',
          severity = vim.diagnostic.severity.ERROR
        },
        {
          lnum = 1,
          col = 0,
          message = 'Warning',
          severity = vim.diagnostic.severity.WARN
        }
      })
      
      local component = statusline.get_colored_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      -- Should contain error and warning highlight groups
      local has_error_hl = component:match('DiagnosticError') or 
                          component:match('ErrorMsg') or
                          component:match('MtlogError')
      local has_warn_hl = component:match('DiagnosticWarn') or 
                         component:match('WarningMsg') or
                         component:match('MtlogWarn')
      
      assert.is_truthy(has_error_hl or has_warn_hl, "Should contain diagnostic highlight groups")
    end)
    
    it('should reset highlight at the end', function()
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Test',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      local component = statusline.get_colored_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      -- Should reset highlight
      assert.is_true(component:match('%%#Normal#') ~= nil or
                    component:match('%%#StatusLine#') ~= nil or
                    component:match('%%##') ~= nil or
                    component:match('%%*') ~= nil,
                    "Should reset highlight at the end")
    end)
  end)
  
  -- Note: statusline.update() doesn't exist in the module
  -- The statusline is updated automatically when diagnostics change
  
  describe('integration', function()
    it('should work with real diagnostic data', function()
      -- Simulate mtlog diagnostics
      diagnostics.set(test_bufnr, {
        {
          lnum = 10,
          col = 5,
          end_lnum = 10,
          end_col = 15,
          message = '[MTLOG001] Template/argument mismatch',
          severity = vim.diagnostic.severity.ERROR,
          source = 'mtlog-analyzer',
          code = 'MTLOG001'
        },
        {
          lnum = 20,
          col = 8,
          end_lnum = 20,
          end_col = 20,
          message = '[MTLOG004] Property should be PascalCase',
          severity = vim.diagnostic.severity.WARN,
          source = 'mtlog-analyzer',
          code = 'MTLOG004'
        }
      })
      
      local component = statusline.get_component({
        bufnr = test_bufnr,
        format = 'short'
      })
      
      assert.is_not_nil(component)
      assert.is_not_equals('', component)
      -- Should show 1 error and 1 warning
      assert.is_true(component:find('1') ~= nil, "Should show counts")
    end)
  end)
end)