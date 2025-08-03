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
- mtlog-analyzer installed:
  ```bash
  go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
  ```

## Installation

1. Install from JetBrains Marketplace:
   - Open GoLand
   - Go to Settings → Plugins → Marketplace
   - Search for "mtlog-analyzer"
   - Click Install

2. Or download the `.zip` file from releases and install manually

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