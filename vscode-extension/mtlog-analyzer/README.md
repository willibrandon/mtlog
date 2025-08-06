# mtlog-analyzer for Visual Studio Code

Real-time validation for [mtlog](https://github.com/willibrandon/mtlog) message templates in Go code.

## Features

- 🔍 **Real-time diagnostics** - See template errors as you type
- 🎯 **Precise error locations** - Jump directly to problematic code
- 📊 **Severity levels** - Errors, warnings, and suggestions
- ⚡ **Quick fixes** - Automatic fixes for common issues
- ⚙️ **Configurable** - Customize analyzer path and flags

## What it catches

```go
// ❌ Error: Template has 2 properties but 1 argument
logger.Information("User {UserId} logged in from {IP}", userId)
//                                                      ^^^^^^ Quick fix: Add 1 missing argument

// ⚠️ Warning: Using @ prefix with basic type
logger.Debug("Count is {@Count}", 42)

// 💡 Suggestion: Property name should be PascalCase
logger.Information("User {userId} completed action", "user123")
//                        ^^^^^^ Quick fix: Change to 'UserId'
```

### Quick Fixes

The extension provides automatic fixes for common issues:

- **PascalCase property names** - Converts `userId` to `UserId`, `user_id` to `UserId`, etc.
- **Argument mismatches** - Adds placeholder arguments for missing properties or removes excess arguments

Apply fixes by clicking the light bulb (💡) or pressing `Ctrl+.` (Windows/Linux) or `Cmd+.` (macOS) when your cursor is on the diagnostic.

## Requirements

- Go 1.21 or later
- [mtlog-analyzer](https://github.com/willibrandon/mtlog/tree/main/cmd/mtlog-analyzer) (automatically detected or can be installed via prompt)

## Installation

1. Install this extension from the VS Code Marketplace

2. The extension will automatically find mtlog-analyzer if it's installed. If not found, you'll be prompted to install it with one click.

### Automatic Detection

The extension searches for mtlog-analyzer in the following locations (in order):
1. Configured path in settings
2. System PATH
3. Go binary locations:
   - `$GOBIN`
   - `$GOPATH/bin`
   - `~/go/bin` (default Go install location)
   - Platform-specific locations (e.g., `/usr/local/go/bin` on macOS)

## Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| `mtlog.analyzerPath` | Path to mtlog-analyzer executable | `mtlog-analyzer` |
| `mtlog.analyzerFlags` | Additional analyzer flags | `[]` |

Example configuration:
```json
{
  "mtlog.analyzerPath": "C:\\Go\\bin\\mtlog-analyzer.exe",
  "mtlog.analyzerFlags": ["-strict"]
}
```

## How it works

The extension runs `mtlog-analyzer` when you save Go files, parsing the output and displaying diagnostics directly in the editor. It's designed to work alongside gopls without interference.

## Troubleshooting

### "mtlog-analyzer not found"

If the extension can't find mtlog-analyzer:

1. **Automatic Installation**: Click "Install" when prompted by the extension
2. **Manual Installation**: Run `go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`
3. **PATH Issues**: 
   - On macOS: If VS Code is launched from Dock/Spotlight, it may not inherit your shell PATH
   - Solution: Launch VS Code from terminal with `code .` or restart VS Code after installation
4. **Custom Location**: Specify the full path in settings:
   ```json
   {
     "mtlog.analyzerPath": "/full/path/to/mtlog-analyzer"
   }
   ```

### No diagnostics appearing
- Check Output panel (View → Output → mtlog-analyzer) for errors
- Verify the analyzer works: `mtlog-analyzer your-file.go`
- Ensure you're saving the file (analysis runs on save)

## License

[MIT](https://github.com/willibrandon/mtlog/blob/main/LICENSE)