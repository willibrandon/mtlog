-- Tests for diagnostic suppression functionality - NO MOCKS, real environment
local test_helpers = require('test_helpers')
local analyzer = require('mtlog.analyzer')

describe('mtlog suppression', function()
  local mtlog
  local config
  local diagnostics
  local test_files = {}
  
  before_each(function()
    -- Ensure analyzer is available
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
    
    -- Clear any module cache
    package.loaded['mtlog'] = nil
    package.loaded['mtlog.config'] = nil
    package.loaded['mtlog.analyzer'] = nil
    package.loaded['mtlog.diagnostics'] = nil
    
    mtlog = require('mtlog')
    config = require('mtlog.config')
    diagnostics = require('mtlog.diagnostics')
    
    -- Setup with default config
    mtlog.setup({
      diagnostics_enabled = true,
      suppressed_diagnostics = {},
    })
    
    -- Clear test files
    test_files = {}
    
    -- Clear any existing MTLOG_SUPPRESS environment variable
    test_helpers.clear_env('MTLOG_SUPPRESS')
  end)
  
  after_each(function()
    -- Clean up test files
    for _, filepath in ipairs(test_files) do
      test_helpers.delete_test_file(filepath)
    end
    
    -- Clear environment
    test_helpers.clear_env('MTLOG_SUPPRESS')
  end)
  
  describe('configuration', function()
    it('should initialize with empty suppressed diagnostics', function()
      local suppressed = config.get('suppressed_diagnostics')
      assert.is_not_nil(suppressed)
      assert.equals(0, #suppressed)
    end)
    
    it('should allow setting suppressed diagnostics', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.equals('MTLOG001', suppressed[1])
      assert.equals('MTLOG004', suppressed[2])
    end)
  end)
  
  describe('kill switch with real diagnostics', function()
    it('should be enabled by default', function()
      assert.is_true(config.get('diagnostics_enabled'))
    end)
    
    it('should toggle diagnostics on/off', function()
      -- Initially enabled
      assert.is_true(config.get('diagnostics_enabled'))
      
      -- Toggle off
      mtlog.toggle_diagnostics()
      assert.is_false(config.get('diagnostics_enabled'))
      
      -- Toggle on
      mtlog.toggle_diagnostics()
      assert.is_true(config.get('diagnostics_enabled'))
    end)
    
    it('should clear real diagnostics when kill switch is activated', function(done)
      local done_fn = done
      -- Create a real test file with issues
      local test_file = test_helpers.create_test_go_file('kill_switch.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Error("Error {Code}")  // Missing argument - will produce MTLOG001
}
]])
      table.insert(test_files, test_file)
      
      local bufnr = vim.fn.bufadd(test_file)
      vim.fn.bufload(bufnr)
      
      -- Analyze to get real diagnostics
      analyzer.analyze_file(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          assert.is_true(#results > 0, "Should have diagnostics")
          
          -- Set diagnostics
          diagnostics.set(bufnr, results)
          
          -- Verify diagnostics are set
          local diags = vim.diagnostic.get(bufnr, { namespace = diagnostics.ns })
          assert.is_true(#diags > 0, "Should have diagnostics set")
          
          -- Disable diagnostics (kill switch)
          config.set('diagnostics_enabled', false)
          mtlog.analyze_buffer(bufnr)
          
          -- Should have no diagnostics
          vim.wait(100, function() return false end)
          diags = vim.diagnostic.get(bufnr, { namespace = diagnostics.ns })
          assert.equals(0, #diags, "Diagnostics should be cleared")
          
          -- Clean up
          vim.api.nvim_buf_delete(bufnr, {force = true})
          done_fn()
        end)
      end)
    end, 10000)
  end)
  
  describe('suppress_diagnostic', function()
    it('should add diagnostic to suppressed list', function()
      mtlog.suppress_diagnostic('MTLOG001', true)  -- Skip prompt in tests
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(1, #suppressed)
      assert.equals('MTLOG001', suppressed[1])
    end)
    
    it('should not duplicate already suppressed diagnostics', function()
      mtlog.suppress_diagnostic('MTLOG001', true)
      mtlog.suppress_diagnostic('MTLOG001', true)
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(1, #suppressed)
    end)
    
    it('should handle multiple suppressions', function()
      mtlog.suppress_diagnostic('MTLOG001', true)
      mtlog.suppress_diagnostic('MTLOG004', true)
      mtlog.suppress_diagnostic('MTLOG006', true)
      
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(3, #suppressed)
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG001'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG004'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG006'))
    end)
  end)
  
  describe('unsuppress_diagnostic', function()
    before_each(function()
      -- Start with some suppressed diagnostics
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004', 'MTLOG006'})
    end)
    
    it('should remove diagnostic from suppressed list', function()
      mtlog.unsuppress_diagnostic('MTLOG004')
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.is_false(vim.tbl_contains(suppressed, 'MTLOG004'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG001'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG006'))
    end)
    
    it('should handle unsuppressing non-suppressed diagnostic', function()
      mtlog.unsuppress_diagnostic('MTLOG999')
      local suppressed = config.get('suppressed_diagnostics')
      -- Should not change the list
      assert.equals(3, #suppressed)
    end)
  end)
  
  describe('unsuppress_all', function()
    it('should clear all suppressions', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004', 'MTLOG006'})
      assert.equals(3, #config.get('suppressed_diagnostics'))
      
      mtlog.unsuppress_all()
      
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(0, #suppressed)
    end)
  end)
  
  describe('show_suppressions', function()
    it('should display suppressed diagnostics', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      
      -- Capture notifications
      local notifications = {}
      local orig_notify = vim.notify
      vim.notify = function(msg, level)
        table.insert(notifications, {msg = msg, level = level})
      end
      
      mtlog.show_suppressions()
      
      -- Restore original notify
      vim.notify = orig_notify
      
      -- Check that suppressions were shown
      assert.is_true(#notifications > 0)
      local found = false
      for _, notif in ipairs(notifications) do
        if notif.msg:match('MTLOG001') and notif.msg:match('MTLOG004') then
          found = true
          break
        end
      end
      assert.is_true(found)
    end)
    
    it('should show message when no suppressions', function()
      config.set('suppressed_diagnostics', {})
      
      -- Capture notifications
      local notifications = {}
      local orig_notify = vim.notify
      vim.notify = function(msg, level)
        table.insert(notifications, {msg = msg, level = level})
      end
      
      mtlog.show_suppressions()
      
      -- Restore original notify
      vim.notify = orig_notify
      
      -- Check message
      assert.is_true(#notifications > 0)
      local found = false
      for _, notif in ipairs(notifications) do
        if notif.msg:match('No diagnostics are currently suppressed') then
          found = true
          break
        end
      end
      assert.is_true(found)
    end)
  end)
  
  describe('analyzer integration with real environment', function()
    it('should pass suppressions via real MTLOG_SUPPRESS environment variable', function(done)
      local done_fn = done
      -- Set suppressions
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      
      -- Create a test file that would normally trigger MTLOG001
      local test_file = test_helpers.create_test_go_file('suppress_test.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("User {UserId} logged in")  // Missing argument - MTLOG001
    log.Debug("Property {user_name} test", "test")  // Non-PascalCase - MTLOG004
}
]])
      table.insert(test_files, test_file)
      
      -- Set real environment variable
      test_helpers.set_env('MTLOG_SUPPRESS', 'MTLOG001,MTLOG004')
      
      -- Run real analyzer with suppression
      test_helpers.run_analyzer(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Should not contain suppressed diagnostics
          local found_mtlog001 = false
          local found_mtlog004 = false
          
          for _, diag in ipairs(results) do
            if diag.code == 'MTLOG001' then
              found_mtlog001 = true
            elseif diag.code == 'MTLOG004' then
              found_mtlog004 = true
            end
          end
          
          assert.is_false(found_mtlog001, "MTLOG001 should be suppressed")
          assert.is_false(found_mtlog004, "MTLOG004 should be suppressed")
          
          done_fn()
        end)
      end)
    end, 10000)
    
    it('should run analyzer without suppression when list is empty', function(done)
      local done_fn = done
      -- No suppressions
      config.set('suppressed_diagnostics', {})
      
      -- Create a test file with issues
      local test_file = test_helpers.create_test_go_file('no_suppress.go', [[
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("User {UserId} logged in")  // Missing argument - MTLOG001
}
]])
      table.insert(test_files, test_file)
      
      -- Ensure no suppression environment variable
      test_helpers.clear_env('MTLOG_SUPPRESS')
      
      -- Run real analyzer without suppression
      test_helpers.run_analyzer(test_file, function(results, err)
        vim.schedule(function()
          assert.is_nil(err)
          assert.is_table(results)
          
          -- Should contain MTLOG001
          local found_mtlog001 = false
          for _, diag in ipairs(results) do
            if diag.code == 'MTLOG001' then
              found_mtlog001 = true
              break
            end
          end
          
          assert.is_true(found_mtlog001, "MTLOG001 should NOT be suppressed")
          
          done_fn()
        end)
      end)
    end, 10000)
  end)
  
  describe('workspace configuration with real files', function()
    local workspace
    local test_config_file
    
    before_each(function()
      -- Reload workspace module to reset state
      package.loaded['mtlog.workspace'] = nil
      workspace = require('mtlog.workspace')
      
      -- Use real config file in test project
      test_config_file = test_helpers.test_project_dir .. '/.mtlog.json'
    end)
    
    after_each(function()
      -- Clean up test file
      if test_config_file then
        os.remove(test_config_file)
      end
      -- Reset config
      config.set('suppressed_diagnostics', {})
    end)
    
    it('should save suppressions to real workspace config file', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      
      -- Use real config path
      workspace.get_config_path = function()
        return test_config_file
      end
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Read the real file
      local file = io.open(test_config_file, 'r')
      assert.is_not_nil(file, "Config file should exist")
      local content = file:read('*a')
      file:close()
      
      -- Parse JSON
      local ok, data = pcall(vim.json.decode, content)
      assert.is_true(ok, "Should be valid JSON")
      assert.is_not_nil(data.suppressed_diagnostics)
      assert.equals(2, #data.suppressed_diagnostics)
      assert.equals('MTLOG001', data.suppressed_diagnostics[1])
      assert.equals('MTLOG004', data.suppressed_diagnostics[2])
    end)
    
    it('should load suppressions from real workspace config file', function()
      -- Create real config file
      local test_data = {
        suppressed_diagnostics = {'MTLOG002', 'MTLOG005'}
      }
      local file = io.open(test_config_file, 'w')
      assert.is_not_nil(file, "Should be able to create config file")
      file:write(vim.json.encode(test_data))
      file:close()
      
      -- Verify file exists
      local verify_file = io.open(test_config_file, 'r')
      assert.is_not_nil(verify_file, "Config file should exist")
      verify_file:close()
      
      -- Use real config path
      workspace.find_config_file = function()
        return test_config_file
      end
      
      -- Prevent reanalysis during test
      local orig_reanalyze = mtlog.reanalyze_all
      mtlog.reanalyze_all = function()
        -- Do nothing in tests
      end
      
      -- Load suppressions
      workspace.load_suppressions()
      
      -- Restore original
      mtlog.reanalyze_all = orig_reanalyze
      
      -- Check that suppressions were loaded
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.equals('MTLOG002', suppressed[1])
      assert.equals('MTLOG005', suppressed[2])
    end)
    
    it('should handle missing config file gracefully', function()
      -- Ensure file doesn't exist
      os.remove(test_config_file)
      
      workspace.find_config_file = function()
        return nil
      end
      
      -- Should not error
      assert.has_no_errors(function()
        workspace.load_suppressions()
      end)
      
      -- Suppressions should be empty
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(0, #suppressed)
    end)
  end)
end)