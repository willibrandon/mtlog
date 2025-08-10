-- Tests for context-aware analyzer control

describe('mtlog context', function()
  local context
  local config
  
  before_each(function()
    -- Clear module cache
    package.loaded['mtlog.context'] = nil
    package.loaded['mtlog.config'] = nil
    package.loaded['mtlog'] = nil
    
    -- Load modules
    context = require('mtlog.context')
    config = require('mtlog.config')
    
    -- Setup with default config
    config.setup({
      use_builtin_rules = false,  -- Start without built-in rules
    })
    
    context.setup()
  end)
  
  after_each(function()
    context.clear_rules()
  end)
  
  describe('rule management', function()
    it('should add and retrieve rules', function()
      local rule = {
        type = context.rule_types.PATH,
        pattern = '*/vendor/*',
        action = context.actions.IGNORE,
        description = 'Test rule',
      }
      
      context.add_rule(rule)
      
      local rules = context.get_rules()
      assert.are.equal(1, #rules)
      assert.are.equal('Test rule', rules[1].description)
    end)
    
    it('should remove rules by index', function()
      context.add_rule({ type = context.rule_types.PATH, action = context.actions.IGNORE })
      context.add_rule({ type = context.rule_types.PATH, action = context.actions.DISABLE })
      
      assert.are.equal(2, #context.get_rules())
      
      context.remove_rule(1)
      
      local rules = context.get_rules()
      assert.are.equal(1, #rules)
      assert.are.equal(context.actions.DISABLE, rules[1].action)
    end)
    
    it('should clear all rules', function()
      context.add_rule({ type = context.rule_types.PATH, action = context.actions.IGNORE })
      context.add_rule({ type = context.rule_types.PATH, action = context.actions.DISABLE })
      
      context.clear_rules()
      
      assert.are.equal(0, #context.get_rules())
    end)
  end)
  
  describe('path matching', function()
    it('should match glob patterns', function()
      context.add_rule({
        type = context.rule_types.PATH,
        pattern = '*/vendor/*',
        action = context.actions.IGNORE,
      })
      
      -- Create a test buffer
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/vendor/lib/file.go')
      vim.bo[bufnr].filetype = 'go'
      
      local action = context.evaluate(bufnr)
      assert.are.equal(context.actions.IGNORE, action)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should match regex patterns', function()
      context.add_rule({
        type = context.rule_types.PATH,
        pattern = '.*\\.pb\\.go$',
        regex = true,
        action = context.actions.IGNORE,
      })
      
      -- Create a test buffer
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/proto/message.pb.go')
      vim.bo[bufnr].filetype = 'go'
      
      local action = context.evaluate(bufnr)
      assert.are.equal(context.actions.IGNORE, action)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('buffer matching', function()
    it('should match buffer line count', function()
      context.add_rule({
        type = context.rule_types.BUFFER,
        line_count = { max = 5 },
        action = context.actions.DISABLE,
      })
      
      -- Create a test buffer with many lines
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/main.go')
      vim.bo[bufnr].filetype = 'go'
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, {
        'package main',
        'import "fmt"',
        'func main() {',
        '  fmt.Println("Hello")',
        '}',
        '// Extra line',
        '// Another line',
      })
      
      -- Verify buffer has expected line count
      local line_count = vim.api.nvim_buf_line_count(bufnr)
      assert.are.equal(7, line_count)  -- We set 7 lines
      
      local action = context.evaluate(bufnr)
      assert.are.equal(context.actions.DISABLE, action)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should match buffer type', function()
      context.add_rule({
        type = context.rule_types.BUFFER,
        buftype = 'nofile',
        action = context.actions.IGNORE,
      })
      
      -- Create a scratch buffer
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/scratch.go')
      vim.bo[bufnr].filetype = 'go'
      vim.bo[bufnr].buftype = 'nofile'
      
      local action = context.evaluate(bufnr)
      assert.are.equal(context.actions.IGNORE, action)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('custom matching', function()
    it('should evaluate custom matcher functions', function()
      local called = false
      context.add_rule({
        type = context.rule_types.CUSTOM,
        action = context.actions.ENABLE,
        matcher = function(bufnr, filepath)
          called = true
          return filepath:match('special') ~= nil
        end,
      })
      
      -- Create a test buffer
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/special_file.go')
      vim.bo[bufnr].filetype = 'go'
      
      local action = context.evaluate(bufnr)
      assert.is_true(called)
      assert.are.equal(context.actions.ENABLE, action)
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('should_analyze', function()
    it('should respect ignored buffers', function()
      -- Create a test buffer with unique name
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/ignored_test_' .. os.time() .. '.go')
      vim.bo[bufnr].filetype = 'go'
      vim.b[bufnr].mtlog_ignored = true
      
      assert.is_false(context.should_analyze(bufnr))
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
    
    it('should respect disable action', function()
      context.add_rule({
        type = context.rule_types.PATH,
        pattern = '*/test/*',
        action = context.actions.DISABLE,
      })
      
      -- Create a test buffer with unique name
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/project/test/disable_test_' .. os.time() .. '.go')
      vim.bo[bufnr].filetype = 'go'
      
      assert.is_false(context.should_analyze(bufnr))
      
      -- Clean up
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
  
  describe('built-in rules', function()
    it('should have built-in rules defined', function()
      assert.is_not_nil(context.builtin_rules.ignore_vendor)
      assert.is_not_nil(context.builtin_rules.ignore_testdata)
      assert.is_not_nil(context.builtin_rules.disable_large_files)
    end)
    
    it('should load built-in rules when configured', function()
      -- Re-setup with built-in rules enabled
      config.setup({
        use_builtin_rules = true,
      })
      context.setup()
      
      local rules = context.get_rules()
      assert.is_true(#rules > 0)
      
      -- Check for vendor rule
      local has_vendor_rule = false
      for _, rule in ipairs(rules) do
        if rule.description and rule.description:match('vendor') then
          has_vendor_rule = true
          break
        end
      end
      assert.is_true(has_vendor_rule)
    end)
  end)
  
  describe('rules summary', function()
    it('should generate summary of rules', function()
      context.add_rule({
        type = context.rule_types.PATH,
        pattern = '*/vendor/*',
        action = context.actions.IGNORE,
        description = 'Ignore vendor',
      })
      
      context.add_rule({
        type = context.rule_types.BUFFER,
        modified = true,
        action = context.actions.DISABLE,
        description = 'Disable for modified',
      })
      
      local summary = context.get_rules_summary()
      assert.is_true(#summary > 0)
      assert.is_not_nil(summary[1]:match('Context Rules'))
    end)
    
    it('should show empty message when no rules', function()
      local summary = context.get_rules_summary()
      assert.are.equal(1, #summary)
      assert.is_not_nil(summary[1]:match('No context rules'))
    end)
  end)
end)