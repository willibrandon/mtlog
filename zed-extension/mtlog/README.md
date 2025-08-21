# mtlog-analyzer for Zed

Static analysis for [mtlog](https://github.com/willibrandon/mtlog) message templates in Go, integrated into the Zed editor.

## Features

- **Real-time diagnostics** for mtlog usage issues
- **All MTLOG001-MTLOG013 diagnostics** including:
  - Template/argument mismatches
  - Format specifier validation
  - Property naming conventions
  - Duplicate property detection
  - Error logging patterns
- **Quick fixes** via code actions
- **Automatic binary detection** in standard Go paths
- **Configurable analyzer flags**

## Installation

### Prerequisites

1. Install mtlog-lsp (includes bundled analyzer):
```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-lsp@latest
```

2. Install the extension in Zed:
   - Open Zed's extension manager
   - Search for "mtlog-analyzer"
   - Click Install

## Configuration

The extension automatically detects mtlog-lsp in these locations:
- `$GOBIN`
- `$GOPATH/bin`
- `$HOME/go/bin`
- `/usr/local/bin`
- System PATH

### Custom Configuration

You can customize the analyzer in your Zed settings:

```json
{
  "lsp": {
    "mtlog-analyzer": {
      "binary": {
        "path": "/custom/path/to/mtlog-lsp",
        "arguments": ["-strict", "-common-keys=tenant_id,org_id"]
      }
    }
  }
}
```

### Available Analyzer Flags

- `-strict` - Enable strict format specifier validation
- `-common-keys` - Additional context keys to suggest as constants
- `-disable` - Disable specific checks (template, naming, etc.)
- `-ignore-dynamic-templates` - Suppress warnings for non-literal templates
- `-strict-logger-types` - Only analyze exact mtlog types
- `-downgrade-errors` - Downgrade errors to warnings for CI migration

## Usage

The extension runs automatically on Go files. Diagnostics appear inline and in Zed's diagnostics panel.

### Example Diagnostics

```go
// MTLOG001: Template has 2 placeholders but 1 argument provided
log.Information("User {UserId} performed {Action}", userId)

// MTLOG002: Property 'user_id' should be PascalCase: 'UserId'
log.Information("User logged in", "user_id", 123)

// MTLOG010: error should be logged using Error level
log.Information("Failed to connect", "error", err)
```

### Quick Fixes

The extension provides quick fixes for common issues:
- Convert property names to PascalCase
- Add missing template arguments
- Remove extra template arguments
- Fix format specifiers

## Troubleshooting

### LSP Server Not Found

If the extension can't find mtlog-lsp:

1. Verify installation:
```bash
which mtlog-lsp
```

2. Ensure Go bin directory is in PATH:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

3. Specify explicit path in Zed settings (see Configuration above)

### No Diagnostics Appearing

1. Check that the file is recognized as Go (`.go` extension)
2. Verify the analyzer is running:
   - Open Zed's LSP logs (View → Toggle LSP Log)
   - Look for mtlog-analyzer entries
3. Test the analyzer directly:
```bash
mtlog-analyzer -json ./your-file.go
```

## Development

This extension is part of the [mtlog](https://github.com/willibrandon/mtlog) project.

### Building from Source

1. Clone the repository:
```bash
git clone https://github.com/willibrandon/mtlog.git
cd mtlog/zed-extension/mtlog
```

2. Build the extension:
```bash
cargo build --release
```

3. The compiled extension will be at `target/wasm32-wasip2/release/mtlog_analyzer.wasm`

## License

MIT License - see [LICENSE](https://github.com/willibrandon/mtlog/blob/main/LICENSE) for details.

## Support

- Report issues: [GitHub Issues](https://github.com/willibrandon/mtlog/issues)
- Documentation: [mtlog docs](https://github.com/willibrandon/mtlog#readme)
- Analyzer docs: [mtlog-analyzer](https://github.com/willibrandon/mtlog/tree/main/cmd/mtlog-analyzer)