-- Analyzer integration module for mtlog.nvim

local M = {}

local config = require('mtlog.config')
local utils = require('mtlog.utils')
local cache = require('mtlog.cache')

-- Analyzer state
local analyzer_version = nil
local analyzer_available = nil
local check_in_progress = false
local last_checked_path = nil

-- Force recheck availability (clears cache)
function M.reset_availability()
  analyzer_available = nil
  analyzer_version = nil
  last_checked_path = nil
end

-- Check if analyzer is available
---@return boolean
function M.is_available()
  -- Always recheck if path changes
  local current_path = vim.g.mtlog_analyzer_path or config.get('analyzer_path') or 'mtlog-analyzer'
  
  if analyzer_available ~= nil and last_checked_path == current_path then
    return analyzer_available
  end
  
  if check_in_progress then
    return false
  end
  
  check_in_progress = true
  last_checked_path = current_path
  
  -- mtlog-analyzer uses -V=full for version
  local result = vim.fn.system(current_path .. ' -V=full 2>&1')
  
  if vim.v.shell_error == 0 and result:match('mtlog%-analyzer') then
    analyzer_available = true
    -- Extract version from output (might be "devel" for development builds)
    analyzer_version = result:match('version%s+([^%s]+)') or 'devel'
  else
    -- Fallback: just check if command exists
    local check = vim.fn.system('which ' .. current_path .. ' 2>/dev/null')
    if vim.v.shell_error == 0 and check ~= '' then
      analyzer_available = true
      analyzer_version = 'detected'
    else
      analyzer_available = false
    end
  end
  
  check_in_progress = false
  return analyzer_available
end

-- Get analyzer version
---@return string? Version or nil
function M.get_version()
  if analyzer_version then
    return analyzer_version
  end
  
  if not M.is_available() then
    return nil
  end
  
  return analyzer_version
end

-- Parse JSON output from analyzer
---@param output string JSON output
---@return table? Parsed diagnostics or nil
local function parse_json_output(output)
  -- Handle empty output
  if not output or output == '' then
    return {}
  end
  
  -- Try to parse JSON
  local ok, result = pcall(vim.json.decode, output)
  if not ok then
    -- Try to extract JSON from output
    local json_start = output:find('[%[{]')
    if json_start then
      local json_str = output:sub(json_start)
      ok, result = pcall(vim.json.decode, json_str)
    end
  end
  
  if ok and type(result) == 'table' then
    -- Debug: log the number of diagnostics found
    if vim.g.mtlog_debug then
      vim.notify(string.format('Analyzer returned %d diagnostics', vim.tbl_islist(result) and #result or 1), vim.log.levels.INFO)
      for i, diag in ipairs(vim.tbl_islist(result) and result or {result}) do
        if diag.code then
          vim.notify(string.format('  [%s] %s', diag.code or diag.diagnosticID or '?', diag.message or '?'), vim.log.levels.INFO)
        end
      end
    end
    return result
  end
  
  return nil
end

-- Convert analyzer diagnostic to Neovim diagnostic
---@param diag table Analyzer diagnostic
---@param filepath string File path
---@return table Neovim diagnostic
local function convert_diagnostic(diag, filepath)
  local severity_levels = config.get('severity_levels')
  
  -- Determine severity based on code
  local severity = vim.diagnostic.severity.HINT
  if diag.code and severity_levels[diag.code] then
    severity = severity_levels[diag.code]
  elseif diag.severity then
    -- Map analyzer severity to Neovim severity
    local severity_map = {
      error = vim.diagnostic.severity.ERROR,
      warning = vim.diagnostic.severity.WARN,
      info = vim.diagnostic.severity.INFO,
      hint = vim.diagnostic.severity.HINT,
      suggestion = vim.diagnostic.severity.HINT,  -- Map suggestion to hint
    }
    severity = severity_map[diag.severity:lower()] or vim.diagnostic.severity.HINT
  end
  
  -- Parse position
  local line = (diag.line or diag.pos and diag.pos.line or 1) - 1  -- Convert to 0-indexed
  local col = (diag.column or diag.pos and diag.pos.column or 1) - 1
  local end_line = (diag.endLine or diag.end_pos and diag.end_pos.line or line + 1) - 1
  local end_col = (diag.endColumn or diag.end_pos and diag.end_pos.column or col + 1) - 1
  
  -- Check if message already contains the code (stdin mode includes it)
  local formatted_message = diag.message
  if diag.code and not formatted_message:match('^%[' .. vim.pesc(diag.code) .. '%]') then
    formatted_message = utils.format_diagnostic_message(diag.code, diag.message)
  end
  
  -- Create diagnostic
  local diagnostic = {
    lnum = line,
    col = col,
    end_lnum = end_line,
    end_col = end_col,
    message = formatted_message,
    severity = severity,
    source = 'mtlog-analyzer',
    code = diag.code,
  }
  
  -- Store suggested fixes in user_data for code actions
  if diag.suggestedFixes or diag.suggested_fixes then
    diagnostic.user_data = {
      suggested_fixes = diag.suggestedFixes or diag.suggested_fixes,
      original = diag,
    }
  end
  
  return diagnostic
end

-- Run analyzer with vim.system (Neovim 0.10+) or fallback
---@param filepath string File to analyze
---@param callback function Callback with (results, error)
---@param bufnr number? Optional buffer number to use for content
local function run_analyzer_async(filepath, callback, bufnr)
  -- ALWAYS check vim.g first, don't cache
  local analyzer_path = vim.g.mtlog_analyzer_path or config.get('analyzer_path') or 'mtlog-analyzer'
  
  -- If it's just 'mtlog-analyzer', try to find the full path
  if analyzer_path == 'mtlog-analyzer' then
    local which_result = vim.fn.system('which mtlog-analyzer 2>/dev/null')
    if vim.v.shell_error == 0 and which_result ~= '' then
      analyzer_path = vim.fn.trim(which_result)
    end
  end
  
  local analyzer_flags = config.get('analyzer_flags') or {}
  
  -- Build command - use stdin mode like VS Code and JetBrains
  local cmd = { analyzer_path, '-stdin' }
  
  -- Add custom flags
  for _, flag in ipairs(analyzer_flags) do
    table.insert(cmd, flag)
  end
  
  -- Get the file content from buffer if available, otherwise from disk
  local file_content
  
  -- Use provided bufnr if given, otherwise look it up
  if not bufnr then
    bufnr = vim.fn.bufnr(filepath)
    
    -- IMPORTANT: bufnr(filepath) might return the wrong buffer if the path doesn't match exactly
    -- Use the current buffer if it matches the filepath
    local current_buf = vim.api.nvim_get_current_buf()
    local current_buf_name = vim.api.nvim_buf_get_name(current_buf)
    
    -- Normalize paths for comparison
    if vim.fn.fnamemodify(current_buf_name, ':p') == vim.fn.fnamemodify(filepath, ':p') then
      bufnr = current_buf
    end
  end
  
  if bufnr and bufnr ~= -1 and vim.api.nvim_buf_is_loaded(bufnr) then
    -- FORCE: Always get from current buffer if it's the file we're analyzing
    -- This ensures we get the latest content
    local current_buf = vim.api.nvim_get_current_buf()
    local current_buf_name = vim.api.nvim_buf_get_name(current_buf)
    if vim.fn.fnamemodify(current_buf_name, ':p') == vim.fn.fnamemodify(filepath, ':p') then
      bufnr = current_buf
    end
    
    -- Use buffer content (might have unsaved changes)
    local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
    file_content = table.concat(lines, '\n')
    if vim.g.mtlog_debug then
      vim.notify(string.format('Using buffer %d content for %s (%d lines)', bufnr, filepath, #lines), vim.log.levels.INFO)
      -- Show what line 49 actually contains
      if lines[49] then
        vim.notify(string.format('Line 49 from buffer: %s', lines[49]), vim.log.levels.INFO)
      end
    end
  else
    -- Fall back to reading from disk
    local ok, lines = pcall(vim.fn.readfile, filepath)
    if ok then
      file_content = table.concat(lines, '\n')
      if vim.g.mtlog_debug then
        vim.notify(string.format('Using file content for %s (%d lines)', filepath, #lines), vim.log.levels.INFO)
      end
    else
      -- File doesn't exist or can't be read
      callback(nil, 'Cannot read file: ' .. filepath)
      return
    end
  end
  
  -- Create stdin input in the format the analyzer expects
  local stdin_input = vim.json.encode({
    filename = filepath,
    content = file_content
  })
  
  if vim.g.mtlog_debug then
    -- Show last few lines to see what we're analyzing
    local lines = vim.split(file_content, '\n')
    local last_lines = {}
    for i = math.max(1, #lines - 5), #lines do
      table.insert(last_lines, string.format('%d: %s', i, lines[i]))
    end
    vim.notify('Analyzing content (last few lines):\n' .. table.concat(last_lines, '\n'), vim.log.levels.INFO)
  end
  
  -- Use vim.system if available (Neovim 0.10+)
  -- Force using jobstart for now since vim.system seems broken
  if false and vim.system then
    vim.system(cmd, {
      text = true,
      stdin = stdin_input,
      stdout = true,
      stderr = true,
    }, function(result)
      vim.schedule(function()
        -- In stdin mode, analyzer outputs JSON to stdout
        -- Non-zero exit code means diagnostics were found
        local json_output = result.stdout or ''
        
        if result.code ~= 0 then
          -- Non-zero exit code means issues were found
          if json_output and json_output ~= '' then
            local diagnostics = parse_json_output(json_output)
            if diagnostics then
              callback(diagnostics, nil)
            else
              callback({}, nil)
            end
          else
            -- No JSON output but non-zero exit code, might be an error
            local error_msg = result.stderr or 'Analyzer failed'
            callback(nil, 'Analyzer error: ' .. error_msg)
          end
        else
          -- Zero exit code might still have diagnostics (warnings/hints)
          local diagnostics = parse_json_output(json_output)
          callback(diagnostics or {}, nil)
        end
      end)
    end)
  else
    -- Fallback to jobstart for older Neovim versions
    local stdout = {}
    local stderr = {}
    
    local job_id = vim.fn.jobstart(cmd, {
      on_stdout = function(_, data)
        if data then
          for _, line in ipairs(data) do
            -- Don't skip empty lines - they might be part of JSON
            table.insert(stdout, line)
          end
        end
      end,
      on_stderr = function(_, data)
        if data then
          for _, line in ipairs(data) do
            if line ~= '' then
              table.insert(stderr, line)
            end
          end
        end
      end,
      on_exit = function(_, exit_code)
        vim.schedule(function()
          if exit_code ~= 0 then
            local stderr_text = table.concat(stderr, '\n')
            callback(nil, 'Analyzer error: ' .. stderr_text)
            return
          end
          
          -- In stdin mode, JSON array is in stdout
          local json_output = table.concat(stdout, '\n')
          local diagnostics = parse_json_output(json_output)
          callback(diagnostics or {}, nil)
        end)
      end,
    })
    
    -- Send the stdin input
    if job_id > 0 then
      vim.fn.chansend(job_id, stdin_input)
      vim.fn.chanclose(job_id, 'stdin')
    end
    
    if job_id <= 0 then
      callback(nil, 'Failed to start analyzer')
    end
  end
end

-- Analyze a single file
---@param filepath string File path to analyze
---@param callback function Callback with (results, error)
---@param bufnr number? Optional buffer number to use
function M.analyze_file(filepath, callback, bufnr)
  -- Don't check availability - just try to run it
  -- The error will be more informative if it fails
  
  -- Get buffer number if not provided
  if not bufnr then
    bufnr = vim.fn.bufnr(filepath)
  end
  
  -- Check if buffer has unsaved changes
  local has_unsaved_changes = bufnr ~= -1 and vim.api.nvim_buf_is_loaded(bufnr) and vim.bo[bufnr].modified
  
  if vim.g.mtlog_debug then
    vim.notify(string.format('Analyzing %s - Buffer: %d, Modified: %s', 
      vim.fn.fnamemodify(filepath, ':t'), 
      bufnr, 
      tostring(has_unsaved_changes)), vim.log.levels.INFO)
  end
  
  -- Check cache first if enabled (skip cache if buffer has unsaved changes)
  if config.get('cache.enabled') and not has_unsaved_changes then
    local cached = cache.get(filepath)
    if cached then
      if vim.g.mtlog_debug then
        vim.notify('Using cached results', vim.log.levels.INFO)
      end
      callback(cached, nil)
      return
    end
  else
    -- If buffer has unsaved changes, invalidate any existing cache entry
    -- This ensures we don't use stale cache after the buffer is saved
    if config.get('cache.enabled') and has_unsaved_changes then
      cache.invalidate(filepath)
      if vim.g.mtlog_debug then
        vim.notify('Invalidated cache due to unsaved changes', vim.log.levels.INFO)
      end
    end
  end
  
  -- Run analyzer (pass bufnr to ensure we use the right buffer content)
  run_analyzer_async(filepath, function(raw_results, err)
    if err then
      callback(nil, err)
      return
    end
    
    -- Convert diagnostics
    local diagnostics = {}
    
    if type(raw_results) == 'table' then
      -- In stdin mode, the JSON is a simple array of diagnostics
      if #raw_results > 0 then
        -- It's an array
        for _, issue in ipairs(raw_results) do
          local diag = {
            line = issue.line or 1,
            column = issue.column or 1,
            message = issue.message or "",
            code = issue.diagnostic_id or "MTLOG",
            severity = issue.severity,
            suggested_fixes = issue.suggestedFixes or issue.suggested_fixes,
          }
          
          table.insert(diagnostics, convert_diagnostic(diag, filepath))
        end
      else
        -- Maybe it's the old format from go vet - check for package structure
        for package_name, package_data in pairs(raw_results) do
          if type(package_data) == 'table' and package_data.mtlog then
            local mtlog_issues = package_data.mtlog
            
            if type(mtlog_issues) == 'table' then
              for _, issue in ipairs(mtlog_issues) do
                if issue.posn and issue.message then
                  -- Parse position format: "file:line:col"
                  local file, line, col = issue.posn:match("([^:]+):(%d+):(%d+)")
                  
                  -- Extract code from message: "[MTLOG001] message"
                  local code, msg = issue.message:match("%[([^%]]+)%]%s*(.*)")
                  
                  local diag = {
                    line = tonumber(line) or 1,
                    column = tonumber(col) or 1,
                    message = msg or issue.message,
                    code = code or "MTLOG",
                    suggested_fixes = issue.suggested_fixes,
                  }
                  
                  table.insert(diagnostics, convert_diagnostic(diag, filepath))
                end
              end
            end
          end
        end
      end
    end
    
    -- Update cache (only if buffer doesn't have unsaved changes)
    -- Re-check the buffer state at this point since time has passed
    local current_bufnr = vim.fn.bufnr(filepath)
    local currently_has_unsaved = current_bufnr ~= -1 and vim.api.nvim_buf_is_loaded(current_bufnr) and vim.bo[current_bufnr].modified
    
    if config.get('cache.enabled') and not currently_has_unsaved then
      cache.set(filepath, diagnostics)
    end
    
    callback(diagnostics, nil)
  end, bufnr)
end

-- Analyze multiple files with rate limiting
---@param filepaths table List of file paths
---@param on_file function Callback for each file (filepath, results, error)
---@param on_complete function Callback when all complete
function M.analyze_files(filepaths, on_file, on_complete)
  local completed = 0
  local total = #filepaths
  
  if total == 0 then
    if on_complete then
      on_complete()
    end
    return
  end
  
  -- Create rate limiter if enabled
  local process_file
  if config.get('rate_limit.enabled') then
    local rate_limiter = utils.rate_limiter(config.get('rate_limit.max_files_per_second'))
    process_file = function(filepath)
      rate_limiter(M.analyze_file, filepath, function(results, err)
        on_file(filepath, results, err)
        completed = completed + 1
        if completed >= total and on_complete then
          on_complete()
        end
      end)
    end
  else
    process_file = function(filepath)
      M.analyze_file(filepath, function(results, err)
        on_file(filepath, results, err)
        completed = completed + 1
        if completed >= total and on_complete then
          on_complete()
        end
      end)
    end
  end
  
  -- Process all files
  for _, filepath in ipairs(filepaths) do
    process_file(filepath)
  end
end

-- Clear analyzer state (for reloading)
function M.reset()
  analyzer_version = nil
  analyzer_available = nil
  check_in_progress = false
end

return M