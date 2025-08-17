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
      -- Mock analyzer as unavailable
      local original_is_available = analyzer.is_available
      analyzer.is_available = function() return false end
      
      -- Should still run without errors
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Should handle missing analyzer gracefully")
      
      -- Restore
      analyzer.is_available = original_is_available
    end)
    
    it('should handle missing go gracefully', function()
      -- Mock executable check
      local original_executable = vim.fn.executable
      vim.fn.executable = function(cmd)
        if cmd == 'go' then
          return 0
        end
        return 1
      end
      
      -- Should still run without errors
      local ok, err = pcall(health.check)
      assert.is_true(ok, "Should handle missing go gracefully")
      
      -- Restore
      vim.fn.executable = original_executable
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