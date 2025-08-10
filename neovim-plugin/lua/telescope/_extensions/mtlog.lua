local has_telescope, telescope = pcall(require, 'telescope')
if not has_telescope then
  error('This extension requires telescope.nvim')
end

local actions = require('telescope.actions')
local action_state = require('telescope.actions.state')
local conf = require('telescope.config').values
local finders = require('telescope.finders')
local pickers = require('telescope.pickers')
local previewers = require('telescope.previewers')
local entry_display = require('telescope.pickers.entry_display')
local utils = require('telescope.utils')

local M = {}

-- Severity icons and highlights
local severity_icons = {
  [vim.diagnostic.severity.ERROR] = { icon = ' ', hl = 'DiagnosticError' },
  [vim.diagnostic.severity.WARN] = { icon = ' ', hl = 'DiagnosticWarn' },
  [vim.diagnostic.severity.INFO] = { icon = ' ', hl = 'DiagnosticInfo' },
  [vim.diagnostic.severity.HINT] = { icon = ' ', hl = 'DiagnosticHint' },
}

-- Get all mtlog diagnostics from all buffers
local function get_all_diagnostics(opts)
  opts = opts or {}
  local diagnostics = {}
  local ns = vim.api.nvim_create_namespace('mtlog-analyzer')
  
  -- Get all loaded buffers
  local buffers = vim.api.nvim_list_bufs()
  
  for _, bufnr in ipairs(buffers) do
    if vim.api.nvim_buf_is_loaded(bufnr) then
      local bufname = vim.api.nvim_buf_get_name(bufnr)
      
      -- Only process Go files
      if bufname:match('%.go$') then
        local buf_diagnostics = vim.diagnostic.get(bufnr, {
          namespace = ns,
          severity = opts.severity
        })
        
        for _, diagnostic in ipairs(buf_diagnostics) do
          table.insert(diagnostics, {
            bufnr = bufnr,
            bufname = bufname,
            lnum = diagnostic.lnum + 1,  -- Convert to 1-based
            col = diagnostic.col + 1,     -- Convert to 1-based
            text = diagnostic.message,
            severity = diagnostic.severity,
            code = diagnostic.code,
            source = diagnostic.source or 'mtlog',
          })
        end
      end
    end
  end
  
  -- Sort by filename and line number
  table.sort(diagnostics, function(a, b)
    if a.bufname ~= b.bufname then
      return a.bufname < b.bufname
    end
    return a.lnum < b.lnum
  end)
  
  return diagnostics
end

-- Create the telescope picker
M.diagnostics = function(opts)
  opts = opts or {}
  
  local diagnostics = get_all_diagnostics(opts)
  
  if #diagnostics == 0 then
    vim.notify('No mtlog diagnostics found', vim.log.levels.INFO)
    return
  end
  
  -- Create displayer
  local displayer = entry_display.create({
    separator = ' ',
    items = {
      { width = 2 },  -- Icon
      { width = 20 }, -- File
      { width = 5 },  -- Line:Col
      { width = 8 },  -- Code
      { remaining = true }, -- Message
    },
  })
  
  -- Create finder
  local finder = finders.new_table({
    results = diagnostics,
    entry_maker = function(diagnostic)
      local filename = vim.fn.fnamemodify(diagnostic.bufname, ':t')
      local severity_info = severity_icons[diagnostic.severity] or { icon = '?', hl = 'Normal' }
      
      return {
        value = diagnostic,
        display = function(entry)
          return displayer({
            { severity_info.icon, severity_info.hl },
            { filename, 'TelescopeResultsField' },
            { string.format('%d:%d', diagnostic.lnum, diagnostic.col), 'TelescopeResultsNumber' },
            { diagnostic.code or '', 'TelescopeResultsComment' },
            { diagnostic.text, 'TelescopeResultsString' },
          })
        end,
        ordinal = string.format('%s %d %s', diagnostic.bufname, diagnostic.lnum, diagnostic.text),
        filename = diagnostic.bufname,
        lnum = diagnostic.lnum,
        col = diagnostic.col,
      }
    end,
  })
  
  -- Create picker
  pickers.new(opts, {
    prompt_title = 'mtlog Diagnostics',
    finder = finder,
    sorter = conf.generic_sorter(opts),
    previewer = conf.grep_previewer(opts),
    attach_mappings = function(prompt_bufnr, map)
      actions.select_default:replace(function()
        actions.close(prompt_bufnr)
        local selection = action_state.get_selected_entry()
        if selection then
          local diagnostic = selection.value
          -- Open the file and jump to location
          vim.cmd('edit ' .. diagnostic.bufname)
          vim.api.nvim_win_set_cursor(0, { diagnostic.lnum, diagnostic.col - 1 })
        end
      end)
      
      -- Add quick fix action
      map('i', '<C-q>', function()
        local selection = action_state.get_selected_entry()
        if selection then
          local diagnostic = selection.value
          -- Apply quick fix if available
          vim.cmd('edit ' .. diagnostic.bufname)
          vim.api.nvim_win_set_cursor(0, { diagnostic.lnum, diagnostic.col - 1 })
          vim.cmd('MtlogQuickFix')
        end
        actions.close(prompt_bufnr)
      end)
      
      return true
    end,
  }):find()
end

-- Picker for managing suppressed diagnostics
M.suppressions = function(opts)
  opts = opts or {}
  
  local config = require('mtlog.config')
  local mtlog_utils = require('mtlog.utils')
  local suppressed = config.get('suppressed_diagnostics') or {}
  
  if #suppressed == 0 then
    vim.notify('No diagnostics are currently suppressed', vim.log.levels.INFO)
    return
  end
  
  -- Create displayer
  local displayer = entry_display.create({
    separator = ' ',
    items = {
      { width = 10 }, -- Diagnostic ID
      { remaining = true }, -- Description
    },
  })
  
  -- Build list of suppressed diagnostics with descriptions
  local items = {}
  for _, id in ipairs(suppressed) do
    table.insert(items, {
      id = id,
      description = mtlog_utils.get_diagnostic_description(id),
    })
  end
  
  -- Create finder
  local finder = finders.new_table({
    results = items,
    entry_maker = function(item)
      return {
        value = item,
        display = function(entry)
          return displayer({
            { item.id, 'TelescopeResultsConstant' },
            { item.description, 'TelescopeResultsString' },
          })
        end,
        ordinal = item.id .. ' ' .. item.description,
      }
    end,
  })
  
  -- Create picker
  pickers.new(opts, {
    prompt_title = 'Suppressed Diagnostics',
    finder = finder,
    sorter = conf.generic_sorter(opts),
    attach_mappings = function(prompt_bufnr, map)
      -- Default action: unsuppress the selected diagnostic
      actions.select_default:replace(function()
        local selection = action_state.get_selected_entry()
        if selection then
          actions.close(prompt_bufnr)
          require('mtlog').unsuppress_diagnostic(selection.value.id)
        end
      end)
      
      -- Multi-select to unsuppress multiple
      map('i', '<Tab>', actions.toggle_selection + actions.move_selection_worse)
      map('i', '<S-Tab>', actions.toggle_selection + actions.move_selection_better)
      
      -- Unsuppress all selected
      map('i', '<CR>', function()
        local picker = action_state.get_current_picker(prompt_bufnr)
        local multi_selections = picker:get_multi_selection()
        
        actions.close(prompt_bufnr)
        
        if #multi_selections > 0 then
          for _, selection in ipairs(multi_selections) do
            require('mtlog').unsuppress_diagnostic(selection.value.id)
          end
        else
          local selection = action_state.get_selected_entry()
          if selection then
            require('mtlog').unsuppress_diagnostic(selection.value.id)
          end
        end
      end)
      
      -- Clear all suppressions
      map('i', '<C-a>', function()
        actions.close(prompt_bufnr)
        require('mtlog').unsuppress_all()
      end)
      
      return true
    end,
  }):find()
end

-- Picker for viewing workspace analysis results
M.workspace = function(opts)
  opts = opts or {}
  
  -- Run workspace analysis first
  local analyzer = require('mtlog.analyzer')
  local results = analyzer.analyze_workspace_sync()
  
  if not results or vim.tbl_isempty(results) then
    vim.notify('No issues found in workspace', vim.log.levels.INFO)
    return
  end
  
  -- Flatten results into diagnostic list
  local diagnostics = {}
  for file, file_diagnostics in pairs(results) do
    for _, diag in ipairs(file_diagnostics) do
      table.insert(diagnostics, vim.tbl_extend('force', diag, {
        bufname = file,
      }))
    end
  end
  
  -- Sort by severity, then file, then line
  table.sort(diagnostics, function(a, b)
    if a.severity ~= b.severity then
      return a.severity < b.severity  -- ERROR < WARN < INFO < HINT
    end
    if a.bufname ~= b.bufname then
      return a.bufname < b.bufname
    end
    return a.lnum < b.lnum
  end)
  
  -- Use the same picker as diagnostics
  opts.prompt_title = 'mtlog Workspace Analysis'
  M.diagnostics(vim.tbl_extend('force', opts, { _diagnostics = diagnostics }))
end

-- Register the extension
return telescope.register_extension({
  setup = function(ext_config, config)
    -- Extension setup
  end,
  exports = {
    mtlog = M.diagnostics,
    diagnostics = M.diagnostics,
    workspace = M.workspace,
    suppressions = M.suppressions,
  },
})