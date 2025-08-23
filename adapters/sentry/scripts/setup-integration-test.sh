#!/bin/bash
set -e

SENTRY_VERSION="24.1.0"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../../.." && pwd )"
DOCKER_DIR="$PROJECT_ROOT/docker"
TEMP_DIR="/tmp/sentry-setup-$$"

echo "Setting up Sentry ${SENTRY_VERSION} for integration testing..."

# Create temp directory
mkdir -p "$TEMP_DIR"
cd "$TEMP_DIR"

# Download Sentry self-hosted
curl -L https://github.com/getsentry/self-hosted/archive/refs/tags/${SENTRY_VERSION}.tar.gz | tar xz
cd self-hosted-${SENTRY_VERSION}

# Generate configs
./install.sh --skip-user-prompt --skip-commit-check --no-report-self-hosted-issues

# Create sentry config directory in docker
mkdir -p "$DOCKER_DIR/sentry-config"

# Copy configs
cp -r sentry relay "$DOCKER_DIR/sentry-config/"
cp .env "$DOCKER_DIR/sentry-config/.env"

# Merge services into docker-compose.test.yml
# This requires yq to be installed
yq eval-all 'select(fileIndex == 0) * {"services": select(fileIndex == 1).services}' \
    "$DOCKER_DIR/docker-compose.test.yml" \
    docker-compose.yml > "$DOCKER_DIR/docker-compose.test.yml.new"

mv "$DOCKER_DIR/docker-compose.test.yml.new" "$DOCKER_DIR/docker-compose.test.yml"

# Cleanup
cd "$PROJECT_ROOT"
rm -rf "$TEMP_DIR"

echo "Sentry integration test setup complete"
echo "Run: cd $DOCKER_DIR && docker-compose -f docker-compose.test.yml up sentry-web"