" Plugin initialization for mtlog.nvim
" This file is automatically loaded by Neovim

if exists('g:loaded_mtlog')
  finish
endif
let g:loaded_mtlog = 1

lua << EOF
-- Begin Lua code

-- Create user commands
vim.api.nvim_create_user_command('MtlogAnalyze', function(opts)
  local mtlog = require('mtlog')
  
  if opts.args and opts.args ~= '' then
    -- Analyze specific file
    local filepath = vim.fn.expand(opts.args)
    local bufnr = vim.fn.bufnr(filepath)
    
    if bufnr == -1 then
      -- File not open, create buffer
      vim.cmd('edit ' .. filepath)
      bufnr = vim.fn.bufnr(filepath)
    end
    
    mtlog.analyze_buffer(bufnr)
  else
    -- Analyze current buffer
    mtlog.analyze_buffer()
  end
end, {
  nargs = '?',
  complete = 'file',
  desc = 'Run mtlog-analyzer on current buffer or specified file',
})

vim.api.nvim_create_user_command('MtlogAnalyzeWorkspace', function()
  require('mtlog').analyze_workspace()
end, {
  desc = 'Run mtlog-analyzer on entire workspace',
})

vim.api.nvim_create_user_command('MtlogClear', function(opts)
  local mtlog = require('mtlog')
  
  if opts.bang then
    -- Clear all diagnostics
    mtlog.clear_all()
    vim.notify('Cleared all mtlog diagnostics', vim.log.levels.INFO)
  else
    -- Clear current buffer
    mtlog.clear()
    vim.notify('Cleared mtlog diagnostics for current buffer', vim.log.levels.INFO)
  end
end, {
  bang = true,
  desc = 'Clear mtlog diagnostics (use ! to clear all buffers)',
})

vim.api.nvim_create_user_command('MtlogEnable', function()
  require('mtlog').enable()
end, {
  desc = 'Enable mtlog-analyzer',
})

vim.api.nvim_create_user_command('MtlogDisable', function()
  require('mtlog').disable()
end, {
  desc = 'Disable mtlog-analyzer',
})

vim.api.nvim_create_user_command('MtlogToggle', function()
  require('mtlog').toggle()
end, {
  desc = 'Toggle mtlog-analyzer',
})

vim.api.nvim_create_user_command('MtlogToggleDiagnostics', function()
  require('mtlog').toggle_diagnostics()
end, {
  desc = 'Toggle global diagnostics kill switch',
})

vim.api.nvim_create_user_command('MtlogSuppress', function(opts)
  require('mtlog').suppress_diagnostic(opts.args ~= '' and opts.args or nil)
end, {
  nargs = '?',
  complete = function()
    return { 'MTLOG001', 'MTLOG002', 'MTLOG003', 'MTLOG004', 'MTLOG005', 'MTLOG006', 'MTLOG007', 'MTLOG008' }
  end,
  desc = 'Suppress a diagnostic ID',
})

vim.api.nvim_create_user_command('MtlogUnsuppress', function(opts)
  require('mtlog').unsuppress_diagnostic(opts.args ~= '' and opts.args or nil)
end, {
  nargs = '?',
  complete = function()
    local config = require('mtlog.config')
    return config.get('suppressed_diagnostics') or {}
  end,
  desc = 'Unsuppress a diagnostic ID',
})

vim.api.nvim_create_user_command('MtlogUnsuppressAll', function()
  require('mtlog').unsuppress_all()
end, {
  desc = 'Clear all diagnostic suppressions',
})

vim.api.nvim_create_user_command('MtlogShowSuppressions', function()
  require('mtlog').show_suppressions()
end, {
  desc = 'Show currently suppressed diagnostics',
})

vim.api.nvim_create_user_command('MtlogManageSuppressions', function()
  -- Use Telescope if available
  local ok, telescope = pcall(require, 'telescope')
  if ok then
    telescope.extensions.mtlog.suppressions()
  else
    -- Fallback to simple UI
    require('mtlog').show_suppressions()
  end
end, {
  desc = 'Manage suppressed diagnostics with Telescope',
})

vim.api.nvim_create_user_command('MtlogWorkspace', function(opts)
  local workspace = require('mtlog.workspace')
  
  if opts.args == 'save' then
    workspace.save_suppressions()
  elseif opts.args == 'load' then
    workspace.load_suppressions()
  elseif opts.args == 'path' then
    vim.notify('Workspace config: ' .. workspace.get_config_path(), vim.log.levels.INFO)
  else
    vim.notify('Usage: :MtlogWorkspace [save|load|path]', vim.log.levels.WARN)
  end
end, {
  nargs = 1,
  complete = function()
    return { 'save', 'load', 'path' }
  end,
  desc = 'Manage mtlog workspace configuration',
})

vim.api.nvim_create_user_command('MtlogStatus', function()
  local mtlog = require('mtlog')
  local analyzer = require('mtlog.analyzer')
  
  -- Force recheck of analyzer availability
  analyzer.reset_availability()
  
  local lines = { 'mtlog.nvim Status', '' }
  
  -- Plugin status
  if mtlog.initialized() then
    table.insert(lines, '✓ Plugin initialized')
    
    if mtlog.enabled() then
      table.insert(lines, '✓ Plugin enabled')
    else
      table.insert(lines, '✗ Plugin disabled')
    end
  else
    table.insert(lines, '✗ Plugin not initialized')
  end
  
  -- Analyzer status
  if analyzer.is_available() then
    local version = analyzer.get_version()
    if version then
      table.insert(lines, string.format('✓ Analyzer version: %s', version))
    else
      table.insert(lines, '✓ Analyzer available')
    end
  else
    table.insert(lines, '✗ Analyzer not found')
  end
  
  -- Diagnostic counts
  table.insert(lines, '')
  table.insert(lines, 'Diagnostics:')
  
  local config = require('mtlog.config')
  local diagnostics_enabled = config.get('diagnostics_enabled')
  if not diagnostics_enabled then
    table.insert(lines, '  ⚠ Kill switch active - diagnostics disabled')
  end
  
  local counts = mtlog.get_counts()
  table.insert(lines, string.format('  Total: %d', counts.total))
  table.insert(lines, string.format('  Errors: %d', counts.errors))
  table.insert(lines, string.format('  Warnings: %d', counts.warnings))
  table.insert(lines, string.format('  Info: %d', counts.info))
  table.insert(lines, string.format('  Hints: %d', counts.hints))
  
  -- Suppressed diagnostics
  local suppressed = config.get('suppressed_diagnostics') or {}
  if #suppressed > 0 then
    table.insert(lines, string.format('  Suppressed: %s', table.concat(suppressed, ', ')))
  end
  
  -- Cache stats
  local cache = require('mtlog.cache')
  local cache_stats = cache.stats()
  table.insert(lines, '')
  table.insert(lines, 'Cache:')
  table.insert(lines, string.format('  Entries: %d', cache_stats.entries))
  
  -- Show in floating window
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.api.nvim_buf_set_option(buf, 'modifiable', false)
  
  local width = 0
  for _, line in ipairs(lines) do
    width = math.max(width, #line)
  end
  
  local win_opts = {
    relative = 'cursor',
    row = 1,
    col = 0,
    width = width + 4,
    height = #lines,
    style = 'minimal',
    border = 'rounded',
  }
  
  -- title and title_pos only available in Neovim 0.9+
  if vim.fn.has('nvim-0.9') == 1 then
    win_opts.title = ' mtlog.nvim '
    win_opts.title_pos = 'center'
  end
  
  local win = vim.api.nvim_open_win(buf, true, win_opts)  -- true to focus the window
  
  -- Close on any key - use vim.schedule to ensure the window ID is captured correctly
  vim.api.nvim_buf_set_keymap(buf, 'n', '<Esc>', '', { 
    silent = true,
    callback = function()
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
    end
  })
  vim.api.nvim_buf_set_keymap(buf, 'n', 'q', '', { 
    silent = true,
    callback = function()
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
    end
  })
  vim.api.nvim_buf_set_keymap(buf, 'n', '<CR>', '', { 
    silent = true,
    callback = function()
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
    end
  })
end, {
  desc = 'Show mtlog-analyzer status',
})

vim.api.nvim_create_user_command('MtlogCache', function(opts)
  local cache = require('mtlog.cache')
  
  if opts.args == 'clear' then
    cache.clear()
    vim.notify('mtlog cache cleared', vim.log.levels.INFO)
  elseif opts.args == 'stats' then
    local stats = cache.stats()
    vim.notify(string.format('Cache: %d entries (hit rate: %.1f%%)',
      stats.entries,
      stats.hit_rate * 100
    ), vim.log.levels.INFO)
  else
    vim.notify('Usage: :MtlogCache [clear|stats]', vim.log.levels.WARN)
  end
end, {
  nargs = 1,
  complete = function()
    return { 'clear', 'stats' }
  end,
  desc = 'Manage mtlog cache',
})

vim.api.nvim_create_user_command('MtlogContext', function(opts)
  local context = require('mtlog.context')
  
  if opts.args == 'show' then
    local lines = context.get_rules_summary()
    
    if #lines == 1 then
      vim.notify(lines[1], vim.log.levels.INFO)
    else
      -- Show in floating window
      local buf = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
      vim.api.nvim_buf_set_option(buf, 'modifiable', false)
      
      local width = 0
      for _, line in ipairs(lines) do
        width = math.max(width, #line)
      end
      
      local win_opts = {
        relative = 'cursor',
        row = 1,
        col = 0,
        width = width + 4,
        height = #lines,
        style = 'minimal',
        border = 'rounded',
      }
      
      if vim.fn.has('nvim-0.9') == 1 then
        win_opts.title = ' Context Rules '
        win_opts.title_pos = 'center'
      end
      
      local win = vim.api.nvim_open_win(buf, true, win_opts)
      
      -- Close on any key
      vim.api.nvim_buf_set_keymap(buf, 'n', '<Esc>', '', { 
        silent = true,
        callback = function()
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_win_close(win, true)
          end
        end
      })
      vim.api.nvim_buf_set_keymap(buf, 'n', 'q', '', { 
        silent = true,
        callback = function()
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_win_close(win, true)
          end
        end
      })
    end
    
  elseif opts.args == 'test' then
    -- Test context rules on current buffer
    local bufnr = vim.api.nvim_get_current_buf()
    local action, rule = context.evaluate(bufnr)
    
    if action then
      vim.notify(string.format('Context match: %s - %s', 
        action, rule.description or 'No description'), vim.log.levels.INFO)
    else
      vim.notify('No context rules match current buffer', vim.log.levels.INFO)
    end
    
  elseif opts.args == 'add-builtin' then
    -- Add built-in rules
    local builtin = context.builtin_rules
    vim.ui.select(vim.tbl_keys(builtin), {
      prompt = 'Select built-in rule to add:',
      format_item = function(key)
        return string.format('%s - %s', key, builtin[key].description)
      end,
    }, function(choice)
      if choice then
        context.add_rule(builtin[choice])
        vim.notify(string.format('Added built-in rule: %s', choice), vim.log.levels.INFO)
      end
    end)
    
  elseif opts.args == 'clear' then
    context.clear_rules()
    vim.notify('Cleared all context rules', vim.log.levels.INFO)
    
  else
    vim.notify('Usage: :MtlogContext [show|test|add-builtin|clear]', vim.log.levels.WARN)
  end
end, {
  nargs = 1,
  complete = function()
    return { 'show', 'test', 'add-builtin', 'clear' }
  end,
  desc = 'Manage context rules',
})

vim.api.nvim_create_user_command('MtlogQueue', function(opts)
  local queue = require('mtlog.queue')
  
  if opts.args == 'clear' then
    queue.clear()
  elseif opts.args == 'pause' then
    queue.pause()
  elseif opts.args == 'resume' then
    queue.resume()
  elseif opts.args == 'stats' then
    local stats = queue.get_stats()
    local lines = {
      'Queue Statistics:',
      string.format('  Pending: %d', stats.pending),
      string.format('  Active: %d/%d', stats.active, stats.max_concurrent),
      string.format('  Completed: %d', stats.completed),
      string.format('  Failed: %d', stats.failed),
      string.format('  Cancelled: %d', stats.cancelled),
      string.format('  Status: %s', stats.paused and 'Paused' or 'Running'),
    }
    vim.notify(table.concat(lines, '\n'), vim.log.levels.INFO)
  elseif opts.args == 'show' then
    -- Show queue contents
    local entries = queue.get_queue()
    if #entries == 0 then
      vim.notify('Queue is empty', vim.log.levels.INFO)
    else
      local lines = { 'Analysis Queue:', '' }
      for i, entry in ipairs(entries) do
        local status_icon = entry.status == 'processing' and '⚡' or '⏳'
        local priority_label = entry.priority == 1 and 'HIGH' or entry.priority == 2 and 'NORMAL' or 'LOW'
        local time_info = ''
        if entry.status == 'processing' and entry.elapsed then
          time_info = string.format(' (%.1fs)', entry.elapsed / 1000)
        elseif entry.waiting then
          time_info = string.format(' (waiting %.1fs)', entry.waiting / 1000)
        end
        table.insert(lines, string.format('%d. %s [%s] %s%s - %s',
          i, status_icon, priority_label, vim.fn.fnamemodify(entry.filepath, ':t'),
          time_info, entry.status))
      end
      
      -- Show in floating window
      local buf = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
      vim.api.nvim_buf_set_option(buf, 'modifiable', false)
      
      local width = 0
      for _, line in ipairs(lines) do
        width = math.max(width, #line)
      end
      
      local win_opts = {
        relative = 'cursor',
        row = 1,
        col = 0,
        width = width + 4,
        height = #lines,
        style = 'minimal',
        border = 'rounded',
      }
      
      if vim.fn.has('nvim-0.9') == 1 then
        win_opts.title = ' Analysis Queue '
        win_opts.title_pos = 'center'
      end
      
      local win = vim.api.nvim_open_win(buf, true, win_opts)
      
      -- Close on any key
      vim.api.nvim_buf_set_keymap(buf, 'n', '<Esc>', '', { 
        silent = true,
        callback = function()
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_win_close(win, true)
          end
        end
      })
      vim.api.nvim_buf_set_keymap(buf, 'n', 'q', '', { 
        silent = true,
        callback = function()
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_win_close(win, true)
          end
        end
      })
    end
  else
    vim.notify('Usage: :MtlogQueue [show|stats|clear|pause|resume]', vim.log.levels.WARN)
  end
end, {
  nargs = 1,
  complete = function()
    return { 'show', 'stats', 'clear', 'pause', 'resume' }
  end,
  desc = 'Manage analysis queue',
})

vim.api.nvim_create_user_command('MtlogCodeAction', function()
  local lsp_integration = require('mtlog.lsp_integration')
  lsp_integration.show_code_actions()
end, {
  desc = 'Show mtlog code actions',
})

vim.api.nvim_create_user_command('MtlogQuickFix', function()
  local diagnostics = require('mtlog.diagnostics')
  local config = require('mtlog.config')
  local diag = diagnostics.get_diagnostic_at_cursor()
  
  if not diag then
    vim.notify('No diagnostic at cursor', vim.log.levels.WARN)
    return
  end
  
  if not diag.user_data or not diag.user_data.suggested_fixes then
    vim.notify('No quick fixes available', vim.log.levels.INFO)
    return
  end
  
  local fixes = diag.user_data.suggested_fixes
  
  if #fixes == 1 then
    -- Apply single fix directly
    if diagnostics.apply_suggested_fix(diag, 1) then
      vim.notify('Applied quick fix', vim.log.levels.INFO)
      -- Clear diagnostics immediately, invalidate cache, and re-analyze
      local bufnr = vim.api.nvim_get_current_buf()
      local filepath = vim.api.nvim_buf_get_name(bufnr)
      diagnostics.clear(bufnr)  -- Clear diagnostics immediately
      require('mtlog.cache').invalidate(filepath)
      -- Save the buffer if auto_save is enabled
      if config.get('quick_fix.auto_save') then
        vim.cmd('write')
      end
      -- Small delay to let the buffer update and file save
      vim.defer_fn(function()
        require('mtlog').analyze_buffer(bufnr)
      end, 500)
    else
      vim.notify('Failed to apply quick fix', vim.log.levels.ERROR)
    end
  else
    -- Show menu for multiple fixes
    vim.ui.select(fixes, {
      prompt = 'Select quick fix:',
      format_item = function(fix)
        return fix.description or fix.title or 'Fix'
      end,
    }, function(fix, idx)
      if fix and idx then
        if diagnostics.apply_suggested_fix(diag, idx) then
          vim.notify('Applied quick fix', vim.log.levels.INFO)
          -- Clear diagnostics immediately, invalidate cache, and re-analyze
          local bufnr = vim.api.nvim_get_current_buf()
          local filepath = vim.api.nvim_buf_get_name(bufnr)
          diagnostics.clear(bufnr)  -- Clear diagnostics immediately
          require('mtlog.cache').invalidate(filepath)
          -- Save the buffer to ensure changes are written
          vim.cmd('write')
          -- Small delay to let the buffer update and file save
          vim.defer_fn(function()
            require('mtlog').analyze_buffer(bufnr)
          end, 500)
        else
          vim.notify('Failed to apply quick fix', vim.log.levels.ERROR)
        end
      end
    end)
  end
end, {
  desc = 'Apply mtlog quick fix at cursor',
})

-- Help commands
vim.api.nvim_create_user_command('MtlogHelp', function(opts)
  local help = require('mtlog.help')
  
  if opts.args == '' then
    help.show_menu()
  elseif opts.args == 'quick' then
    help.show_quick_reference()
  else
    -- Try to show specific topic
    help.show_topic(opts.args)
  end
end, {
  nargs = '?',
  complete = function()
    local help = require('mtlog.help')
    local topics = vim.tbl_keys(help.topics)
    table.insert(topics, 'quick')
    return topics
  end,
  desc = 'Show mtlog help',
})

vim.api.nvim_create_user_command('MtlogExplain', function(opts)
  local help = require('mtlog.help')
  
  if opts.args == '' then
    -- Explain diagnostic at cursor
    help.explain_at_cursor()
  else
    -- Explain specific diagnostic code
    help.explain_diagnostic(opts.args)
  end
end, {
  nargs = '?',
  complete = function()
    return { 'MTLOG001', 'MTLOG002', 'MTLOG003', 'MTLOG004', 
             'MTLOG005', 'MTLOG006', 'MTLOG007', 'MTLOG008' }
  end,
  desc = 'Explain mtlog diagnostic',
})

-- Set up autocommands for lazy loading
local group = vim.api.nvim_create_augroup('MtlogLazyLoad', { clear = true })

-- Load plugin when entering a Go file
vim.api.nvim_create_autocmd({ 'FileType' }, {
  group = group,
  pattern = 'go',
  once = true,
  callback = function()
    -- Check if we should auto-setup
    if vim.g.mtlog_auto_setup ~= false then
      local ok, mtlog = pcall(require, 'mtlog')
      if ok and not mtlog.initialized() then
        -- Use default configuration
        mtlog.setup()
      end
    end
  end,
})

-- LSP integration is now handled by the lsp_integration module during setup

EOF
" End of plugin/mtlog.vim