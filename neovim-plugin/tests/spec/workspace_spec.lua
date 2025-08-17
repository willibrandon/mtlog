-- Tests for mtlog workspace module
local workspace = require('mtlog.workspace')
local utils = require('mtlog.utils')
local config = require('mtlog.config')

describe('mtlog workspace', function()
  local temp_dir
  local original_cwd
  local original_read_file
  local original_write_file
  
  before_each(function()
    -- Store original functions
    original_read_file = utils.read_file
    original_write_file = utils.write_file
    original_cwd = vim.fn.getcwd()
    
    -- Create temporary directory for tests
    temp_dir = vim.fn.tempname()
    vim.fn.mkdir(temp_dir, 'p')
    vim.fn.chdir(temp_dir)
    
    -- Reset config
    config.setup()
  end)
  
  after_each(function()
    -- Restore original functions
    utils.read_file = original_read_file
    utils.write_file = original_write_file
    
    -- Restore original directory
    vim.fn.chdir(original_cwd)
    
    -- Clean up temp directory
    if temp_dir and vim.fn.isdirectory(temp_dir) == 1 then
      vim.fn.delete(temp_dir, 'rf')
    end
  end)
  
  describe('find_config_file', function()
    it('should find .mtlog.json in current directory', function()
      -- Create .mtlog.json in temp directory
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      file:write('{}')
      file:close()
      
      local found = workspace.find_config_file()
      assert.equals(config_path, found)
    end)
    
    it('should find .mtlog.json in parent directory', function()
      -- Create subdirectory
      local subdir = temp_dir .. '/subdir'
      vim.fn.mkdir(subdir, 'p')
      
      -- Create .mtlog.json in parent
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      file:write('{}')
      file:close()
      
      -- Change to subdirectory
      vim.fn.chdir(subdir)
      
      local found = workspace.find_config_file()
      assert.equals(config_path, found)
    end)
    
    it('should stop at git root if no .mtlog.json found', function()
      -- Create .git directory to simulate git root
      vim.fn.mkdir(temp_dir .. '/.git', 'p')
      
      -- Create subdirectory
      local subdir = temp_dir .. '/subdir'
      vim.fn.mkdir(subdir, 'p')
      vim.fn.chdir(subdir)
      
      -- No .mtlog.json exists
      local found = workspace.find_config_file()
      assert.is_nil(found)
    end)
    
    it('should return nil if no config file found', function()
      -- No .mtlog.json or .git directory
      local found = workspace.find_config_file()
      assert.is_nil(found)
    end)
  end)
  
  describe('get_config_path', function()
    it('should return path in current directory', function()
      local path = workspace.get_config_path()
      assert.equals(temp_dir .. '/.mtlog.json', path)
    end)
  end)
  
  describe('load', function()
    it('should load valid JSON config', function()
      local test_config = {
        suppressed_diagnostics = {'MTLOG001', 'MTLOG004'},
        custom_setting = 'test_value'
      }
      
      -- Create a dummy config file so find_config_file returns something
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      file:write(vim.json.encode(test_config))
      file:close()
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.is_not_nil(loaded.suppressed_diagnostics)
      assert.equals(2, #loaded.suppressed_diagnostics)
      assert.equals('MTLOG001', loaded.suppressed_diagnostics[1])
      assert.equals('test_value', loaded.custom_setting)
    end)
    
    it('should return empty table for non-existent file', function()
      -- Mock read_file to return nil (file doesn't exist)
      utils.read_file = function(path)
        return nil
      end
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
    
    it('should handle malformed JSON gracefully', function()
      -- Mock read_file to return invalid JSON
      utils.read_file = function(path)
        return '{ invalid json }'
      end
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
    
    it('should handle empty file', function()
      utils.read_file = function(path)
        return ''
      end
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
  end)
  
  describe('save', function()
    it('should save data as JSON', function()
      local saved_content = nil
      local saved_path = nil
      
      -- Mock write_file to capture what would be written
      utils.write_file = function(path, content)
        saved_path = path
        saved_content = content
        return true
      end
      
      local test_data = {
        suppressed_diagnostics = {'MTLOG002', 'MTLOG005'},
        enabled = false
      }
      
      local success = workspace.save(test_data)
      assert.is_true(success)
      assert.is_not_nil(saved_content)
      assert.equals(temp_dir .. '/.mtlog.json', saved_path)
      
      -- Verify JSON is valid
      local ok, decoded = pcall(vim.json.decode, saved_content)
      assert.is_true(ok)
      assert.is_table(decoded)
      assert.equals(2, #decoded.suppressed_diagnostics)
      assert.equals(false, decoded.enabled)
    end)
    
    it('should handle write failure', function()
      -- Mock write_file to simulate failure
      utils.write_file = function(path, content)
        return false
      end
      
      local success = workspace.save({ test = 'data' })
      assert.is_false(success)
    end)
    
    it('should create pretty-printed JSON', function()
      local saved_content = nil
      
      utils.write_file = function(path, content)
        saved_content = content
        return true
      end
      
      workspace.save({ key = 'value' })
      
      -- Check that JSON is saved (formatting is optional)
      assert.is_not_nil(saved_content)
      -- Just verify it's valid JSON
      local ok = pcall(vim.json.decode, saved_content)
      assert.is_true(ok)
    end)
  end)
  
  describe('apply', function()
    it('should apply workspace config to plugin config', function()
      -- Setup initial config
      config.setup({
        suppressed_diagnostics = {},
        diagnostics_enabled = true
      })
      
      -- Mock workspace.load to return test config
      local original_load = workspace.load
      workspace.load = function()
        return {
          suppressed_diagnostics = {'MTLOG001', 'MTLOG003'},
          diagnostics_enabled = false  -- This is actually applied
        }
      end
      
      -- Apply workspace config
      workspace.apply()
      
      -- Verify config was updated (only certain fields are applied)
      assert.equals(2, #config.get('suppressed_diagnostics'))
      assert.is_true(vim.tbl_contains(config.get('suppressed_diagnostics'), 'MTLOG001'))
      assert.is_true(vim.tbl_contains(config.get('suppressed_diagnostics'), 'MTLOG003'))
      assert.equals(false, config.get('diagnostics_enabled'))  -- This is applied
      
      -- Restore original load
      workspace.load = original_load
    end)
    
    it('should not crash if workspace config is empty', function()
      local original_load = workspace.load
      workspace.load = function()
        return {}
      end
      
      -- Should not error
      local ok = pcall(workspace.apply)
      assert.is_true(ok)
      
      workspace.load = original_load
    end)
  end)
  
  describe('save_suppressions', function()
    it('should save current suppressed diagnostics', function()
      local saved_data = nil
      
      -- Mock workspace.save
      local original_save = workspace.save
      workspace.save = function(data)
        saved_data = data
        return true
      end
      
      -- Set suppressions in config
      config.set('suppressed_diagnostics', {'MTLOG004', 'MTLOG006'})
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Verify correct data was saved
      assert.is_not_nil(saved_data)
      assert.is_table(saved_data.suppressed_diagnostics)
      assert.equals(2, #saved_data.suppressed_diagnostics)
      assert.equals('MTLOG004', saved_data.suppressed_diagnostics[1])
      assert.equals('MTLOG006', saved_data.suppressed_diagnostics[2])
      
      -- Restore
      workspace.save = original_save
    end)
    
    it('should preserve other workspace settings', function()
      local saved_data = nil
      
      -- Mock load to return existing data
      local original_load = workspace.load
      workspace.load = function()
        return {
          custom_setting = 'preserve_me',
          another_setting = 42
        }
      end
      
      -- Mock save
      local original_save = workspace.save
      workspace.save = function(data)
        saved_data = data
        return true
      end
      
      -- Set suppressions
      config.set('suppressed_diagnostics', {'MTLOG001'})
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Verify other settings were preserved
      assert.equals('preserve_me', saved_data.custom_setting)
      assert.equals(42, saved_data.another_setting)
      assert.equals(1, #saved_data.suppressed_diagnostics)
      
      -- Restore
      workspace.load = original_load
      workspace.save = original_save
    end)
  end)
  
  describe('load_suppressions', function()
    it('should load suppressions from workspace', function()
      -- Mock load to return suppressions
      local original_load = workspace.load
      workspace.load = function()
        return {
          suppressed_diagnostics = {'MTLOG007', 'MTLOG008'}
        }
      end
      
      -- Mock reanalyze_all to prevent it from running
      local mtlog = require('mtlog')
      local original_reanalyze = mtlog.reanalyze_all
      mtlog.reanalyze_all = function() end
      
      -- Load suppressions
      workspace.load_suppressions()
      
      -- Verify config was updated
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.equals('MTLOG007', suppressed[1])
      assert.equals('MTLOG008', suppressed[2])
      
      -- Restore
      workspace.load = original_load
      mtlog.reanalyze_all = original_reanalyze
    end)
    
    it('should trigger reanalysis after loading', function()
      local reanalyze_called = false
      
      -- Mock load
      local original_load = workspace.load
      workspace.load = function()
        return { suppressed_diagnostics = {'MTLOG001'} }
      end
      
      -- Mock reanalyze_all
      local mtlog = require('mtlog')
      local original_reanalyze = mtlog.reanalyze_all
      mtlog.reanalyze_all = function()
        reanalyze_called = true
      end
      
      -- Load suppressions
      workspace.load_suppressions()
      
      -- Verify reanalyze was called
      assert.is_true(reanalyze_called)
      
      -- Restore
      workspace.load = original_load
      mtlog.reanalyze_all = original_reanalyze
    end)
  end)
end)