-- Tests for mtlog utils module
local utils = require('mtlog.utils')

describe('mtlog utils', function()
  describe('get_diagnostic_description', function()
    it('should return descriptions for all standard diagnostic codes', function()
      -- Test standard diagnostic codes
      local standard_codes = {
        MTLOG001 = "Template/argument mismatch",
        MTLOG002 = "Invalid format specifier",
        MTLOG003 = "Missing error in Error/Fatal log",
        MTLOG004 = "Property name not PascalCase",
        MTLOG005 = "Complex type needs LogValue() method",
        MTLOG006 = "Duplicate property in template",
        MTLOG007 = "String context key should be constant",
        MTLOG008 = "General information",
      }
      
      for code, expected_desc in pairs(standard_codes) do
        local desc = utils.get_diagnostic_description(code)
        assert.equals(expected_desc, desc, "Description for " .. code .. " should match")
      end
    end)
    
    it('should return descriptions for With() diagnostic codes', function()
      -- Test new With() diagnostic codes
      local with_codes = {
        MTLOG009 = "With() odd argument count",
        MTLOG010 = "With() non-string key",
        MTLOG011 = "With() cross-call duplicate",
        MTLOG012 = "With() reserved property",
        MTLOG013 = "With() empty key",
      }
      
      for code, expected_desc in pairs(with_codes) do
        local desc = utils.get_diagnostic_description(code)
        assert.equals(expected_desc, desc, "Description for " .. code .. " should match")
      end
    end)
    
    it('should return default message for unknown codes', function()
      local desc = utils.get_diagnostic_description("MTLOG999")
      assert.equals("Unknown diagnostic", desc)
      
      desc = utils.get_diagnostic_description("INVALID")
      assert.equals("Unknown diagnostic", desc)
      
      desc = utils.get_diagnostic_description("")
      assert.equals("Unknown diagnostic", desc)
    end)
    
    it('should handle all known diagnostic codes', function()
      -- Test that all 13 codes are handled
      local all_codes = {
        "MTLOG001", "MTLOG002", "MTLOG003", "MTLOG004", "MTLOG005",
        "MTLOG006", "MTLOG007", "MTLOG008", "MTLOG009", "MTLOG010",
        "MTLOG011", "MTLOG012", "MTLOG013"
      }
      
      for _, code in ipairs(all_codes) do
        local desc = utils.get_diagnostic_description(code)
        assert.is_not_nil(desc)
        assert.not_equals("Unknown diagnostic", desc, code .. " should have a proper description")
        assert.is_true(#desc > 0, code .. " should have non-empty description")
      end
    end)
  end)
  
  describe('format_diagnostic_message', function()
    local config
    
    before_each(function()
      config = require('mtlog.config')
      config.setup({})
    end)
    
    it('should format message with code when show_codes is true', function()
      config.set('show_codes', true)
      local formatted = utils.format_diagnostic_message('MTLOG001', 'Template expects 2 arguments')
      assert.equals('[MTLOG001] Template expects 2 arguments', formatted)
    end)
    
    it('should format message without code when show_codes is false', function()
      config.set('show_codes', false)
      local formatted = utils.format_diagnostic_message('MTLOG001', 'Template expects 2 arguments')
      assert.equals('Template expects 2 arguments', formatted)
    end)
    
    it('should show codes by default', function()
      -- Default config should show codes
      local formatted = utils.format_diagnostic_message('MTLOG009', 'With() requires even arguments')
      assert.equals('[MTLOG009] With() requires even arguments', formatted)
    end)
  end)
  
  describe('parse_position', function()
    it('should parse line:column format', function()
      local line, col = utils.parse_position('10:25')
      assert.equals(10, line)
      assert.equals(25, col)
    end)
    
    it('should handle single digit positions', function()
      local line, col = utils.parse_position('1:1')
      assert.equals(1, line)
      assert.equals(1, col)
    end)
    
    it('should handle large numbers', function()
      local line, col = utils.parse_position('1234:5678')
      assert.equals(1234, line)
      assert.equals(5678, col)
    end)
    
    it('should return nil for invalid format', function()
      local line, col = utils.parse_position('invalid')
      assert.is_nil(line)
      assert.is_nil(col)
      
      line, col = utils.parse_position('10')
      assert.is_nil(line)
      assert.is_nil(col)
      
      line, col = utils.parse_position('')
      assert.is_nil(line)
      assert.is_nil(col)
    end)
  end)
  
  describe('offset_to_position', function()
    it('should convert byte offset to line and column', function()
      local content = 'line1\nline2\nline3'
      
      -- First character
      local line, col = utils.offset_to_position(content, 1)
      assert.equals(1, line)
      assert.equals(1, col)
      
      -- After first newline (start of line 2)
      line, col = utils.offset_to_position(content, 7)
      assert.equals(2, line)
      assert.equals(1, col)
      
      -- Middle of line 2
      line, col = utils.offset_to_position(content, 10)
      assert.equals(2, line)
      assert.equals(4, col)
    end)
    
    it('should handle empty content', function()
      local line, col = utils.offset_to_position('', 1)
      assert.equals(1, line)
      assert.equals(1, col)
    end)
    
    it('should handle offset beyond content', function()
      local content = 'short'
      local line, col = utils.offset_to_position(content, 100)
      assert.equals(1, line)
      assert.equals(6, col)  -- Should stop at end of content
    end)
  end)
  
  describe('relative_path', function()
    it('should return relative path from cwd', function()
      local cwd = vim.fn.getcwd()
      local filepath = cwd .. '/test/file.go'
      local relative = utils.relative_path(filepath)
      assert.equals('test/file.go', relative)
    end)
    
    it('should return full path if not under cwd', function()
      local filepath = '/completely/different/path/file.go'
      local relative = utils.relative_path(filepath)
      assert.equals('/completely/different/path/file.go', relative)
    end)
  end)
  
  describe('debounce', function()
    it('should debounce function calls', function()
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
      
      -- Wait for debounce period
      vim.wait(60)
      
      -- Should have been called once
      assert.equals(1, call_count)
    end)
    
    it('should pass arguments to debounced function', function()
      local received_args = nil
      local debounced = utils.debounce(function(...)
        received_args = {...}
      end, 20)
      
      debounced('arg1', 'arg2', 123)
      
      vim.wait(30)
      
      assert.is_not_nil(received_args)
      assert.equals('arg1', received_args[1])
      assert.equals('arg2', received_args[2])
      assert.equals(123, received_args[3])
    end)
  end)
  
  describe('throttle', function()
    it('should throttle function calls', function()
      local call_count = 0
      local throttled = utils.throttle(function()
        call_count = call_count + 1
      end, 100)
      
      -- First call should go through immediately
      throttled()
      assert.equals(1, call_count)
      
      -- Rapid calls should be throttled
      throttled()
      throttled()
      throttled()
      assert.equals(1, call_count)  -- Still 1
      
      -- Wait for throttle period
      vim.wait(110)
      
      -- Last call should have gone through
      assert.equals(2, call_count)
    end)
  end)
  
  describe('check_neovim_version', function()
    it('should check for minimum Neovim version', function()
      -- Should pass for older version
      local ok, err = utils.check_neovim_version('0.5.0')
      assert.is_true(ok)
      assert.is_nil(err)
      
      -- Should fail for future version
      ok, err = utils.check_neovim_version('99.0.0')
      assert.is_false(ok)
      assert.is_not_nil(err)
      assert.is_true(err:match('Neovim 99.0.0 or higher required') ~= nil)
    end)
  end)
end)