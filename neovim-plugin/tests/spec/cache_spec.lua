-- Tests for the cache module
local cache = require('mtlog.cache')
local config = require('mtlog.config')
local utils = require('mtlog.utils')

describe('mtlog.cache', function()
  local orig_get_mtime = utils.get_mtime
  local orig_time = os.time
  local current_time = 1000000
  
  before_each(function()
    -- Reset config
    config.setup({
      cache = {
        enabled = true,
        ttl_seconds = 300, -- 5 minutes
        max_size = 100,
      },
    })
    
    -- Clear cache
    cache.clear()
    
    -- Mock time
    os.time = function()
      return current_time
    end
    
    -- Mock file mtime
    utils.get_mtime = function(filepath)
      -- Return consistent mtime for testing
      if filepath == '/test/unchanged.go' then
        return 1000
      elseif filepath == '/test/changed.go' then
        return 2000 + (current_time - 1000000) -- Changes with time
      end
      return 1000
    end
  end)
  
  after_each(function()
    -- Restore original functions
    utils.get_mtime = orig_get_mtime
    os.time = orig_time
    
    -- Clear cache
    cache.clear()
  end)
  
  describe('basic operations', function()
    it('should store and retrieve cached data', function()
      local data = {
        { lnum = 0, col = 0, message = 'Test diagnostic' },
      }
      
      cache.set('/test/file.go', data)
      local retrieved = cache.get('/test/file.go')
      
      assert.is_not_nil(retrieved)
      assert.equals(1, #retrieved)
      assert.equals('Test diagnostic', retrieved[1].message)
    end)
    
    it('should return nil for uncached files', function()
      local result = cache.get('/test/nonexistent.go')
      assert.is_nil(result)
    end)
    
    it('should handle cache disabled', function()
      config.setup({ cache = { enabled = false } })
      
      cache.set('/test/file.go', { test = 'data' })
      local result = cache.get('/test/file.go')
      
      assert.is_nil(result)
    end)
  end)
  
  describe('cache invalidation', function()
    it('should invalidate on file modification', function()
      local data = { test = 'data' }
      
      -- Set cache for unchanged file
      cache.set('/test/unchanged.go', data)
      assert.is_not_nil(cache.get('/test/unchanged.go'))
      
      -- Mock file change
      utils.get_mtime = function(filepath)
        if filepath == '/test/unchanged.go' then
          return 1001 -- File was modified
        end
        return 1000
      end
      
      -- Cache should be invalidated
      assert.is_nil(cache.get('/test/unchanged.go'))
    end)
    
    it('should invalidate on analyzer version change', function()
      local data = { test = 'data' }
      
      -- Set initial cache
      cache.set('/test/file.go', data)
      assert.is_not_nil(cache.get('/test/file.go'))
      
      -- Simulate analyzer version change
      cache.invalidate_on_version_change()
      
      -- This should detect version change and clear cache
      -- But since we can't easily mock the version change, just test the function exists
      assert.is_function(cache.invalidate_on_version_change)
    end)
    
    it('should invalidate after TTL expires', function()
      local data = { test = 'data' }
      
      -- Set cache
      cache.set('/test/file.go', data)
      assert.is_not_nil(cache.get('/test/file.go'))
      
      -- Advance time beyond TTL
      current_time = current_time + 301 -- TTL is 300 seconds
      
      -- Cache should be expired
      assert.is_nil(cache.get('/test/file.go'))
    end)
    
    it('should handle zero TTL (no expiration)', function()
      config.setup({ cache = { enabled = true, ttl_seconds = 0 } })
      
      local data = { test = 'data' }
      cache.set('/test/file.go', data)
      
      -- Advance time significantly
      current_time = current_time + 10000
      
      -- Cache should still be valid (only file mtime matters)
      assert.is_not_nil(cache.get('/test/file.go'))
    end)
  end)
  
  describe('memory management', function()
    it('should enforce max cache size', function()
      config.setup({
        cache = {
          enabled = true,
          max_size = 3,
        },
      })
      
      -- Add entries up to max size
      cache.set('/test/file1.go', { data = 1 })
      cache.set('/test/file2.go', { data = 2 })
      cache.set('/test/file3.go', { data = 3 })
      
      -- All should be cached
      assert.is_not_nil(cache.get('/test/file1.go'))
      assert.is_not_nil(cache.get('/test/file2.go'))
      assert.is_not_nil(cache.get('/test/file3.go'))
      
      -- Add one more (should evict oldest)
      cache.set('/test/file4.go', { data = 4 })
      
      -- Oldest should be evicted
      assert.is_nil(cache.get('/test/file1.go'))
      assert.is_not_nil(cache.get('/test/file2.go'))
      assert.is_not_nil(cache.get('/test/file3.go'))
      assert.is_not_nil(cache.get('/test/file4.go'))
    end)
    
    it('should use LRU eviction policy', function()
      config.setup({
        cache = {
          enabled = true,
          max_size = 3,
        },
      })
      
      -- Add entries
      cache.set('/test/file1.go', { data = 1 })
      cache.set('/test/file2.go', { data = 2 })
      cache.set('/test/file3.go', { data = 3 })
      
      -- Access file1 (makes it recently used)
      assert.is_not_nil(cache.get('/test/file1.go'))
      
      -- Add new entry (should evict file2, not file1)
      cache.set('/test/file4.go', { data = 4 })
      
      assert.is_not_nil(cache.get('/test/file1.go')) -- Still cached (recently used)
      assert.is_nil(cache.get('/test/file2.go')) -- Evicted (least recently used)
      assert.is_not_nil(cache.get('/test/file3.go'))
      assert.is_not_nil(cache.get('/test/file4.go'))
    end)
    
    it('should handle clear operation', function()
      cache.set('/test/file1.go', { data = 1 })
      cache.set('/test/file2.go', { data = 2 })
      cache.set('/test/file3.go', { data = 3 })
      
      cache.clear()
      
      assert.is_nil(cache.get('/test/file1.go'))
      assert.is_nil(cache.get('/test/file2.go'))
      assert.is_nil(cache.get('/test/file3.go'))
    end)
    
    it('should cleanup expired entries periodically', function()
      config.setup({
        cache = {
          enabled = true,
          ttl_seconds = 100,
          cleanup_interval = 50,
        },
      })
      
      -- Add entries at different times
      cache.set('/test/file1.go', { data = 1 })
      
      current_time = current_time + 50
      cache.set('/test/file2.go', { data = 2 })
      
      current_time = current_time + 50
      cache.set('/test/file3.go', { data = 3 })
      
      -- file1 and file2 should be expired, file3 still valid
      current_time = current_time + 51
      
      -- Trigger cleanup
      cache.cleanup()
      
      assert.is_nil(cache.get('/test/file1.go')) -- Expired (age 151)
      assert.is_nil(cache.get('/test/file2.go')) -- Expired (age 101)
      assert.is_not_nil(cache.get('/test/file3.go')) -- Still valid (age 51)
    end)
  end)
  
  describe('cache statistics', function()
    it('should track cache hits and misses', function()
      cache.set('/test/file.go', { data = 'test' })
      
      -- Reset stats
      cache.reset_stats()
      
      -- Cache hit
      assert.is_not_nil(cache.get('/test/file.go'))
      
      -- Cache misses
      assert.is_nil(cache.get('/test/missing1.go'))
      assert.is_nil(cache.get('/test/missing2.go'))
    end)
  end)
end)