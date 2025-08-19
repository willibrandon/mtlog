-- Utility functions for mtlog.nvim

local M = {}

-- Check if current directory is a Go project
---@return boolean
function M.is_go_project()
  -- Check for go.mod in current directory and parent directories
  local cwd = vim.fn.getcwd()
  local path = cwd
  
  while path and path ~= '/' do
    local go_mod = path .. '/go.mod'
    if vim.fn.filereadable(go_mod) == 1 then
      return true
    end
    
    -- Check for .go files as fallback
    local go_files = vim.fn.glob(path .. '/*.go', false, true)
    if #go_files > 0 then
      return true
    end
    
    -- Move to parent directory
    local parent = vim.fn.fnamemodify(path, ':h')
    if parent == path then
      break
    end
    path = parent
  end
  
  return false
end

-- Check if path is in vendor directory
---@param filepath string File path to check
---@return boolean
function M.is_vendor_path(filepath)
  -- Normalize path
  filepath = vim.fn.fnamemodify(filepath, ':p')
  
  -- Check for vendor in path
  if filepath:match('/vendor/') or filepath:match('\\vendor\\') then
    return true
  end
  
  -- Check if file starts with vendor/
  local relative = vim.fn.fnamemodify(filepath, ':.')
  if relative:match('^vendor[/\\]') then
    return true
  end
  
  return false
end

-- Get list of Go files respecting .gitignore
---@return table List of Go file paths
function M.get_go_files()
  local files = {}
  local config = require('mtlog.config')
  local ignore_patterns = config.get('ignore_patterns') or {}
  
  -- Use vim.fn.glob as primary method (safe and portable)
  local glob_files = vim.fn.glob('**/*.go', false, true)
  
  -- If in a git repository, filter using git status
  local git_root = M.get_git_root()
  local is_git_repo = git_root ~= nil
  local git_ignored = {}
  
  if is_git_repo then
    -- Get list of ignored files using git check-ignore
    -- This is safer than using shell pipes
    for _, file in ipairs(glob_files) do
      local result = vim.fn.system({'git', 'check-ignore', file})
      if vim.v.shell_error == 0 then
        -- File is ignored by git
        git_ignored[file] = true
      end
    end
  end
  
  -- Filter files
  for _, file in ipairs(glob_files) do
    local filepath = vim.fn.fnamemodify(file, ':p')
    local should_ignore = false
    
    -- Skip git-ignored files
    if git_ignored[file] then
      should_ignore = true
    end
    
    -- Check against ignore patterns
    if not should_ignore then
      for _, pattern in ipairs(ignore_patterns) do
        if filepath:match(pattern) then
          should_ignore = true
          break
        end
      end
    end
    
    -- Check vendor path
    if not should_ignore and not M.is_vendor_path(filepath) then
      table.insert(files, filepath)
    end
  end
  
  return files
end

-- Get Git root directory
---@return string? Git root or nil
function M.get_git_root()
  -- Use vim.fn.system for safer execution
  local result = vim.fn.system({'git', 'rev-parse', '--show-toplevel'})
  if vim.v.shell_error == 0 then
    -- Remove trailing newline
    local root = vim.trim(result)
    if root ~= '' then
      return root
    end
  end
  
  -- Fallback: check for .git directory in parent directories
  local path = vim.fn.getcwd()
  while path and path ~= '/' do
    if vim.fn.isdirectory(path .. '/.git') == 1 then
      return path
    end
    local parent = vim.fn.fnamemodify(path, ':h')
    if parent == path then
      break
    end
    path = parent
  end
  
  return nil
end

-- Debounce function for rate limiting
---@param fn function Function to debounce
---@param ms number Milliseconds to wait
---@return function Debounced function
function M.debounce(fn, ms)
  local timer = nil
  local scheduled_args = nil
  
  return function(...)
    scheduled_args = { ... }
    
    if timer then
      vim.fn.timer_stop(timer)
    end
    
    timer = vim.fn.timer_start(ms, function()
      timer = nil
      fn(unpack(scheduled_args))
    end)
  end
end

-- Throttle function for rate limiting
---@param fn function Function to throttle
---@param ms number Milliseconds between calls
---@return function Throttled function
function M.throttle(fn, ms)
  local last_call = 0
  local timer = nil
  local pending_args = nil
  
  return function(...)
    local now = vim.loop.now()
    local remaining = ms - (now - last_call)
    
    if remaining <= 0 then
      last_call = now
      fn(...)
    else
      pending_args = { ... }
      
      if not timer then
        timer = vim.fn.timer_start(remaining, function()
          timer = nil
          last_call = vim.loop.now()
          if pending_args then
            fn(unpack(pending_args))
            pending_args = nil
          end
        end)
      end
    end
  end
end

-- Rate limiter for multiple file operations
---@param max_per_second number Maximum operations per second
---@return function Rate limited executor
function M.rate_limiter(max_per_second)
  local queue = {}
  local processing = false
  local interval = 1000 / max_per_second
  
  local function process_queue()
    if #queue == 0 then
      processing = false
      return
    end
    
    processing = true
    local item = table.remove(queue, 1)
    item.fn(unpack(item.args))
    
    vim.fn.timer_start(interval, process_queue)
  end
  
  return function(fn, ...)
    table.insert(queue, { fn = fn, args = { ... } })
    
    if not processing then
      process_queue()
    end
  end
end

-- Parse line:column from string
---@param str string String containing line:column
---@return number? line, number? column
function M.parse_position(str)
  local line, col = str:match('(%d+):(%d+)')
  if line and col then
    return tonumber(line), tonumber(col)
  end
  return nil, nil
end

-- Convert byte offset to line and column
---@param content string File content
---@param offset number Byte offset
---@return number line, number column
function M.offset_to_position(content, offset)
  local line = 1
  local col = 1
  local current = 1
  
  for i = 1, math.min(offset - 1, #content) do
    if content:sub(i, i) == '\n' then
      line = line + 1
      col = 1
    else
      col = col + 1
    end
  end
  
  return line, col
end

-- Check if Neovim has required features
---@param min_version string Minimum version (e.g., "0.7.0")
---@return boolean, string? Has features, error message
function M.check_neovim_version(min_version)
  -- Simple version check for compatibility
  local has_version = vim.fn.has('nvim-' .. min_version)
  
  if has_version == 0 then
    return false, string.format(
      'Neovim %s or higher required',
      min_version
    )
  end
  
  -- Check for required features
  if not vim.system then
    -- Fallback for older versions
    if not vim.loop then
      return false, 'Missing required libuv bindings'
    end
  end
  
  return true, nil
end

-- Get relative path from current working directory
---@param filepath string Absolute file path
---@return string Relative path
function M.relative_path(filepath)
  local cwd = vim.fn.getcwd()
  if vim.startswith(filepath, cwd) then
    return filepath:sub(#cwd + 2)  -- +2 to skip the separator
  end
  return filepath
end

-- Create directory if it doesn't exist
---@param path string Directory path
---@return boolean Success
function M.ensure_directory(path)
  if vim.fn.isdirectory(path) == 0 then
    return vim.fn.mkdir(path, 'p') == 1
  end
  return true
end

-- Read file contents
---@param filepath string File path
---@return string? Content or nil on error
function M.read_file(filepath)
  local file = io.open(filepath, 'r')
  if not file then
    return nil
  end
  
  local content = file:read('*a')
  file:close()
  return content
end

-- Write file contents
---@param filepath string File path
---@param content string Content to write
---@return boolean Success
function M.write_file(filepath, content)
  local file = io.open(filepath, 'w')
  if not file then
    return false
  end
  
  file:write(content)
  file:close()
  return true
end

-- Get file modification time
---@param filepath string File path
---@return number? Modification time or nil
function M.get_mtime(filepath)
  local stat = vim.loop.fs_stat(filepath)
  if stat then
    return stat.mtime.sec
  end
  return nil
end

-- Format diagnostic message with code
---@param code string Diagnostic code
---@param message string Diagnostic message
---@return string Formatted message
function M.format_diagnostic_message(code, message)
  local config = require('mtlog.config')
  
  if config.get('show_codes') ~= false then
    return string.format('[%s] %s', code, message)
  end
  
  return message
end

-- Get human-readable description for diagnostic codes
---@param code string Diagnostic code (e.g., "MTLOG001")
---@return string Description
function M.get_diagnostic_description(code)
  local descriptions = {
    MTLOG001 = "Template/argument mismatch",
    MTLOG002 = "Invalid format specifier",
    MTLOG003 = "Missing error in Error/Fatal log",
    MTLOG004 = "Property name not PascalCase",
    MTLOG005 = "Complex type needs LogValue() method",
    MTLOG006 = "Duplicate property in template",
    MTLOG007 = "String context key should be constant",
    MTLOG008 = "General information",
    MTLOG009 = "With() odd argument count",
    MTLOG010 = "With() non-string key",
    MTLOG011 = "With() cross-call duplicate",
    MTLOG012 = "With() reserved property",
    MTLOG013 = "With() empty key",
  }
  
  return descriptions[code] or "Unknown diagnostic"
end

-- Get all known diagnostic codes
---@return table List of all diagnostic codes
function M.get_all_diagnostic_codes()
  return {
    "MTLOG001", "MTLOG002", "MTLOG003", "MTLOG004", "MTLOG005",
    "MTLOG006", "MTLOG007", "MTLOG008", "MTLOG009", "MTLOG010",
    "MTLOG011", "MTLOG012", "MTLOG013"
  }
end

return M