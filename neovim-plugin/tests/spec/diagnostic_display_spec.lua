-- Test for diagnostic display timing issue
local diagnostics = require('mtlog.diagnostics')
local analyzer = require('mtlog.analyzer')
local config = require('mtlog.config')

describe('diagnostic display timing', function()
  local test_bufnr
  local namespace
  
  before_each(function()
    -- Reset config with fast updatetime
    config.setup({
      analyzer_path = vim.g.mtlog_analyzer_path or 'mtlog-analyzer',
    })
    
    -- Create a test buffer with Go code
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/tmp/test_timing.go')
    vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, {
      'package main',
      '',
      'import "github.com/willibrandon/mtlog"',
      '',
      'func main() {',
      '    log := mtlog.New()',
      '    log.Debug("Processing {RequestId}")', -- Missing argument
      '    log.Information("User {userid} logged in", 123)', -- Wrong case
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
  
  describe('immediate display', function()
    it('should display diagnostics immediately after setting', function()
      -- Create test diagnostics
      local test_diags = {
        {
          lnum = 6,  -- Line 7 (0-indexed)
          col = 4,
          end_lnum = 6,
          end_col = 40,
          message = '[MTLOG001] Template expects 1 argument but got 0',
          severity = vim.diagnostic.severity.ERROR,
          source = 'mtlog-analyzer',
          code = 'MTLOG001',
        },
        {
          lnum = 7,  -- Line 8 (0-indexed)
          col = 29,
          end_lnum = 7,
          end_col = 35,
          message = '[MTLOG004] Property name should be PascalCase: userid → UserId',
          severity = vim.diagnostic.severity.WARN,
          source = 'mtlog-analyzer',
          code = 'MTLOG004',
        },
      }
      
      -- Set diagnostics
      diagnostics.set(test_bufnr, test_diags)
      
      -- Check they are immediately available
      local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      assert.equals(2, #diags, "Diagnostics should be set immediately")
      assert.equals('MTLOG001', diags[1].code)
      assert.equals('MTLOG004', diags[2].code)
    end)
    
    it('should display diagnostics without delay', function()
      -- Track timing
      local start_time = vim.loop.hrtime()
      
      -- Set diagnostics
      local test_diag = {
        lnum = 6,
        col = 4,
        message = 'Test diagnostic',
        severity = vim.diagnostic.severity.ERROR,
      }
      
      diagnostics.set(test_bufnr, {test_diag})
      
      -- Check immediately (should not need to wait for updatetime)
      local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
      local elapsed = (vim.loop.hrtime() - start_time) / 1e9
      
      assert.equals(1, #diags, "Diagnostic should be available immediately")
      assert.is_true(elapsed < 0.1, string.format("Diagnostic set took %.3f seconds, should be < 0.1", elapsed))
    end)
    
    it('should handle rapid diagnostic updates', function()
      -- Simulate rapid updates like during typing
      for i = 1, 5 do
        local diag = {
          lnum = i - 1,
          col = 0,
          message = string.format('Diagnostic %d', i),
          severity = vim.diagnostic.severity.WARN,
        }
        
        diagnostics.set(test_bufnr, {diag})
        
        -- Verify immediately
        local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
        assert.equals(1, #diags)
        assert.equals(string.format('Diagnostic %d', i), diags[1].message)
      end
    end)
  end)
  
  describe('analyzer integration', function()
    it('should analyze and display results quickly', function(done)
      -- Skip if analyzer not available
      if not analyzer.is_available() then
        pending("mtlog-analyzer not available")
        return
      end
      
      local start_time = vim.loop.hrtime()
      
      -- Write test content to a real file for analyzer
      local test_file = '/tmp/mtlog_test_' .. os.time() .. '.go'
      local file = io.open(test_file, 'w')
      file:write([[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Processing {RequestId}")  // Missing argument
}
]])
      file:close()
      
      -- Analyze the file
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          -- Clean up test file
          os.remove(test_file)
          
          if err then
            error("Analyzer failed: " .. err)
            done()
            return
          end
          
          local elapsed = (vim.loop.hrtime() - start_time) / 1e9
          
          -- Should complete quickly (under 2 seconds even with analyzer startup)
          assert.is_true(elapsed < 2.0, string.format("Analysis took %.3f seconds, should be < 2", elapsed))
          
          -- Should have found the issue
          assert.is_true(#results > 0, "Should find at least one diagnostic")
          
          -- Set the diagnostics
          diagnostics.set(test_bufnr, results)
          
          -- Verify they're immediately available
          local diags = vim.diagnostic.get(test_bufnr, { namespace = namespace })
          assert.equals(#results, #diags, "All diagnostics should be available immediately")
          
          done()
        end)
      end)
    end, 3000) -- 3 second timeout for this async test
  end)
end)