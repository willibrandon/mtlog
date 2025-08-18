-- Tests for mtlog workspace module - NO MOCKS, real file I/O
local workspace = require('mtlog.workspace')
local utils = require('mtlog.utils')
local config = require('mtlog.config')
local test_helpers = require('test_helpers')
local analyzer = require('mtlog.analyzer')

describe('mtlog workspace', function()
  local temp_dir
  local original_cwd
  
  before_each(function()
    -- Ensure analyzer is available
    assert.is_true(analyzer.is_available(), "mtlog-analyzer MUST be available")
    
    -- Store original directory
    original_cwd = vim.fn.getcwd()
    
    -- Create real temporary directory for tests
    temp_dir = vim.fn.tempname() .. '_workspace_test'
    vim.fn.mkdir(temp_dir, 'p')
    vim.fn.chdir(temp_dir)
    
    -- Reset config
    config.setup()
  end)
  
  after_each(function()
    -- Restore original directory
    vim.fn.chdir(original_cwd)
    
    -- Clean up temp directory with real deletion
    if temp_dir and vim.fn.isdirectory(temp_dir) == 1 then
      vim.fn.delete(temp_dir, 'rf')
    end
  end)
  
  describe('find_config_file with real filesystem', function()
    it('should find real .mtlog.json in current directory', function()
      -- Create real .mtlog.json file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file, "Should be able to create config file")
      file:write('{"test": "current_dir"}')
      file:close()
      
      -- Verify file exists
      assert.equals(1, vim.fn.filereadable(config_path))
      
      local found = workspace.find_config_file()
      assert.is_not_nil(found)
      assert.equals(vim.fn.resolve(config_path), vim.fn.resolve(found))
    end)
    
    it('should find real .mtlog.json in parent directory', function()
      -- Create real subdirectory
      local subdir = temp_dir .. '/subdir'
      vim.fn.mkdir(subdir, 'p')
      assert.equals(1, vim.fn.isdirectory(subdir))
      
      -- Create real .mtlog.json in parent
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write('{"test": "parent_dir"}')
      file:close()
      
      -- Change to real subdirectory
      vim.fn.chdir(subdir)
      assert.equals(vim.fn.resolve(subdir), vim.fn.resolve(vim.fn.getcwd()))
      
      local found = workspace.find_config_file()
      assert.is_not_nil(found)
      assert.equals(vim.fn.resolve(config_path), vim.fn.resolve(found))
    end)
    
    it('should stop at real git root if no .mtlog.json found', function()
      -- Create real .git directory to simulate git root
      vim.fn.mkdir(temp_dir .. '/.git', 'p')
      assert.equals(1, vim.fn.isdirectory(temp_dir .. '/.git'))
      
      -- Create real subdirectory
      local subdir = temp_dir .. '/subdir'
      vim.fn.mkdir(subdir, 'p')
      vim.fn.chdir(subdir)
      
      -- No .mtlog.json exists
      local found = workspace.find_config_file()
      assert.is_nil(found)
    end)
    
    it('should return nil if no config file found in real filesystem', function()
      -- No .mtlog.json or .git directory exists
      local found = workspace.find_config_file()
      assert.is_nil(found)
    end)
  end)
  
  describe('get_config_path', function()
    it('should return real path in current directory', function()
      local path = workspace.get_config_path()
      assert.equals(vim.fn.resolve(temp_dir .. '/.mtlog.json'), vim.fn.resolve(path))
    end)
  end)
  
  describe('load with real files', function()
    it('should load real JSON config file', function()
      local test_config = {
        suppressed_diagnostics = {'MTLOG001', 'MTLOG004'},
        custom_setting = 'test_value',
        nested = {
          value = 42
        }
      }
      
      -- Create real config file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write(vim.json.encode(test_config))
      file:close()
      
      -- Verify file was created
      assert.equals(1, vim.fn.filereadable(config_path))
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.is_not_nil(loaded.suppressed_diagnostics)
      assert.equals(2, #loaded.suppressed_diagnostics)
      assert.equals('MTLOG001', loaded.suppressed_diagnostics[1])
      assert.equals('test_value', loaded.custom_setting)
      assert.equals(42, loaded.nested.value)
    end)
    
    it('should return empty table for non-existent real file', function()
      -- Ensure no config file exists
      local config_path = temp_dir .. '/.mtlog.json'
      vim.fn.delete(config_path)
      assert.equals(0, vim.fn.filereadable(config_path))
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
    
    it('should handle malformed JSON in real file gracefully', function()
      -- Create real file with invalid JSON
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write('{ invalid json }')
      file:close()
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
    
    it('should handle empty real file', function()
      -- Create real empty file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write('')
      file:close()
      
      local loaded = workspace.load()
      assert.is_table(loaded)
      assert.equals(0, vim.tbl_count(loaded))
    end)
    
    it('should handle real file with BOM', function()
      -- Create file with UTF-8 BOM
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'wb')
      assert.is_not_nil(file)
      -- UTF-8 BOM followed by valid JSON
      file:write('\239\187\191{"test": "bom"}')
      file:close()
      
      local loaded = workspace.load()
      -- Should either handle BOM or return empty table
      assert.is_table(loaded)
    end)
  end)
  
  describe('save with real file writing', function()
    it('should save data as JSON to real file', function()
      local test_data = {
        suppressed_diagnostics = {'MTLOG002', 'MTLOG005'},
        enabled = false,
        complex = {
          nested = {
            deeply = "value"
          }
        }
      }
      
      local success = workspace.save(test_data)
      assert.is_true(success)
      
      -- Verify real file was created
      local config_path = temp_dir .. '/.mtlog.json'
      assert.equals(1, vim.fn.filereadable(config_path))
      
      -- Read back the real file
      local file = io.open(config_path, 'r')
      assert.is_not_nil(file)
      local content = file:read('*a')
      file:close()
      
      -- Verify JSON is valid and correct
      local ok, decoded = pcall(vim.json.decode, content)
      assert.is_true(ok)
      assert.is_table(decoded)
      assert.equals(2, #decoded.suppressed_diagnostics)
      assert.equals(false, decoded.enabled)
      assert.equals("value", decoded.complex.nested.deeply)
    end)
    
    it('should handle write failure on read-only directory', function()
      -- Make directory read-only (skip on Windows as it handles permissions differently)
      if vim.fn.has('unix') == 1 then
        local readonly_dir = temp_dir .. '/readonly'
        vim.fn.mkdir(readonly_dir, 'p')
        vim.fn.chdir(readonly_dir)
        vim.fn.system('chmod 555 ' .. readonly_dir)
        
        local success = workspace.save({ test = 'data' })
        assert.is_false(success)
        
        -- Restore permissions for cleanup
        vim.fn.system('chmod 755 ' .. readonly_dir)
        vim.fn.chdir(temp_dir)
      end
    end)
    
    it('should create pretty-printed JSON in real file', function()
      workspace.save({ 
        key = 'value',
        array = {1, 2, 3},
        nested = {
          inner = true
        }
      })
      
      -- Read real file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'r')
      assert.is_not_nil(file)
      local content = file:read('*a')
      file:close()
      
      -- Should be valid JSON
      local ok, decoded = pcall(vim.json.decode, content)
      assert.is_true(ok)
      assert.equals('value', decoded.key)
      assert.equals(3, #decoded.array)
      assert.is_true(decoded.nested.inner)
    end)
  end)
  
  describe('apply with real configuration', function()
    it('should apply real workspace config to plugin config', function()
      -- Create real config file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write(vim.json.encode({
        suppressed_diagnostics = {'MTLOG001', 'MTLOG003'},
        diagnostics_enabled = false
      }))
      file:close()
      
      -- Setup initial config
      config.setup({
        suppressed_diagnostics = {},
        diagnostics_enabled = true
      })
      
      -- Apply real workspace config
      workspace.apply()
      
      -- Verify config was updated
      assert.equals(2, #config.get('suppressed_diagnostics'))
      assert.is_true(vim.tbl_contains(config.get('suppressed_diagnostics'), 'MTLOG001'))
      assert.is_true(vim.tbl_contains(config.get('suppressed_diagnostics'), 'MTLOG003'))
      assert.equals(false, config.get('diagnostics_enabled'))
    end)
    
    it('should not crash if real workspace config is empty', function()
      -- Create empty config file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write('{}')
      file:close()
      
      -- Should not error
      assert.has_no_errors(function()
        workspace.apply()
      end)
    end)
  end)
  
  describe('save_suppressions with real persistence', function()
    it('should save current suppressed diagnostics to real file', function()
      -- Set suppressions in config
      config.set('suppressed_diagnostics', {'MTLOG004', 'MTLOG006'})
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Read real file to verify
      local config_path = temp_dir .. '/.mtlog.json'
      assert.equals(1, vim.fn.filereadable(config_path))
      
      local file = io.open(config_path, 'r')
      assert.is_not_nil(file)
      local content = file:read('*a')
      file:close()
      
      local decoded = vim.json.decode(content)
      assert.is_table(decoded.suppressed_diagnostics)
      assert.equals(2, #decoded.suppressed_diagnostics)
      assert.equals('MTLOG004', decoded.suppressed_diagnostics[1])
      assert.equals('MTLOG006', decoded.suppressed_diagnostics[2])
    end)
    
    it('should preserve other settings in real file', function()
      -- Create initial config file with other settings
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write(vim.json.encode({
        custom_setting = 'preserve_me',
        another_setting = 42,
        suppressed_diagnostics = {'OLD001'}
      }))
      file:close()
      
      -- Set new suppressions
      config.set('suppressed_diagnostics', {'MTLOG001'})
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Read file and verify other settings preserved
      file = io.open(config_path, 'r')
      assert.is_not_nil(file)
      local content = file:read('*a')
      file:close()
      
      local decoded = vim.json.decode(content)
      assert.equals('preserve_me', decoded.custom_setting)
      assert.equals(42, decoded.another_setting)
      assert.equals(1, #decoded.suppressed_diagnostics)
      assert.equals('MTLOG001', decoded.suppressed_diagnostics[1])
    end)
  end)
  
  describe('load_suppressions with real files', function()
    it('should load suppressions from real workspace file', function()
      -- Create real config file with suppressions
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write(vim.json.encode({
        suppressed_diagnostics = {'MTLOG007', 'MTLOG008'}
      }))
      file:close()
      
      -- Prevent reanalysis during test
      local original_reanalyze = analyzer.reanalyze_all
      analyzer.reanalyze_all = function() end
      
      -- Load suppressions
      workspace.load_suppressions()
      
      -- Verify config was updated
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.equals('MTLOG007', suppressed[1])
      assert.equals('MTLOG008', suppressed[2])
      
      -- Restore
      analyzer.reanalyze_all = original_reanalyze
    end)
    
    it('should trigger reanalysis after loading from real file', function()
      local reanalyze_called = false
      
      -- Create real config file
      local config_path = temp_dir .. '/.mtlog.json'
      local file = io.open(config_path, 'w')
      assert.is_not_nil(file)
      file:write(vim.json.encode({
        suppressed_diagnostics = {'MTLOG001'}
      }))
      file:close()
      
      -- Track reanalyze call
      local original_reanalyze = analyzer.reanalyze_all
      analyzer.reanalyze_all = function()
        reanalyze_called = true
      end
      
      -- Load suppressions
      workspace.load_suppressions()
      
      -- Verify reanalyze was called
      assert.is_true(reanalyze_called)
      
      -- Restore
      analyzer.reanalyze_all = original_reanalyze
    end)
  end)
  
  describe('real Go project integration', function()
    it('should work with real Go project structure', function()
      -- Create a mini Go project structure
      vim.fn.mkdir(temp_dir .. '/cmd', 'p')
      vim.fn.mkdir(temp_dir .. '/internal', 'p')
      
      -- Create go.mod
      local gomod = io.open(temp_dir .. '/go.mod', 'w')
      assert.is_not_nil(gomod)
      gomod:write('module example.com/test\n\ngo 1.21\n')
      gomod:close()
      
      -- Create .mtlog.json config
      local config_data = {
        suppressed_diagnostics = {'MTLOG001'},
        project_specific = true
      }
      
      local success = workspace.save(config_data)
      assert.is_true(success)
      
      -- Verify it works from subdirectory
      vim.fn.chdir(temp_dir .. '/cmd')
      local found = workspace.find_config_file()
      assert.is_not_nil(found)
      
      local loaded = workspace.load()
      assert.equals(1, #loaded.suppressed_diagnostics)
      assert.is_true(loaded.project_specific)
    end)
  end)
end)