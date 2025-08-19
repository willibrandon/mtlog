-- Interactive help system for mtlog.nvim
-- Provides quick access to documentation and diagnostic explanations

local M = {}

-- Quick help topics
M.topics = {
  getting_started = {
    title = "Getting Started",
    content = {
      "1. Install mtlog-analyzer:",
      "   go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest",
      "",
      "2. Basic usage:",
      "   :MtlogAnalyze - Analyze current buffer",
      "   :MtlogQuickFix - Apply fix at cursor",
      "   :MtlogStatus - Check plugin status",
      "",
      "3. Key mappings (recommended):",
      "   <leader>ma - Analyze buffer",
      "   <leader>mf - Quick fix",
      "   <leader>ms - Suppress diagnostic",
      "",
      "See :help mtlog for full documentation",
    }
  },
  
  diagnostics = {
    title = "Understanding Diagnostics",
    content = {
      "MTLOG001: Template/argument mismatch",
      "  The number of properties in template doesn't match arguments",
      "",
      "MTLOG002: Invalid format specifier",
      "  Format specifier like {Count:X} is invalid",
      "",
      "MTLOG003: Duplicate property",
      "  Same property appears multiple times in template",
      "",
      "MTLOG004: Property naming convention",
      "  Properties should use PascalCase like {UserId}",
      "",
      "MTLOG005: Missing capturing hints",
      "  Complex types should implement slog.LogValue",
      "",
      "MTLOG006: Missing error in Error/Fatal",
      "  Error() and Fatal() should include an error value",
      "",
      "MTLOG007: String should be constant",
      "  Common strings should be defined as constants",
      "",
      "MTLOG008: Dynamic template warning",
      "  Template is not a string literal",
      "",
      "MTLOG009: With() odd argument count",
      "  With() requires even number of key-value pairs",
      "",
      "MTLOG010: With() non-string key",
      "  With() keys must be string type",
      "",
      "MTLOG011: With() cross-call duplicate",
      "  Property set in With() duplicated across calls",
      "",
      "MTLOG012: With() reserved property",
      "  Property name is reserved (e.g., Timestamp, Level)",
      "",
      "MTLOG013: With() empty key",
      "  With() key is an empty string",
    }
  },
  
  quickfixes = {
    title = "Using Quick Fixes",
    content = {
      "Apply fixes in multiple ways:",
      "",
      "1. Command: :MtlogQuickFix",
      "2. LSP: :lua vim.lsp.buf.code_action()",
      "3. Keybinding: <leader>mf (if configured)",
      "",
      "Available fixes:",
      "• Add/remove arguments (MTLOG001)",
      "• Fix format specifiers (MTLOG002)",
      "• Add error parameter (MTLOG003)",
      "• Convert to PascalCase (MTLOG004)",
      "• Remove duplicates (MTLOG005)",
      "• Extract to constant (MTLOG007)",
      "",
      "Fixes are applied immediately and the file is re-analyzed.",
    }
  },
  
  suppressions = {
    title = "Managing Suppressions",
    content = {
      "Suppress unwanted diagnostics:",
      "",
      "1. Session suppression:",
      "   :MtlogSuppress MTLOG004",
      "",
      "2. Workspace suppression:",
      "   :MtlogSuppress MTLOG004",
      "   Choose 'Yes' to save to .mtlog.json",
      "",
      "3. Environment variable:",
      "   export MTLOG_SUPPRESS='MTLOG004,MTLOG008'",
      "",
      "4. Inline in code:",
      "   // mtlog:disable-next-line MTLOG004",
      "",
      "View suppressions: :MtlogShowSuppressions",
      "Remove: :MtlogUnsuppress MTLOG004",
      "Clear all: :MtlogUnsuppressAll",
    }
  },
  
  lsp_integration = {
    title = "LSP Integration",
    content = {
      "mtlog creates a fake LSP client for native integration:",
      "",
      "1. Ensure gopls is installed:",
      "   go install golang.org/x/tools/gopls@latest",
      "",
      "2. mtlog actions appear in:",
      "   :lua vim.lsp.buf.code_action()",
      "",
      "3. Check status:",
      "   :LspInfo - Should show 'mtlog-analyzer'",
      "",
      "4. Configure in setup:",
      "   lsp_integration = {",
      "     enabled = true,",
      "     show_suppress_action = true,",
      "   }",
      "",
      "Actions work alongside gopls and other LSP providers.",
    }
  },
  
  context_rules = {
    title = "Context Rules (Unique to Neovim)",
    content = {
      "Automatically control analysis based on context:",
      "",
      "Rule types:",
      "• path - Match file paths",
      "• buffer - Match buffer properties",
      "• project - Match project structure",
      "• custom - Custom Lua functions",
      "",
      "Example: Disable for test files:",
      "  {",
      "    type = 'path',",
      "    pattern = '_test%.go$',",
      "    action = 'disable',",
      "  }",
      "",
      "Commands:",
      "  :MtlogContext show - View active rules",
      "  :MtlogContext test - Test current buffer",
      "  :MtlogContext add-builtin - Add preset rules",
    }
  },
  
  performance = {
    title = "Performance Optimization",
    content = {
      "Tips for better performance:",
      "",
      "1. Adjust debounce delay:",
      "   debounce_ms = 500  -- Default: 250",
      "",
      "2. Limit concurrent analyses:",
      "   queue = { max_concurrent = 2 }",
      "",
      "3. Use context rules to skip files:",
      "   • Disable for test files",
      "   • Disable for generated code",
      "   • Disable for large files",
      "",
      "4. Monitor queue:",
      "   :MtlogQueue stats",
      "   :MtlogQueue show",
      "",
      "5. Clear cache if needed:",
      "   :MtlogCache clear",
    }
  },
  
  troubleshooting = {
    title = "Troubleshooting",
    content = {
      "Common issues and solutions:",
      "",
      "Analyzer not found:",
      "  • Install: go install .../mtlog-analyzer@latest",
      "  • Check PATH: which mtlog-analyzer",
      "  • Set path: analyzer_path = '/path/to/analyzer'",
      "",
      "No diagnostics:",
      "  • Check: :MtlogStatus",
      "  • Test: :MtlogAnalyze",
      "  • Kill switch: :MtlogToggleDiagnostics",
      "  • Context: :MtlogContext test",
      "",
      "LSP not working:",
      "  • Check: :LspInfo",
      "  • Manual: :MtlogCodeAction",
      "",
      "See :help mtlog-troubleshooting for more",
    }
  },
}

-- Diagnostic code explanations
M.diagnostic_explanations = {
  MTLOG001 = {
    name = "Template/argument mismatch",
    description = "The number of placeholders in the message template doesn't match the number of arguments provided.",
    example = {
      wrong = 'log.Info("User {UserId} logged in", userId, timestamp)',
      correct = 'log.Info("User {UserId} logged in at {Timestamp}", userId, timestamp)',
    },
    fix = "Add missing placeholders to the template or remove extra arguments.",
  },
  
  MTLOG002 = {
    name = "Invalid Format Specifier",
    description = "The format specifier in a placeholder is not valid or supported.",
    example = {
      wrong = 'log.Info("Count: {Count:X}")',
      correct = 'log.Info("Count: {Count:D}")',
    },
    fix = "Use valid format specifiers like :D (decimal), :F2 (float with 2 decimals), :P (percentage).",
  },
  
  MTLOG003 = {
    name = "Duplicate Property",
    description = "The same property appears multiple times in the message template.",
    example = {
      wrong = 'log.Info("From {UserId} to {UserId}")',
      correct = 'log.Info("From {FromUserId} to {ToUserId}")',
    },
    fix = "Use unique property names or remove duplicates.",
  },
  
  MTLOG004 = {
    name = "Property Naming Convention",
    description = "Properties in message templates should use PascalCase for consistency.",
    example = {
      wrong = 'log.Info("User {user_id} action {actionType}")',
      correct = 'log.Info("User {UserId} action {ActionType}")',
    },
    fix = "Convert property names to PascalCase.",
  },
  
  MTLOG005 = {
    name = "Missing Capturing Hints",
    description = "Complex types need capturing hints for proper structured logging. Use @ prefix for value capture or implement LogValue interface.",
    example = {
      wrong = 'log.Info("User {User}", complexUser)',
      correct = '// Implement LogValue() slog.Value on the User type',
    },
    fix = "Add @ prefix to capture the value, or implement the LogValue interface on the type.",
  },
  
  MTLOG006 = {
    name = "Missing Error Parameter",
    description = "Error() and Fatal() methods should include an error value as the last parameter.",
    example = {
      wrong = 'log.Error("Operation failed")',
      correct = 'log.Error("Operation failed", err)',
    },
    fix = "Add the error value as the last argument.",
  },
  
  MTLOG007 = {
    name = "String Should Be Constant",
    description = "Frequently used string values should be defined as constants.",
    example = {
      wrong = 'log.WithProperty("tenant_id", "acme-corp")',
      correct = 'const TenantAcmeCorp = "acme-corp"\nlog.WithProperty("tenant_id", TenantAcmeCorp)',
    },
    fix = "Extract the string to a constant.",
  },
  
  MTLOG008 = {
    name = "Dynamic Template Warning",
    description = "The message template is not a string literal, which prevents static analysis.",
    example = {
      wrong = 'template := "User {UserId}"\nlog.Info(template, userId)',
      correct = 'log.Info("User {UserId}", userId)',
    },
    fix = "Use string literals for message templates when possible.",
  },
  
  MTLOG009 = {
    name = "With() Odd Argument Count",
    description = "The With() method requires an even number of arguments as key-value pairs.",
    example = {
      wrong = 'log.With("UserId", 123, "Action")',
      correct = 'log.With("UserId", 123, "Action", "login")',
    },
    fix = "Add a value for the last key or remove the dangling key.",
  },
  
  MTLOG010 = {
    name = "With() Non-String Key",
    description = "Keys in With() method must be string type for property names.",
    example = {
      wrong = 'log.With(123, "value")',
      correct = 'log.With("Id", 123)',
    },
    fix = "Convert the key to a string or use a string constant.",
  },
  
  MTLOG011 = {
    name = "With() Cross-Call Duplicate",
    description = "The same property is being set in multiple With() calls, which may indicate a mistake.",
    example = {
      wrong = 'log.With("UserId", 1).With("UserId", 2)',
      correct = 'log.With("UserId", 1).With("Action", "login")',
    },
    fix = "Use unique property names or remove duplicate properties.",
  },
  
  MTLOG012 = {
    name = "With() Reserved Property",
    description = "The property name is reserved by the logging system and may cause confusion.",
    example = {
      wrong = 'log.With("Timestamp", time.Now())',
      correct = 'log.With("RequestTime", time.Now())',
    },
    fix = "Use a different property name that doesn't conflict with reserved names.",
  },
  
  MTLOG013 = {
    name = "With() Empty Key",
    description = "The key in With() method is an empty string, which is not valid.",
    example = {
      wrong = 'log.With("", "value")',
      correct = 'log.With("Property", "value")',
    },
    fix = "Provide a non-empty string as the property key.",
  },
}

-- Show help for a specific topic
function M.show_topic(topic_name)
  local topic = M.topics[topic_name]
  if not topic then
    vim.notify("Unknown help topic: " .. topic_name, vim.log.levels.WARN)
    return
  end
  
  M._show_in_float(topic.title, topic.content)
end

-- Show diagnostic explanation
function M.explain_diagnostic(code, diag)
  local explanation = M.diagnostic_explanations[code]
  if not explanation then
    vim.notify("Unknown diagnostic code: " .. code, vim.log.levels.WARN)
    return
  end
  
  local lines = {
    explanation.name,
    "",
    explanation.description,
    "",
  }
  
  -- For MTLOG005, check if the diagnostic message suggests @ prefix vs LogValue
  if code == "MTLOG005" and diag and diag.message then
    if diag.message:match("@") or diag.message:match("prefix") then
      -- Show @ prefix example
      table.insert(lines, "Example:")
      table.insert(lines, "  Wrong:   log.Info(\"User {User}\", complexUser)")
      table.insert(lines, "  Correct: log.Info(\"User {@User}\", complexUser)")
      table.insert(lines, "")
      table.insert(lines, "Fix: Add @ prefix to capture the value, or implement LogValue() on the type.")
    else
      -- Show LogValue implementation example
      table.insert(lines, "Example:")
      table.insert(lines, "  Wrong:   " .. explanation.example.wrong)
      table.insert(lines, "  Correct: " .. explanation.example.correct)
      table.insert(lines, "")
      table.insert(lines, "Fix: " .. explanation.fix)
    end
  else
    -- Default behavior for other diagnostics
    table.insert(lines, "Example:")
    table.insert(lines, "  Wrong:   " .. explanation.example.wrong)
    table.insert(lines, "  Correct: " .. explanation.example.correct)
    table.insert(lines, "")
    table.insert(lines, "Fix: " .. explanation.fix)
  end
  
  -- If we have the actual diagnostic, show its specific message too
  if diag and diag.message then
    table.insert(lines, "")
    table.insert(lines, "Diagnostic Message:")
    table.insert(lines, "  " .. diag.message)
  end
  
  M._show_in_float(code .. ": " .. explanation.name, lines)
end

-- Show main help menu
function M.show_menu()
  local items = {}
  for key, topic in pairs(M.topics) do
    table.insert(items, { key = key, title = topic.title })
  end
  
  -- Sort alphabetically
  table.sort(items, function(a, b) return a.title < b.title end)
  
  vim.ui.select(items, {
    prompt = "Select help topic:",
    format_item = function(item)
      return item.title
    end,
  }, function(choice)
    if choice then
      M.show_topic(choice.key)
    end
  end)
end

-- Explain diagnostic at cursor
function M.explain_at_cursor()
  local diagnostics = require('mtlog.diagnostics')
  local diag = diagnostics.get_diagnostic_at_cursor()
  
  if not diag or not diag.code then
    vim.notify("No mtlog diagnostic at cursor", vim.log.levels.INFO)
    return
  end
  
  M.explain_diagnostic(diag.code, diag)
end

-- Show quick reference card
function M.show_quick_reference()
  local lines = {
    "COMMANDS",
    "  :MtlogAnalyze      - Analyze buffer",
    "  :MtlogQuickFix     - Apply fix",
    "  :MtlogSuppress     - Suppress diagnostic",
    "  :MtlogStatus       - Show status",
    "  :MtlogHelp         - This help",
    "",
    "DIAGNOSTIC CODES",
    "  MTLOG001 - Template/argument mismatch",
    "  MTLOG002 - Invalid format specifier",
    "  MTLOG003 - Duplicate property",
    "  MTLOG004 - Use PascalCase properties",
    "  MTLOG005 - Missing capturing hints",
    "  MTLOG006 - Missing error parameter",
    "  MTLOG007 - Use string constant",
    "  MTLOG008 - Dynamic template",
    "  MTLOG009 - With() odd arguments",
    "  MTLOG010 - With() non-string key",
    "  MTLOG011 - With() duplicate",
    "  MTLOG012 - With() reserved name",
    "  MTLOG013 - With() empty key",
    "",
    "KEY BINDINGS (recommended)",
    "  <leader>ma - Analyze",
    "  <leader>mf - Quick fix",
    "  <leader>ms - Suppress",
    "  <leader>mh - Help",
    "",
    "Press ? on a diagnostic for explanation",
  }
  
  M._show_in_float("mtlog Quick Reference", lines)
end

-- Internal: Show content in floating window
function M._show_in_float(title, lines)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.api.nvim_buf_set_option(buf, 'modifiable', false)
  vim.api.nvim_buf_set_option(buf, 'filetype', 'markdown')
  
  local width = math.min(80, vim.o.columns - 10)
  local height = math.min(#lines + 2, vim.o.lines - 10)
  
  -- Calculate max line width for proper sizing
  local max_width = #title
  for _, line in ipairs(lines) do
    max_width = math.max(max_width, #line)
  end
  width = math.min(max_width + 4, width)
  
  local win_opts = {
    relative = 'editor',
    row = math.floor((vim.o.lines - height) / 2),
    col = math.floor((vim.o.columns - width) / 2),
    width = width,
    height = height,
    style = 'minimal',
    border = 'rounded',
  }
  
  -- Add title for Neovim 0.9+
  if vim.fn.has('nvim-0.9') == 1 then
    win_opts.title = ' ' .. title .. ' '
    win_opts.title_pos = 'center'
  end
  
  local win = vim.api.nvim_open_win(buf, true, win_opts)
  
  -- Set up keymaps to close
  local close_keys = {'<Esc>', 'q', '<CR>'}
  for _, key in ipairs(close_keys) do
    vim.api.nvim_buf_set_keymap(buf, 'n', key, '', {
      silent = true,
      callback = function()
        if vim.api.nvim_win_is_valid(win) then
          vim.api.nvim_win_close(win, true)
        end
      end
    })
  end
  
  -- Set window options for better appearance
  vim.api.nvim_win_set_option(win, 'wrap', true)
  vim.api.nvim_win_set_option(win, 'linebreak', true)
end

-- Check if this is first time setup
function M.is_first_time()
  -- Check if user has ever used the plugin
  local data_dir = vim.fn.stdpath('data')
  local marker_file = data_dir .. '/mtlog_initialized'
  return vim.fn.filereadable(marker_file) == 0
end

-- Mark as initialized
function M.mark_initialized()
  local data_dir = vim.fn.stdpath('data')
  local marker_file = data_dir .. '/mtlog_initialized'
  vim.fn.writefile({os.date()}, marker_file)
end

-- Show welcome message for first-time users
function M.show_welcome()
  local lines = {
    "Welcome to mtlog.nvim!",
    "",
    "This plugin provides static analysis for the mtlog logging library.",
    "",
    "QUICK START:",
    "",
    "1. Install the analyzer:",
    "   go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest",
    "",
    "2. Try these commands:",
    "   :MtlogAnalyze  - Analyze current file",
    "   :MtlogStatus   - Check plugin status",
    "   :MtlogHelp     - Open help menu",
    "",
    "3. Add to your config for key bindings:",
    "   vim.keymap.set('n', '<leader>ma', ':MtlogAnalyze<CR>')",
    "   vim.keymap.set('n', '<leader>mf', ':MtlogQuickFix<CR>')",
    "   vim.keymap.set('n', '<leader>mh', ':MtlogHelp<CR>')",
    "",
    "For full documentation: :help mtlog",
    "",
    "Press any key to continue...",
  }
  
  M._show_in_float("Welcome to mtlog.nvim", lines)
  M.mark_initialized()
end

return M