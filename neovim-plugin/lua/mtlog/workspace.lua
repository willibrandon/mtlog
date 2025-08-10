-- Workspace configuration persistence for mtlog.nvim

local M = {}

local utils = require('mtlog.utils')
local config = require('mtlog.config')

-- Default workspace config filename
local WORKSPACE_CONFIG_FILE = '.mtlog.json'

-- Find workspace configuration file
---@return string? Path to configuration file or nil
function M.find_config_file()
  local cwd = vim.fn.getcwd()
  local path = cwd
  
  while path and path ~= '/' do
    local config_file = path .. '/' .. WORKSPACE_CONFIG_FILE
    if vim.fn.filereadable(config_file) == 1 then
      return config_file
    end
    
    -- Stop at git root
    local git_dir = path .. '/.git'
    if vim.fn.isdirectory(git_dir) == 1 then
      break
    end
    
    -- Move to parent directory
    local parent = vim.fn.fnamemodify(path, ':h')
    if parent == path then
      break
    end
    path = parent
  end
  
  return nil
end

-- Get workspace config file path (creates in git root or cwd)
---@return string Path to workspace config file
function M.get_config_path()
  local existing = M.find_config_file()
  if existing then
    return existing
  end
  
  -- Create in git root if available
  local git_root = utils.get_git_root()
  if git_root then
    return git_root .. '/' .. WORKSPACE_CONFIG_FILE
  end
  
  -- Otherwise create in current directory
  return vim.fn.getcwd() .. '/' .. WORKSPACE_CONFIG_FILE
end

-- Load workspace configuration
---@return table Configuration table
function M.load()
  local config_file = M.find_config_file()
  if not config_file then
    return {}
  end
  
  local content = utils.read_file(config_file)
  if not content then
    return {}
  end
  
  local ok, data = pcall(vim.json.decode, content)
  if not ok then
    vim.notify('Failed to parse ' .. WORKSPACE_CONFIG_FILE, vim.log.levels.WARN)
    return {}
  end
  
  return data or {}
end

-- Save workspace configuration
---@param data table Configuration to save
---@return boolean Success
function M.save(data)
  local config_path = M.get_config_path()
  
  local ok, json = pcall(vim.json.encode, data)
  if not ok then
    vim.notify('Failed to encode workspace config', vim.log.levels.ERROR)
    return false
  end
  
  -- Pretty format the JSON
  local formatted = vim.fn.system('jq . 2>/dev/null', json)
  if vim.v.shell_error ~= 0 then
    -- Fallback to unformatted if jq is not available
    formatted = json
  end
  
  if not utils.write_file(config_path, formatted) then
    vim.notify('Failed to write ' .. config_path, vim.log.levels.ERROR)
    return false
  end
  
  return true
end

-- Apply workspace configuration to current config
function M.apply()
  local workspace_config = M.load()
  
  if workspace_config.suppressed_diagnostics then
    config.set('suppressed_diagnostics', workspace_config.suppressed_diagnostics)
  end
  
  if workspace_config.analyzer_flags then
    config.set('analyzer_flags', workspace_config.analyzer_flags)
  end
  
  if workspace_config.diagnostics_enabled ~= nil then
    config.set('diagnostics_enabled', workspace_config.diagnostics_enabled)
  end
  
  if workspace_config.severity_levels then
    local current_levels = config.get('severity_levels') or {}
    config.set('severity_levels', vim.tbl_extend('force', current_levels, workspace_config.severity_levels))
  end
end

-- Save current suppressions to workspace config
function M.save_suppressions()
  local workspace_config = M.load()
  workspace_config.suppressed_diagnostics = config.get('suppressed_diagnostics') or {}
  
  if M.save(workspace_config) then
    vim.notify('Saved suppressions to ' .. WORKSPACE_CONFIG_FILE, vim.log.levels.INFO)
  end
end

-- Load suppressions from workspace config
function M.load_suppressions()
  local workspace_config = M.load()
  
  if workspace_config.suppressed_diagnostics then
    config.set('suppressed_diagnostics', workspace_config.suppressed_diagnostics)
    vim.notify('Loaded suppressions from ' .. WORKSPACE_CONFIG_FILE, vim.log.levels.INFO)
    
    -- Re-analyze to apply suppressions
    require('mtlog').reanalyze_all()
  else
    vim.notify('No suppressions found in workspace config', vim.log.levels.INFO)
  end
end

-- Check if workspace has config file
---@return boolean
function M.has_config()
  return M.find_config_file() ~= nil
end

return M