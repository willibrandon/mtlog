-- Tests for mtlog health module
local health = require('mtlog.health')
local analyzer = require('mtlog.analyzer')

describe('mtlog health', function()
  describe('check', function()
    it('should run without errors', function()
      -- Since health checks output to a special buffer that's hard to capture,
      -- we'll just verify the function runs without throwing errors
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Health check should run without errors: " .. tostring(err))
    end)
    
    it('should handle missing analyzer gracefully', function()
      -- Temporarily rename analyzer to simulate it being unavailable
      local original_path = vim.g.mtlog_analyzer_path
      local original_env = vim.env.MTLOG_ANALYZER_PATH
      
      vim.g.mtlog_analyzer_path = '/nonexistent/analyzer'
      vim.env.MTLOG_ANALYZER_PATH = '/nonexistent/analyzer'
      
      -- Should still run without errors
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Should handle missing analyzer gracefully")
      
      -- Restore
      vim.g.mtlog_analyzer_path = original_path
      vim.env.MTLOG_ANALYZER_PATH = original_env
    end)
    
    it('should handle missing go gracefully', function()
      -- Save original go executable path
      local go_path = vim.fn.exepath('go')
      if go_path == '' then
        -- If go is not installed on the test system, test passes by default
        assert.is_true(true, "Go not found on test system - test passes")
        return
      end
      
      -- Create a temporary directory and create a stub 'go' that exits with error
      local temp_dir = vim.fn.tempname()
      vim.fn.system('mkdir -p ' .. temp_dir)
      
      -- Create a fake go executable that always fails
      local fake_go = temp_dir .. '/go'
      local fake_go_content = '#!/bin/bash\nexit 127'
      vim.fn.writefile({fake_go_content}, fake_go)
      vim.fn.system('chmod +x ' .. fake_go)
      
      -- Prepend our temp directory to PATH so our fake go is found first
      local original_path = vim.env.PATH
      vim.env.PATH = temp_dir .. ':' .. original_path
      
      -- Run health check - it should handle missing go gracefully
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Should handle missing go gracefully: " .. tostring(err))
      
      -- Restore PATH
      vim.env.PATH = original_path
      
      -- Clean up temp directory
      vim.fn.delete(temp_dir, 'rf')
    end)
    
    it('should work with custom configuration', function()
      local config = require('mtlog.config')
      config.setup({
        diagnostics_enabled = false,
        debounce_ms = 1000,
        suppressed_diagnostics = {'MTLOG001', 'MTLOG002'}
      })
      
      -- Should run with custom config
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Should work with custom configuration")
    end)
  end)
  
  -- Note: Individual check functions (check_analyzer, check_go_installation, etc.)
  -- are not exported by the module, so we can only test the main check() function.
  -- The internal functions are tested indirectly through the main check() function.
end)