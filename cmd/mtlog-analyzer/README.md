# mtlog-analyzer

A production-ready static analysis tool for mtlog that catches common mistakes at compile time. The analyzer provides comprehensive validation with support for receiver type checking, template caching, severity levels, and suggested fixes.

## Features

### Core Checks
- **Template/argument mismatch detection** - Catches when property count doesn't match argument count
- **Format specifier validation** - Validates format specifiers like `{Count:000}` or `{Price:F2}`
- **Property naming checks** - Warns about empty properties, spaces, or properties starting with numbers
- **Duplicate property detection** - Catches when the same property appears multiple times
- **Capturing hints** - Suggests using `@` prefix for complex types
- **Error logging patterns** - Warns when using Error level without an actual error
- **Context key suggestions** - Suggests constants for commonly used context keys
- **With() method validation** - Validates key-value pairs in structured field logging

### Advanced Features
- **Receiver type checking** - Reduces false positives by verifying logger types
- **Template caching** - Improves performance by caching parsed templates
- **Severity levels** - Differentiates between errors, warnings, and suggestions
- **Suggested fixes** - Provides automatic fixes for common issues
- **Error type verification** - Validates that error arguments are actually error types
- **String-to-constant extraction** - Automatically extracts repeated context keys to constants
- **LogValue() stub generation** - Creates safe logging implementations for complex types

## Installation

```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
```

After installation, ensure the Go binary directory is in your PATH:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export PATH="$PATH:$(go env GOPATH)/bin"

# On Windows (PowerShell)
$env:PATH += ";$(go env GOPATH)\bin"
```

To verify the installation:

```bash
mtlog-analyzer -h
```

## Usage

### With go vet

```bash
go vet -vettool=$(which mtlog-analyzer) ./...
```

### Standalone

```bash
mtlog-analyzer ./...
```

### In CI

Add to your GitHub Actions or other CI:

```yaml
- name: Install mtlog-analyzer
  run: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest

- name: Run mtlog-analyzer
  run: go vet -vettool=$(which mtlog-analyzer) ./...
```

## Examples

The analyzer catches issues like:

```go
// ❌ Template has 2 properties but 1 argument provided
log.Information("User {UserId} logged in from {IP}", userId)

// ❌ Duplicate property 'UserId'
log.Information("User {UserId} did {Action} as {UserId}", id, "login", id)

// ❌ Using @ prefix for basic type
log.Information("Count is {@Count}", 42)

// ⚠️ Repeated context key - suggests extracting to constant
log.ForContext("user_id", 123).Information("Login")
log.ForContext("user_id", 456).Information("Logout")
// Quick fix creates: const userIDContextKey = "user_id"

// ⚠️ Invalid format specifier - suggests correction
log.Information("Count: {Count:d3}", 42)
// Quick fix changes to: {Count:000}

// 💡 Complex type without capturing - suggests LogValue() stub
log.Information("User details: {User}", user)
// Quick fix generates LogValue() method with sensitive field detection

// ✅ Correct usage
log.Information("User {@User} has {Count} items", user, count)

// With() method checks
// ❌ Odd number of arguments
log.With("key1", "value1", "key2")  // MTLOG009: requires even number of arguments

// ❌ Non-string keys
log.With(123, "value")  // MTLOG010: key must be a string

// ❌ Duplicate keys
log.With("id", 1, "name", "test", "id", 2)  // MTLOG003: duplicate key 'id'

// ❌ Cross-call duplicates (overriding properties)
logger := log.With("service", "api")
logger.With("service", "auth")  // MTLOG011: overrides property 'service'

// Method chaining
log.With("id", 1).With("id", 2)  // MTLOG011: overrides in chain

// ❌ Empty keys
log.With("", "value")  // MTLOG013: empty key will be ignored

// ⚠️ Reserved properties (when -check-reserved is enabled)
log.With("Message", "custom")  // MTLOG012: shadows built-in property

// ✅ Correct With() usage
log.With("userId", 123, "requestId", "abc-123")
```

## Severity Levels

The analyzer reports issues at three severity levels:

1. **Error** - Critical issues that will cause runtime problems
   - Template/argument count mismatches (MTLOG001)
   - Invalid property names (spaces, starting with numbers) (MTLOG004)
   - With() odd argument count (MTLOG009)
   - With() non-string keys (MTLOG010)
   - With() empty keys (MTLOG013)

2. **Warning** - Issues that may indicate mistakes
   - Duplicate properties (MTLOG003)
   - Using `@` prefix with basic types (MTLOG005)
   - Dynamic template strings (MTLOG008)
   - Cross-call property overrides in With()/ForContext() (MTLOG011)

3. **Suggestion** - Best practice recommendations
   - PascalCase property naming (MTLOG004)
   - Missing `@` prefix for complex types (MTLOG005)
   - Error logging without error values (MTLOG006)
   - Common context keys without constants (MTLOG007)
   - Reserved property names in With() (MTLOG012, requires -check-reserved flag)

## Importable Package

The analyzer can also be imported as a package for use in custom tools:

```go
import "github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"

// Use analyzer.Analyzer in your own analysis tools
```

## Performance

The analyzer includes several performance optimizations:
- Template caching to avoid redundant parsing
- Receiver type checking to skip non-mtlog calls early
- Efficient property extraction with minimal allocations

### Performance Considerations

For very large codebases, the cross-call duplicate detection (MTLOG011) performs data flow analysis that tracks logger variable assignments across the entire package. While optimized, this can add processing time for packages with thousands of logger instances.

If you experience performance issues:
```bash
# Disable only cross-call duplicate detection
mtlog-analyzer -disable=with-cross-call ./...

# Or suppress the specific diagnostic
mtlog-analyzer -suppress=MTLOG011 ./...
```

This only affects the cross-call duplicate detection. Single-call duplicate detection (MTLOG003) and other With() diagnostics remain active.

## Configuration

The analyzer supports several configuration options via flags:

```bash
# Enable strict format specifier validation
mtlog-analyzer -strict ./...

# Configure common context keys
mtlog-analyzer -common-keys=user_id,tenant_id,request_id ./...

# Disable specific checks
mtlog-analyzer -disable=naming,capturing ./...

# Ignore dynamic template warnings
mtlog-analyzer -ignore-dynamic-templates ./...

# Enable strict logger type checking (only accept exact mtlog types)
mtlog-analyzer -strict-logger-types ./...

# Downgrade errors to warnings (useful for CI during migration)
mtlog-analyzer -downgrade-errors ./...
```

Available flags:
- `-strict` - Enable strict format specifier validation
- `-common-keys` - Comma-separated list of context keys to treat as common (appends to defaults)
- `-disable` - Comma-separated list of checks to disable
- `-ignore-dynamic-templates` - Suppress warnings for non-literal template strings
- `-strict-logger-types` - Only analyze exact mtlog logger types (disable lenient checking)
- `-downgrade-errors` - Downgrade all errors to warnings (useful for CI environments during migration)
- `-check-reserved` - Enable checking for reserved property names in With() calls
- `-reserved-props` - Comma-separated list of reserved property names (overrides defaults)
- `-suppress` - Comma-separated list of diagnostic IDs to suppress (e.g., MTLOG001,MTLOG004)

Available check names for `-disable`:
- `template` - Template/argument mismatch detection
- `duplicate` - Duplicate property detection (includes With() duplicates)
- `naming` - Property naming checks (including PascalCase suggestions)
- `capturing` - Capturing hint suggestions
- `error` - Error logging pattern checks
- `context` - Context key suggestions
- `with-odd` - With() odd argument count check
- `with-nonstring` - With() non-string key check
- `with-empty` - With() empty key check
- `with-cross-call` - With()/ForContext() cross-call duplicate detection

### Ignoring Specific Warnings

You can ignore specific warnings using standard Go vet comments:

```go
//lint:ignore mtlog reason
log.Information("User {userId} logged in", id) // lowercase property name
```

### Diagnostic Suppression

The analyzer supports multiple methods for suppressing specific diagnostics:

#### 1. Command-line Flag
Use the `-suppress` flag with comma-separated diagnostic IDs:

```bash
# Suppress specific diagnostics
mtlog-analyzer -suppress=MTLOG001,MTLOG009 ./...

# Suppress With() odd arguments and non-string key diagnostics
mtlog-analyzer -suppress=MTLOG009,MTLOG010 ./...
```

#### 2. Environment Variable
Set the `MTLOG_SUPPRESS` environment variable:

```bash
# Suppress specific diagnostics via environment variable
export MTLOG_SUPPRESS=MTLOG001,MTLOG009
mtlog-analyzer ./...

# Windows (PowerShell)
$env:MTLOG_SUPPRESS="MTLOG001,MTLOG009"
mtlog-analyzer ./...
```

#### 3. IDE Integration
All IDE integrations support suppression:
- **VS Code**: Configure in settings or use quick fix to suppress
- **GoLand**: Use Alt+Enter → Suppress for statement/function/file
- **Neovim**: Use `:MtlogSuppress [id]` command or code actions menu

#### Suppressible Diagnostics

| ID | Description | Example |
|----|-------------|---------|
| MTLOG001 | Template/argument mismatch | `log.Info("User {Id}", 1, 2)` |
| MTLOG002 | Invalid format specifier | `log.Info("{Value:Z}", 1)` |
| MTLOG003 | Duplicate property | `log.Info("{Id} {Id}", 1, 2)` |
| MTLOG004 | Property naming suggestion | `log.Info("{userId}", 1)` |
| MTLOG005 | Capturing hint | `log.Info("{User}", user)` |
| MTLOG006 | Error logging pattern | `log.Error("Failed")` |
| MTLOG007 | Context key suggestion | `log.ForContext("tenant", id)` |
| MTLOG008 | Dynamic template warning | `log.Info(fmt.Sprintf(...))` |
| MTLOG009 | With() odd arguments | `log.With("key1", "v1", "key2")` |
| MTLOG010 | With() non-string key | `log.With(123, "value")` |
| MTLOG011 | With() cross-call duplicate | Multiple With() calls with same key |
| MTLOG012 | With() reserved property | `log.With("Message", "custom")` |
| MTLOG013 | With() empty key | `log.With("", "value")` |

## IDE Integration

Both VS Code and GoLand extensions provide real-time validation with automatic analyzer detection and installation:

### VS Code

Install the [mtlog-analyzer extension](https://marketplace.visualstudio.com/items?itemName=mtlog.mtlog-analyzer) from the VS Code Marketplace:

1. Search for "mtlog-analyzer" in Extensions
2. Click Install
3. The extension will automatically find mtlog-analyzer or prompt to install it

Features:
- 🔍 Real-time diagnostics with squiggly underlines
- 🎯 Precise error locations - click to jump to issues
- 💡 Quick fixes for common issues:
  - Property naming (snake_case → PascalCase)
  - Template argument mismatches (add/remove arguments)
  - Missing error parameters (detect and add error variables)
  - String-to-constant extraction (create constants for repeated keys)
- 🚀 Auto-detection in standard Go locations (`$GOBIN`, `$GOPATH/bin`, `~/go/bin`)
- 📦 One-click installation if not found
- ⚙️ Configurable analyzer path and flags

### GoLand / IntelliJ IDEA

Install the [mtlog-analyzer plugin](https://plugins.jetbrains.com/plugin/24877-mtlog-analyzer) from JetBrains Marketplace:

1. Go to **Settings → Plugins → Marketplace**
2. Search for "mtlog-analyzer"
3. Click Install and restart
4. The plugin will automatically find mtlog-analyzer or show an install prompt

Features:
- 🔍 Real-time validation as you type
- 🎨 Intelligent highlighting by severity
- 💡 Quick fixes via Alt+Enter
- 🚀 Auto-detection in standard Go locations
- 📦 One-click installation notification
- 📊 Status bar widget for quick access
- 🔇 Diagnostic suppression support

### Manual IDE Setup

If you prefer not to use the extensions:

**VS Code** (via Go extension):
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--enable=govet"],
  "go.vetFlags": ["-vettool=$(which mtlog-analyzer)"]
}
```

**GoLand** (via Go vet settings):
1. Go to **Settings → Go → Linters → Go vet**
2. Add custom vet tool:
   - Click **+** to add a new tool
   - Set path to `mtlog-analyzer`
   - Add any desired flags (e.g., `-strict`)

### Neovim

Install the comprehensive [mtlog.nvim plugin](https://github.com/willibrandon/mtlog/tree/main/neovim-plugin):

```lua
-- Using lazy.nvim
{
  'willibrandon/mtlog',
  rtp = 'neovim-plugin',
  ft = 'go',
  config = function()
    require('mtlog').setup()
  end,
}
```

Features:
- 🔍 Real-time analysis on save with debouncing
- 🎯 LSP integration for code actions
- 💡 Quick fixes for all diagnostics
- 📊 Statusline diagnostic counts
- 🚫 Diagnostic suppression with persistence
- ⚡ Performance optimized with caching
- 🎮 Kill switch for instant enable/disable
- 📋 Context rules for selective analysis
- 🔭 Telescope extension for browsing

### Vim/Neovim (with vim-go)

For basic integration using vim-go, add to your `.vimrc` or `init.vim`:
```vim
let g:go_metalinter_enabled = ['vet', 'golint', 'errcheck']
let g:go_metalinter_command = 'golangci-lint'
let g:go_vet_command = ['go', 'vet', '-vettool=' . $GOPATH . '/bin/mtlog-analyzer']
```

Use `:GoFix` to apply suggested fixes.

### Applying Fixes via Command Line

If your IDE doesn't support automatic fixes, you can use `gofmt` with the analyzer's suggestions:

```bash
# Generate fixes
go vet -vettool=$(which mtlog-analyzer) -json ./... > fixes.json

# Apply fixes manually based on the JSON output
# (Each suggested fix includes exact text edits)
```

## Go Version Compatibility

The analyzer requires Go 1.23 or later due to its dependency on newer analysis framework features. The analyzer is built with Go 1.24.1 toolchain for optimal performance.

## Limitations

- Only analyzes static template strings (dynamic templates are reported as warnings)
- Requires type information for advanced checks (run with `go vet` for best results)
- Cannot analyze method calls through interfaces (only concrete logger types)
- Aliased types (e.g., `type MyLogger = mtlog.Logger`) may not be fully supported
- Context keys with multiple separators (e.g., `user_id.test-value`) are concatenated into a single PascalCase constant name (e.g., `ctxUserIdTestValue`)
- Templates with thousands of properties are supported but may impact performance (tested up to 10,000 properties in <1 second)

## Detailed Check Documentation

### MTLOG012 - Reserved Property Names

The analyzer can check for property names that might shadow mtlog's built-in properties. This check is **disabled by default** to avoid surprising users with new warnings.

Default reserved properties:
- `Timestamp` - The log event timestamp
- `Level` - The log level (INFO, WARN, etc.)
- `Message` - The rendered message
- `MessageTemplate` - The original message template
- `Exception` - Exception details if present
- `SourceContext` - Logger context/category

To enable and customize:
```bash
# Enable with default reserved list
mtlog-analyzer -check-reserved ./...

# Use custom reserved properties
mtlog-analyzer -check-reserved -reserved-props=Timestamp,Level,Message,RequestId ./...

# Add to your project's reserved list
mtlog-analyzer -check-reserved -reserved-props=Timestamp,Level,Message,MyCompanyProperty ./...
```

Note: mtlog uses `${...}` syntax for built-ins in output templates, so these don't cause actual conflicts, but the check helps maintain clarity in your logs.

## Troubleshooting

### "command not found" error
If you get a "command not found" error after installation:

1. Ensure Go bin directory is in your PATH:
   ```bash
   export PATH=$PATH:$(go env GOPATH)/bin
   ```

2. Verify installation:
   ```bash
   ls $(go env GOPATH)/bin/mtlog-analyzer
   ```

3. Use full path if needed:
   ```bash
   go vet -vettool=$(go env GOPATH)/bin/mtlog-analyzer ./...
   ```

### Missing diagnostics
If the analyzer isn't reporting expected issues:

1. Ensure you're using `go vet` (not just running the analyzer directly) for full type information
2. Check that the logger type is named `Logger` and has the expected methods
3. Verify the file is being analyzed by adding an obvious error temporarily

### Too many false positives
If you're seeing diagnostics for non-mtlog loggers:

1. The analyzer checks for types named `Logger` with logging methods
2. Consider renaming non-mtlog logger types to avoid conflicts
3. Use `-disable` flag to turn off specific checks temporarily