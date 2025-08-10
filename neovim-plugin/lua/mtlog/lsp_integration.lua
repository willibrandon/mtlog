-- LSP integration for mtlog.nvim
-- Creates a fake LSP client to provide code actions

local M = {}

local diagnostics = require('mtlog.diagnostics')
local config = require('mtlog.config')

-- Check if LSP is available
local has_lsp = vim.lsp ~= nil

-- Store our fake client ID
local mtlog_client_id = nil

-- Initialize the fake LSP client
function M.setup()
  if not has_lsp then
    return
  end
  
  -- Only setup once
  if mtlog_client_id then
    return
  end
  
  -- Create a fake LSP client that only handles code actions
  local client_config = {
    name = "mtlog-analyzer",
    cmd = function()
      -- Return a fake RPC client that handles requests
      return {
        request = function(method, params, callback)
          -- Handle initialize request
          if method == "initialize" then
            callback(nil, {
              capabilities = {
                codeActionProvider = true,
                executeCommandProvider = {
                  commands = { "mtlog.suppress_diagnostic", "mtlog.apply_fix" }
                },
              },
              serverInfo = {
                name = "mtlog-analyzer",
                version = "1.0.0",
              },
            })
          -- Handle textDocument/codeAction
          elseif method == "textDocument/codeAction" then
            local bufnr = vim.uri_to_bufnr(params.textDocument.uri)
            local actions = M.get_code_actions(bufnr, params.range)
            callback(nil, actions)
          -- Handle workspace/applyEdit
          elseif method == "workspace/applyEdit" then
            -- Apply the workspace edit
            vim.lsp.util.apply_workspace_edit(params.edit, 'utf-8')
            callback(nil, { applied = true })
          -- Handle workspace/executeCommand
          elseif method == "workspace/executeCommand" then
            if params.command == "mtlog.suppress_diagnostic" then
              local code = params.arguments and params.arguments[1]
              if code then
                require('mtlog').suppress_diagnostic(code)
              end
            elseif params.command == "mtlog.apply_fix" then
              -- Apply the fix using our working mechanism
              local args = params.arguments
              if args and args[1] and args[2] and args[3] then
                local bufnr = args[1]
                local diag = args[2]
                local fix_index = args[3]
                
                -- Use the same logic as MtlogQuickFix command
                local diagnostics = require('mtlog.diagnostics')
                if diagnostics.apply_suggested_fix(diag, fix_index) then
                  -- Clear diagnostics immediately, invalidate cache, and re-analyze
                  local filepath = vim.api.nvim_buf_get_name(bufnr)
                  diagnostics.clear(bufnr)
                  require('mtlog.cache').invalidate(filepath)
                  -- Save the buffer to ensure changes are written
                  vim.cmd('write')
                  -- Small delay to let the buffer update and file save
                  vim.defer_fn(function()
                    require('mtlog').analyze_buffer(bufnr)
                  end, 500)
                end
              end
            end
            callback(nil, nil)
          -- Handle shutdown
          elseif method == "shutdown" then
            callback(nil, nil)
          else
            -- Return empty result for other requests
            callback(nil, {})
          end
        end,
        notify = function() end,
        is_closing = function() return false end,
        terminate = function() end,
      }
    end,
    -- Only support code actions
    capabilities = {
      textDocument = {
        codeAction = {
          dynamicRegistration = false,
          codeActionLiteralSupport = {
            codeActionKind = {
              valueSet = { "quickfix", "refactor" }
            }
          }
        }
      }
    },
    -- Attach to Go files
    filetypes = { "go" },
    -- Don't interfere with other features
    on_attach = function(client, bufnr)
      -- Disable all capabilities except code actions
      client.server_capabilities.documentFormattingProvider = false
      client.server_capabilities.documentRangeFormattingProvider = false
      client.server_capabilities.documentHighlightProvider = false
      client.server_capabilities.documentSymbolProvider = false
      client.server_capabilities.hoverProvider = false
      client.server_capabilities.completionProvider = false
      client.server_capabilities.signatureHelpProvider = false
      client.server_capabilities.definitionProvider = false
      client.server_capabilities.referencesProvider = false
      client.server_capabilities.renameProvider = false
      
      -- Only keep code action provider
      client.server_capabilities.codeActionProvider = true
    end,
  }
  
  -- Start the fake client
  mtlog_client_id = vim.lsp.start(client_config)
end

-- Get code actions for a specific range
---@param bufnr number Buffer number
---@param range table LSP range
---@return table[] Code actions
function M.get_code_actions(bufnr, range)
  local actions = {}
  
  -- Convert LSP range to Neovim positions (LSP is 0-based, Neovim is 1-based for lines)
  local start_line = range.start.line
  local end_line = range['end'].line
  
  -- Get diagnostics in the range
  local diags = vim.diagnostic.get(bufnr, {
    namespace = diagnostics.get_namespace(),
    lnum = start_line,
    end_lnum = end_line,
  })
  
  -- Process each diagnostic
  for _, diag in ipairs(diags) do
    if diag.user_data and diag.user_data.suggested_fixes then
      for i, fix in ipairs(diag.user_data.suggested_fixes) do
        -- Create LSP CodeAction with a command instead of workspace edit
        -- This ensures we use our working fix application logic
        local action = {
          title = fix.title or ('Fix ' .. diag.code),
          kind = 'quickfix',
          diagnostics = { M._diagnostic_to_lsp(diag) },
          -- Use command instead of edit to apply the fix through our working mechanism
          command = {
            title = 'Apply mtlog fix',
            command = 'mtlog.apply_fix',
            arguments = { bufnr, diag, i },
          },
          isPreferred = (i == 1),  -- First fix is preferred
          data = {
            source = 'mtlog-analyzer',
            diagnostic_code = diag.code,
            fix_index = i,
          },
        }
        
        table.insert(actions, action)
      end
    end
    
    -- Add suppression action if configured
    if config.get('lsp_integration.show_suppress_action') ~= false then
      if diag.code and not M._is_suppressed(diag.code) then
        table.insert(actions, {
          title = string.format('Suppress %s', diag.code),
          kind = 'refactor',
          diagnostics = { M._diagnostic_to_lsp(diag) },
          command = {
            title = 'Suppress diagnostic',
            command = 'mtlog.suppress_diagnostic',
            arguments = { diag.code },
          },
          data = {
            source = 'mtlog-analyzer',
            action_type = 'suppress',
          },
        })
      end
    end
  end
  
  return actions
end

-- Convert Neovim diagnostic to LSP diagnostic format
function M._diagnostic_to_lsp(diag)
  return {
    range = {
      start = {
        line = diag.lnum,
        character = diag.col,
      },
      ['end'] = {
        line = diag.end_lnum or diag.lnum,
        character = diag.end_col or (diag.col + 1),
      },
    },
    severity = M._severity_to_lsp(diag.severity),
    code = diag.code,
    source = diag.source,
    message = diag.message,
  }
end

-- Convert Neovim severity to LSP severity
function M._severity_to_lsp(severity)
  local map = {
    [vim.diagnostic.severity.ERROR] = 1,
    [vim.diagnostic.severity.WARN] = 2,
    [vim.diagnostic.severity.INFO] = 3,
    [vim.diagnostic.severity.HINT] = 4,
  }
  return map[severity] or 3
end

-- Create LSP WorkspaceEdit from fix edits
function M._create_workspace_edit(bufnr, edits)
  local uri = vim.uri_from_bufnr(bufnr)
  local text_edits = {}
  
  if not edits then
    return { changes = { [uri] = text_edits } }
  end
  
  for _, edit in ipairs(edits) do
    local text_edit
    
    -- Handle different edit formats from analyzer
    if edit.range then
      -- LSP-style format with range
      text_edit = {
        range = edit.range,
        newText = edit.newText or edit.new_text or '',
      }
    elseif edit.pos and edit['end'] then
      -- Analyzer stdin format with pos/end strings
      -- Parse positions from "file:line:col" format
      local start_parts = vim.split(edit.pos, ':', { plain = true })
      local end_parts = vim.split(edit['end'], ':', { plain = true })
      
      if #start_parts >= 3 and #end_parts >= 3 then
        -- Convert from 1-based to 0-based for LSP
        local start_line = tonumber(start_parts[#start_parts - 1]) - 1
        local start_col = tonumber(start_parts[#start_parts]) - 1
        local end_line = tonumber(end_parts[#end_parts - 1]) - 1
        local end_col = tonumber(end_parts[#end_parts]) - 1
        
        text_edit = {
          range = {
            start = { line = start_line, character = start_col },
            ['end'] = { line = end_line, character = end_col },
          },
          newText = edit.newText or '',
        }
      end
    elseif edit.start_line then
      -- Our internal format
      text_edit = {
        range = {
          start = {
            line = (edit.start_line or edit.line or 1) - 1,  -- Convert to 0-based
            character = (edit.start_col or edit.col or 1) - 1,
          },
          ['end'] = {
            line = (edit.end_line or edit.start_line or edit.line or 1) - 1,
            character = (edit.end_col or edit.start_col or edit.col or 1) - 1,
          },
        },
        newText = edit.new_text or edit.newText or '',
      }
    elseif edit.start and edit['end'] and edit.new then
      -- Byte offset format - don't handle for now
      -- Would need buffer content to convert
      vim.notify('Unsupported edit format (byte offsets)', vim.log.levels.WARN)
    end
    
    if text_edit then
      table.insert(text_edits, text_edit)
    end
  end
  
  return {
    changes = {
      [uri] = text_edits,
    },
  }
end

-- Check if a diagnostic is suppressed
function M._is_suppressed(code)
  local suppressed = config.get('suppressed_diagnostics') or {}
  for _, suppressed_code in ipairs(suppressed) do
    if suppressed_code == code then
      return true
    end
  end
  return false
end

-- Register mtlog commands as LSP commands
function M.register_commands()
  if not has_lsp then
    return
  end
  
  -- Register suppress command
  vim.lsp.commands['mtlog.suppress_diagnostic'] = function(command, ctx)
    local args = command.arguments
    if args and args[1] then
      local mtlog = require('mtlog')
      mtlog.suppress_diagnostic(args[1])
    end
  end
  
  -- Register quick fix command
  vim.lsp.commands['mtlog.apply_fix'] = function(command, ctx)
    local args = command.arguments
    if args and args[1] and args[2] and args[3] then
      local bufnr = args[1]
      local diag = args[2]
      local fix_index = args[3]
      
      -- Apply the fix
      if diagnostics.apply_suggested_fix(diag, fix_index) then
        -- Clear and re-analyze
        local filepath = vim.api.nvim_buf_get_name(bufnr)
        diagnostics.clear(bufnr)
        require('mtlog.cache').invalidate(filepath)
        vim.cmd('write')
        vim.defer_fn(function()
          require('mtlog').analyze_buffer(bufnr)
        end, 500)
      end
    end
  end
end

-- Enable LSP integration for a buffer
---@param bufnr number Buffer number
function M.attach(bufnr)
  if not has_lsp then
    return
  end
  
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  
  -- Attach our fake client to this buffer if it's a Go file
  if vim.bo[bufnr].filetype == 'go' and mtlog_client_id then
    vim.lsp.buf_attach_client(bufnr, mtlog_client_id)
  end
  
  -- Add buffer-local commands if needed
  vim.api.nvim_buf_create_user_command(bufnr, 'MtlogCodeAction', function()
    -- Trigger code action at cursor
    if vim.lsp.buf.code_action then
      vim.lsp.buf.code_action({
        filter = function(action)
          return action.data and action.data.source == 'mtlog-analyzer'
        end,
      })
    end
  end, {
    desc = 'Show mtlog code actions',
  })
end

-- Clean up - stop the fake client
function M.stop()
  if mtlog_client_id then
    vim.lsp.stop_client(mtlog_client_id)
    mtlog_client_id = nil
  end
end

-- Alternative: Provide code actions without LSP override
-- This can be used when users don't want to use the fake LSP client
function M.get_code_actions_at_cursor()
  local bufnr = vim.api.nvim_get_current_buf()
  local row, col = unpack(vim.api.nvim_win_get_cursor(0))
  
  -- Create range for cursor position
  local range = {
    start = { line = row - 1, character = col },
    ['end'] = { line = row - 1, character = col + 1 },
  }
  
  return M.get_code_actions(bufnr, range)
end

-- Show code actions menu manually
function M.show_code_actions()
  local actions = M.get_code_actions_at_cursor()
  
  if not actions or #actions == 0 then
    vim.notify('No mtlog code actions available', vim.log.levels.INFO)
    return
  end
  
  -- Create menu items
  local items = {}
  for i, action in ipairs(actions) do
    table.insert(items, string.format('%d. %s', i, action.title))
  end
  
  -- Show selection menu
  vim.ui.select(items, {
    prompt = 'Select code action:',
  }, function(choice, idx)
    if not choice then
      return
    end
    
    local action = actions[idx]
    
    -- For quick fixes, we need to apply them directly since the LSP format conversion
    -- may not match what the diagnostics module expects
    if action.data and action.data.source == 'mtlog-analyzer' then
      if action.data.action_type == 'suppress' then
        -- Handle suppress command
        local mtlog = require('mtlog')
        mtlog.suppress_diagnostic(action.command.arguments[1])
      else
        -- It's a fix action - find and apply it
        local cursor_line = vim.fn.line('.') - 1
        local diags = vim.diagnostic.get(0, {
          lnum = cursor_line,
          namespace = diagnostics.get_namespace(),
        })
        
        -- Find the diagnostic that matches our action
        for _, diag in ipairs(diags) do
          if diag.code == action.data.diagnostic_code and diag.user_data and diag.user_data.suggested_fixes then
            local fix_index = action.data.fix_index or 1
            -- Clear the diagnostic immediately to prevent flash
            local bufnr = vim.api.nvim_get_current_buf()
            local other_diags = {}
            for _, d in ipairs(diags) do
              if d ~= diag then
                table.insert(other_diags, d)
              end
            end
            vim.diagnostic.set(diagnostics.get_namespace(), bufnr, other_diags)
            
            -- Apply the fix
            vim.schedule(function()
              diagnostics.apply_suggested_fix(diag, fix_index)
            end)
            break
          end
        end
      end
    elseif action.edit and action.edit.changes then
      -- Apply workspace edit (for other LSP servers)
      vim.lsp.util.apply_workspace_edit(action.edit, 'utf-8')
    elseif action.command then
      -- Execute command
      if vim.lsp.commands[action.command.command] then
        vim.lsp.commands[action.command.command](action.command)
      end
    end
  end)
end

return M