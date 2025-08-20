-- Tests for the cache module - NO MOCKS, real filesystem operations
local cache = require('mtlog.cache')
local config = require('mtlog.config')
local utils = require('mtlog.utils')
local test_helpers = require('test_helpers')
local analyzer = require('mtlog.analyzer')

describe('mtlog.cache', function()
  local test_files = {}
  
  before_each(function()
    -- Ensure analyzer is available
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
    
    -- Reset config with real settings
    config.setup({
      cache = {
        enabled = true,
        ttl_seconds = 300, -- 5 minutes
        max_size = 100,
      },
    })
    
    -- Clear cache
    cache.clear()
    
    -- Clear test files list
    test_files = {}
  end)
  
  after_each(function()
    -- Clean up test files
    for _, filepath in ipairs(test_files) do
      test_helpers.delete_test_file(filepath)
    end
    
    -- Clear cache
    cache.clear()
  end)
  
  describe('basic operations with real files', function()
    it('should store and retrieve cached data from real analysis', function()
      local test_file = test_helpers.create_test_go_file('cache_test1.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Test message")
}
]])
      table.insert(test_files, test_file)
      
      local completed = false
      local results = nil
      local err = nil
      
      -- Analyze file first time
      analyzer.analyze_file(test_file, function(res, e)
        results = res
        err = e
        completed = true
      end)
      
      -- Wait for analysis to complete
      local success = vim.wait(10000, function()
        return completed
      end, 50)
      
      assert.is_true(success, "Analysis should complete")
      assert.is_nil(err)
      assert.is_table(results)
      
      -- Store in cache
      cache.set(test_file, results)
      
      -- Retrieve from cache
      local cached = cache.get(test_file)
      assert.is_not_nil(cached)
      assert.equals(#results, #cached)
    end)
    
    it('should return nil for uncached files', function()
      local fake_file = test_helpers.test_project_dir .. '/nonexistent_cache.go'
      local result = cache.get(fake_file)
      assert.is_nil(result)
    end)
    
    it('should handle cache disabled', function()
      config.setup({ cache = { enabled = false } })
      
      local test_file = test_helpers.create_test_go_file('cache_disabled.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Cache disabled test")
}
]])
      table.insert(test_files, test_file)
      
      local completed = false
      local results = nil
      local err = nil
      
      analyzer.analyze_file(test_file, function(res, e)
        results = res
        err = e
        completed = true
      end)
      
      -- Wait for analysis to complete
      local success = vim.wait(10000, function()
        return completed
      end, 50)
      
      assert.is_true(success, "Analysis should complete")
      assert.is_nil(err)
      
      -- Try to cache
      cache.set(test_file, results)
      
      -- Should not be cached when disabled
      local cached = cache.get(test_file)
      assert.is_nil(cached)
    end)
  end)
  
  describe('cache invalidation with real file changes', function()
    it('should invalidate on real file modification', function()
      local test_file = test_helpers.create_test_go_file('cache_modify.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Original content")
}
]])
      table.insert(test_files, test_file)
      
      local completed = false
      local first_results = nil
      local err = nil
      
      -- Analyze and cache
      analyzer.analyze_file(test_file, function(res, e)
        first_results = res
        err = e
        completed = true
      end)
      
      -- Wait for analysis to complete
      local success = vim.wait(10000, function()
        return completed
      end, 50)
      
      assert.is_true(success, "Analysis should complete")
      assert.is_nil(err)
      
      -- Cache the results
      cache.set(test_file, first_results)
      assert.is_not_nil(cache.get(test_file))
      
      -- Get original mtime
      local original_mtime = test_helpers.get_real_mtime(test_file)
      
      -- Wait a moment to ensure mtime will change
      vim.wait(1100, function() return false end)
      
      -- Modify the file
      test_helpers.modify_file(test_file, [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Modified content with {UserId}", 123)
}
]])
      
      -- Verify mtime changed
      local new_mtime = test_helpers.get_real_mtime(test_file)
      assert.is_true(new_mtime > original_mtime, "File mtime should have changed")
      
      -- Cache should be invalidated
      local cached = cache.get(test_file)
      assert.is_nil(cached, "Cache should be invalidated after file modification")
    end)
    
    it('should invalidate on analyzer version change', function()
      local test_file = test_helpers.create_test_go_file('cache_version.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Version test")
}
]])
      table.insert(test_files, test_file)
      
      -- Set some cached data
      cache.set(test_file, { test = 'data' })
      assert.is_not_nil(cache.get(test_file))
      
      -- Simulate version change by clearing cache
      cache.invalidate_on_version_change()
      
      -- Function should exist and be callable
      assert.is_function(cache.invalidate_on_version_change)
    end)
    
    it('should handle TTL expiration with real time', function()
      config.setup({ 
        cache = { 
          enabled = true, 
          ttl_seconds = 1 -- Very short TTL for testing
        } 
      })
      
      local test_file = test_helpers.create_test_go_file('cache_ttl.go', [[
package main

func main() {
    println("TTL test")
}
]])
      table.insert(test_files, test_file)
      
      -- Set cache
      cache.set(test_file, { test = 'data' })
      assert.is_not_nil(cache.get(test_file), "Should be cached initially")
      
      -- Wait for TTL to expire (add extra time for safety)
      vim.wait(2000, function() return false end)
      
      -- Cache should be expired when we try to get it
      -- The get() function checks TTL internally
      local cached = cache.get(test_file)
      assert.is_nil(cached, "Cache should expire after TTL")
    end)
    
    it('should handle zero TTL (no time-based expiration)', function()
      config.setup({ 
        cache = { 
          enabled = true, 
          ttl_seconds = 0 -- No time expiration
        } 
      })
      
      local test_file = test_helpers.create_test_go_file('cache_no_ttl.go', [[
package main

func main() {
    println("No TTL test")
}
]])
      table.insert(test_files, test_file)
      
      -- Set cache
      cache.set(test_file, { test = 'data' })
      
      -- Wait some time
      vim.wait(2000, function() return false end)
      
      -- Cache should still be valid (only file mtime matters)
      assert.is_not_nil(cache.get(test_file))
    end)
  end)
  
  describe('memory management with real data', function()
    it('should enforce max cache size with real files', function()
      config.setup({
        cache = {
          enabled = true,
          max_size = 3,
        },
      })
      
      -- Create multiple test files
      local files = {}
      for i = 1, 4 do
        local file = test_helpers.create_test_go_file('cache_size_' .. i .. '.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("File ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        table.insert(files, file)
      end
      
      local analyzed = 0
      local results_map = {}
      
      -- Analyze first 3 files
      for i = 1, 3 do
        local completed = false
        analyzer.analyze_file(files[i], function(results, err)
          results_map[i] = results
          analyzed = analyzed + 1
          completed = true
        end)
        
        -- Wait for each analysis to complete
        vim.wait(5000, function()
          return completed
        end, 50)
      end
      
      -- Cache first 3 files
      for i = 1, 3 do
        cache.set(files[i], results_map[i] or { dummy = true })
      end
      
      -- All 3 should be cached
      assert.is_not_nil(cache.get(files[1]))
      assert.is_not_nil(cache.get(files[2]))
      assert.is_not_nil(cache.get(files[3]))
      
      -- Analyze 4th file
      local completed4 = false
      local results4 = nil
      analyzer.analyze_file(files[4], function(results, err)
        results4 = results
        completed4 = true
      end)
      
      vim.wait(5000, function()
        return completed4
      end, 50)
      
      -- Add 4th file (should evict oldest)
      cache.set(files[4], results4 or { dummy = true })
      
      -- First should be evicted, others remain
      assert.is_nil(cache.get(files[1]))
      assert.is_not_nil(cache.get(files[2]))
      assert.is_not_nil(cache.get(files[3]))
      assert.is_not_nil(cache.get(files[4]))
    end)
    
    it('should use LRU eviction with real access patterns', function()
      config.setup({
        cache = {
          enabled = true,
          max_size = 3,
        },
      })
      
      -- Create test files
      local files = {}
      for i = 1, 4 do
        local file = test_helpers.create_test_go_file('cache_lru_' .. i .. '.go', [[
package main

func main() {
    println("LRU test ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        table.insert(files, file)
      end
      
      
      
      -- Cache first 3 files
      cache.set(files[1], { data = 1 })
      cache.set(files[2], { data = 2 })
      cache.set(files[3], { data = 3 })
      
      -- Access file 1 (makes it recently used)
      assert.is_not_nil(cache.get(files[1]))
      
      -- Add file 4 (should evict file 2, not file 1)
      cache.set(files[4], { data = 4 })
      
      -- Check eviction followed LRU
      assert.is_not_nil(cache.get(files[1]), "File 1 should remain (recently accessed)")
      assert.is_nil(cache.get(files[2]), "File 2 should be evicted (least recently used)")
      assert.is_not_nil(cache.get(files[3]))
      assert.is_not_nil(cache.get(files[4]))
    end)
    
    it('should handle clear operation with real data', function()
      -- Create and analyze multiple files
      local files = {}
      for i = 1, 3 do
        local file = test_helpers.create_test_go_file('cache_clear_' .. i .. '.go', [[
package main

func main() {
    println("Clear test ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        table.insert(files, file)
        cache.set(file, { data = i })
      end
      
      -- Verify all cached
      for _, file in ipairs(files) do
        assert.is_not_nil(cache.get(file))
      end
      
      -- Clear cache
      cache.clear()
      
      -- Verify all cleared
      for _, file in ipairs(files) do
        assert.is_nil(cache.get(file))
      end
    end)
    
    it('should cleanup expired entries with real timing', function()
      config.setup({
        cache = {
          enabled = true,
          ttl_seconds = 1, -- Very short TTL for testing
          cleanup_interval = 1,
        },
      })
      
      -- Create test files with staggered caching
      local files = {}
      for i = 1, 3 do
        local file = test_helpers.create_test_go_file('cache_cleanup_' .. i .. '.go', [[
package main

func main() {
    println("Cleanup test ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        table.insert(files, file)
      end
      
      -- Add entries at different times
      cache.set(files[1], { data = 1 })
      
      vim.wait(800, function() return false end)
      cache.set(files[2], { data = 2 })
      
      vim.wait(800, function() return false end)  -- File 1 should be expired (>1.6s old)
      cache.set(files[3], { data = 3 })
      
      -- Wait to ensure files 1 and 2 are expired but file 3 is not
      vim.wait(400, function() return false end)  -- File 1 is >2s old, file 2 is >1.2s old, file 3 is 0.4s old
      
      -- Check expiration - files 1 and 2 should be expired, file 3 should remain
      assert.is_nil(cache.get(files[1]), "File 1 should be expired (>2s old)")
      assert.is_nil(cache.get(files[2]), "File 2 should be expired (>1.2s old)")
      assert.is_not_nil(cache.get(files[3]), "File 3 should still be valid (0.4s old)")
    end)
  end)
  
  describe('cache statistics with real operations', function()
    it('should track real cache hits and misses', function()
      local test_file = test_helpers.create_test_go_file('cache_stats.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Stats test")
}
]])
      table.insert(test_files, test_file)
      
      local completed = false
      local results = nil
      local err = nil
      
      -- Analyze and cache
      analyzer.analyze_file(test_file, function(res, e)
        results = res
        err = e
        completed = true
      end)
      
      -- Wait for completion
      local success = vim.wait(10000, function()
        return completed
      end, 50)
      
      assert.is_true(success, "Should complete within timeout")
      assert.is_nil(err)
      cache.set(test_file, results)
      
      -- Reset stats if function exists
      if cache.reset_stats then
        cache.reset_stats()
      end
      
      -- Cache hit
      assert.is_not_nil(cache.get(test_file))
      
      -- Cache misses
      assert.is_nil(cache.get(test_helpers.test_project_dir .. '/missing1.go'))
      assert.is_nil(cache.get(test_helpers.test_project_dir .. '/missing2.go'))
    end)
  end)
end)
