-- Context-aware analyzer control for mtlog.nvim
-- Provides auto-command support for enabling/disabling analysis based on context

local M = {}

local config = require('mtlog.config')
local utils = require('mtlog.utils')

-- Context rules storage
local rules = {}
local compiled_patterns = {}

-- Rule types
M.rule_types = {
  PATH = 'path',           -- Match file path patterns
  PROJECT = 'project',     -- Match project markers (go.mod, etc.)
  BUFFER = 'buffer',       -- Match buffer properties
  CUSTOM = 'custom',       -- Custom function
}

-- Actions
M.actions = {
  ENABLE = 'enable',
  DISABLE = 'disable',
  IGNORE = 'ignore',
  CUSTOM = 'custom',
}

-- Setup context rules
function M.setup()
  rules = config.get('context_rules') or {}
  
  -- Add built-in rules if configured
  if config.get('use_builtin_rules') then
    -- Add ignore vendor rule
    table.insert(rules, M.builtin_rules.ignore_vendor)
    -- Add ignore test data rule
    table.insert(rules, M.builtin_rules.ignore_testdata)
    -- Add disable for large files
    table.insert(rules, M.builtin_rules.disable_large_files)
  end
  
  M._compile_patterns()
end

-- Add a context rule
---@param rule table Rule definition
function M.add_rule(rule)
  -- Validate rule
  if not rule.type or not rule.action then
    error('Context rule must have type and action')
  end
  
  -- Add to rules
  table.insert(rules, rule)
  
  -- Recompile patterns
  if rule.type == M.rule_types.PATH and rule.pattern then
    M._compile_pattern(#rules, rule)
  end
end

-- Remove a rule by index
---@param index number Rule index
function M.remove_rule(index)
  if rules[index] then
    table.remove(rules, index)
    compiled_patterns[index] = nil
    M._compile_patterns()
  end
end

-- Clear all rules
function M.clear_rules()
  rules = {}
  compiled_patterns = {}
end

-- Evaluate context for a buffer
---@param bufnr number Buffer number
---@return string? action Action to take or nil
---@return table? rule Matching rule or nil
function M.evaluate(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return nil
  end
  
  local filepath = vim.api.nvim_buf_get_name(bufnr)
  local filetype = vim.bo[bufnr].filetype
  
  -- Only process Go files
  if filetype ~= 'go' and not filepath:match('%.go$') then
    return nil
  end
  
  -- Evaluate rules in order (first match wins)
  for i, rule in ipairs(rules) do
    local matches = false
    
    if rule.type == M.rule_types.PATH then
      matches = M._match_path(filepath, rule, i)
    elseif rule.type == M.rule_types.PROJECT then
      matches = M._match_project(filepath, rule)
    elseif rule.type == M.rule_types.BUFFER then
      matches = M._match_buffer(bufnr, rule)
    elseif rule.type == M.rule_types.CUSTOM then
      matches = M._match_custom(bufnr, filepath, rule)
    end
    
    if matches then
      return rule.action, rule
    end
  end
  
  return nil
end

-- Apply action based on context
---@param bufnr number Buffer number
---@return boolean applied Whether an action was applied
function M.apply_context(bufnr)
  local action, rule = M.evaluate(bufnr)
  
  if not action then
    return false
  end
  
  local mtlog = require('mtlog')
  
  if action == M.actions.ENABLE then
    -- Enable analysis for this buffer
    if not mtlog.enabled() then
      mtlog.enable()
    end
    
    -- Trigger analysis if configured
    if rule.analyze_immediately then
      vim.defer_fn(function()
        mtlog.analyze_buffer(bufnr)
      end, 100)
    end
    
    return true
    
  elseif action == M.actions.DISABLE then
    -- Skip analysis for this buffer
    return true
    
  elseif action == M.actions.IGNORE then
    -- Mark buffer as ignored
    vim.b[bufnr].mtlog_ignored = true
    return true
    
  elseif action == M.actions.CUSTOM and rule.handler then
    -- Run custom handler
    rule.handler(bufnr)
    return true
  end
  
  return false
end

-- Check if buffer should be analyzed based on context
---@param bufnr number Buffer number
---@return boolean should_analyze
function M.should_analyze(bufnr)
  -- Check if buffer is explicitly ignored
  if vim.b[bufnr].mtlog_ignored then
    return false
  end
  
  local action, _ = M.evaluate(bufnr)
  
  if action == M.actions.DISABLE or action == M.actions.IGNORE then
    return false
  end
  
  return true
end

-- Private: Compile patterns for efficiency
function M._compile_patterns()
  compiled_patterns = {}
  for i, rule in ipairs(rules) do
    if rule.type == M.rule_types.PATH and rule.pattern then
      M._compile_pattern(i, rule)
    end
  end
end

-- Private: Compile a single pattern
function M._compile_pattern(index, rule)
  if rule.regex then
    -- It's a regex pattern
    local ok, regex = pcall(vim.regex, rule.pattern)
    if ok then
      compiled_patterns[index] = { type = 'regex', pattern = regex }
    end
  else
    -- It's a glob pattern
    compiled_patterns[index] = { type = 'glob', pattern = rule.pattern }
  end
end

-- Private: Match path pattern
function M._match_path(filepath, rule, index)
  if not rule.pattern then
    return false
  end
  
  local compiled = compiled_patterns[index]
  if not compiled then
    return false
  end
  
  if compiled.type == 'regex' then
    return compiled.pattern:match_str(filepath) ~= nil
  else
    -- Glob pattern - use glob2regpat for proper glob matching
    local pattern = vim.fn.glob2regpat(compiled.pattern)
    return vim.fn.match(filepath, pattern) >= 0
  end
end

-- Private: Match project markers
function M._match_project(filepath, rule)
  if not rule.markers then
    return false
  end
  
  -- Find project root
  local project_root = utils.find_project_root(filepath)
  if not project_root then
    return false
  end
  
  -- Check for markers
  for _, marker in ipairs(rule.markers) do
    local marker_path = project_root .. '/' .. marker
    if vim.fn.filereadable(marker_path) == 1 or vim.fn.isdirectory(marker_path) == 1 then
      -- Check marker content if specified
      if rule.marker_content and rule.marker_content[marker] then
        local content = vim.fn.readfile(marker_path)
        local pattern = rule.marker_content[marker]
        for _, line in ipairs(content) do
          if line:match(pattern) then
            return true
          end
        end
        return false
      end
      return true
    end
  end
  
  return false
end

-- Private: Match buffer properties
function M._match_buffer(bufnr, rule)
  -- At least one property must be specified for buffer rules
  local has_criteria = false
  
  if rule.modified ~= nil then
    has_criteria = true
    if vim.bo[bufnr].modified ~= rule.modified then
      return false
    end
  end
  
  if rule.readonly ~= nil then
    has_criteria = true
    if vim.bo[bufnr].readonly ~= rule.readonly then
      return false
    end
  end
  
  if rule.buftype then
    has_criteria = true
    if vim.bo[bufnr].buftype ~= rule.buftype then
      return false
    end
  end
  
  if rule.filesize then
    has_criteria = true
    local filepath = vim.api.nvim_buf_get_name(bufnr)
    if filepath ~= '' then
      local size = vim.fn.getfsize(filepath)
      -- Check if we should trigger based on file size
      -- max: trigger when size EXCEEDS max
      -- min: trigger when size is BELOW min
      if rule.filesize.max and rule.filesize.min then
        -- Both max and min: trigger if outside range
        if size > rule.filesize.min and size <= rule.filesize.max then
          return false  -- Within range, don't match
        end
      elseif rule.filesize.max then
        -- Only max: trigger when exceeding
        if size <= rule.filesize.max then
          return false  -- Within limit, don't match
        end
      elseif rule.filesize.min then
        -- Only min: trigger when below
        if size >= rule.filesize.min then
          return false  -- Above minimum, don't match
        end
      end
    else
      -- No file path, can't check size
      return false
    end
  end
  
  if rule.line_count then
    has_criteria = true
    local lines = vim.api.nvim_buf_line_count(bufnr)
    -- Check if we should trigger based on line count
    -- max: trigger when lines EXCEED max
    -- min: trigger when lines are BELOW min
    if rule.line_count.max and rule.line_count.min then
      -- Both max and min: trigger if outside range
      if lines >= rule.line_count.min and lines <= rule.line_count.max then
        return false  -- Within range, don't match
      end
    elseif rule.line_count.max then
      -- Only max: trigger when exceeding
      if lines <= rule.line_count.max then
        return false  -- Within limit, don't match
      end
    elseif rule.line_count.min then
      -- Only min: trigger when below
      if lines >= rule.line_count.min then
        return false  -- Above minimum, don't match
      end
    end
  end
  
  -- Must have at least one criteria to match
  return has_criteria
end

-- Private: Match custom function
function M._match_custom(bufnr, filepath, rule)
  if not rule.matcher then
    return false
  end
  
  local ok, result = pcall(rule.matcher, bufnr, filepath)
  if ok then
    return result
  end
  
  return false
end

-- Get all rules
---@return table[] rules
function M.get_rules()
  return vim.deepcopy(rules)
end

-- Get rules summary for display
---@return string[] lines
function M.get_rules_summary()
  local lines = {}
  
  if #rules == 0 then
    table.insert(lines, 'No context rules defined')
    return lines
  end
  
  table.insert(lines, 'Context Rules:')
  table.insert(lines, '')
  
  for i, rule in ipairs(rules) do
    local desc = string.format('%d. [%s] %s', i, rule.type:upper(), rule.action)
    
    if rule.description then
      desc = desc .. ' - ' .. rule.description
    elseif rule.type == M.rule_types.PATH and rule.pattern then
      desc = desc .. ' - Pattern: ' .. rule.pattern
    elseif rule.type == M.rule_types.PROJECT and rule.markers then
      desc = desc .. ' - Markers: ' .. table.concat(rule.markers, ', ')
    end
    
    table.insert(lines, desc)
  end
  
  return lines
end

-- Built-in context rules
M.builtin_rules = {
  -- Ignore vendor directories
  ignore_vendor = {
    type = M.rule_types.PATH,
    pattern = '*/vendor/*',
    action = M.actions.IGNORE,
    description = 'Ignore vendor directories',
  },
  
  -- Ignore test data
  ignore_testdata = {
    type = M.rule_types.PATH,
    pattern = '*/testdata/*',
    action = M.actions.IGNORE,
    description = 'Ignore test data files',
  },
  
  -- Ignore generated files
  ignore_generated = {
    type = M.rule_types.PATH,
    pattern = '*.pb.go',
    action = M.actions.IGNORE,
    description = 'Ignore protobuf generated files',
  },
  
  -- Disable for large files
  disable_large_files = {
    type = M.rule_types.BUFFER,
    filesize = { max = 100000 },  -- 100KB
    action = M.actions.DISABLE,
    description = 'Disable for files larger than 100KB',
  },
  
  -- Enable for mtlog projects
  enable_mtlog_projects = {
    type = M.rule_types.PROJECT,
    markers = { 'go.mod' },
    marker_content = {
      ['go.mod'] = 'github.com/willibrandon/mtlog',
    },
    action = M.actions.ENABLE,
    analyze_immediately = true,
    description = 'Auto-enable for projects using mtlog',
  },
}

return M