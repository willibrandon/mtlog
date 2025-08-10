-- Tests for diagnostic suppression functionality

describe('mtlog suppression', function()
  local mtlog
  local config
  local analyzer
  local diagnostics
  
  before_each(function()
    -- Clear any module cache
    package.loaded['mtlog'] = nil
    package.loaded['mtlog.config'] = nil
    package.loaded['mtlog.analyzer'] = nil
    package.loaded['mtlog.diagnostics'] = nil
    
    mtlog = require('mtlog')
    config = require('mtlog.config')
    analyzer = require('mtlog.analyzer')
    diagnostics = require('mtlog.diagnostics')
    
    -- Setup with default config
    mtlog.setup({
      diagnostics_enabled = true,
      suppressed_diagnostics = {},
    })
  end)
  
  describe('configuration', function()
    it('should initialize with empty suppressed diagnostics', function()
      local suppressed = config.get('suppressed_diagnostics')
      assert.is_not_nil(suppressed)
      assert.are.equal(0, #suppressed)
    end)
    
    it('should allow setting suppressed diagnostics', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      local suppressed = config.get('suppressed_diagnostics')
      assert.are.equal(2, #suppressed)
      assert.are.equal('MTLOG001', suppressed[1])
      assert.are.equal('MTLOG004', suppressed[2])
    end)
  end)
  
  describe('kill switch', function()
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
    
    it('should clear diagnostics when kill switch is activated', function()
      -- Create a test buffer
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, 'test.go')
      
      -- Add some test diagnostics
      local test_diags = {
        {
          lnum = 0,
          col = 0,
          message = 'Test diagnostic',
          severity = vim.diagnostic.severity.ERROR,
        }
      }
      diagnostics.set(bufnr, test_diags)
      
      -- Verify diagnostics are set
      local diags = vim.diagnostic.get(bufnr)
      assert.are.equal(1, #diags)
      
      -- Disable diagnostics
      config.set('diagnostics_enabled', false)
      mtlog.analyze_buffer(bufnr)
      
      -- Should have no diagnostics
      diags = vim.diagnostic.get(bufnr)
      assert.are.equal(0, #diags)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, {force = true})
    end)
  end)
  
  describe('suppress_diagnostic', function()
    it('should add diagnostic to suppressed list', function()
      mtlog.suppress_diagnostic('MTLOG001', true)  -- Skip prompt in tests
      local suppressed = config.get('suppressed_diagnostics')
      assert.are.equal(1, #suppressed)
      assert.are.equal('MTLOG001', suppressed[1])
    end)
    
    it('should not duplicate already suppressed diagnostics', function()
      mtlog.suppress_diagnostic('MTLOG001', true)  -- Skip prompt in tests
      mtlog.suppress_diagnostic('MTLOG001', true)  -- Skip prompt in tests
      local suppressed = config.get('suppressed_diagnostics')
      assert.are.equal(1, #suppressed)
    end)
    
    it('should handle multiple suppressions', function()
      mtlog.suppress_diagnostic('MTLOG001', true)  -- Skip prompt in tests
      mtlog.suppress_diagnostic('MTLOG004', true)  -- Skip prompt in tests
      mtlog.suppress_diagnostic('MTLOG006', true)  -- Skip prompt in tests
      
      local suppressed = config.get('suppressed_diagnostics')
      assert.are.equal(3, #suppressed)
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
      assert.are.equal(2, #suppressed)
      assert.is_false(vim.tbl_contains(suppressed, 'MTLOG004'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG001'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG006'))
    end)
    
    it('should handle unsuppressing non-suppressed diagnostic', function()
      mtlog.unsuppress_diagnostic('MTLOG999')
      local suppressed = config.get('suppressed_diagnostics')
      -- Should not change the list
      assert.are.equal(3, #suppressed)
    end)
  end)
  
  describe('unsuppress_all', function()
    it('should clear all suppressions', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004', 'MTLOG006'})
      assert.are.equal(3, #config.get('suppressed_diagnostics'))
      
      mtlog.unsuppress_all()
      
      local suppressed = config.get('suppressed_diagnostics')
      assert.are.equal(0, #suppressed)
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
  
  describe('analyzer integration', function()
    it('should pass suppressions to analyzer', function()
      -- Mock reanalyze_all to prevent issues in tests
      local orig_reanalyze = mtlog.reanalyze_all
      mtlog.reanalyze_all = function()
        -- Do nothing in tests
      end
      
      -- Set suppressions
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      
      -- Mock the analyzer to capture the command
      local captured_cmd = nil
      local orig_jobstart = vim.fn.jobstart
      local orig_chansend = vim.fn.chansend
      local orig_chanclose = vim.fn.chanclose
      
      vim.fn.jobstart = function(cmd, opts)
        captured_cmd = cmd
        -- Don't actually run the job
        return 1
      end
      vim.fn.chansend = function(id, data)
        -- Mock channel send
        return 0
      end
      vim.fn.chanclose = function(id, stream)
        -- Mock channel close
        return 0
      end
      
      -- Mock file read to prevent actual file I/O
      local orig_readfile = vim.fn.readfile
      vim.fn.readfile = function(path)
        return {
          'package main',
          'import "github.com/willibrandon/mtlog"',
          'func main() {',
          '  log := mtlog.New()',
          '  log.Debug("Test {Property}")',
          '}',
        }
      end
      
      -- Run analysis (use a dummy callback)
      analyzer.analyze_file('/tmp/test.go', function() end, nil)
      
      -- Restore originals
      vim.fn.jobstart = orig_jobstart
      vim.fn.chansend = orig_chansend
      vim.fn.chanclose = orig_chanclose
      vim.fn.readfile = orig_readfile
      mtlog.reanalyze_all = orig_reanalyze
      
      -- Check that command includes environment variable
      assert.is_not_nil(captured_cmd, "Command should have been captured")
      if type(captured_cmd) == 'table' and captured_cmd[1] == 'sh' then
        -- Command is wrapped in sh -c
        assert.are.equal('sh', captured_cmd[1])
        assert.are.equal('-c', captured_cmd[2])
        assert.is_not_nil(captured_cmd[3]:match('MTLOG_SUPPRESS=MTLOG001,MTLOG004'), 
          "Command should include MTLOG_SUPPRESS environment variable")
      end
    end)
  end)
  
  describe('workspace configuration', function()
    local workspace
    local test_config_file = '/tmp/.mtlog.json'
    
    before_each(function()
      -- Reload workspace module to reset state
      package.loaded['mtlog.workspace'] = nil
      workspace = require('mtlog.workspace')
    end)
    
    after_each(function()
      -- Clean up test file
      os.remove(test_config_file)
      -- Reset config
      config.set('suppressed_diagnostics', {})
    end)
    
    it('should save suppressions to workspace config', function()
      config.set('suppressed_diagnostics', {'MTLOG001', 'MTLOG004'})
      
      -- Mock the config path
      workspace.get_config_path = function()
        return test_config_file
      end
      
      -- Save suppressions
      workspace.save_suppressions()
      
      -- Read the file
      local file = io.open(test_config_file, 'r')
      assert.is_not_nil(file)
      local content = file:read('*a')
      file:close()
      
      -- Parse JSON
      local ok, data = pcall(vim.json.decode, content)
      assert.is_true(ok)
      assert.is_not_nil(data.suppressed_diagnostics)
      assert.are.equal(2, #data.suppressed_diagnostics)
      assert.are.equal('MTLOG001', data.suppressed_diagnostics[1])
      assert.are.equal('MTLOG004', data.suppressed_diagnostics[2])
    end)
    
    it('should load suppressions from workspace config', function()
      -- Create test config file
      local test_data = {
        suppressed_diagnostics = {'MTLOG002', 'MTLOG005'}
      }
      local file = io.open(test_config_file, 'w')
      file:write(vim.json.encode(test_data))
      file:close()
      
      -- Mock the config path
      workspace.find_config_file = function()
        return test_config_file
      end
      
      -- Mock reanalyze_all to prevent channel errors in tests
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
      assert.are.equal(2, #suppressed)
      assert.are.equal('MTLOG002', suppressed[1])
      assert.are.equal('MTLOG005', suppressed[2])
    end)
  end)
end)