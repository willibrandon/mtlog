# mtlog-analyzer Release Process

## Overview

The mtlog-analyzer is released as a standalone CLI tool that can be installed via:
1. `go install` 
2. Pre-built binaries from GitHub releases
3. As a go vet tool

## Release Strategy

### Versioning
- The analyzer uses semantic versioning: `vMAJOR.MINOR.PATCH`
- Tags are prefixed with `analyzer/` to distinguish from main mtlog releases
- Example: `analyzer/v1.0.0`

### Release Process

1. **Update Version**
   - Update CHANGELOG.md with release notes
   - Ensure all tests pass
   - Update documentation if needed

2. **Create Tag**
   ```bash
   git tag -a analyzer/v1.0.0 -m "Release mtlog-analyzer v1.0.0"
   git push origin analyzer/v1.0.0
   ```

3. **Automated Release**
   - GitHub Actions workflow triggers on `analyzer/v*` tags
   - GoReleaser builds binaries for multiple platforms
   - Creates GitHub release with:
     - Pre-built binaries for Linux, macOS, Windows
     - Checksums file
     - Installation instructions

### Supported Platforms

- **Operating Systems**: Linux, macOS, Windows
- **Architectures**: amd64, arm64, 386
- **Go Versions**: 1.23+

### Binary Naming Convention
```
mtlog-analyzer_<version>_<OS>_<arch>
```

Examples:
- `mtlog-analyzer_v1.0.0_Linux_x86_64.tar.gz`
- `mtlog-analyzer_v1.0.0_Darwin_arm64.tar.gz`
- `mtlog-analyzer_v1.0.0_Windows_x86_64.zip`

## Integration with Main Project

The analyzer is:
- Part of the main mtlog repository
- Has its own go.mod for independent versioning
- Tested in main CI pipeline
- Released independently from main mtlog library

## Future Considerations

1. **Package Distribution**
   - Consider publishing to package managers (brew, apt, chocolatey)
   - Docker image for CI/CD pipelines

2. **Plugin Architecture**
   - Allow custom checks via plugins
   - Configuration file support

3. **IDE Extensions**
   - VS Code extension
   - GoLand plugin