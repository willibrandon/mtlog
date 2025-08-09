-- Plugin initialization for mtlog.nvim
-- This file is automatically loaded by Neovim

if vim.g.loaded_mtlog then
  return
end
vim.g.loaded_mtlog = 1

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
  
  local counts = mtlog.get_counts()
  table.insert(lines, string.format('  Total: %d', counts.total))
  table.insert(lines, string.format('  Errors: %d', counts.errors))
  table.insert(lines, string.format('  Warnings: %d', counts.warnings))
  table.insert(lines, string.format('  Info: %d', counts.info))
  table.insert(lines, string.format('  Hints: %d', counts.hints))
  
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
  
  -- Close on any key
  local close_cmd = string.format('<cmd>lua vim.api.nvim_win_close(%d, true)<CR>', win)
  vim.api.nvim_buf_set_keymap(buf, 'n', '<Esc>', close_cmd, { silent = true })
  vim.api.nvim_buf_set_keymap(buf, 'n', 'q', close_cmd, { silent = true })
  vim.api.nvim_buf_set_keymap(buf, 'n', '<CR>', close_cmd, { silent = true })
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

vim.api.nvim_create_user_command('MtlogQuickFix', function()
  local diagnostics = require('mtlog.diagnostics')
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
      -- Save the buffer to ensure changes are written
      vim.cmd('write')
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

-- Provide code action integration (only for Neovim 0.8+)
if vim.fn.has('nvim-0.8') == 1 then
  vim.api.nvim_create_autocmd('LspAttach', {
    group = group,
    callback = function(args)
      local bufnr = args.buf
      local client = vim.lsp.get_client_by_id(args.data.client_id)
      
      if client and vim.bo[bufnr].filetype == 'go' then
        -- Add mtlog code actions
        local original_handler = vim.lsp.handlers['textDocument/codeAction']
        
        vim.lsp.handlers['textDocument/codeAction'] = function(err, result, ctx, config)
          if not err and result then
            -- Add mtlog quick fixes as code actions
          local diagnostics = require('mtlog.diagnostics')
          local diag = diagnostics.get_diagnostic_at_cursor(bufnr)
          
          if diag and diag.user_data and diag.user_data.suggested_fixes then
            result = result or {}
            
            for i, fix in ipairs(diag.user_data.suggested_fixes) do
              table.insert(result, {
                title = fix.description or fix.title or 'mtlog: Apply fix',
                kind = 'quickfix',
                diagnostics = { diag },
                command = {
                  title = 'Apply mtlog fix',
                  command = 'mtlog.applyFix',
                  arguments = { diag, i },
                },
              })
            end
          end
        end
        
        -- Call original handler
        if original_handler then
          original_handler(err, result, ctx, config)
        end
      end
    end
  end,
  })
  
  -- Register LSP command for applying fixes
  vim.lsp.commands['mtlog.applyFix'] = function(command, ctx)
  local diagnostics = require('mtlog.diagnostics')
  local diag = command.arguments[1]
  local fix_index = command.arguments[2]
  
    if diagnostics.apply_suggested_fix(diag, fix_index) then
      vim.notify('Applied mtlog fix', vim.log.levels.INFO)
      -- Re-analyze buffer
      require('mtlog').analyze_buffer(ctx.bufnr)
    else
      vim.notify('Failed to apply mtlog fix', vim.log.levels.ERROR)
    end
  end
end