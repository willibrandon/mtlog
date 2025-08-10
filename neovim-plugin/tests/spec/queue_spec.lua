-- Tests for analysis queue management

describe('mtlog queue', function()
  local queue
  local analyzer
  
  before_each(function()
    -- Clear module cache
    package.loaded['mtlog.queue'] = nil
    package.loaded['mtlog.analyzer'] = nil
    package.loaded['mtlog.config'] = nil
    
    -- Load modules
    queue = require('mtlog.queue')
    analyzer = require('mtlog.analyzer')
    
    -- Setup queue
    queue.setup()
    
    -- Pause queue by default for most tests to prevent automatic processing
    queue.pause()
    
    -- Mock analyzer.analyze_file to be instant
    analyzer.analyze_file = function(filepath, callback, bufnr)
      -- Simulate some processing time
      vim.defer_fn(function()
        callback({}, nil)  -- Return empty results
      end, 10)
    end
  end)
  
  after_each(function()
    queue.reset()
  end)
  
  describe('enqueue', function()
    it('should add tasks to the queue', function()
      local task_id = queue.enqueue('/tmp/test1.go', function() end)
      assert.is_not_nil(task_id)
      assert.are.equal(1, queue.get_queue_size())
    end)
    
    it('should respect priority ordering', function()
      -- Add tasks with different priorities
      queue.enqueue('/tmp/low.go', function() end, { priority = queue.priority.LOW })
      queue.enqueue('/tmp/high.go', function() end, { priority = queue.priority.HIGH })
      queue.enqueue('/tmp/normal.go', function() end, { priority = queue.priority.NORMAL })
      
      local entries = queue.get_queue()
      assert.are.equal(3, #entries)
      
      -- Check priority order (HIGH=1, NORMAL=2, LOW=3)
      assert.are.equal(queue.priority.HIGH, entries[1].priority)
      assert.are.equal(queue.priority.NORMAL, entries[2].priority)
      assert.are.equal(queue.priority.LOW, entries[3].priority)
    end)
    
    it('should not duplicate same file unless forced', function()
      queue.enqueue('/tmp/test.go', function() end)
      queue.enqueue('/tmp/test.go', function() end)  -- Should not add duplicate
      
      assert.are.equal(1, queue.get_queue_size())
      
      -- Force should allow duplicate
      queue.enqueue('/tmp/test.go', function() end, { force = true })
      assert.are.equal(2, queue.get_queue_size())
    end)
    
    it('should update priority if higher priority task is added', function()
      queue.enqueue('/tmp/test.go', function() end, { priority = queue.priority.LOW })
      queue.enqueue('/tmp/test.go', function() end, { priority = queue.priority.HIGH })
      
      local entries = queue.get_queue()
      assert.are.equal(1, #entries)  -- Still only one entry
      assert.are.equal(queue.priority.HIGH, entries[1].priority)  -- But with higher priority
    end)
  end)
  
  describe('cancellation', function()
    it('should cancel specific task', function()
      local task_id = queue.enqueue('/tmp/test.go', function() end)
      assert.are.equal(1, queue.get_queue_size())
      
      local success = queue.cancel(task_id)
      assert.is_true(success)
      assert.are.equal(0, queue.get_queue_size())
      
      local stats = queue.get_stats()
      assert.are.equal(1, stats.cancelled)
    end)
    
    it('should cancel all tasks for a file', function()
      queue.enqueue('/tmp/test.go', function() end, { force = true })
      queue.enqueue('/tmp/test.go', function() end, { force = true })
      queue.enqueue('/tmp/other.go', function() end)
      
      assert.are.equal(3, queue.get_queue_size())
      
      local count = queue.cancel_file('/tmp/test.go')
      assert.are.equal(2, count)
      assert.are.equal(1, queue.get_queue_size())
    end)
    
    it('should clear entire queue', function()
      queue.enqueue('/tmp/test1.go', function() end)
      queue.enqueue('/tmp/test2.go', function() end)
      queue.enqueue('/tmp/test3.go', function() end)
      
      assert.are.equal(3, queue.get_queue_size())
      
      queue.clear()
      assert.are.equal(0, queue.get_queue_size())
      
      local stats = queue.get_stats()
      assert.are.equal(3, stats.cancelled)
    end)
  end)
  
  describe('pause and resume', function()
    it('should pause queue processing', function()
      -- Already paused in before_each
      local stats = queue.get_stats()
      assert.is_true(stats.paused)
      
      -- Tasks should still be added but not processed
      queue.enqueue('/tmp/test.go', function() end)
      assert.are.equal(1, queue.get_queue_size())
      assert.are.equal(0, queue.get_active_count())
    end)
    
    it('should resume queue processing', function()
      -- Queue is already paused from before_each
      queue.enqueue('/tmp/test.go', function() end)
      
      queue.resume()
      
      local stats = queue.get_stats()
      assert.is_false(stats.paused)
      
      -- Give time for processing to start
      vim.wait(50)
      
      -- Task should be processing or completed
      assert.is_true(queue.get_active_count() > 0 or queue.get_stats().completed > 0)
    end)
  end)
  
  describe('statistics', function()
    it('should track queue statistics', function()
      local completed_count = 0
      
      -- Add some tasks
      queue.enqueue('/tmp/test1.go', function() 
        completed_count = completed_count + 1
      end)
      queue.enqueue('/tmp/test2.go', function() 
        completed_count = completed_count + 1
      end)
      
      -- Resume queue to process tasks
      queue.resume()
      
      -- Wait for completion
      vim.wait(100, function()
        return completed_count == 2
      end)
      
      local stats = queue.get_stats()
      assert.are.equal(2, stats.queued)
      assert.are.equal(2, stats.completed)
      assert.are.equal(0, stats.failed)
    end)
    
    it('should report max concurrent properly', function()
      local stats = queue.get_stats()
      assert.is_not_nil(stats.max_concurrent)
      assert.is_true(stats.max_concurrent >= 1)
    end)
  end)
  
  describe('concurrent processing', function()
    it('should process multiple tasks concurrently', function()
      local processing_count = 0
      local completed_count = 0
      
      -- Mock analyzer with longer delay
      analyzer.analyze_file = function(filepath, callback, bufnr)
        processing_count = processing_count + 1
        vim.defer_fn(function()
          processing_count = processing_count - 1
          completed_count = completed_count + 1
          callback({}, nil)
        end, 50)
      end
      
      -- Add multiple tasks
      for i = 1, 5 do
        queue.enqueue('/tmp/test' .. i .. '.go', function() end)
      end
      
      -- Resume queue to start processing
      queue.resume()
      
      -- Wait a bit for processing to start
      vim.wait(20)
      
      -- Should have multiple tasks processing (up to max_concurrent)
      local stats = queue.get_stats()
      local active = queue.get_active_count()
      assert.is_true(active > 0)
      assert.is_true(active <= stats.max_concurrent)
      
      -- Wait for all to complete
      vim.wait(200, function()
        return completed_count == 5
      end)
      
      assert.are.equal(5, completed_count)
    end)
  end)
  
  describe('queue display', function()
    it('should return queue contents for display', function()
      queue.enqueue('/tmp/test1.go', function() end, { priority = queue.priority.HIGH })
      queue.enqueue('/tmp/test2.go', function() end, { priority = queue.priority.LOW })
      
      local entries = queue.get_queue()
      assert.are.equal(2, #entries)
      
      -- Check entry structure
      assert.is_not_nil(entries[1].id)
      assert.is_not_nil(entries[1].filepath)
      assert.is_not_nil(entries[1].priority)
      assert.is_not_nil(entries[1].status)
      assert.is_not_nil(entries[1].waiting)
    end)
  end)
end)