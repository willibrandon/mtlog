local M = {}

-- Create a With() diagnostic object for testing
function M.create_with_diagnostic(opts)
  local defaults = {
    lnum = (opts.line or 1) - 1,
    col = (opts.col or 0) - 1,
    end_lnum = opts.end_line and (opts.end_line - 1) or (opts.line or 1) - 1,
    end_col = opts.end_col and (opts.end_col - 1) or (opts.col or 0) - 1,
    message = string.format('[%s] %s', opts.code, opts.message or M.get_default_message(opts.code)),
    severity = opts.severity or M.get_default_severity(opts.code),
    source = 'mtlog-analyzer',
    code = opts.code,
    user_data = opts.fixes and { suggested_fixes = opts.fixes } or nil
  }
  return vim.tbl_extend('force', defaults, opts.override or {})
end

-- Create a pos/end format edit
function M.create_pos_end_edit(start_pos, end_pos, new_text)
  return {
    pos = start_pos,
    ['end'] = end_pos,
    newText = new_text
  }
end

-- Get default message for a diagnostic code
function M.get_default_message(code)
  local messages = {
    MTLOG009 = "With() requires an even number of arguments",
    MTLOG010 = "With() key must be a string",
    MTLOG011 = "Property already set in previous With() call",
    MTLOG012 = "Reserved property name",
    MTLOG013 = "With() key cannot be an empty string",
  }
  return messages[code] or "Unknown diagnostic"
end

-- Get default severity for a diagnostic code
function M.get_default_severity(code)
  local severities = {
    MTLOG009 = vim.diagnostic.severity.ERROR,
    MTLOG010 = vim.diagnostic.severity.WARN,
    MTLOG011 = vim.diagnostic.severity.INFO,
    MTLOG012 = vim.diagnostic.severity.WARN,
    MTLOG013 = vim.diagnostic.severity.ERROR,
  }
  return severities[code] or vim.diagnostic.severity.ERROR
end

return M