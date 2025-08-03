# Release Process

This document describes the release process for mtlog and its associated tools.

## Components Released

Each release includes:
- **mtlog library** - The core Go library
- **mtlog-analyzer** - Static analysis tool (binaries for Linux, macOS, Windows)
- **VS Code extension** - Real-time validation in Visual Studio Code
- **GoLand plugin** - Real-time validation in JetBrains IDEs

All components share the same version number for consistency.

## Release Steps

### 1. Prepare the Release

```bash
# Update version numbers
# - vscode-extension/mtlog-analyzer/package.json
# - goland-plugin/build.gradle.kts

# Update CHANGELOG.md
# Move items from Unreleased to new version section

# Update SECURITY.md
# Add new version to supported versions

# Commit changes
git add -A
git commit -m "chore: prepare v0.7.0 release"
git push origin main
```

### 2. Create and Push Tag

```bash
git tag v0.7.0
git push origin v0.7.0
```

### 3. Automated Release Process

The GitHub Actions workflow will automatically:
1. Build mtlog-analyzer binaries for all platforms
2. Package the VS Code extension (.vsix)
3. Build the GoLand plugin (.zip)
4. Create GitHub release with all artifacts
5. Publish VS Code extension to marketplace
6. Publish GoLand plugin to JetBrains marketplace

### 4. Manual Steps (if needed)

#### First-time GoLand Plugin Release
The GoLand plugin must be uploaded manually to JetBrains Marketplace once before automated publishing works:
1. Download the `.zip` from the GitHub release
2. Go to https://plugins.jetbrains.com/
3. Create vendor account if needed
4. Upload plugin and fill in required fields
5. Submit for review

Future releases will publish automatically.

## Version Numbering

We follow semantic versioning (MAJOR.MINOR.PATCH):
- **MAJOR**: Breaking API changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes only

## Troubleshooting

### Release workflow fails
- Check GitHub Actions logs for specific errors
- Both marketplace publish steps have `continue-on-error: true` to prevent total failure
- Artifacts will still be uploaded to GitHub release even if publishing fails

### To re-run a release
```bash
# Delete the tag locally and remotely
git tag -d v0.7.0
git push --delete origin v0.7.0

# Recreate and push
git tag v0.7.0
git push origin v0.7.0
```

## CI/CD Configuration

The release process is defined in `.github/workflows/release.yml` and triggers on tags matching `v*`.