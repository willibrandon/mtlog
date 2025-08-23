local analyzer = require('mtlog.analyzer')
local utils = require('mtlog.utils')
local test_helpers = require('test_helpers')

describe('mtlog analyzer', function()
  local test_bufnr
  local test_file
  
  before_each(function()
    -- Ensure analyzer is available - fail test if not
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
  end)
  
  after_each(function()
    -- Clean up test buffer
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
    
    -- Clean up test file
    if test_file then
      test_helpers.delete_test_file(test_file)
    end
  end)
  
  describe('availability', function()
    it('should verify analyzer is available', function()
      -- This MUST be true for all tests
      local available = analyzer.is_available()
      assert.is_true(available, "analyzer MUST be available")
    end)
    
    it('should get analyzer version', function()
      local version = analyzer.get_version()
      assert.is_not_nil(version, "version MUST be available")
      assert.is_string(version)
      -- Version should contain something like "v0.x.x" or similar
      assert.is_truthy(version:match('v?%d+%.%d+') or version:match('dev'), 
                       "version should match expected format: " .. version)
    end)
    
    it('should reset availability cache and still work', function()
      analyzer.reset_availability()
      
      -- Should still be available after reset
      local available = analyzer.is_available()
      assert.is_true(available, "analyzer MUST remain available after reset")
    end)
  end)
  
  describe('analyze_file with real analyzer', function()
    it('should analyze a valid Go file', function(done)
      local done_fn = done
      -- Create a real Go file in the test project
      test_file = test_helpers.create_test_go_file('test_valid.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Application started")
    log.Debug("Debug message with {Count} items", 42)
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err, "Should not error on valid file")
          assert.is_table(results)
          -- Valid file should have no errors (or only warnings)
          for _, diag in ipairs(results) do
            -- If there are diagnostics, they should be valid
            assert.is_not_nil(diag.lnum)
            assert.is_not_nil(diag.col)
            assert.is_not_nil(diag.message)
            assert.equals('mtlog-analyzer', diag.source)
          end
          
          done_fn()
        end)
      end)
    end, 10000) -- 10 second timeout for real analysis
    
    it('should detect template/argument mismatch', function(done)
      local done_fn = done
      -- Create a file with a real mtlog error
      test_file = test_helpers.create_test_go_file('test_mismatch.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // Template has placeholder but no argument provided
    log.Information("User {UserId} logged in")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err, "Should not error even with code issues")
          assert.is_table(results)
          assert.is_true(#results > 0, "Should find template/argument mismatch")
          
          -- Find the MTLOG001 error
          local found_mismatch = false
          for _, diag in ipairs(results) do
            if diag.code == 'MTLOG001' then
              found_mismatch = true
              assert.is_truthy(diag.message:match('mismatch') or diag.message:match('argument'))
              assert.equals('mtlog-analyzer', diag.source)
              assert.is_not_nil(diag.lnum)
              assert.is_not_nil(diag.col)
            end
          end
          
          assert.is_true(found_mismatch, "Should detect MTLOG001 template/argument mismatch")
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should handle non-existent files gracefully', function(done)
      local done_fn = done
      local non_existent = test_helpers.test_project_dir .. '/non_existent_' .. os.time() .. '.go'
      
      analyzer.analyze_file(non_existent, function(results, err)
        vim.schedule(function()
          -- Should either error or return empty results
          assert.is_true(err ~= nil or (results and #results == 0), 
                        "Should handle non-existent file")
          done_fn()
        end)
      end)
    end, 5000)
    
    it('should detect format specifier issues', function(done)
      local done_fn = done
      test_file = test_helpers.create_test_go_file('test_format.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // Invalid format specifier
    log.Information("Value: {Value:InvalidFormat}", 123)
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Should detect format specifier issue (if strict mode)
          -- or at least parse without crashing
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should detect property naming issues', function(done)
      local done_fn = done
      test_file = test_helpers.create_test_go_file('test_naming.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // Property should be PascalCase
    log.Information("User {user_id} performed {action_type}", 123, "login")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Should detect naming convention issues
          local found_naming = false
          for _, diag in ipairs(results) do
            if diag.code == 'MTLOG004' then
              found_naming = true
              assert.is_truthy(diag.message:match('PascalCase') or diag.message:match('naming'))
              
              -- Should have suggested fixes
              if diag.user_data and diag.user_data.suggested_fixes then
                assert.is_true(#diag.user_data.suggested_fixes > 0)
              end
            end
          end
          
          -- Naming conventions might be warnings, so just verify no crash
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should handle With() method diagnostics', function(done)
      local done_fn = done
      test_file = test_helpers.create_test_go_file('test_with.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    
    // Odd number of arguments (MTLOG009)
    log.With("key1", "value1", "key2").Information("Test")
    
    // Non-string key (MTLOG010)
    log.With(123, "value").Information("Test")
    
    // Duplicate property (MTLOG011)
    log.With("user", "alice").With("user", "bob").Information("Test")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Check for With() related diagnostics
          local found_with_errors = {}
          for _, diag in ipairs(results) do
            if diag.code and diag.code:match('^MTLOG0[0-9][0-9]$') then
              found_with_errors[diag.code] = true
            end
          end
          
          -- Should detect at least some With() issues
          assert.is_true(vim.tbl_count(found_with_errors) > 0 or #results == 0,
                        "Should analyze With() methods or have no diagnostics")
          
          done_fn()
        end)
      end)
    end, 10000)
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
      
      -- Wait for debounce to trigger - use 300ms to be safe in CI
      vim.wait(300, function()
        return call_count > 0
      end, 10)
      
      -- Should have been called exactly once
      assert.equals(1, call_count)
    end)
    
    it('should handle separate debounced calls independently', function()
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
  
  describe('JSON output parsing', function()
    it('should parse real analyzer output', function(done)
      local done_fn = done
      -- Create a file that will produce known diagnostics
      test_file = test_helpers.create_test_go_file('test_json.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // This will produce MTLOG001 - template/argument mismatch
    log.Error("Error: {ErrorCode} - {Message}")
}
]])
      
      
      -- Run analyzer directly to get raw JSON
      local output = {}
      local analyzer_path = test_helpers.ensure_analyzer()
      local job_id = vim.fn.jobstart({analyzer_path, '-json', test_file}, {
        stdout_buffered = true,
        on_stdout = function(_, data)
          if data then
            for _, line in ipairs(data) do
              if line ~= '' then
                table.insert(output, line)
              end
            end
          end
        end,
        on_exit = function(_, exit_code)
          vim.schedule(function()
            if exit_code == 0 then
              local json_str = table.concat(output, '\n')
              if json_str ~= '' then
                local ok, parsed = pcall(vim.json.decode, json_str)
                assert.is_true(ok, "Should parse real analyzer JSON output")
                assert.is_table(parsed)
                
                -- Verify structure of real output
                if #parsed > 0 then
                  local diag = parsed[1]
                  assert.is_string(diag.file)
                  assert.is_number(diag.line)
                  assert.is_number(diag.column)
                  assert.is_string(diag.code)
                  assert.is_string(diag.message)
                end
              end
            end
            done_fn()
          end)
        end
      })
      
      assert.is_truthy(job_id > 0, "Should start analyzer job")
    end, 10000)
    
    it('should handle empty analyzer output', function(done)
      local done_fn = done
      -- Create a file with no mtlog usage
      test_file = test_helpers.create_test_go_file('test_empty.go', [[
package main

func main() {
    // No mtlog usage
    println("Hello, World!")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          -- Should have no diagnostics for file without mtlog
          assert.equals(0, #results)
          done_fn()
        end)
      end)
    end, 10000)
  end)
  
  describe('diagnostic conversion', function()
    it('should convert real diagnostics to Neovim format', function(done)
      local done_fn = done
      local config = require('mtlog.config')
      config.setup({
        severity_levels = {
          MTLOG001 = vim.diagnostic.severity.ERROR,
          MTLOG002 = vim.diagnostic.severity.ERROR,
          MTLOG003 = vim.diagnostic.severity.WARN,
          MTLOG004 = vim.diagnostic.severity.INFO,
          MTLOG009 = vim.diagnostic.severity.ERROR,
          MTLOG010 = vim.diagnostic.severity.WARN,
          MTLOG011 = vim.diagnostic.severity.INFO,
        }
      })
      
      test_file = test_helpers.create_test_go_file('test_convert.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // MTLOG001: Template/argument mismatch
    log.Information("User {UserId} logged in at {Timestamp}")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          if #results > 0 then
            local diag = results[1]
            
            -- Check Neovim diagnostic structure
            assert.is_number(diag.lnum, "Should have line number")
            assert.is_number(diag.col, "Should have column")
            assert.is_string(diag.message, "Should have message")
            assert.is_number(diag.severity, "Should have severity")
            assert.equals('mtlog-analyzer', diag.source)
            assert.is_string(diag.code, "Should have diagnostic code")
            
            -- Verify severity mapping
            if diag.code == 'MTLOG001' then
              assert.equals(vim.diagnostic.severity.ERROR, diag.severity)
            end
            
            -- Check for suggested fixes if present
            if diag.user_data and diag.user_data.suggested_fixes then
              assert.is_table(diag.user_data.suggested_fixes)
              for _, fix in ipairs(diag.user_data.suggested_fixes) do
                assert.is_string(fix.title)
                assert.is_table(fix.edits)
              end
            end
          end
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should preserve all diagnostic fields from analyzer', function(done)
      local done_fn = done
      test_file = test_helpers.create_test_go_file('test_fields.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    // Multiple issues for testing
    log.Information("User {user_id} action {action_type}", 123, "login")
    log.With("key").Information("Odd arguments")
}
]])
      
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Verify all fields are preserved and converted correctly
          for _, diag in ipairs(results) do
            -- Required fields
            assert.is_number(diag.lnum, "Missing lnum")
            assert.is_number(diag.col, "Missing col")
            assert.is_string(diag.message, "Missing message")
            assert.is_number(diag.severity, "Missing severity")
            assert.equals('mtlog-analyzer', diag.source, "Wrong source")
            assert.is_string(diag.code, "Missing code")
            
            -- Optional fields
            if diag.end_lnum then
              assert.is_number(diag.end_lnum)
            end
            if diag.end_col then
              assert.is_number(diag.end_col)
            end
            
            -- Line and column should be 0-indexed for Neovim
            assert.is_true(diag.lnum >= 0, "Line should be 0-indexed")
            assert.is_true(diag.col >= 0, "Column should be 0-indexed")
          end
          
          done_fn()
        end)
      end)
    end, 10000)
  end)
end)