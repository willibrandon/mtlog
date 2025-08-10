-- Configuration module for mtlog.nvim

local M = {}

-- Default configuration
local defaults = {
  -- Path to mtlog-analyzer executable
  analyzer_path = 'mtlog-analyzer',
  
  -- Global diagnostics kill switch
  diagnostics_enabled = true,
  
  -- Automatically enable for Go projects
  auto_enable = true,
  
  -- Automatically analyze on save/change
  auto_analyze = true,
  
  -- Debounce time in milliseconds
  debounce_ms = 500,
  
  -- Virtual text configuration
  virtual_text = {
    enabled = true,
    prefix = '■ ',
    spacing = 2,
    severity_limit = vim.diagnostic.severity.HINT,
  },
  
  -- Sign column configuration
  signs = {
    enabled = true,
    priority = 10,
    text = {
      [vim.diagnostic.severity.ERROR] = '✗',
      [vim.diagnostic.severity.WARN] = '⚠',
      [vim.diagnostic.severity.INFO] = 'ℹ',
      [vim.diagnostic.severity.HINT] = '💡',
    },
  },
  
  -- Underline configuration
  underline = {
    enabled = true,
    severity_limit = vim.diagnostic.severity.WARN,
  },
  
  -- Severity level mappings for diagnostic codes
  severity_levels = {
    MTLOG001 = vim.diagnostic.severity.ERROR,  -- Template/argument mismatch
    MTLOG002 = vim.diagnostic.severity.ERROR,  -- Invalid format specifier
    MTLOG003 = vim.diagnostic.severity.ERROR,  -- Missing error in Error/Fatal
    MTLOG004 = vim.diagnostic.severity.WARN,   -- Non-PascalCase property
    MTLOG005 = vim.diagnostic.severity.WARN,   -- Complex type needs LogValue
    MTLOG006 = vim.diagnostic.severity.WARN,   -- Duplicate property
    MTLOG007 = vim.diagnostic.severity.HINT,   -- String context key suggestion
    MTLOG008 = vim.diagnostic.severity.INFO,   -- General info
  },
  
  -- Rate limiting configuration
  rate_limit = {
    enabled = true,
    max_files_per_second = 10,
  },
  
  -- Cache configuration
  cache = {
    enabled = true,
    ttl_seconds = 300,  -- 5 minutes
  },
  
  -- Show error notifications
  show_errors = false,  -- Set to false by default to avoid startup errors
  
  -- Analyzer command flags
  analyzer_flags = {},
  
  -- Suppressed diagnostic IDs
  suppressed_diagnostics = {},
  
  -- File patterns to ignore
  ignore_patterns = {
    'vendor/',
    '%.pb%.go$',
    '_test%.go$',
  },
  
  -- Keybinding configuration
  keymaps = {
    toggle = '<leader>mt',
    suppress = '<leader>ms', 
    quick_fix = '<leader>mf',
    manage_suppressions = '<leader>mS',
  },
  
  -- Performance settings
  performance = {
    max_concurrent = nil,  -- Max concurrent analyses (nil = CPU count - 1)
  },
  
  -- Debug settings
  debug = {
    log_level = 'INFO',  -- ERROR, WARN, INFO, DEBUG
    show_analyzer_output = false,
  },
  
  -- Auto-command patterns
  autocmds = {
    disable_in_test_files = true,  -- Auto-disable in _test.go files
    disable_patterns = {},  -- File patterns to auto-disable
    enable_patterns = {},  -- File patterns to auto-enable
  },
  
  -- Diagnostic float configuration
  float = {
    focusable = false,
    style = 'minimal',
    border = 'rounded',
    source = 'always',
    header = '',
    prefix = '',
  },
}

-- Current configuration
local config = {}

-- Validate configuration
local function validate_config(opts)
  vim.validate({
    analyzer_path = { opts.analyzer_path, 'string', true },
    diagnostics_enabled = { opts.diagnostics_enabled, 'boolean', true },
    auto_enable = { opts.auto_enable, 'boolean', true },
    auto_analyze = { opts.auto_analyze, 'boolean', true },
    debounce_ms = { opts.debounce_ms, 'number', true },
    virtual_text = { opts.virtual_text, { 'table', 'boolean' }, true },
    signs = { opts.signs, { 'table', 'boolean' }, true },
    underline = { opts.underline, { 'table', 'boolean' }, true },
    severity_levels = { opts.severity_levels, 'table', true },
    rate_limit = { opts.rate_limit, { 'table', 'boolean' }, true },
    cache = { opts.cache, { 'table', 'boolean' }, true },
    show_errors = { opts.show_errors, 'boolean', true },
    analyzer_flags = { opts.analyzer_flags, 'table', true },
    suppressed_diagnostics = { opts.suppressed_diagnostics, 'table', true },
    ignore_patterns = { opts.ignore_patterns, 'table', true },
    float = { opts.float, 'table', true },
    keymaps = { opts.keymaps, 'table', true },
    performance = { opts.performance, 'table', true },
    debug = { opts.debug, 'table', true },
    autocmds = { opts.autocmds, 'table', true },
  })
  
  -- Validate nested tables
  if type(opts.virtual_text) == 'table' then
    vim.validate({
      ['virtual_text.enabled'] = { opts.virtual_text.enabled, 'boolean', true },
      ['virtual_text.prefix'] = { opts.virtual_text.prefix, 'string', true },
      ['virtual_text.spacing'] = { opts.virtual_text.spacing, 'number', true },
      ['virtual_text.severity_limit'] = { opts.virtual_text.severity_limit, 'number', true },
    })
  end
  
  if type(opts.signs) == 'table' then
    vim.validate({
      ['signs.enabled'] = { opts.signs.enabled, 'boolean', true },
      ['signs.priority'] = { opts.signs.priority, 'number', true },
      ['signs.text'] = { opts.signs.text, 'table', true },
    })
  end
  
  if type(opts.rate_limit) == 'table' then
    vim.validate({
      ['rate_limit.enabled'] = { opts.rate_limit.enabled, 'boolean', true },
      ['rate_limit.max_files_per_second'] = { opts.rate_limit.max_files_per_second, 'number', true },
    })
  end
  
  if type(opts.cache) == 'table' then
    vim.validate({
      ['cache.enabled'] = { opts.cache.enabled, 'boolean', true },
      ['cache.ttl_seconds'] = { opts.cache.ttl_seconds, 'number', true },
    })
  end
  
  return true
end

-- Deep merge two tables
local function deep_merge(t1, t2)
  local result = vim.tbl_deep_extend('force', {}, t1)
  
  for k, v in pairs(t2) do
    if type(v) == 'table' and type(result[k]) == 'table' then
      -- Handle boolean shortcuts for virtual_text, signs, etc.
      if k == 'virtual_text' or k == 'signs' or k == 'underline' then
        if type(v) == 'boolean' then
          result[k].enabled = v
        else
          result[k] = vim.tbl_deep_extend('force', result[k], v)
        end
      elseif k == 'rate_limit' or k == 'cache' then
        if type(v) == 'boolean' then
          result[k].enabled = v
        else
          result[k] = vim.tbl_deep_extend('force', result[k], v)
        end
      else
        result[k] = vim.tbl_deep_extend('force', result[k], v)
      end
    else
      result[k] = v
    end
  end
  
  return result
end

-- Setup configuration
---@param opts table? User configuration
function M.setup(opts)
  opts = opts or {}
  
  -- Validate user options
  if not pcall(validate_config, opts) then
    vim.notify('mtlog.nvim: Invalid configuration', vim.log.levels.ERROR)
    return
  end
  
  -- Merge with defaults
  config = deep_merge(defaults, opts)
  
  -- Handle boolean shortcuts
  if type(config.virtual_text) == 'boolean' then
    config.virtual_text = vim.tbl_extend('force', defaults.virtual_text, { enabled = config.virtual_text })
  end
  
  if type(config.signs) == 'boolean' then
    config.signs = vim.tbl_extend('force', defaults.signs, { enabled = config.signs })
  end
  
  if type(config.underline) == 'boolean' then
    config.underline = vim.tbl_extend('force', defaults.underline, { enabled = config.underline })
  end
  
  if type(config.rate_limit) == 'boolean' then
    config.rate_limit = vim.tbl_extend('force', defaults.rate_limit, { enabled = config.rate_limit })
  end
  
  if type(config.cache) == 'boolean' then
    config.cache = vim.tbl_extend('force', defaults.cache, { enabled = config.cache })
  end
end

-- Get configuration value
---@param key string? Configuration key (dot notation supported)
---@return any Configuration value
function M.get(key)
  if not key then
    return config
  end
  
  local value = config
  for part in key:gmatch('[^.]+') do
    if type(value) ~= 'table' then
      return nil
    end
    value = value[part]
  end
  
  return value
end

-- Update configuration value
---@param key string Configuration key (dot notation supported)
---@param value any New value
function M.set(key, value)
  local parts = {}
  for part in key:gmatch('[^.]+') do
    table.insert(parts, part)
  end
  
  if #parts == 0 then
    return
  end
  
  local current = config
  for i = 1, #parts - 1 do
    local part = parts[i]
    if type(current[part]) ~= 'table' then
      current[part] = {}
    end
    current = current[part]
  end
  
  current[parts[#parts]] = value
end

-- Get diagnostic handler options
---@return table Diagnostic handler options
function M.get_diagnostic_opts()
  local opts = {
    severity_sort = true,
    update_in_insert = false,
  }
  
  -- Virtual text
  if config.virtual_text.enabled then
    opts.virtual_text = {
      prefix = config.virtual_text.prefix,
      spacing = config.virtual_text.spacing,
      severity = { min = config.virtual_text.severity_limit },
    }
  else
    opts.virtual_text = false
  end
  
  -- Signs
  if config.signs.enabled then
    opts.signs = {
      priority = config.signs.priority,
      text = config.signs.text,
    }
  else
    opts.signs = false
  end
  
  -- Underline
  if config.underline.enabled then
    opts.underline = {
      severity = { min = config.underline.severity_limit },
    }
  else
    opts.underline = false
  end
  
  -- Float
  opts.float = config.float
  
  return opts
end

return M