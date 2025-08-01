# mtlog analyzer for Visual Studio Code

Real-time validation for [mtlog](https://github.com/willibrandon/mtlog) message templates in Go code.

## Features

- 🔍 **Real-time diagnostics** - See template errors as you type
- 🎯 **Precise error locations** - Jump directly to problematic code
- 📊 **Severity levels** - Errors, warnings, and suggestions
- ⚙️ **Configurable** - Customize analyzer path and flags

## What it catches

```go
// ❌ Error: Template has 2 properties but 1 argument
logger.Information("User {UserId} logged in from {IP}", userId)

// ⚠️ Warning: Using @ prefix with basic type
logger.Debug("Count is {@Count}", 42)

// 💡 Suggestion: Property name should be PascalCase
logger.Information("User {userId} completed action", "user123")
```

## Requirements

- Go 1.21 or later
- [mtlog-analyzer](https://github.com/willibrandon/mtlog/cmd/mtlog-analyzer) installed and in PATH

## Installation

1. Install mtlog-analyzer:
   ```bash
   go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
   ```

2. Install this extension from the VS Code Marketplace

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
- Ensure mtlog-analyzer is installed: `go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`
- Add Go bin directory to PATH: `export PATH=$PATH:$(go env GOPATH)/bin`
- Or specify full path in settings: `"mtlog.analyzerPath": "/full/path/to/mtlog-analyzer"`

### No diagnostics appearing
- Check Output panel (View → Output → mtlog-analyzer) for errors
- Verify the analyzer works: `mtlog-analyzer your-file.go`
- Ensure you're saving the file (analysis runs on save)

## Development & Releases

This extension is built and released through the main mtlog repository's CI/CD pipeline.

### Release Process

The extension follows a dual-tagging strategy:

- **Library releases** (`v1.0.0`): Creates GitHub release with Go binaries and VSIX file
- **Extension releases** (`ext/v0.1.0`): Same as above but also publishes to VS Code Marketplace

To release the extension to the marketplace, create a tag with the `ext/v` prefix:

```bash
git tag ext/v0.1.0
git push origin ext/v0.1.0
```

The CI workflow automatically builds, tests, and packages the extension on every push. Extension-specific releases are published to the VS Code Marketplace only when using `ext/v*` tags.

## License

MIT