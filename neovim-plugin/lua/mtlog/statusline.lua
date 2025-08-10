-- Statusline integration module for mtlog.nvim

local M = {}

local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

-- Icons for different severity levels
local icons = {
  error = '✗',
  warning = '⚠',
  info = 'ℹ',
  hint = '💡',
}

-- Nerd font icons (optional)
local nerd_icons = {
  error = '',
  warning = '',
  info = '',
  hint = '',
}

-- Get icon for severity
---@param severity string Severity name
---@param use_nerd_fonts boolean Use nerd fonts
---@return string Icon
local function get_icon(severity, use_nerd_fonts)
  if use_nerd_fonts then
    return nerd_icons[severity] or severity:sub(1, 1):upper()
  else
    return icons[severity] or severity:sub(1, 1):upper()
  end
end

-- Get statusline component
---@param opts table? Options
---@return string Statusline component
function M.get_component(opts)
  opts = opts or {}
  
  local bufnr = opts.bufnr or vim.api.nvim_get_current_buf()
  local use_nerd_fonts = opts.nerd_fonts ~= false
  local show_zero = opts.show_zero
  local separator = opts.separator or ' '
  local format = opts.format or 'short'  -- 'short', 'long', 'minimal'
  
  -- Get counts
  local counts = diagnostics.get_counts(bufnr)
  
  -- Return empty if no diagnostics and not showing zero
  if counts.total == 0 and not show_zero then
    return ''
  end
  
  local parts = {}
  
  if format == 'minimal' then
    -- Just show total count with icon
    if counts.total > 0 then
      local icon = counts.errors > 0 and get_icon('error', use_nerd_fonts)
        or counts.warnings > 0 and get_icon('warning', use_nerd_fonts)
        or counts.info > 0 and get_icon('info', use_nerd_fonts)
        or get_icon('hint', use_nerd_fonts)
      
      table.insert(parts, string.format('%s %d', icon, counts.total))
    end
  elseif format == 'short' then
    -- Show counts for each severity with icons
    if counts.errors > 0 or show_zero then
      table.insert(parts, string.format('%s %d', get_icon('error', use_nerd_fonts), counts.errors))
    end
    if counts.warnings > 0 or show_zero then
      table.insert(parts, string.format('%s %d', get_icon('warning', use_nerd_fonts), counts.warnings))
    end
    if counts.info > 0 and opts.show_info ~= false then
      table.insert(parts, string.format('%s %d', get_icon('info', use_nerd_fonts), counts.info))
    end
    if counts.hints > 0 and opts.show_hints ~= false then
      table.insert(parts, string.format('%s %d', get_icon('hint', use_nerd_fonts), counts.hints))
    end
  else  -- format == 'long'
    -- Show descriptive text
    if counts.errors > 0 then
      table.insert(parts, string.format('%d error%s', counts.errors, counts.errors > 1 and 's' or ''))
    end
    if counts.warnings > 0 then
      table.insert(parts, string.format('%d warning%s', counts.warnings, counts.warnings > 1 and 's' or ''))
    end
    if counts.info > 0 and opts.show_info ~= false then
      table.insert(parts, string.format('%d info', counts.info))
    end
    if counts.hints > 0 and opts.show_hints ~= false then
      table.insert(parts, string.format('%d hint%s', counts.hints, counts.hints > 1 and 's' or ''))
    end
  end
  
  if #parts == 0 then
    return ''
  end
  
  -- Add prefix/suffix
  local result = table.concat(parts, separator)
  if opts.prefix then
    result = opts.prefix .. result
  end
  if opts.suffix then
    result = result .. opts.suffix
  end
  
  return result
end

-- Simple status function for statusline (no parameters needed)
---@return string
function M.status()
  local counts = diagnostics.get_counts()
  return string.format('E:%d W:%d', counts.errors, counts.warnings)
end

-- Get colored statusline component
---@param opts table? Options
---@return string Statusline component with highlight groups
function M.get_colored_component(opts)
  opts = opts or {}
  
  local bufnr = opts.bufnr or vim.api.nvim_get_current_buf()
  local use_nerd_fonts = opts.nerd_fonts ~= false
  local show_zero = opts.show_zero
  local separator = opts.separator or ' '
  
  -- Get counts
  local counts = diagnostics.get_counts(bufnr)
  
  -- Return empty if no diagnostics and not showing zero
  if counts.total == 0 and not show_zero then
    return ''
  end
  
  local parts = {}
  
  -- Add error count
  if counts.errors > 0 or show_zero then
    table.insert(parts, string.format(
      '%%#DiagnosticError#%s %d%%*',
      get_icon('error', use_nerd_fonts),
      counts.errors
    ))
  end
  
  -- Add warning count
  if counts.warnings > 0 or show_zero then
    table.insert(parts, string.format(
      '%%#DiagnosticWarn#%s %d%%*',
      get_icon('warning', use_nerd_fonts),
      counts.warnings
    ))
  end
  
  -- Add info count
  if (counts.info > 0 or show_zero) and opts.show_info ~= false then
    table.insert(parts, string.format(
      '%%#DiagnosticInfo#%s %d%%*',
      get_icon('info', use_nerd_fonts),
      counts.info
    ))
  end
  
  -- Add hint count
  if (counts.hints > 0 or show_zero) and opts.show_hints ~= false then
    table.insert(parts, string.format(
      '%%#DiagnosticHint#%s %d%%*',
      get_icon('hint', use_nerd_fonts),
      counts.hints
    ))
  end
  
  if #parts == 0 then
    return ''
  end
  
  return table.concat(parts, separator)
end

-- Get current diagnostic at cursor for statusline
---@param opts table? Options
---@return string Diagnostic message or empty
function M.get_current_diagnostic(opts)
  opts = opts or {}
  
  local bufnr = opts.bufnr or vim.api.nvim_get_current_buf()
  local max_length = opts.max_length or 50
  local show_code = opts.show_code ~= false
  
  local diag = diagnostics.get_diagnostic_at_cursor(bufnr)
  if not diag then
    return ''
  end
  
  local msg = diag.message
  
  -- Add code if requested
  if show_code and diag.code then
    msg = string.format('[%s] %s', diag.code, msg)
  end
  
  -- Truncate if too long
  if #msg > max_length then
    msg = msg:sub(1, max_length - 3) .. '...'
  end
  
  -- Add severity color if requested
  if opts.colored then
    local hl = diagnostics.severity_highlight(diag.severity)
    msg = string.format('%%#%s#%s%%*', hl, msg)
  end
  
  return msg
end

-- Get analyzer status for statusline
---@param opts table? Options
---@return string Status component
function M.get_analyzer_status(opts)
  opts = opts or {}
  
  local mtlog = require('mtlog')
  
  if not mtlog.initialized() then
    return ''
  end
  
  if not mtlog.enabled() then
    if opts.show_disabled then
      return opts.colored and '%#Comment#[mtlog: disabled]%*' or '[mtlog: disabled]'
    end
    return ''
  end
  
  -- Check if analyzer is available
  if not mtlog.is_available() then
    if opts.show_errors then
      return opts.colored and '%#DiagnosticError#[mtlog: not found]%*' or '[mtlog: not found]'
    end
    return ''
  end
  
  -- Show version if requested
  if opts.show_version then
    local version = mtlog.get_version()
    if version then
      return opts.colored and string.format('%%#Comment#[mtlog: v%s]%%*', version) 
        or string.format('[mtlog: v%s]', version)
    end
  end
  
  -- Default: show active status
  if opts.show_active ~= false then
    return opts.colored and '%#DiagnosticOk#[mtlog]%*' or '[mtlog]'
  end
  
  return ''
end

-- Lualine integration
---@return table Lualine component
function M.lualine_component()
  return {
    function()
      return M.get_component({
        nerd_fonts = true,
        format = 'short',
        show_info = false,
        show_hints = false,
      })
    end,
    cond = function()
      local counts = diagnostics.get_counts()
      return counts.total > 0
    end,
    color = { gui = 'bold' },
  }
end

-- Feline integration
---@return table Feline component
function M.feline_component()
  return {
    provider = function()
      return M.get_component({
        nerd_fonts = true,
        format = 'short',
        prefix = ' ',
        suffix = ' ',
      })
    end,
    enabled = function()
      local counts = diagnostics.get_counts()
      return counts.total > 0
    end,
    hl = function()
      local counts = diagnostics.get_counts()
      if counts.errors > 0 then
        return 'DiagnosticError'
      elseif counts.warnings > 0 then
        return 'DiagnosticWarn'
      else
        return 'DiagnosticHint'
      end
    end,
  }
end

-- Galaxyline integration
---@return table Galaxyline component
function M.galaxyline_component()
  return {
    MtlogDiagnostics = {
      provider = function()
        return M.get_component({
          nerd_fonts = true,
          format = 'short',
          prefix = ' ',
        })
      end,
      condition = function()
        local counts = diagnostics.get_counts()
        return counts.total > 0
      end,
      highlight = { 'DiagnosticError', 'StatusLine' },
    },
  }
end

return M