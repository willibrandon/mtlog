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
  
  -- Try to use git ls-files first
  local git_cmd = 'git ls-files --cached --others --exclude-standard | grep -E "\\.go$"'
  local handle = io.popen(git_cmd .. ' 2>/dev/null')
  
  if handle then
    for line in handle:lines() do
      local filepath = vim.fn.fnamemodify(line, ':p')
      local should_ignore = false
      
      -- Check against ignore patterns
      for _, pattern in ipairs(ignore_patterns) do
        if filepath:match(pattern) then
          should_ignore = true
          break
        end
      end
      
      if not should_ignore and not M.is_vendor_path(filepath) then
        table.insert(files, filepath)
      end
    end
    handle:close()
    
    if #files > 0 then
      return files
    end
  end
  
  -- Fallback to find command
  local find_cmd = 'find . -type f -name "*.go" 2>/dev/null'
  handle = io.popen(find_cmd)
  
  if handle then
    for line in handle:lines() do
      local filepath = vim.fn.fnamemodify(line, ':p')
      local should_ignore = false
      
      -- Check against ignore patterns
      for _, pattern in ipairs(ignore_patterns) do
        if filepath:match(pattern) then
          should_ignore = true
          break
        end
      end
      
      if not should_ignore and not M.is_vendor_path(filepath) then
        table.insert(files, filepath)
      end
    end
    handle:close()
  end
  
  -- Final fallback to vim.fn.glob
  if #files == 0 then
    local glob_files = vim.fn.glob('**/*.go', false, true)
    for _, file in ipairs(glob_files) do
      local filepath = vim.fn.fnamemodify(file, ':p')
      local should_ignore = false
      
      -- Check against ignore patterns
      for _, pattern in ipairs(ignore_patterns) do
        if filepath:match(pattern) then
          should_ignore = true
          break
        end
      end
      
      if not should_ignore and not M.is_vendor_path(filepath) then
        table.insert(files, filepath)
      end
    end
  end
  
  return files
end

-- Get Git root directory
---@return string? Git root or nil
function M.get_git_root()
  local handle = io.popen('git rev-parse --show-toplevel 2>/dev/null')
  if handle then
    local root = handle:read('*l')
    handle:close()
    if root and root ~= '' then
      return root
    end
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

return M