-- Diagnostics management module for mtlog.nvim

local M = {}

local config = require('mtlog.config')

-- Namespace for mtlog diagnostics
local namespace = nil

-- Diagnostic counts by buffer
local buffer_counts = {}

-- Setup diagnostics namespace and handlers
function M.setup()
  -- Create namespace
  namespace = vim.api.nvim_create_namespace('mtlog-analyzer')
  
  -- Get configuration options
  local opts = config.get_diagnostic_opts()
  
  -- Force immediate display by setting these options
  opts.update_in_insert = false
  
  -- Ensure virtual_text, signs, and underline are properly configured
  if opts.virtual_text == nil then
    opts.virtual_text = true
  end
  if opts.signs == nil then
    opts.signs = true
  end
  if opts.underline == nil then
    opts.underline = true
  end
  
  -- Configure diagnostics for our namespace
  -- This is crucial for display to work
  vim.diagnostic.config(opts, namespace)
  
  -- Also ensure global diagnostic config allows display
  -- Some users might have diagnostics disabled globally
  local global_opts = vim.diagnostic.config()
  if global_opts and (global_opts.virtual_text == false and 
                      global_opts.signs == false and 
                      global_opts.underline == false) then
    -- Warn user that diagnostics are globally disabled
    vim.schedule(function()
      vim.notify('Warning: Diagnostics appear to be globally disabled. Run :lua vim.diagnostic.config({virtual_text=true, signs=true}) to enable them.', vim.log.levels.WARN)
    end)
  end
end

-- Get namespace
---@return number Namespace ID
function M.get_namespace()
  if not namespace then
    -- Check if namespace already exists before creating
    local existing_namespaces = vim.api.nvim_get_namespaces()
    for name, id in pairs(existing_namespaces) do
      if name == 'mtlog-analyzer' then
        namespace = id
        return namespace
      end
    end
    -- Create new namespace if it doesn't exist
    M.setup()
  end
  return namespace
end

-- Set diagnostics for a buffer
---@param bufnr number Buffer number
---@param diagnostics table List of diagnostics
function M.set(bufnr, diagnostics)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  
  -- Ensure namespace exists
  if not namespace then
    M.setup()
  end
  
  -- Clear and set diagnostics
  vim.diagnostic.set(namespace, bufnr, diagnostics)
  
  -- CRITICAL: Force diagnostics to show immediately
  -- This is necessary because Neovim sometimes doesn't display them right away
  vim.schedule(function()
    if vim.api.nvim_buf_is_valid(bufnr) then
      -- Show diagnostics for this namespace and buffer
      vim.diagnostic.show(namespace, bufnr)
      
      -- Force a redraw to ensure display updates
      vim.cmd('redraw')
    end
  end)
  
  -- Update counts
  M.update_counts(bufnr)
  
  -- Trigger user event (data field only available in 0.9+)
  if vim.fn.has('nvim-0.9') == 1 then
    vim.api.nvim_exec_autocmds('User', {
      pattern = 'MtlogDiagnosticsChanged',
      data = {
        bufnr = bufnr,
        count = #diagnostics,
      },
    })
  else
    -- For older versions, just trigger without data
    vim.cmd('doautocmd User MtlogDiagnosticsChanged')
  end
end

-- Clear diagnostics for a buffer
---@param bufnr number Buffer number
function M.clear(bufnr)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  
  -- Ensure namespace exists
  if not namespace then
    M.setup()
  end
  
  -- Clear diagnostics
  vim.diagnostic.set(namespace, bufnr, {})
  
  -- Clear counts
  buffer_counts[bufnr] = nil
  
  -- Trigger user event (data field only available in 0.9+)
  if vim.fn.has('nvim-0.9') == 1 then
    vim.api.nvim_exec_autocmds('User', {
      pattern = 'MtlogDiagnosticsChanged',
      data = {
        bufnr = bufnr,
        count = 0,
      },
    })
  else
    -- For older versions, just trigger without data
    vim.cmd('doautocmd User MtlogDiagnosticsChanged')
  end
end

-- Clear all diagnostics
function M.clear_all()
  -- Ensure namespace exists
  if not namespace then
    M.setup()
  end
  
  -- Clear all diagnostics in namespace
  for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_valid(bufnr) then
      vim.diagnostic.set(namespace, bufnr, {})
    end
  end
  
  -- Clear all counts
  buffer_counts = {}
  
  -- Trigger user event
  if vim.fn.has('nvim-0.9') == 1 then
    vim.api.nvim_exec_autocmds('User', {
      pattern = 'MtlogDiagnosticsCleared',
    })
  else
    vim.cmd('doautocmd User MtlogDiagnosticsCleared')
  end
end

-- Update diagnostic counts for a buffer
---@param bufnr number Buffer number
function M.update_counts(bufnr)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    buffer_counts[bufnr] = nil
    return
  end
  
  local diagnostics = vim.diagnostic.get(bufnr, { namespace = namespace })
  local counts = {
    total = #diagnostics,
    errors = 0,
    warnings = 0,
    info = 0,
    hints = 0,
  }
  
  for _, diag in ipairs(diagnostics) do
    if diag.severity == vim.diagnostic.severity.ERROR then
      counts.errors = counts.errors + 1
    elseif diag.severity == vim.diagnostic.severity.WARN then
      counts.warnings = counts.warnings + 1
    elseif diag.severity == vim.diagnostic.severity.INFO then
      counts.info = counts.info + 1
    elseif diag.severity == vim.diagnostic.severity.HINT then
      counts.hints = counts.hints + 1
    end
  end
  
  buffer_counts[bufnr] = counts
end

-- Get diagnostic counts
---@param bufnr number? Buffer number (nil for all buffers)
---@return table Counts by severity
function M.get_counts(bufnr)
  if bufnr then
    return buffer_counts[bufnr] or {
      total = 0,
      errors = 0,
      warnings = 0,
      info = 0,
      hints = 0,
    }
  end
  
  -- Aggregate counts for all buffers
  local total_counts = {
    total = 0,
    errors = 0,
    warnings = 0,
    info = 0,
    hints = 0,
  }
  
  for _, counts in pairs(buffer_counts) do
    total_counts.total = total_counts.total + counts.total
    total_counts.errors = total_counts.errors + counts.errors
    total_counts.warnings = total_counts.warnings + counts.warnings
    total_counts.info = total_counts.info + counts.info
    total_counts.hints = total_counts.hints + counts.hints
  end
  
  return total_counts
end

-- Get diagnostics at cursor position
---@param bufnr number? Buffer number (defaults to current)
---@return table? Diagnostic at cursor or nil
function M.get_diagnostic_at_cursor(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  
  if not namespace then
    return nil
  end
  
  local cursor = vim.api.nvim_win_get_cursor(0)
  local line = cursor[1] - 1
  local col = cursor[2]
  
  local diagnostics = vim.diagnostic.get(bufnr, {
    namespace = namespace,
    lnum = line,
  })
  
  -- Find diagnostic at cursor column
  for _, diag in ipairs(diagnostics) do
    if col >= diag.col and col <= (diag.end_col or diag.col) then
      return diag
    end
  end
  
  -- Return first diagnostic on line if no exact match
  return diagnostics[1]
end

-- Apply suggested fix from diagnostic
---@param diagnostic table Diagnostic with suggested fixes
---@param fix_index number? Index of fix to apply (defaults to 1)
---@return boolean Success
function M.apply_suggested_fix(diagnostic, fix_index)
  if not diagnostic or not diagnostic.user_data then
    return false
  end
  
  local fixes = diagnostic.user_data.suggested_fixes
  if not fixes or #fixes == 0 then
    return false
  end
  
  fix_index = fix_index or 1
  local fix = fixes[fix_index]
  if not fix then
    return false
  end
  
  -- Apply text edits (handle both 'edits' and 'textEdits' fields)
  local edits = fix.edits or fix.textEdits
  if edits then
    local bufnr = vim.api.nvim_get_current_buf()
    
    -- Sort edits by position (in reverse order to apply from end to start)
    local sorted_edits = {}
    for _, edit in ipairs(edits) do
      table.insert(sorted_edits, edit)
    end
    table.sort(sorted_edits, function(a, b)
      -- Get position for sorting
      local function get_sort_key(edit)
        if edit.range and edit.range.start then
          return edit.range.start.line * 100000 + edit.range.start.column
        elseif edit.pos then
          -- Parse "file:line:col" format
          local parts = vim.split(edit.pos, ':', { plain = true })
          if #parts >= 3 then
            local line = tonumber(parts[#parts - 1]) or 0
            local col = tonumber(parts[#parts]) or 0
            return line * 100000 + col
          end
        elseif edit.start then
          return edit.start
        end
        return 0
      end
      
      -- Sort in reverse order (highest position first)
      return get_sort_key(a) > get_sort_key(b)
    end)
    
    -- Show progress notification for multiple edits
    if #sorted_edits > 1 then
      vim.notify(string.format('Applying %d edits...', #sorted_edits), vim.log.levels.INFO)
    end
    
    local any_edit_failed = false
    for _, edit in ipairs(sorted_edits) do
      local bufname = vim.api.nvim_buf_get_name(bufnr)
      
      -- Only apply edits for the current file
      if edit.filename and not bufname:match(vim.fn.fnamemodify(edit.filename, ':t') .. '$') then
        goto continue
      end
      
      -- Handle line/column format (from tests and some analyzer modes)
      if edit.range and edit.range.start and edit.range['end'] and edit.newText ~= nil then
        local start_line = edit.range.start.line - 1  -- Convert to 0-indexed
        local start_col = edit.range.start.column - 1  -- Convert to 0-indexed
        local end_line = edit.range['end'].line - 1
        local end_col = edit.range['end'].column - 1
        
        -- Validate line numbers are within buffer bounds
        local line_count = vim.api.nvim_buf_line_count(bufnr)
        if start_line < 0 or start_line >= line_count or end_line < 0 or end_line >= line_count then
          vim.notify(string.format('Line out of range: start=%d, end=%d, buffer lines=%d', 
            start_line + 1, end_line + 1, line_count), vim.log.levels.WARN)
          any_edit_failed = true
          goto continue
        end
        
        -- Use nvim_buf_set_text which handles positions better
        -- Note: nvim_buf_set_text expects 0-based positions for everything
        local ok, err = pcall(function()
          -- Handle newlines in the replacement text
          local replacement_lines = vim.split(edit.newText, '\n', { plain = true })
          
          -- For insertions (start == end), use the same position
          if start_line == end_line and start_col == end_col then
            -- This is an insertion
            vim.api.nvim_buf_set_text(bufnr, start_line, start_col, end_line, end_col, replacement_lines)
          else
            -- This is a replacement
            vim.api.nvim_buf_set_text(bufnr, start_line, start_col, end_line, end_col, replacement_lines)
          end
        end)
        
        if not ok then
          vim.notify(string.format('Failed to apply fix: %s', tostring(err)), vim.log.levels.WARN)
          any_edit_failed = true
          -- Fall back to the old method if set_text fails
          local fallback_ok = pcall(function()
            local lines = vim.api.nvim_buf_get_lines(bufnr, start_line, end_line + 1, false)
            if #lines > 0 then
              if start_line == end_line then
                -- Single line edit
                local line = lines[1] or ""
                -- For insertion, don't skip any characters
                if start_col == end_col then
                  local new_line = line:sub(1, start_col) .. edit.newText .. line:sub(start_col + 1)
                  lines[1] = new_line
                else
                  local new_line = line:sub(1, start_col) .. edit.newText .. line:sub(end_col + 1)
                  lines[1] = new_line
                end
              else
                -- Multi-line edit
                local first_line = (lines[1] or ""):sub(1, start_col) .. edit.newText
                local last_line = (lines[#lines] or ""):sub(end_col + 1)
                lines = {first_line .. last_line}
              end
              
              -- Only set lines if newText doesn't contain newlines
              if not edit.newText:match('\n') then
                local set_ok = pcall(vim.api.nvim_buf_set_lines, bufnr, start_line, end_line + 1, false, lines)
          if not set_ok then
            any_edit_failed = true
          end
              end
            end
          end)
          -- Don't clear the error flag even if fallback succeeds
          -- The fact that we needed a fallback means something is wrong
        end
      -- Handle analyzer stdin mode format (pos/end/newText)
      -- Format: "filename:line:column" where line and column are 1-indexed
      -- IMPORTANT: The 'end' position is exclusive (like Go slices)
      -- Example: To replace "userid" at columns 17-23, the end position 23 
      -- means "up to but not including column 23", so we replace columns 17-22
      elseif edit.pos and edit['end'] and edit.newText then
        -- Parse positions from "file:line:col" format
        local start_parts = vim.split(edit.pos, ':', { plain = true })
        local end_parts = vim.split(edit['end'], ':', { plain = true })
        
        -- Validate position format
        if #start_parts < 3 or #end_parts < 3 then
          vim.notify('Invalid position format in quick fix: expected "file:line:col"', vim.log.levels.WARN)
          goto continue
        end
        
        local start_line = tonumber(start_parts[#start_parts - 1])
        local start_col = tonumber(start_parts[#start_parts])
        local end_line = tonumber(end_parts[#end_parts - 1])
        local end_col = tonumber(end_parts[#end_parts])
        
        if not start_line or not start_col or not end_line or not end_col then
          vim.notify('Invalid position values in quick fix', vim.log.levels.WARN)
          goto continue
        end
        
        -- Convert to appropriate indices
        start_line = start_line - 1  -- Convert to 0-indexed for nvim_buf_get_lines
        end_line = end_line - 1      -- Convert to 0-indexed for nvim_buf_get_lines
        -- Keep columns as 1-indexed for Lua string operations
        
        -- Get the lines
        local lines = vim.api.nvim_buf_get_lines(bufnr, start_line, end_line + 1, false)
        
        if #lines > 0 then
          if start_line == end_line then
            -- Single line edit
            local line = lines[1]
            if start_col == end_col then
              -- Insertion: insert at the position (1-indexed)
              local new_line = line:sub(1, start_col - 1) .. edit.newText .. line:sub(start_col)
              lines[1] = new_line
            else
              -- Replacement: The analyzer gives us exclusive end position
              -- start_col and end_col are 1-indexed
              -- The end position is exclusive (points to char after last to replace)
              -- To replace "userid" at columns 28-34 (exclusive):
              -- We keep [1, 27], add newText, then keep [34, end]
              local new_line = line:sub(1, start_col - 1) .. edit.newText .. line:sub(end_col)
              lines[1] = new_line
            end
          else
            -- Multi-line edit (1-indexed columns)
            local first_line = lines[1]:sub(1, start_col - 1) .. edit.newText
            local last_line = lines[#lines]:sub(end_col)
            lines = {first_line .. last_line}
          end
          
          local set_ok = pcall(vim.api.nvim_buf_set_lines, bufnr, start_line, end_line + 1, false, lines)
          if not set_ok then
            any_edit_failed = true
          end
        end
      -- Handle byte offset format (legacy)
      elseif edit.start and edit['end'] and edit.new then
        -- Get buffer content as a single string
        local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
        local content = table.concat(lines, '\n')
        
        -- Apply the edit
        -- The analyzer gives 0-based byte offsets where 'end' appears to be exclusive
        -- For 0-based position N, the Lua position is N+1
        -- So to replace bytes [start, end) we use sub(1, start) and sub(end+1)
        
        -- WORKAROUND: The analyzer for MTLOG004 includes quotes in the replacement
        -- but the byte range starts after the opening quote. Check for this case.
        local to_replace = content:sub(edit.start + 1, edit['end'])
        local looks_like_string_literal = to_replace:match('^%s*%w+%s*{.*}.*"')
        local replacement_has_quotes = edit.new:match('^".*"$')
        
        if looks_like_string_literal and replacement_has_quotes then
          -- Strip quotes from replacement since the range doesn't include the opening quote
          edit.new = edit.new:sub(2, -2)
        end
        
        local before = content:sub(1, edit.start)  -- chars 1 to start (0-based, so this is correct)
        local after = content:sub(edit['end'] + 1)  -- chars from end+1 onwards (0-based end is exclusive)
        local new_content = before .. edit.new .. after
        
        -- Set the new content
        local new_lines = vim.split(new_content, '\n', { plain = true })
        local set_ok = pcall(vim.api.nvim_buf_set_lines, bufnr, 0, -1, false, new_lines)
        if not set_ok then
          any_edit_failed = true
        end
      end
      
      ::continue::
    end
    
    -- Return false if any edit failed
    if any_edit_failed then
      return false
    end
    
    return true
  end
  
  return false
end

-- Show diagnostic float at cursor
---@param opts table? Options for diagnostic float
function M.show_float(opts)
  opts = opts or {}
  opts.namespace = namespace
  
  -- Use config float options
  local float_opts = config.get('float')
  if float_opts then
    opts = vim.tbl_extend('force', float_opts, opts)
  end
  
  vim.diagnostic.open_float(opts)
end

-- Jump to next diagnostic
---@param opts table? Options for jump
function M.goto_next(opts)
  opts = opts or {}
  opts.namespace = namespace
  
  vim.diagnostic.goto_next(opts)
end

-- Jump to previous diagnostic
---@param opts table? Options for jump
function M.goto_prev(opts)
  opts = opts or {}
  opts.namespace = namespace
  
  vim.diagnostic.goto_prev(opts)
end

-- Set diagnostic locations to quickfix list
---@param opts table? Options for setqflist
function M.setqflist(opts)
  opts = opts or {}
  opts.namespace = namespace
  
  vim.diagnostic.setqflist(opts)
end

-- Set diagnostic locations to location list
---@param winnr number? Window number (defaults to current)
---@param opts table? Options for setloclist
function M.setloclist(winnr, opts)
  winnr = winnr or 0
  opts = opts or {}
  opts.namespace = namespace
  
  vim.diagnostic.setloclist(winnr, opts)
end

-- Format diagnostic for display
---@param diagnostic table Diagnostic to format
---@return string Formatted diagnostic
function M.format(diagnostic)
  local parts = {}
  
  -- Add code if present
  if diagnostic.code then
    table.insert(parts, string.format('[%s]', diagnostic.code))
  end
  
  -- Add message
  table.insert(parts, diagnostic.message)
  
  -- Add source if not mtlog-analyzer
  if diagnostic.source and diagnostic.source ~= 'mtlog-analyzer' then
    table.insert(parts, string.format('(%s)', diagnostic.source))
  end
  
  return table.concat(parts, ' ')
end

-- Get severity name from severity level
---@param severity number Severity level
---@return string Severity name
function M.severity_name(severity)
  if severity == vim.diagnostic.severity.ERROR then
    return 'Error'
  elseif severity == vim.diagnostic.severity.WARN then
    return 'Warning'
  elseif severity == vim.diagnostic.severity.INFO then
    return 'Info'
  elseif severity == vim.diagnostic.severity.HINT then
    return 'Hint'
  end
  return 'Unknown'
end

-- Get severity highlight group
---@param severity number Severity level
---@return string Highlight group
function M.severity_highlight(severity)
  if severity == vim.diagnostic.severity.ERROR then
    return 'DiagnosticError'
  elseif severity == vim.diagnostic.severity.WARN then
    return 'DiagnosticWarn'
  elseif severity == vim.diagnostic.severity.INFO then
    return 'DiagnosticInfo'
  elseif severity == vim.diagnostic.severity.HINT then
    return 'DiagnosticHint'
  end
  return 'Normal'
end

return M