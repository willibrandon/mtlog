-- Tests for analysis queue management - NO MOCKS, real async operations
local test_helpers = require('test_helpers')
local analyzer = require('mtlog.analyzer')

describe('mtlog queue', function()
  local queue
  local test_files = {}
  
  before_each(function()
    -- Ensure analyzer is available
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
    
    -- Clear module cache
    package.loaded['mtlog.queue'] = nil
    package.loaded['mtlog.analyzer'] = nil
    package.loaded['mtlog.config'] = nil
    
    -- Load modules
    queue = require('mtlog.queue')
    
    -- Setup queue
    queue.setup()
    
    -- Pause queue by default for most tests to prevent automatic processing
    queue.pause()
    
    -- Clear test files
    test_files = {}
  end)
  
  after_each(function()
    queue.reset()
    
    -- Clean up test files
    for _, filepath in ipairs(test_files) do
      test_helpers.delete_test_file(filepath)
    end
  end)
  
  describe('enqueue with real files', function()
    it('should add real analysis tasks to the queue', function()
      local test_file = test_helpers.create_test_go_file('queue_test1.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Queue test")
}
]])
      table.insert(test_files, test_file)
      
      local task_id = queue.enqueue(test_file, function() end)
      assert.is_not_nil(task_id)
      assert.equals(1, queue.get_queue_size())
    end)
    
    it('should respect priority ordering with real files', function()
      -- Create real test files
      local low_file = test_helpers.create_test_go_file('low_priority.go', [[
package main
func main() { println("low") }
]])
      local high_file = test_helpers.create_test_go_file('high_priority.go', [[
package main
func main() { println("high") }
]])
      local normal_file = test_helpers.create_test_go_file('normal_priority.go', [[
package main
func main() { println("normal") }
]])
      
      table.insert(test_files, low_file)
      table.insert(test_files, high_file)
      table.insert(test_files, normal_file)
      
      -- Add tasks with different priorities
      queue.enqueue(low_file, function() end, { priority = queue.priority.LOW })
      queue.enqueue(high_file, function() end, { priority = queue.priority.HIGH })
      queue.enqueue(normal_file, function() end, { priority = queue.priority.NORMAL })
      
      local entries = queue.get_queue()
      assert.equals(3, #entries)
      
      -- Check priority order (HIGH=1, NORMAL=2, LOW=3)
      assert.equals(queue.priority.HIGH, entries[1].priority)
      assert.equals(queue.priority.NORMAL, entries[2].priority)
      assert.equals(queue.priority.LOW, entries[3].priority)
    end)
    
    it('should not duplicate same real file unless forced', function()
      local test_file = test_helpers.create_test_go_file('no_duplicate.go', [[
package main
func main() { println("test") }
]])
      table.insert(test_files, test_file)
      
      queue.enqueue(test_file, function() end)
      queue.enqueue(test_file, function() end)  -- Should not add duplicate
      
      assert.equals(1, queue.get_queue_size())
      
      -- Force should allow duplicate
      queue.enqueue(test_file, function() end, { force = true })
      assert.equals(2, queue.get_queue_size())
    end)
    
    it('should update priority if higher priority task is added', function()
      local test_file = test_helpers.create_test_go_file('priority_update.go', [[
package main
func main() { println("priority test") }
]])
      table.insert(test_files, test_file)
      
      queue.enqueue(test_file, function() end, { priority = queue.priority.LOW })
      queue.enqueue(test_file, function() end, { priority = queue.priority.HIGH })
      
      local entries = queue.get_queue()
      assert.equals(1, #entries)  -- Still only one entry
      assert.equals(queue.priority.HIGH, entries[1].priority)  -- But with higher priority
    end)
  end)
  
  describe('cancellation with real tasks', function()
    it('should cancel specific real task', function()
      local test_file = test_helpers.create_test_go_file('cancel_test.go', [[
package main
func main() { println("cancel") }
]])
      table.insert(test_files, test_file)
      
      local task_id = queue.enqueue(test_file, function() end)
      assert.equals(1, queue.get_queue_size())
      
      local success = queue.cancel(task_id)
      assert.is_true(success)
      assert.equals(0, queue.get_queue_size())
      
      local stats = queue.get_stats()
      assert.equals(1, stats.cancelled)
    end)
    
    it('should cancel all tasks for a real file', function()
      local test_file = test_helpers.create_test_go_file('multi_cancel.go', [[
package main
func main() { println("multi") }
]])
      local other_file = test_helpers.create_test_go_file('other_cancel.go', [[
package main
func main() { println("other") }
]])
      
      table.insert(test_files, test_file)
      table.insert(test_files, other_file)
      
      queue.enqueue(test_file, function() end, { force = true })
      queue.enqueue(test_file, function() end, { force = true })
      queue.enqueue(other_file, function() end)
      
      assert.equals(3, queue.get_queue_size())
      
      local count = queue.cancel_file(test_file)
      assert.equals(2, count)
      assert.equals(1, queue.get_queue_size())
    end)
    
    it('should clear entire queue of real tasks', function()
      for i = 1, 3 do
        local file = test_helpers.create_test_go_file('clear_' .. i .. '.go', [[
package main
func main() { println("clear ]] .. i .. [[") }
]])
        table.insert(test_files, file)
        queue.enqueue(file, function() end)
      end
      
      assert.equals(3, queue.get_queue_size())
      
      queue.clear()
      assert.equals(0, queue.get_queue_size())
      
      local stats = queue.get_stats()
      assert.equals(3, stats.cancelled)
    end)
  end)
  
  describe('pause and resume with real analysis', function()
    it('should pause real queue processing', function()
      -- Already paused in before_each
      local stats = queue.get_stats()
      assert.is_true(stats.paused)
      
      -- Tasks should still be added but not processed
      local test_file = test_helpers.create_test_go_file('pause_test.go', [[
package main
func main() { println("paused") }
]])
      table.insert(test_files, test_file)
      
      queue.enqueue(test_file, function() end)
      assert.equals(1, queue.get_queue_size())
      assert.equals(0, queue.get_active_count())
    end)
    
    it('should resume real queue processing', function()
      local test_file = test_helpers.create_test_go_file('resume_test.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Resume test")
}
]])
      table.insert(test_files, test_file)
      
      local task_completed = false
      queue.enqueue(test_file, function()
        task_completed = true
      end)
      
      queue.resume()
      
      local stats = queue.get_stats()
      assert.is_false(stats.paused)
      
      -- Wait for real processing to complete
      local success = vim.wait(5000, function()
        return task_completed
      end, 50)
      
      assert.is_true(success, "Task should complete within timeout")
      assert.is_true(queue.get_stats().completed > 0)
    end)
  end)
  
  describe('statistics with real analysis', function()
    it('should track real queue statistics', function()
      local completed_count = 0
      
      -- Add some real tasks
      for i = 1, 2 do
        local file = test_helpers.create_test_go_file('stats_' .. i .. '.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Stats test ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        
        queue.enqueue(file, function() 
          completed_count = completed_count + 1
        end)
      end
      
      -- Resume queue to process tasks
      queue.resume()
      
      -- Wait for real completion
      local success = vim.wait(10000, function()
        return completed_count == 2
      end, 100)
      
      assert.is_true(success, "Should complete both tasks")
      local stats = queue.get_stats()
      assert.equals(2, stats.queued)
      assert.equals(2, stats.completed)
      assert.equals(0, stats.failed)
    end)
    
    it('should report max concurrent properly', function()
      local stats = queue.get_stats()
      assert.is_not_nil(stats.max_concurrent)
      assert.is_true(stats.max_concurrent >= 1)
    end)
  end)
  
  describe('concurrent processing with real analyzer', function()
    it('should process multiple real tasks concurrently', function()
      local completed_count = 0
      local max_active = 0
      
      -- Create multiple test files with real mtlog code
      local files = {}
      for i = 1, 5 do
        local file = test_helpers.create_test_go_file('concurrent_' .. i .. '.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Concurrent test ]] .. i .. [[")
    log.Debug("Processing file ]] .. i .. [[")
}
]])
        table.insert(test_files, file)
        table.insert(files, file)
      end
      
      -- Add all tasks
      for _, file in ipairs(files) do
        queue.enqueue(file, function() 
          completed_count = completed_count + 1
        end)
      end
      
      -- Resume queue to start processing
      queue.resume()
      
      -- Monitor active count while waiting
      local check_interval = 50
      local success = vim.wait(20000, function()
        local active = queue.get_active_count()
        if active > max_active then
          max_active = active
        end
        return completed_count == 5
      end, check_interval)
      
      assert.is_true(success, "Should complete all 5 tasks")
      
      -- Should have processed concurrently
      local stats = queue.get_stats()
      assert.is_true(max_active > 0, "Should have active tasks")
      assert.is_true(max_active <= stats.max_concurrent, "Should not exceed max concurrent")
      assert.equals(5, completed_count)
    end)
  end)
  
  describe('queue display with real data', function()
    it('should return real queue contents for display', function()
      local high_file = test_helpers.create_test_go_file('display_high.go', [[
package main
func main() { println("high priority") }
]])
      local low_file = test_helpers.create_test_go_file('display_low.go', [[
package main
func main() { println("low priority") }
]])
      
      table.insert(test_files, high_file)
      table.insert(test_files, low_file)
      
      queue.enqueue(high_file, function() end, { priority = queue.priority.HIGH })
      queue.enqueue(low_file, function() end, { priority = queue.priority.LOW })
      
      local entries = queue.get_queue()
      assert.equals(2, #entries)
      
      -- Check entry structure
      assert.is_not_nil(entries[1].id)
      assert.is_string(entries[1].filepath)
      assert.is_true(entries[1].filepath:match('display_high%.go') ~= nil)
      assert.is_not_nil(entries[1].priority)
      assert.is_not_nil(entries[1].status)
      assert.is_not_nil(entries[1].waiting)
      
      -- Second entry should be low priority file
      assert.is_true(entries[2].filepath:match('display_low%.go') ~= nil)
    end)
  end)
  
  describe('error handling with real analyzer', function()
    it('should handle analyzer errors gracefully', function()
      -- Create an invalid Go file that will cause analyzer errors
      local bad_file = test_helpers.create_test_go_file('invalid.go', [[
This is not valid Go code!
]])
      table.insert(test_files, bad_file)
      
      local error_caught = false
      queue.enqueue(bad_file, function(results, err)
        if err then
          error_caught = true
        end
      end)
      
      queue.resume()
      
      -- Wait for processing
      local success = vim.wait(5000, function()
        local stats = queue.get_stats()
        return stats.completed > 0 or stats.failed > 0
      end, 100)
      
      -- Should handle the error gracefully
      assert.is_true(success, "Should process the file even with errors")
    end)
  end)
end)
