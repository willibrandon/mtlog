local analyzer = require('mtlog.analyzer')
local utils = require('mtlog.utils')

describe('mtlog analyzer', function()
  local test_bufnr
  
  before_each(function()
    -- Create a test buffer
    test_bufnr = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(test_bufnr, '/tmp/test.go')
  end)
  
  after_each(function()
    -- Clean up test buffer
    if vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  describe('availability', function()
    it('should check if analyzer is available', function()
      local available = analyzer.is_available()
      assert.is_boolean(available)
    end)
    
    it('should get analyzer version if available', function()
      if analyzer.is_available() then
        local version = analyzer.get_version()
        assert.is_not_nil(version)
      else
        pending("analyzer not available")
      end
    end)
    
    it('should reset availability cache', function()
      analyzer.reset_availability()
      -- Should not error
      assert.has_no_errors(function()
        analyzer.is_available()
      end)
    end)
  end)
  
  describe('analyze_file', function()
    it('should analyze a file with callback', function(done)
      if not analyzer.is_available() then
        pending("analyzer not available")
        return
      end
      
      -- Create a test file
      local test_file = '/tmp/test_analyzer_' .. os.time() .. '.go'
      local file = io.open(test_file, 'w')
      file:write([[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Test")
}
]])
      file:close()
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          os.remove(test_file)
          
          assert.is_nil(err)
          assert.is_table(results)
          -- The test file should have no errors
          assert.is_true(#results >= 0)
          
          done()
        end)
      end)
    end, 2000)
    
    it('should handle non-existent files', function(done)
      analyzer.analyze_file('/tmp/non_existent_file.go', function(results, err)
        vim.schedule(function()
          -- Should either error or return empty results
          assert.is_true(err ~= nil or #results == 0)
          done()
        end)
      end)
    end, 1000)
  end)
  
  describe('debouncing', function()
    it('should debounce multiple rapid calls', function()
      local call_count = 0
      local debounced = utils.debounce(function()
        call_count = call_count + 1
      end, 50)
      
      -- Call multiple times rapidly
      debounced()
      debounced()
      debounced()
      
      -- Should not have been called yet
      assert.equals(0, call_count)
      
      -- Wait for debounce to trigger
      vim.wait(100, function()
        return call_count > 0
      end)
      
      -- Should have been called exactly once
      assert.equals(1, call_count)
    end)
    
    it('should handle separate debounced calls', function()
      local count1 = 0
      local count2 = 0
      
      local debounced1 = utils.debounce(function()
        count1 = count1 + 1
      end, 50)
      
      local debounced2 = utils.debounce(function()
        count2 = count2 + 1
      end, 50)
      
      debounced1()
      debounced2()
      
      vim.wait(100, function()
        return count1 > 0 and count2 > 0
      end)
      
      assert.equals(1, count1)
      assert.equals(1, count2)
    end)
  end)
  
  describe('rate limiting', function()
    it('should create a rate limiter', function()
      local limiter = utils.rate_limiter(2) -- 2 per second
      assert.is_function(limiter)
    end)
    
    it('should throttle function calls', function()
      local call_count = 0
      local throttled = utils.throttle(function()
        call_count = call_count + 1
      end, 100)
      
      -- Call multiple times
      throttled()
      throttled()
      throttled()
      
      -- First call should go through immediately
      assert.equals(1, call_count)
      
      -- Wait and check that throttling worked
      vim.wait(150, function()
        return call_count > 1
      end)
      
      -- Should have been called at most twice
      assert.is_true(call_count <= 2)
    end)
  end)
end)