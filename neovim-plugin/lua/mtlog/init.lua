-- mtlog.nvim - Neovim plugin for mtlog-analyzer
-- Main entry point and public API

local M = {}

-- Module dependencies
local config = require('mtlog.config')
local analyzer = require('mtlog.analyzer')
local diagnostics = require('mtlog.diagnostics')
local utils = require('mtlog.utils')
local cache = require('mtlog.cache')

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
  
  -- Initialize diagnostics namespace
  diagnostics.setup()
  
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
        callback = function()
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
        if not utils.is_vendor_path(args.file) then
          debounced_analyze(args.buf)
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
  
  -- Run analyzer (pass bufnr to ensure correct buffer content is used)
  analyzer.analyze_file(filepath, function(results, err)
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
  end, bufnr)
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
  
  for _, filepath in ipairs(go_files) do
    analyzer.analyze_file(filepath, function(results, err)
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
    end)
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

-- Export public API
M.initialized = function() return initialized end
M.enabled = function() return enabled end

return M