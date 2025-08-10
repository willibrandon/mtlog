-- mtlog.nvim - Neovim plugin for mtlog-analyzer
-- Main entry point and public API

local M = {}

-- Module dependencies
local config = require('mtlog.config')
local analyzer = require('mtlog.analyzer')
local diagnostics = require('mtlog.diagnostics')
local utils = require('mtlog.utils')
local cache = require('mtlog.cache')
local queue = require('mtlog.queue')
local context = require('mtlog.context')

-- Plugin state
local initialized = false
local enabled = false
local autocmd_group = nil

-- Setup function with configuration options
---@param opts table? User configuration options
function M.setup(opts)
  if initialized then
    return
  end

  -- Load and validate configuration
  config.setup(opts)
  
  -- Load workspace configuration if available
  local workspace = require('mtlog.workspace')
  if workspace.has_config() then
    workspace.apply()
  end
  
  -- Initialize diagnostics namespace
  diagnostics.setup()
  
  -- Initialize queue system
  queue.setup()
  
  -- Initialize context rules
  context.setup()
  
  -- Create autocmd group
  autocmd_group = vim.api.nvim_create_augroup('MtlogAnalyzer', { clear = true })
  
  -- Smart activation for Go projects
  if config.get('auto_enable') then
    if utils.is_go_project() then
      M.enable()
    else
      -- Set up autocmd to check when entering Go buffers
      vim.api.nvim_create_autocmd({ 'BufEnter', 'BufNewFile' }, {
        group = autocmd_group,
        pattern = '*.go',
        callback = function(args)
          -- Apply context rules first
          if context.apply_context(args.buf) then
            return
          end
          
          -- Default behavior
          if utils.is_go_project() and not enabled then
            M.enable()
          end
        end,
      })
    end
  end
  
  initialized = true
end

-- Enable the analyzer
function M.enable()
  if enabled then
    return
  end
  
  enabled = true
  
  -- Set up autocmds for automatic analysis
  if config.get('auto_analyze') then
    local debounced_analyze = utils.debounce(function(bufnr)
      if vim.api.nvim_buf_is_valid(bufnr) then
        M.analyze_buffer(bufnr)
      end
    end, config.get('debounce_ms'))
    
    vim.api.nvim_create_autocmd({ 'BufWritePost', 'TextChanged', 'InsertLeave' }, {
      group = autocmd_group,
      pattern = '*.go',
      callback = function(args)
        -- Check context rules
        if not context.should_analyze(args.buf) then
          return
        end
        
        if not utils.is_vendor_path(args.file) then
          debounced_analyze(args.buf)
        end
      end,
    })
    
    -- Cancel pending analyses when leaving a buffer quickly
    vim.api.nvim_create_autocmd('BufLeave', {
      group = autocmd_group,
      pattern = '*.go',
      callback = function(args)
        -- Cancel low-priority analyses for this file if switching buffers
        local filepath = vim.api.nvim_buf_get_name(args.buf)
        if filepath and filepath ~= '' then
          -- Only cancel if there are other pending analyses
          local stats = queue.get_stats()
          if stats.pending > 2 then
            queue.cancel_file(filepath)
          end
        end
      end,
    })
  end
  
  -- Skip initial analysis - user will run :MtlogAnalyze manually
  -- This avoids errors when analyzer path isn't configured yet
  --[[
  for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(bufnr) then
      local name = vim.api.nvim_buf_get_name(bufnr)
      if name:match('%.go$') and not utils.is_vendor_path(name) then
        M.analyze_buffer(bufnr)
      end
    end
  end
  --]]
end

-- Disable the analyzer
function M.disable()
  if not enabled then
    return
  end
  
  enabled = false
  
  -- Clear autocmds
  if autocmd_group then
    vim.api.nvim_clear_autocmds({ group = autocmd_group })
  end
  
  -- Clear all diagnostics
  diagnostics.clear_all()
end

-- Analyze a specific buffer
---@param bufnr number? Buffer number (defaults to current)
function M.analyze_buffer(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  
  -- Check global kill switch
  if not config.get('diagnostics_enabled') then
    diagnostics.clear(bufnr)
    return
  end
  
  -- Check context rules
  if not context.should_analyze(bufnr) then
    return
  end
  
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  
  if vim.g.mtlog_debug then
    local lines = vim.api.nvim_buf_get_lines(bufnr, 48, 50, false)
    if lines and lines[1] then
      vim.notify(string.format('analyze_buffer: Line 49 is: %s', lines[1]), vim.log.levels.INFO)
    end
  end
  
  local filepath = vim.api.nvim_buf_get_name(bufnr)
  if filepath == '' or not filepath:match('%.go$') then
    return
  end
  
  if utils.is_vendor_path(filepath) then
    return
  end
  
  -- vim.notify('Analyzing: ' .. filepath, vim.log.levels.INFO)
  
  -- Check cache first (but skip if buffer has unsaved changes)
  if not vim.bo[bufnr].modified then
    local cached = cache.get(filepath)
    if cached then
      -- vim.notify('Using cached results', vim.log.levels.INFO)
      diagnostics.set(bufnr, cached)
      return
    end
  end
  
  -- Determine priority based on buffer visibility
  local priority = queue.priority.NORMAL
  if bufnr == vim.api.nvim_get_current_buf() then
    priority = queue.priority.HIGH
  elseif vim.fn.bufwinid(bufnr) ~= -1 then
    priority = queue.priority.NORMAL
  else
    priority = queue.priority.LOW
  end
  
  -- Queue the analysis with priority
  queue.enqueue(filepath, function(results, err)
    -- Already wrapped in vim.schedule in the analyzer callback
    if err then
      -- Always show errors when manually running command
      vim.notify('mtlog-analyzer: ' .. err, vim.log.levels.ERROR)
      return
    end
    
    if results and #results > 0 then
      -- Cache results
      cache.set(filepath, results)
      
      -- Set diagnostics - this will display them immediately
      diagnostics.set(bufnr, results)
      
      -- Show notification
      vim.notify(string.format('Found %d diagnostic%s', #results, #results == 1 and '' or 's'), vim.log.levels.INFO)
    else
      -- Clear any existing diagnostics
      diagnostics.clear(bufnr)
      vim.notify('No issues found', vim.log.levels.INFO)
    end
  end, { bufnr = bufnr, priority = priority })
end

-- Analyze entire workspace
function M.analyze_workspace()
  local go_files = utils.get_go_files()
  
  if #go_files == 0 then
    vim.notify('No Go files found in workspace', vim.log.levels.WARN)
    return
  end
  
  local progress = 0
  local total = #go_files
  
  -- Show progress
  local progress_id = 'mtlog_workspace_analysis'
  vim.notify('Analyzing workspace...', vim.log.levels.INFO, {
    title = 'mtlog-analyzer',
    icon = '🔍',
    replace = progress_id,
  })
  
  -- Queue all files with low priority for workspace analysis
  for _, filepath in ipairs(go_files) do
    queue.enqueue(filepath, function(results, err)
      progress = progress + 1
      
      if not err and results then
        -- Cache results
        cache.set(filepath, results)
        
        -- Find buffer if open
        for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
          if vim.api.nvim_buf_get_name(bufnr) == filepath then
            diagnostics.set(bufnr, results)
            break
          end
        end
      end
      
      -- Update progress
      if progress < total then
        vim.notify(string.format('Analyzing workspace... (%d/%d)', progress, total), 
          vim.log.levels.INFO, {
            title = 'mtlog-analyzer',
            icon = '🔍',
            replace = progress_id,
          })
      else
        vim.notify('Workspace analysis complete', vim.log.levels.INFO, {
          title = 'mtlog-analyzer',
          icon = '✅',
          replace = progress_id,
        })
      end
    end, { priority = queue.priority.LOW })
  end
end

-- Clear diagnostics for a buffer
---@param bufnr number? Buffer number (defaults to current)
function M.clear(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  diagnostics.clear(bufnr)
end

-- Clear all diagnostics
function M.clear_all()
  diagnostics.clear_all()
end

-- Get diagnostic counts for statusline
---@return table Counts by severity
function M.get_counts()
  return diagnostics.get_counts()
end

-- Check if analyzer is available
---@return boolean
function M.is_available()
  return analyzer.is_available()
end

-- Get analyzer version
---@return string? Version string or nil
function M.get_version()
  return analyzer.get_version()
end

-- Toggle enabled state
function M.toggle()
  if enabled then
    M.disable()
    vim.notify('mtlog-analyzer disabled', vim.log.levels.INFO)
  else
    M.enable()
    vim.notify('mtlog-analyzer enabled', vim.log.levels.INFO)
  end
end

-- Toggle diagnostics kill switch
function M.toggle_diagnostics()
  local current = config.get('diagnostics_enabled')
  config.set('diagnostics_enabled', not current)
  
  if not current then
    vim.notify('mtlog diagnostics enabled', vim.log.levels.INFO)
    -- Re-analyze all buffers
    for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
      if vim.api.nvim_buf_is_loaded(bufnr) then
        local name = vim.api.nvim_buf_get_name(bufnr)
        if name:match('%.go$') and not utils.is_vendor_path(name) then
          M.analyze_buffer(bufnr)
        end
      end
    end
  else
    vim.notify('mtlog diagnostics disabled', vim.log.levels.INFO)
    -- Clear all diagnostics
    diagnostics.clear_all()
  end
end

-- Suppress a diagnostic ID
---@param diagnostic_id string? Diagnostic ID to suppress (e.g., 'MTLOG001')
---@param skip_prompt boolean? Skip the workspace save prompt (for tests/programmatic use)
function M.suppress_diagnostic(diagnostic_id, skip_prompt)
  if not diagnostic_id then
    -- Try to get from cursor position
    local diag = diagnostics.get_diagnostic_at_cursor()
    if diag and diag.code then
      diagnostic_id = diag.code
    else
      -- Prompt user
      vim.ui.input({
        prompt = 'Enter diagnostic ID to suppress (e.g., MTLOG001): ',
      }, function(input)
        if input and input:match('^MTLOG%d%d%d$') then
          M.suppress_diagnostic(input)
        end
      end)
      return
    end
  end
  
  local suppressed = config.get('suppressed_diagnostics') or {}
  if not vim.tbl_contains(suppressed, diagnostic_id) then
    table.insert(suppressed, diagnostic_id)
    config.set('suppressed_diagnostics', suppressed)
    vim.notify(string.format('Suppressed diagnostic %s', diagnostic_id), vim.log.levels.INFO)
    
    -- Optionally save to workspace config (skip in tests or when running headless)
    if not skip_prompt and vim.fn.has('nvim-0.7') == 1 and not vim.opt.headless:get() then
      vim.ui.select({'Yes', 'No'}, {
        prompt = 'Save suppression to workspace config?',
      }, function(choice)
        if choice == 'Yes' then
          local workspace = require('mtlog.workspace')
          workspace.save_suppressions()
        end
      end)
    end
    
    -- Re-analyze to apply suppression
    M.reanalyze_all()
  else
    vim.notify(string.format('%s is already suppressed', diagnostic_id), vim.log.levels.WARN)
  end
end

-- Unsuppress a diagnostic ID
---@param diagnostic_id string? Diagnostic ID to unsuppress
function M.unsuppress_diagnostic(diagnostic_id)
  if not diagnostic_id then
    local suppressed = config.get('suppressed_diagnostics') or {}
    if #suppressed == 0 then
      vim.notify('No diagnostics are currently suppressed', vim.log.levels.INFO)
      return
    end
    
    vim.ui.select(suppressed, {
      prompt = 'Select diagnostic to unsuppress:',
      format_item = function(item)
        return string.format('%s - %s', item, utils.get_diagnostic_description(item))
      end,
    }, function(choice)
      if choice then
        M.unsuppress_diagnostic(choice)
      end
    end)
    return
  end
  
  local suppressed = config.get('suppressed_diagnostics') or {}
  local idx = nil
  for i, id in ipairs(suppressed) do
    if id == diagnostic_id then
      idx = i
      break
    end
  end
  
  if idx then
    table.remove(suppressed, idx)
    config.set('suppressed_diagnostics', suppressed)
    vim.notify(string.format('Unsuppressed diagnostic %s', diagnostic_id), vim.log.levels.INFO)
    
    -- Re-analyze to apply change
    M.reanalyze_all()
  else
    vim.notify(string.format('%s is not suppressed', diagnostic_id), vim.log.levels.WARN)
  end
end

-- Clear all suppressions
function M.unsuppress_all()
  config.set('suppressed_diagnostics', {})
  vim.notify('Cleared all diagnostic suppressions', vim.log.levels.INFO)
  M.reanalyze_all()
end

-- Show suppressed diagnostics
function M.show_suppressions()
  local suppressed = config.get('suppressed_diagnostics') or {}
  if #suppressed == 0 then
    vim.notify('No diagnostics are currently suppressed', vim.log.levels.INFO)
    return
  end
  
  local lines = { 'Suppressed mtlog diagnostics:', '' }
  for _, id in ipairs(suppressed) do
    table.insert(lines, string.format('  %s - %s', id, utils.get_diagnostic_description(id)))
  end
  
  vim.notify(table.concat(lines, '\n'), vim.log.levels.INFO)
end

-- Re-analyze all open buffers
function M.reanalyze_all()
  cache.clear()  -- Clear cache to force re-analysis
  for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(bufnr) then
      local name = vim.api.nvim_buf_get_name(bufnr)
      if name:match('%.go$') and not utils.is_vendor_path(name) then
        M.analyze_buffer(bufnr)
      end
    end
  end
end

-- Export public API
M.initialized = function() return initialized end
M.enabled = function() return enabled end

return M