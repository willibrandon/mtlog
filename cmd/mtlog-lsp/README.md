# mtlog-lsp

Language Server Protocol implementation for mtlog-analyzer, providing real-time diagnostics and code actions for mtlog usage in Go code.

## Overview

mtlog-lsp is a Language Server Protocol (LSP) server that bundles mtlog-analyzer directly, providing zero-subprocess overhead for editor integrations. It implements the JSON-RPC 2.0 protocol and is specifically designed for editors like Zed that require persistent LSP servers.

## Installation

```bash
go install github.com/willibrandon/mtlog/cmd/mtlog-lsp@latest
```

## Features

- **Bundled analyzer** - No subprocess spawning, analyzer runs in-process
- **Real-time diagnostics** - All MTLOG001-MTLOG013 issues detected as you type
- **Code actions** - Quick fixes for common issues
- **Diagnostic suppression** - Suppress specific diagnostic codes via workspace configuration
- **Performance optimized** - 5-minute package cache, concurrent-safe, diagnostic batching
- **UTF-16 support** - Proper position conversion for LSP compatibility

## Usage

### With Zed Editor

mtlog-lsp is primarily designed for the [Zed extension](../../zed-extension/mtlog/README.md). Once installed, the extension automatically launches mtlog-lsp when you open Go files.

### Standalone

You can also run mtlog-lsp standalone for debugging:

```bash
# Start the LSP server (communicates via stdin/stdout)
mtlog-lsp
```

The server expects LSP messages in JSON-RPC format on stdin and sends responses on stdout.

## Configuration

Configuration is passed via LSP initialization options. Example `.zed/settings.json`:

```json
{
  "lsp": {
    "mtlog-analyzer": {
      "initialization_options": {
        "suppressedCodes": ["MTLOG003", "MTLOG009"],
        "severityOverrides": {
          "MTLOG002": "warning"
        },
        "disableAll": false,
        "commonKeys": ["tenant_id", "org_id"],
        "strictMode": false,
        "ignoreDynamicTemplates": false
      }
    }
  }
}
```

### Configuration Options

- `suppressedCodes` - Array of diagnostic codes to suppress (e.g., `["MTLOG001", "MTLOG003"]`)
- `severityOverrides` - Map of diagnostic codes to severity levels (`error`, `warning`, `information`, `hint`)
- `disableAll` - Disable all diagnostics (useful for temporarily turning off)
- `commonKeys` - Additional context keys to suggest as constants
- `strictMode` - Enable strict format specifier validation
- `ignoreDynamicTemplates` - Suppress warnings for non-literal templates

## Performance

mtlog-lsp includes several performance optimizations:

- **Package caching** - 5-minute TTL cache for loaded packages
- **Concurrent safety** - Mutex-protected cache access
- **Diagnostic batching** - Limits to 100 diagnostics per file
- **Direct integration** - No subprocess overhead

## Architecture

```
┌─────────────┐     JSON-RPC 2.0      ┌──────────────┐
│             │◄──────────────────────►│              │
│  Zed Editor │                        │  mtlog-lsp   │
│             │                        │              │
└─────────────┘                        └──────┬───────┘
                                              │
                                              ▼
                                       ┌──────────────┐
                                       │              │
                                       │  Bundled     │
                                       │  Analyzer    │
                                       │              │
                                       └──────────────┘
```

## Diagnostic Codes

mtlog-lsp reports the following diagnostic codes:

- `MTLOG001` - Template/argument count mismatch
- `MTLOG002` - Property name should be PascalCase
- `MTLOG003` - Duplicate property in template
- `MTLOG004` - Invalid property name
- `MTLOG005` - Complex type needs capturing hint
- `MTLOG006` - Invalid format specifier
- `MTLOG007` - Error should use Error level
- `MTLOG008` - Context key should be constant
- `MTLOG009` - With() requires even arguments
- `MTLOG010` - With() key must be string
- `MTLOG011` - Property override warning
- `MTLOG012` - Empty key in With()
- `MTLOG013` - Reserved property shadowing

## Development

### Building

```bash
cd cmd/mtlog-lsp
go build
```

### Testing

```bash
go test -v ./...
```

### Running with Debug Logging

Debug output goes to stderr while LSP communication uses stdout:

```bash
mtlog-lsp 2>debug.log
```

## Troubleshooting

### No Diagnostics Appearing

1. Check that mtlog-lsp is running (check process list)
2. Verify the file is recognized as Go (`.go` extension)
3. Check debug output in stderr for errors

### Binary Not Found

Ensure mtlog-lsp is in your PATH or configure the explicit path in your editor settings.

### Performance Issues

If analysis is slow:
- The package cache may be expired (5-minute TTL)
- Large packages may take time on first analysis
- Check if you're hitting the 100 diagnostic limit per file

## License

MIT License - see [LICENSE](../../LICENSE) for details.