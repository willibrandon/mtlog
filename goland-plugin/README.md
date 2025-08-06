# mtlog-analyzer GoLand Plugin

Real-time validation for mtlog message templates in GoLand and other JetBrains IDEs with Go support.

## Features

- 🔍 Real-time template validation as you type
- 🎯 Three severity levels: errors, warnings, and suggestions
- 🎨 Intelligent highlighting:
  - Template/argument errors highlight the template string and arguments
  - Property naming warnings highlight only the property name
- 🔧 Quick fixes for common issues (PascalCase conversion, argument count)
- ⚙️ Configurable analyzer path and flags
- 🚀 Performance optimized with caching and debouncing
- 🖥️ Full support for Windows, macOS, and Linux

## Requirements

- GoLand 2024.2 or later (or IntelliJ IDEA Ultimate with Go plugin)
- mtlog-analyzer (automatically detected or can be installed via notification)

## Installation

1. Install from JetBrains Marketplace:
   - Open GoLand
   - Go to Settings → Plugins → Marketplace
   - Search for "mtlog-analyzer"
   - Click Install

2. The plugin will automatically find mtlog-analyzer if it's installed. If not found, you'll see a notification with an "Install" button for one-click installation.

### Automatic Detection

The plugin searches for mtlog-analyzer in the following locations (in order):
1. Configured path in settings
2. System PATH
3. Go binary locations:
   - `$GOBIN`
   - `$GOPATH/bin`
   - `~/go/bin` (default Go install location)
   - Platform-specific locations (e.g., `%LOCALAPPDATA%\Microsoft\WindowsApps` on Windows)

## Configuration

Go to Settings → Tools → mtlog-analyzer to configure:

- **Analyzer Path**: Path to mtlog-analyzer executable (auto-detected by default)
- **Additional Flags**: Extra command-line flags for the analyzer
- **Severity Levels**: Customize how errors, warnings, and suggestions are displayed

## Usage

The plugin automatically analyzes Go files as you type, showing diagnostics inline:

- **Red underline**: Template/argument mismatch errors
- **Yellow underline**: Warnings (e.g., using @ with basic types)
- **Gray underline**: Suggestions (e.g., property naming conventions)

Use Alt+Enter on any diagnostic to see available quick fixes.

## Suppression

You can suppress diagnostics using:
- `//noinspection MTLog` - Suppress on the next line
- `// MTLOG-IGNORE` - Inline suppression

## Troubleshooting

### "mtlog-analyzer not found"

If the plugin can't find mtlog-analyzer:

1. **Automatic Installation**: Click "Install" in the notification that appears
2. **Manual Installation**: Run `go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`
3. **Custom Location**: Go to Settings → Tools → mtlog-analyzer and specify the full path
4. **PATH Issues**: Ensure your Go bin directory is in your system PATH

### No diagnostics appearing

- Check the Event Log for analyzer errors
- Verify mtlog-analyzer works from terminal: `mtlog-analyzer your-file.go`
- Check Settings → Tools → mtlog-analyzer to ensure the plugin is enabled
- Try restarting the analyzer from the status bar widget

## Development

### Building

```bash
./gradlew buildPlugin
```

### Running

```bash
./gradlew runIde
```

### Testing

```bash
./gradlew test
```

## License

[MIT](https://github.com/willibrandon/mtlog/blob/main/LICENSE) - Same as the main mtlog project.