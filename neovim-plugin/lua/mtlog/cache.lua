-- Cache module for mtlog.nvim

local M = {}

local config = require('mtlog.config')
local utils = require('mtlog.utils')

-- Cache storage
local cache = {}

-- Cache statistics
local stats = {
  hits = 0,
  misses = 0,
  size = 0,
}

-- LRU tracking
local lru_order = {} -- List of keys in LRU order (most recent at end)

-- Analyzer version for cache invalidation
local analyzer_version = nil

-- Cache entry structure
---@class CacheEntry
---@field data any Cached data
---@field mtime number File modification time
---@field version string Analyzer version
---@field timestamp number Cache timestamp

-- Get cache key for file
---@param filepath string File path
---@return string Cache key
local function get_cache_key(filepath)
  return vim.fn.fnamemodify(filepath, ':p')
end

-- Update LRU order for a key
---@param key string Cache key
local function update_lru(key)
  -- Remove from current position if exists
  for i, k in ipairs(lru_order) do
    if k == key then
      table.remove(lru_order, i)
      break
    end
  end
  -- Add to end (most recent)
  table.insert(lru_order, key)
end

-- Evict LRU entries if max size exceeded
local function enforce_max_size()
  local max_size = config.get('cache.max_size')
  if not max_size or max_size <= 0 then
    return
  end
  
  while #lru_order > max_size do
    local oldest_key = table.remove(lru_order, 1)
    cache[oldest_key] = nil
    stats.size = stats.size - 1
  end
end

-- Get current analyzer version
---@return string Version string
local function get_analyzer_version()
  if analyzer_version then
    return analyzer_version
  end
  
  -- Get version from analyzer module
  local analyzer = require('mtlog.analyzer')
  analyzer_version = analyzer.get_version() or 'unknown'
  
  return analyzer_version
end

-- Check if cache entry is valid
---@param entry CacheEntry Cache entry
---@param filepath string File path
---@return boolean Is valid
local function is_cache_valid(entry, filepath)
  -- Check if cache is enabled
  if not config.get('cache.enabled') then
    return false
  end
  
  -- Check analyzer version
  if entry.version ~= get_analyzer_version() then
    return false
  end
  
  -- Check file modification time
  local current_mtime = utils.get_mtime(filepath)
  if not current_mtime or current_mtime ~= entry.mtime then
    return false
  end
  
  -- Check TTL
  local ttl = config.get('cache.ttl_seconds')
  if ttl and ttl > 0 then
    local age = os.time() - entry.timestamp
    if age > ttl then
      return false
    end
  end
  
  return true
end

-- Get cached data
---@param filepath string File path
---@return any? Cached data or nil
function M.get(filepath)
  if not config.get('cache.enabled') then
    return nil
  end
  
  local key = get_cache_key(filepath)
  local entry = cache[key]
  
  if not entry then
    stats.misses = stats.misses + 1
    return nil
  end
  
  -- Validate cache entry
  if not is_cache_valid(entry, filepath) then
    cache[key] = nil
    stats.size = stats.size - 1
    stats.misses = stats.misses + 1
    -- Remove from LRU
    for i, k in ipairs(lru_order) do
      if k == key then
        table.remove(lru_order, i)
        break
      end
    end
    return nil
  end
  
  stats.hits = stats.hits + 1
  update_lru(key)
  return entry.data
end

-- Set cached data
---@param filepath string File path
---@param data any Data to cache
function M.set(filepath, data)
  if not config.get('cache.enabled') then
    return
  end
  
  local key = get_cache_key(filepath)
  local mtime = utils.get_mtime(filepath)
  
  if not mtime then
    return
  end
  
  -- Check if updating existing entry
  local is_new = cache[key] == nil
  
  cache[key] = {
    data = data,
    mtime = mtime,
    version = get_analyzer_version(),
    timestamp = os.time(),
  }
  
  if is_new then
    stats.size = stats.size + 1
  end
  
  update_lru(key)
  enforce_max_size()
end

-- Invalidate cache for file
---@param filepath string File path
function M.invalidate(filepath)
  local key = get_cache_key(filepath)
  cache[key] = nil
end

-- Clear entire cache
function M.clear()
  cache = {}
  lru_order = {}
  stats.size = 0
  -- Don't reset hits/misses unless explicitly asked
end

-- Clear expired entries
function M.cleanup()
  local ttl = config.get('cache.ttl_seconds')
  if not ttl or ttl <= 0 then
    return
  end
  
  local now = os.time()
  local expired = {}
  
  for key, entry in pairs(cache) do
    if (now - entry.timestamp) > ttl then
      table.insert(expired, key)
    end
  end
  
  for _, key in ipairs(expired) do
    cache[key] = nil
    stats.size = stats.size - 1
    -- Remove from LRU order
    for i, k in ipairs(lru_order) do
      if k == key then
        table.remove(lru_order, i)
        break
      end
    end
  end
end

-- Get cache statistics
---@return table Statistics
function M.stats()
  return {
    size = stats.size,
    hits = stats.hits,
    misses = stats.misses,
    hit_rate = stats.hits > 0 and (stats.hits / (stats.hits + stats.misses)) or 0,
    entries = stats.size,  -- Alias for compatibility
  }
end

-- Reset cache statistics
function M.reset_stats()
  stats.hits = 0
  stats.misses = 0
  -- Don't reset size as it's tracked automatically
end

-- Invalidate cache on analyzer version change
function M.invalidate_on_version_change()
  local new_version = get_analyzer_version()
  
  if analyzer_version and analyzer_version ~= new_version then
    -- Version changed, clear cache
    M.clear()
  end
  
  analyzer_version = new_version
end

-- Save cache to disk (for persistence across sessions)
---@param filepath string Path to cache file
---@return boolean Success
function M.save_to_disk(filepath)
  -- Create cache directory
  local cache_dir = vim.fn.fnamemodify(filepath, ':h')
  if not utils.ensure_directory(cache_dir) then
    return false
  end
  
  -- Serialize cache
  local data = vim.json.encode(cache)
  if not data then
    return false
  end
  
  -- Write to file
  return utils.write_file(filepath, data)
end

-- Load cache from disk
---@param filepath string Path to cache file
---@return boolean Success
function M.load_from_disk(filepath)
  -- Read file
  local data = utils.read_file(filepath)
  if not data then
    return false
  end
  
  -- Deserialize cache
  local ok, loaded = pcall(vim.json.decode, data)
  if not ok or type(loaded) ~= 'table' then
    return false
  end
  
  -- Validate and load entries
  for key, entry in pairs(loaded) do
    if type(entry) == 'table' and entry.data and entry.mtime and entry.version and entry.timestamp then
      cache[key] = entry
    end
  end
  
  -- Clean up expired entries
  M.cleanup()
  
  return true
end

-- Get cache file path
---@return string Cache file path
function M.get_cache_file()
  local cache_dir = vim.fn.stdpath('cache')
  return cache_dir .. '/mtlog-analyzer-cache.json'
end

-- Setup automatic cache management
function M.setup()
  -- Load cache from disk on startup
  local cache_file = M.get_cache_file()
  if vim.fn.filereadable(cache_file) == 1 then
    M.load_from_disk(cache_file)
  end
  
  -- Set up periodic cleanup
  local cleanup_timer = vim.loop.new_timer()
  cleanup_timer:start(60000, 60000, vim.schedule_wrap(function()
    M.cleanup()
  end))
  
  -- Save cache on exit
  vim.api.nvim_create_autocmd('VimLeavePre', {
    callback = function()
      M.save_to_disk(cache_file)
    end,
  })
  
  -- Invalidate cache on analyzer update
  vim.api.nvim_create_autocmd('User', {
    pattern = 'MtlogAnalyzerUpdated',
    callback = function()
      M.invalidate_on_version_change()
    end,
  })
end

return M