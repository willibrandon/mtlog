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

### Advanced Features
- **Receiver type checking** - Reduces false positives by verifying logger types
- **Template caching** - Improves performance by caching parsed templates
- **Severity levels** - Differentiates between errors, warnings, and suggestions
- **Suggested fixes** - Provides automatic fixes for common issues
- **Error type verification** - Validates that error arguments are actually error types

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

// ✅ Correct usage
log.Information("User {@User} has {Count} items", user, count)
```

## Severity Levels

The analyzer reports issues at three severity levels:

1. **Error** - Critical issues that will cause runtime problems
   - Template/argument count mismatches
   - Invalid property names (spaces, starting with numbers)
   - Duplicate properties

2. **Warning** - Issues that may indicate mistakes
   - Using `@` prefix with basic types
   - Dynamic template strings

3. **Suggestion** - Best practice recommendations
   - PascalCase property naming
   - Missing `@` prefix for complex types
   - Error logging without error values
   - Common context keys without constants

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

Available check names for `-disable`:
- `template` - Template/argument mismatch detection
- `duplicate` - Duplicate property detection
- `naming` - Property naming checks (including PascalCase suggestions)
- `capturing` - Capturing hint suggestions
- `error` - Error logging pattern checks
- `context` - Context key suggestions

### Ignoring Specific Warnings

You can ignore specific warnings using standard Go vet comments:

```go
//lint:ignore mtlog reason
log.Information("User {userId} logged in", id) // lowercase property name
```

## IDE Integration

The analyzer provides suggested fixes that can be automatically applied in IDEs. Here's how to set it up:

### VS Code

1. **Using Go extension** (recommended):
   - Install the official Go extension
   - Add to your workspace settings (`.vscode/settings.json`):
     ```json
     {
       "go.lintTool": "golangci-lint",
       "go.lintFlags": [
         "--enable=govet"
       ],
       "go.vetFlags": [
         "-vettool=$(which mtlog-analyzer)"
       ]
     }
     ```
   - Use `Ctrl+.` (or `Cmd+.` on Mac) on diagnostics to apply suggested fixes

2. **Using gopls directly**:
   - Configure gopls to use the analyzer:
     ```json
     {
       "gopls": {
         "analyses": {
           "mtlog": true
         }
       }
     }
     ```

### GoLand / IntelliJ IDEA

1. Go to **Settings → Go → Linters → Go vet**
2. Add custom vet tool:
   - Click **+** to add a new tool
   - Set path to `mtlog-analyzer`
   - Add any desired flags (e.g., `-strict`)
3. Use **Alt+Enter** on highlighted issues to apply suggested fixes

### Vim/Neovim (with vim-go)

Add to your `.vimrc` or `init.vim`:
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

The analyzer requires Go 1.23.0 or later due to its dependency on newer analysis framework features. The analyzer is built with Go 1.24.1 toolchain for optimal performance.

## Limitations

- Only analyzes static template strings (dynamic templates are reported as warnings)
- Requires type information for advanced checks (run with `go vet` for best results)
- Cannot analyze method calls through interfaces (only concrete logger types)
- Aliased types (e.g., `type MyLogger = mtlog.Logger`) may not be fully supported
- Context keys with multiple separators (e.g., `user_id.test-value`) are concatenated into a single PascalCase constant name (e.g., `ctxUserIdTestValue`)
- Templates with thousands of properties are supported but may impact performance (tested up to 10,000 properties in <1 second)

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