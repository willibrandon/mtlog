-- Test helpers for mtlog-analyzer tests
-- NO MOCKS - Only real operations

local M = {}

-- Test project directory (must be a real Go project)
M.test_project_dir = vim.env.MTLOG_TEST_PROJECT_DIR or '/tmp/mtlog-test-project'

-- Ensure mtlog-analyzer is available
function M.ensure_analyzer()
  local analyzer_path = vim.env.MTLOG_ANALYZER_PATH or vim.fn.exepath('mtlog-analyzer')
  if not analyzer_path or analyzer_path == '' then
    error("mtlog-analyzer MUST be installed. Set MTLOG_ANALYZER_PATH or ensure mtlog-analyzer is in PATH")
  end
  
  -- Verify it's executable by trying to analyze a non-existent file
  -- This should return exit code 1 (file not found) rather than crash
  local result = vim.fn.system(analyzer_path .. ' -json /dev/null 2>&1')
  if vim.v.shell_error > 1 then -- Exit code 1 is ok (no Go files), >1 means real error
    error("mtlog-analyzer is not executable or returned an error: " .. result)
  end
  
  return analyzer_path
end

-- Create a real Go project structure
function M.setup_go_project()
  local project_dir = M.test_project_dir
  
  -- Check if go.mod already exists (project already set up)
  if vim.fn.filereadable(project_dir .. '/go.mod') == 1 then
    -- Project already exists, just verify it's valid
    local result = vim.fn.system('cd ' .. project_dir .. ' && go build -o test-binary main.go 2>&1')
    if vim.v.shell_error == 0 then
      return project_dir -- Already set up and working
    end
    -- If build fails, clean up and recreate
  end
  
  -- Clean up any existing project directory
  if vim.fn.isdirectory(project_dir) == 1 then
    vim.fn.system('rm -rf ' .. project_dir)
  end
  
  -- Create fresh project directory
  vim.fn.system('mkdir -p ' .. project_dir)
  
  -- Initialize Go module properly
  local result = vim.fn.system('cd ' .. project_dir .. ' && go mod init github.com/test/mtlog-test 2>&1')
  if vim.v.shell_error ~= 0 then
    error("Failed to initialize Go module: " .. result)
  end
  
  -- Create a simple main.go FIRST so go knows what we need
  local main_go_content = [[package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Test message")
}
]]
  
  local main_go_file = project_dir .. '/main.go'
  local file = io.open(main_go_file, 'w')
  if not file then
    error("Failed to create main.go at " .. main_go_file)
  end
  file:write(main_go_content)
  file:close()
  
  -- Now run go mod tidy which should fetch all dependencies
  result = vim.fn.system('cd ' .. project_dir .. ' && go mod tidy 2>&1')
  if vim.v.shell_error ~= 0 then
    -- If tidy fails, try explicit go get
    result = vim.fn.system('cd ' .. project_dir .. ' && go get github.com/willibrandon/mtlog 2>&1')
    if vim.v.shell_error ~= 0 then
      error("Failed to get mtlog module: " .. result)
    end
  end
  
  -- Verify mtlog is actually in the module list
  local mod_list = vim.fn.system('cd ' .. project_dir .. ' && go list -m all 2>&1')
  if not mod_list:match("github%.com/willibrandon/mtlog") then
    error("mtlog module not found in go list -m all output: " .. mod_list)
  end
  
  -- Verify the project builds
  result = vim.fn.system('cd ' .. project_dir .. ' && go build -o test-binary main.go 2>&1')
  if vim.v.shell_error ~= 0 then
    -- Try to get more info about what's wrong
    local mod_list = vim.fn.system('cd ' .. project_dir .. ' && go list -m all 2>&1')
    local go_sum_exists = vim.fn.filereadable(project_dir .. '/go.sum') == 1
    error("Go project does not build:\nBuild error: " .. result .. 
          "\ngo.sum exists: " .. tostring(go_sum_exists) ..
          "\nModules: " .. (mod_list or "none"))
  end
  
  return project_dir
end

-- Create a test Go file with real content
function M.create_test_go_file(filename, content)
  local project_dir = M.test_project_dir
  -- Clean up any double slashes
  local filepath = vim.fn.simplify(project_dir .. '/' .. filename)
  
  -- Ensure the file is in the project directory (escape special regex characters)
  local escaped_dir = project_dir:gsub("([%-%.%+%[%]%(%)%$%^%%%?%*])", "%%%1")
  if not filepath:match('^' .. escaped_dir) then
    error("Test file must be in project directory: " .. project_dir)
  end
  
  -- Create parent directories if needed
  local dir = vim.fn.fnamemodify(filepath, ':h')
  vim.fn.system('mkdir -p ' .. dir)
  
  -- Write the file
  local file = io.open(filepath, 'w')
  if not file then
    error("Failed to create file: " .. filepath)
  end
  file:write(content)
  file:close()
  
  -- Ensure file has real modification time
  vim.fn.system('touch ' .. filepath)
  
  return filepath
end

-- Delete a test file
function M.delete_test_file(filepath)
  if vim.fn.filereadable(filepath) == 1 then
    os.remove(filepath)
  end
end

-- Run mtlog-analyzer on a real file
function M.run_analyzer(filepath, callback)
  local analyzer_path = M.ensure_analyzer()
  
  -- Ensure file exists
  if vim.fn.filereadable(filepath) ~= 1 then
    callback(nil, "File does not exist: " .. filepath)
    return
  end
  
  -- Run analyzer asynchronously
  local output = {}
  local job_id = vim.fn.jobstart({analyzer_path, '-json', filepath}, {
    stdout_buffered = true,
    stderr_buffered = true,
    on_stdout = function(_, data)
      if data then
        for _, line in ipairs(data) do
          if line ~= '' then
            table.insert(output, line)
          end
        end
      end
    end,
    on_stderr = function(_, data)
      if data and data[1] ~= '' then
        callback(nil, table.concat(data, '\n'))
      end
    end,
    on_exit = function(_, exit_code)
      if exit_code == 0 then
        local json_str = table.concat(output, '\n')
        if json_str == '' then
          callback({}, nil) -- No diagnostics
        else
          local ok, results = pcall(vim.json.decode, json_str)
          if ok then
            callback(results, nil)
          else
            callback(nil, "Failed to parse JSON: " .. json_str)
          end
        end
      else
        callback(nil, "Analyzer exited with code: " .. exit_code)
      end
    end
  })
  
  if job_id <= 0 then
    callback(nil, "Failed to start analyzer job")
  end
  
  return job_id
end

-- Wait for real file system changes
function M.wait_for_file_change(filepath, timeout_ms)
  timeout_ms = timeout_ms or 1000
  local original_mtime = vim.fn.getftime(filepath)
  
  -- Make a real change to ensure mtime updates
  vim.fn.system('touch ' .. filepath)
  
  local changed = vim.wait(timeout_ms, function()
    local new_mtime = vim.fn.getftime(filepath)
    return new_mtime > original_mtime
  end, 10)
  
  return changed
end

-- Create a real .mtlog.json configuration file
function M.create_mtlog_config(config)
  local project_dir = M.test_project_dir
  local config_file = project_dir .. '/.mtlog.json'
  
  local file = io.open(config_file, 'w')
  if not file then
    error("Failed to create .mtlog.json")
  end
  
  file:write(vim.json.encode(config))
  file:close()
  
  return config_file
end

-- Set real environment variables
function M.set_env(name, value)
  vim.fn.setenv(name, value)
  -- Verify it was set
  local actual = vim.fn.getenv(name)
  if actual ~= value then
    error("Failed to set environment variable " .. name)
  end
end

-- Clear environment variable
function M.clear_env(name)
  vim.fn.setenv(name, vim.NIL)
  -- Verify it was cleared
  local actual = vim.fn.getenv(name)
  if actual ~= vim.NIL then
    error("Failed to clear environment variable " .. name)
  end
end

-- Clean up test project
function M.cleanup_project()
  if vim.fn.isdirectory(M.test_project_dir) == 1 then
    vim.fn.system('rm -rf ' .. M.test_project_dir)
  end
end

-- Create test buffer with real Go file
function M.create_test_buffer(content)
  local filename = 'test_' .. os.time() .. '.go'
  local filepath = M.create_test_go_file(filename, content)
  
  -- Create buffer and load the file
  local bufnr = vim.fn.bufadd(filepath)
  vim.fn.bufload(bufnr)
  
  return bufnr, filepath
end

-- Verify Go build environment
function M.verify_go_environment()
  -- Check Go is installed
  local go_version = vim.fn.system('go version')
  if vim.v.shell_error ~= 0 then
    error("Go is not installed or not in PATH")
  end
  
  -- Check GOPATH/GOROOT
  local gopath = vim.fn.system('go env GOPATH')
  if vim.v.shell_error ~= 0 or gopath == '' then
    error("GOPATH is not set")
  end
  
  return true
end

-- Wait for real async operation
function M.wait_for_async(condition_fn, timeout_ms)
  timeout_ms = timeout_ms or 5000 -- Default 5 seconds for real operations
  return vim.wait(timeout_ms, condition_fn, 50) -- Check every 50ms
end

-- Get real file modification time
function M.get_real_mtime(filepath)
  if vim.fn.filereadable(filepath) ~= 1 then
    error("File does not exist: " .. filepath)
  end
  return vim.fn.getftime(filepath)
end

-- Make real file system change
function M.modify_file(filepath, new_content)
  if vim.fn.filereadable(filepath) ~= 1 then
    error("File does not exist: " .. filepath)
  end
  
  local file = io.open(filepath, 'w')
  if not file then
    error("Failed to open file for writing: " .. filepath)
  end
  
  file:write(new_content)
  file:close()
  
  -- Ensure mtime is updated
  vim.fn.system('touch ' .. filepath)
end

return M