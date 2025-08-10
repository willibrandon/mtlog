-- Analysis queue manager for mtlog.nvim
-- Provides controlled concurrency and priority-based queue management

local M = {}

local config = require('mtlog.config')
local utils = require('mtlog.utils')

-- Queue state
local queue = {}
local active_analyses = {}
local max_concurrent = nil
local queue_paused = false
local stats = {
  queued = 0,
  processing = 0,
  completed = 0,
  failed = 0,
  cancelled = 0,
}

-- Priority levels
M.priority = {
  HIGH = 1,    -- Current buffer changes
  NORMAL = 2,  -- Visible buffers
  LOW = 3,     -- Background/workspace analysis
}

-- Initialize the queue system
function M.setup()
  -- Determine max concurrent analyses
  max_concurrent = config.get('analyzer.max_concurrent')
  if not max_concurrent then
    -- Default to CPU count - 1, minimum 1
    local cpu_count = tonumber(vim.fn.system('nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4'))
    max_concurrent = math.max(1, (cpu_count or 4) - 1)
  end
  
  -- Reset stats
  stats = {
    queued = 0,
    processing = 0,
    completed = 0,
    failed = 0,
    cancelled = 0,
  }
end

-- Add an analysis task to the queue
---@param filepath string Path to file to analyze
---@param callback function Callback with (results, error)
---@param options table? Options: priority, bufnr, force
---@return string Task ID
function M.enqueue(filepath, callback, options)
  options = options or {}
  local priority = options.priority or M.priority.NORMAL
  local bufnr = options.bufnr
  local force = options.force or false
  
  -- Generate unique task ID
  local task_id = string.format('%s_%d_%d', filepath, os.time(), math.random(1000))
  
  -- Check if file is already queued (unless forced)
  if not force then
    for _, task in ipairs(queue) do
      if task.filepath == filepath and task.status == 'pending' then
        -- Update priority if higher
        if priority < task.priority then
          task.priority = priority
          M._sort_queue()
        end
        -- Return existing task ID
        return task.id
      end
    end
  end
  
  -- Create task
  local task = {
    id = task_id,
    filepath = filepath,
    callback = callback,
    priority = priority,
    bufnr = bufnr,
    status = 'pending',
    enqueued_at = vim.loop.now(),
    started_at = nil,
    completed_at = nil,
    error = nil,
  }
  
  -- Add to queue
  table.insert(queue, task)
  stats.queued = stats.queued + 1
  
  -- Sort by priority
  M._sort_queue()
  
  -- Process queue
  M._process_queue()
  
  -- Notify if configured
  if config.get('analyzer.show_progress') then
    local queue_size = M.get_queue_size()
    if queue_size > 5 then
      vim.notify(string.format('Analysis queued (%d pending)', queue_size), vim.log.levels.INFO)
    end
  end
  
  return task_id
end

-- Cancel a specific task
---@param task_id string Task ID to cancel
---@return boolean Success
function M.cancel(task_id)
  -- Check queue
  for i, task in ipairs(queue) do
    if task.id == task_id then
      if task.status == 'pending' then
        table.remove(queue, i)
        stats.cancelled = stats.cancelled + 1
        return true
      elseif task.status == 'processing' then
        -- Mark for cancellation
        task.cancel_requested = true
        return true
      end
    end
  end
  return false
end

-- Cancel all tasks for a specific file
---@param filepath string File path
---@return number Count of cancelled tasks
function M.cancel_file(filepath)
  local count = 0
  
  -- Remove from queue
  for i = #queue, 1, -1 do
    if queue[i].filepath == filepath then
      if queue[i].status == 'pending' then
        table.remove(queue, i)
        stats.cancelled = stats.cancelled + 1
        count = count + 1
      elseif queue[i].status == 'processing' then
        queue[i].cancel_requested = true
        count = count + 1
      end
    end
  end
  
  return count
end

-- Clear the entire queue
function M.clear()
  -- Cancel pending tasks
  local pending_count = 0
  for _, task in ipairs(queue) do
    if task.status == 'pending' then
      pending_count = pending_count + 1
      stats.cancelled = stats.cancelled + 1
    elseif task.status == 'processing' then
      task.cancel_requested = true
    end
  end
  
  -- Clear queue
  queue = {}
  
  if pending_count > 0 then
    vim.notify(string.format('Cleared %d pending analyses', pending_count), vim.log.levels.INFO)
  end
end

-- Pause queue processing
function M.pause()
  queue_paused = true
  vim.notify('Analysis queue paused', vim.log.levels.INFO)
end

-- Resume queue processing
function M.resume()
  queue_paused = false
  vim.notify('Analysis queue resumed', vim.log.levels.INFO)
  M._process_queue()
end

-- Get queue statistics
---@return table Statistics
function M.get_stats()
  return {
    queued = stats.queued,
    processing = stats.processing,
    completed = stats.completed,
    failed = stats.failed,
    cancelled = stats.cancelled,
    pending = M.get_queue_size(),
    active = M.get_active_count(),
    max_concurrent = max_concurrent,
    paused = queue_paused,
  }
end

-- Get pending queue size
---@return number Count
function M.get_queue_size()
  local count = 0
  for _, task in ipairs(queue) do
    if task.status == 'pending' then
      count = count + 1
    end
  end
  return count
end

-- Get active analysis count
---@return number Count
function M.get_active_count()
  local count = 0
  for _, task in ipairs(queue) do
    if task.status == 'processing' then
      count = count + 1
    end
  end
  return count
end

-- Get queue contents for display
---@return table[] Queue entries
function M.get_queue()
  local entries = {}
  for _, task in ipairs(queue) do
    if task.status == 'pending' or task.status == 'processing' then
      table.insert(entries, {
        id = task.id,
        filepath = task.filepath,
        priority = task.priority,
        status = task.status,
        elapsed = task.started_at and (vim.loop.now() - task.started_at) or nil,
        waiting = task.enqueued_at and (vim.loop.now() - task.enqueued_at) or nil,
      })
    end
  end
  return entries
end

-- Private: Sort queue by priority
function M._sort_queue()
  table.sort(queue, function(a, b)
    -- Only sort pending tasks
    if a.status ~= 'pending' or b.status ~= 'pending' then
      return false
    end
    -- Sort by priority (lower number = higher priority)
    if a.priority ~= b.priority then
      return a.priority < b.priority
    end
    -- Then by enqueue time (FIFO within same priority)
    return a.enqueued_at < b.enqueued_at
  end)
end

-- Private: Process the queue
function M._process_queue()
  if queue_paused then
    return
  end
  
  -- Count active analyses
  local active_count = M.get_active_count()
  
  -- Process tasks up to max concurrent
  while active_count < max_concurrent do
    -- Find next pending task
    local next_task = nil
    for _, task in ipairs(queue) do
      if task.status == 'pending' then
        next_task = task
        break
      end
    end
    
    if not next_task then
      break  -- No more pending tasks
    end
    
    -- Start processing
    M._process_task(next_task)
    active_count = active_count + 1
  end
end

-- Private: Process a single task
---@param task table Task to process
function M._process_task(task)
  -- Mark as processing
  task.status = 'processing'
  task.started_at = vim.loop.now()
  stats.processing = stats.processing + 1
  
  -- Get the analyzer module
  local analyzer = require('mtlog.analyzer')
  
  -- Show progress if configured
  if config.get('analyzer.show_progress') then
    local active = M.get_active_count()
    local pending = M.get_queue_size()
    if active > 1 or pending > 0 then
      vim.notify(string.format('Analyzing %s (%d active, %d pending)', 
        vim.fn.fnamemodify(task.filepath, ':t'),
        active, pending), vim.log.levels.INFO)
    end
  end
  
  -- Run analysis
  analyzer.analyze_file(task.filepath, function(results, err)
    -- Check if cancelled
    if task.cancel_requested then
      stats.cancelled = stats.cancelled + 1
    else
      -- Update stats
      task.completed_at = vim.loop.now()
      if err then
        task.status = 'failed'
        task.error = err
        stats.failed = stats.failed + 1
      else
        task.status = 'completed'
        stats.completed = stats.completed + 1
      end
      
      -- Call original callback
      if task.callback then
        task.callback(results, err)
      end
    end
    
    -- Remove completed task after a delay
    vim.defer_fn(function()
      for i, t in ipairs(queue) do
        if t.id == task.id then
          table.remove(queue, i)
          break
        end
      end
    end, 5000)  -- Keep completed tasks for 5 seconds for debugging
    
    -- Update processing count
    stats.processing = math.max(0, stats.processing - 1)
    
    -- Process next in queue
    M._process_queue()
  end, task.bufnr)
end

-- Reset queue (for testing)
function M.reset()
  queue = {}
  active_analyses = {}
  queue_paused = false
  stats = {
    queued = 0,
    processing = 0,
    completed = 0,
    failed = 0,
    cancelled = 0,
  }
end

return M