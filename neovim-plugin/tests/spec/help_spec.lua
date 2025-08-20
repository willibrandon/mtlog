-- Tests for mtlog help module
local help = require('mtlog.help')
local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

describe('mtlog help', function()
  local test_bufnr
  local original_notify
  local notifications
  
  before_each(function()
    -- Setup config
    config.setup()
    
    -- Clean up any existing test buffers first
    for _, buf in ipairs(vim.api.nvim_list_bufs()) do
      local name = vim.api.nvim_buf_get_name(buf)
      if name:match('help_test') then
        pcall(vim.api.nvim_buf_delete, buf, { force = true })
      end
    end
    
    -- Create test buffer with unique name using timestamp
    test_bufnr = vim.api.nvim_create_buf(false, true)
    local unique_name = string.format('/test/help_test_%d.go', vim.loop.hrtime())
    vim.api.nvim_buf_set_name(test_bufnr, unique_name)
    vim.api.nvim_set_current_buf(test_bufnr)
    
    -- Setup diagnostics
    diagnostics.setup()
    
    -- Mock vim.notify to capture messages
    notifications = {}
    original_notify = vim.notify
    vim.notify = function(msg, level, opts)
      table.insert(notifications, {
        message = msg,
        level = level,
        opts = opts
      })
    end
  end)
  
  after_each(function()
    -- Restore original notify
    vim.notify = original_notify
    
    -- Clean up
    diagnostics.clear_all()
    if test_bufnr and vim.api.nvim_buf_is_valid(test_bufnr) then
      vim.api.nvim_buf_delete(test_bufnr, { force = true })
    end
  end)
  
  describe('diagnostic_explanations', function()
    it('should have explanations for all standard diagnostic codes', function()
      local standard_codes = {
        'MTLOG001', 'MTLOG002', 'MTLOG003', 'MTLOG004',
        'MTLOG005', 'MTLOG006', 'MTLOG007', 'MTLOG008'
      }
      
      for _, code in ipairs(standard_codes) do
        local explanation = help.diagnostic_explanations[code]
        assert.is_not_nil(explanation, code .. " should have an explanation")
        assert.is_not_nil(explanation.name, code .. " should have a name")
        assert.is_not_nil(explanation.description, code .. " should have a description")
        assert.is_not_nil(explanation.example, code .. " should have an example")
        assert.is_not_nil(explanation.fix, code .. " should have a fix")
      end
    end)
    
    it('should have explanations for With() diagnostic codes', function()
      local with_codes = {
        'MTLOG009', 'MTLOG010', 'MTLOG011', 'MTLOG012', 'MTLOG013'
      }
      
      for _, code in ipairs(with_codes) do
        local explanation = help.diagnostic_explanations[code]
        assert.is_not_nil(explanation, code .. " should have an explanation")
        assert.is_not_nil(explanation.name, code .. " should have a name")
        assert.is_not_nil(explanation.description, code .. " should have a description")
        assert.is_not_nil(explanation.example, code .. " should have an example")
        assert.is_not_nil(explanation.fix, code .. " should have a fix")
      end
    end)
    
    it('should have comprehensive explanations', function()
      -- Check a specific explanation for completeness
      local explanation = help.diagnostic_explanations.MTLOG001
      assert.is_not_nil(explanation)
      assert.equals("Template/argument mismatch", explanation.name)
      assert.is_true(#explanation.description > 20, "Description should be detailed")
      assert.is_table(explanation.example, "Example should be a table")
      assert.is_not_nil(explanation.example.wrong, "Should have wrong example")
      assert.is_not_nil(explanation.example.correct, "Should have correct example")
      assert.is_true(#explanation.fix > 10, "Fix should provide guidance")
    end)
  end)
  
  describe('explain_diagnostic', function()
    it('should explain a specific diagnostic code', function()
      -- explain_diagnostic shows a float window, not a notification
      -- We should check that a buffer was created
      local initial_bufs = vim.api.nvim_list_bufs()
      
      help.explain_diagnostic('MTLOG001')
      
      local after_bufs = vim.api.nvim_list_bufs()
      -- Should have created a new buffer for the float
      assert.is_true(#after_bufs > #initial_bufs, "Should create a buffer for float window")
    end)
    
    it('should handle unknown diagnostic codes', function()
      help.explain_diagnostic('MTLOG999')
      
      -- Should show message about unknown code
      assert.equals(1, #notifications)
      local notif = notifications[1]
      assert.is_not_nil(notif.message)
      assert.is_true(notif.message:match('Unknown') ~= nil or 
                    notif.message:match('No explanation') ~= nil,
                    "Should indicate unknown diagnostic")
    end)
    
    it('should format explanation nicely', function()
      local initial_bufs = vim.api.nvim_list_bufs()
      
      help.explain_diagnostic('MTLOG004')
      
      local after_bufs = vim.api.nvim_list_bufs()
      -- Should have created a new buffer for the float
      assert.is_true(#after_bufs > #initial_bufs, "Should create a buffer for float window")
    end)
  end)
  
  describe('explain_at_cursor', function()
    it('should explain diagnostic at cursor position', function()
      -- Set a diagnostic
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        end_lnum = 0,
        end_col = 10,
        message = '[MTLOG002] Invalid format specifier',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG002'
      }})
      
      -- Move cursor to diagnostic position
      vim.api.nvim_win_set_cursor(0, {1, 0})
      
      -- Explain at cursor
      local initial_bufs = vim.api.nvim_list_bufs()
      help.explain_at_cursor()
      local after_bufs = vim.api.nvim_list_bufs()
      
      -- Should have created a float window buffer
      assert.is_true(#after_bufs > #initial_bufs, "Should create float window for explanation")
    end)
    
    it('should show message when no diagnostic at cursor', function()
      -- No diagnostics set
      vim.api.nvim_win_set_cursor(0, {1, 0})
      
      help.explain_at_cursor()
      
      -- Should show "no diagnostic" message
      assert.equals(1, #notifications)
      local notif = notifications[1]
      assert.is_true(notif.message:match('No') ~= nil or
                    notif.message:match('no') ~= nil,
                    "Should indicate no diagnostic found")
    end)
    
    it('should handle diagnostics without codes', function()
      -- Set a diagnostic without a code
      diagnostics.set(test_bufnr, {{
        lnum = 0,
        col = 0,
        message = 'Generic error',
        severity = vim.diagnostic.severity.ERROR
      }})
      
      vim.api.nvim_win_set_cursor(0, {1, 0})
      
      help.explain_at_cursor()
      
      -- Should handle gracefully
      assert.is_true(#notifications > 0)
    end)
  end)
  
  describe('show_menu', function()
    it('should create menu with diagnostic codes', function()
      local original_ui_select = vim.ui.select
      local captured_items = nil
      local captured_opts = nil
      
      -- Mock vim.ui.select
      vim.ui.select = function(items, opts, on_choice)
        captured_items = items
        captured_opts = opts
        -- Simulate selecting the first item (which has key info)
        if items and #items > 0 then
          on_choice(items[1], 1)
        else
          on_choice(nil, nil)
        end
      end
      
      help.show_menu()
      
      -- Restore
      vim.ui.select = original_ui_select
      
      -- Check that menu was shown
      assert.is_not_nil(captured_items)
      assert.is_table(captured_items)
      assert.is_true(#captured_items > 0, "Should have menu items")
      
      -- Should include help topics (not diagnostic codes directly)
      local has_diagnostics_topic = false
      for _, item in ipairs(captured_items) do
        if type(item) == 'table' and item.title then
          -- Check if there's a diagnostics-related topic
          if item.title:match('Diagnostic') or item.key == 'diagnostics' then
            has_diagnostics_topic = true
            break
          end
        end
      end
      assert.is_true(has_diagnostics_topic, "Menu should include diagnostics topic")
      
      -- Check that selection triggered show_topic (which creates a float)
      -- The menu items are topics, not diagnostic codes directly
      -- So we just check that the menu was shown with items
      assert.is_true(#captured_items > 0, "Menu should have items")
    end)
    
    it('should handle menu cancellation', function()
      local original_ui_select = vim.ui.select
      
      -- Mock vim.ui.select to simulate cancellation
      vim.ui.select = function(items, opts, on_choice)
        -- Call with nil to simulate cancellation
        on_choice(nil, nil)
      end
      
      help.show_menu()
      
      -- Restore
      vim.ui.select = original_ui_select
      
      -- Should not crash and should not show explanation
      assert.equals(0, #notifications, "Should not show explanation on cancellation")
    end)
    
    it('should format menu items nicely', function()
      local original_ui_select = vim.ui.select
      local captured_items = nil
      
      vim.ui.select = function(items, opts, on_choice)
        captured_items = items
        on_choice(nil, nil)  -- Cancel
      end
      
      help.show_menu()
      
      vim.ui.select = original_ui_select
      
      assert.is_not_nil(captured_items)
      -- Items are topic objects with key and title
      assert.is_not_nil(captured_items)
      assert.is_true(#captured_items > 0, "Should have menu items")
      -- The items are tables with 'key' and 'title' fields
      if type(captured_items[1]) == 'table' then
        assert.is_not_nil(captured_items[1].title, "Menu items should have titles")
      end
    end)
  end)
  
  describe('show_quick_reference', function()
    it('should display quick reference card', function()
      local initial_bufs = vim.api.nvim_list_bufs()
      
      help.show_quick_reference()
      
      local after_bufs = vim.api.nvim_list_bufs()
      -- Should have created a new buffer for the float
      assert.is_true(#after_bufs > #initial_bufs, "Should create a buffer for quick reference")
    end)
  end)
  
  describe('integration', function()
    it('should work with real diagnostic data', function()
      -- Add some content to the buffer so we can position the cursor
      local lines = {}
      for i = 1, 10 do
        lines[i] = string.format('// Line %d', i)
      end
      lines[6] = 'log.With("key1", value1, "key2")  // MTLOG009 error here'
      vim.api.nvim_buf_set_lines(test_bufnr, 0, -1, false, lines)
      
      -- Set a real mtlog diagnostic
      diagnostics.set(test_bufnr, {{
        lnum = 5,  -- 0-indexed for diagnostic
        col = 10,
        end_lnum = 5,
        end_col = 30,
        message = '[MTLOG009] With() requires an even number of arguments',
        severity = vim.diagnostic.severity.ERROR,
        source = 'mtlog-analyzer',
        code = 'MTLOG009'
      }})
      
      -- Set cursor to the diagnostic position (1-indexed for cursor)
      vim.api.nvim_win_set_cursor(0, {6, 10})  -- Line 6 (1-indexed), col 10
      
      local initial_bufs = vim.api.nvim_list_bufs()
      help.explain_at_cursor()
      local after_bufs = vim.api.nvim_list_bufs()
      
      -- Should have created a float window to explain MTLOG009
      assert.is_true(#after_bufs > #initial_bufs, "Should create float window for MTLOG009 explanation")
    end)
  end)
end)