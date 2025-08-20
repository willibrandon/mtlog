#!/bin/bash
# Automatically update Zed extension after mtlog release

set -e

VERSION=$1
COMMIT=$2
EXTENSIONS_DIR="$HOME/zed-extensions-fork"

if [ -z "$VERSION" ] || [ -z "$COMMIT" ]; then
    echo "Usage: $0 <version> <commit>"
    echo "Example: $0 v0.9.1 abc123def"
    exit 1
fi

echo "Updating Zed extension to version $VERSION (commit: $COMMIT)"

# Clone or update fork
if [ -d "$EXTENSIONS_DIR" ]; then
    echo "Updating existing fork..."
    cd "$EXTENSIONS_DIR"
    git checkout main
    git pull upstream main
else
    echo "Cloning extensions repository..."
    git clone https://github.com/${GITHUB_ACTOR:-$USER}/extensions.git "$EXTENSIONS_DIR" || {
        echo "Fork not found. Creating fork first..."
        gh repo fork zed-industries/extensions --clone=false
        git clone https://github.com/${GITHUB_ACTOR:-$USER}/extensions.git "$EXTENSIONS_DIR"
    }
    cd "$EXTENSIONS_DIR"
    git remote add upstream https://github.com/zed-industries/extensions || true
fi

# Ensure we're up to date with upstream
git fetch upstream
git checkout main
git reset --hard upstream/main

# Update submodule to point to new commit
if [ -d "extensions/mtlog-analyzer" ]; then
    echo "Updating existing mtlog-analyzer submodule..."
    cd extensions/mtlog-analyzer
    git fetch
    git checkout "$COMMIT"
    cd ../..
else
    echo "Adding mtlog-analyzer as submodule..."
    git submodule add https://github.com/willibrandon/mtlog.git extensions/mtlog-analyzer
    cd extensions/mtlog-analyzer
    git checkout "$COMMIT"
    cd ../..
fi

# Update extensions.toml with new version
if grep -q "mtlog-analyzer" extensions.toml; then
    echo "Updating version in extensions.toml..."
    # Update existing entry
    sed -i.bak "/\[mtlog-analyzer\]/,/^\[/ s/version = \".*\"/version = \"${VERSION#v}\"/" extensions.toml
else
    echo "Adding mtlog-analyzer to extensions.toml..."
    # Add new entry
    cat >> extensions.toml << EOF

[mtlog-analyzer]
path = "extensions/mtlog-analyzer/zed-extension/mtlog"
version = "${VERSION#v}"
EOF
fi

# Clean up backup file
rm -f extensions.toml.bak

# Create branch and commit
BRANCH="update-mtlog-analyzer-${VERSION#v}"
git checkout -b "$BRANCH"
git add .
git commit -m "Update mtlog-analyzer to $VERSION

- Updates mtlog-analyzer Zed extension to $VERSION
- Commit: $COMMIT
- Release: https://github.com/willibrandon/mtlog/releases/tag/$VERSION"

# Push branch
git push origin "$BRANCH"

# Create PR using GitHub CLI
echo "Creating pull request..."
gh pr create \
  --repo zed-industries/extensions \
  --title "Update mtlog-analyzer to $VERSION" \
  --body "Updates the mtlog-analyzer extension to version $VERSION.

## Changes
- Extension version: $VERSION
- Source commit: willibrandon/mtlog@$COMMIT
- Release notes: https://github.com/willibrandon/mtlog/releases/tag/$VERSION

## Testing
- [ ] Extension builds successfully
- [ ] Diagnostics appear correctly in Zed
- [ ] Quick fixes work as expected

cc @willibrandon" \
  --head "${GITHUB_ACTOR:-$USER}:$BRANCH" \
  --base main

echo "✓ Pull request created successfully!"