#!/bin/bash
# Setup script for mtlog-analyzer Neovim plugin test environment
# This script ensures all requirements are met for running tests without mocks

set -e

echo "==================================================================="
echo "mtlog-analyzer Neovim Plugin Test Environment Setup"
echo "==================================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_info() {
    echo "[INFO] $1"
}

# Check if running from correct directory
if [ ! -f "tests/minimal_init.lua" ]; then
    print_error "Please run this script from the neovim-plugin directory"
    exit 1
fi

echo "Step 1: Checking Go installation..."
echo "-----------------------------------"
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
print_success "Go is installed: $GO_VERSION"

# Check Go version (require 1.21+)
GO_VERSION_NUM=$(echo $GO_VERSION | sed 's/go//' | cut -d. -f1-2 | tr -d .)
if [ "$GO_VERSION_NUM" -lt "121" ]; then
    print_warning "Go version 1.21+ is recommended (you have $GO_VERSION)"
fi

echo ""
echo "Step 2: Checking mtlog-analyzer installation..."
echo "------------------------------------------------"

# Check if mtlog-analyzer is installed
ANALYZER_PATH=""
if [ -n "$MTLOG_ANALYZER_PATH" ]; then
    if [ -x "$MTLOG_ANALYZER_PATH" ]; then
        ANALYZER_PATH="$MTLOG_ANALYZER_PATH"
        print_success "Using mtlog-analyzer from MTLOG_ANALYZER_PATH: $ANALYZER_PATH"
    else
        print_error "MTLOG_ANALYZER_PATH is set but file is not executable: $MTLOG_ANALYZER_PATH"
        exit 1
    fi
elif command -v mtlog-analyzer &> /dev/null; then
    ANALYZER_PATH=$(which mtlog-analyzer)
    print_success "Found mtlog-analyzer in PATH: $ANALYZER_PATH"
else
    print_warning "mtlog-analyzer not found in PATH"
    echo "Installing mtlog-analyzer..."
    
    # Try to install from the parent directory
    if [ -d "../cmd/mtlog-analyzer" ]; then
        print_info "Building mtlog-analyzer from source..."
        (cd ../cmd/mtlog-analyzer && go install)
        
        if command -v mtlog-analyzer &> /dev/null; then
            ANALYZER_PATH=$(which mtlog-analyzer)
            print_success "Successfully installed mtlog-analyzer: $ANALYZER_PATH"
        else
            print_error "Failed to install mtlog-analyzer"
            echo "Please install manually: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest"
            exit 1
        fi
    else
        print_info "Installing mtlog-analyzer from GitHub..."
        go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
        
        if command -v mtlog-analyzer &> /dev/null; then
            ANALYZER_PATH=$(which mtlog-analyzer)
            print_success "Successfully installed mtlog-analyzer: $ANALYZER_PATH"
        else
            print_error "Failed to install mtlog-analyzer"
            exit 1
        fi
    fi
fi

# Verify analyzer works (test with a simple help command)
if ! $ANALYZER_PATH -json /dev/null &> /dev/null; then
    # Fallback: just check if binary executes
    if ! $ANALYZER_PATH 2>&1 | grep -q "mtlog"; then
        print_error "mtlog-analyzer exists but doesn't run correctly"
        exit 1
    fi
fi

# Try to get version info
ANALYZER_VERSION="mtlog-analyzer (installed)"
if $ANALYZER_PATH -V=full 2>&1 | grep -q "mtlog"; then
    ANALYZER_VERSION=$($ANALYZER_PATH -V=full 2>&1 | head -n1)
fi
print_success "mtlog-analyzer is working: $ANALYZER_VERSION"

echo ""
echo "Step 3: Setting up test Go project..."
echo "--------------------------------------"

TEST_PROJECT_DIR="${MTLOG_TEST_PROJECT_DIR:-/tmp/mtlog-test-project-$$}"
export MTLOG_TEST_PROJECT_DIR="$TEST_PROJECT_DIR"

if [ -d "$TEST_PROJECT_DIR" ]; then
    print_info "Cleaning existing test project directory..."
    rm -rf "$TEST_PROJECT_DIR"
fi

print_info "Creating test project at: $TEST_PROJECT_DIR"
mkdir -p "$TEST_PROJECT_DIR"

# Create go.mod
cat > "$TEST_PROJECT_DIR/go.mod" << 'EOF'
module github.com/test/mtlog-test

go 1.21

require github.com/willibrandon/mtlog v0.8.1
EOF

# Create a simple main.go to verify the project
cat > "$TEST_PROJECT_DIR/main.go" << 'EOF'
package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Information("Test project ready")
}
EOF

# Download dependencies
print_info "Downloading Go dependencies..."
# First, ensure we get the mtlog module
(cd "$TEST_PROJECT_DIR" && go get github.com/willibrandon/mtlog@v0.8.1)

# Then run go mod tidy to clean up
print_info "Running go mod tidy..."
(cd "$TEST_PROJECT_DIR" && go mod tidy)

# Verify dependencies are downloaded
if [ ! -f "$TEST_PROJECT_DIR/go.sum" ]; then
    print_warning "go.sum not created, trying to download dependencies again..."
    (cd "$TEST_PROJECT_DIR" && go mod download github.com/willibrandon/mtlog)
fi

# Verify the project builds
print_info "Verifying test project builds..."
if (cd "$TEST_PROJECT_DIR" && go build -o test-binary main.go); then
    print_success "Test project builds successfully"
    rm -f "$TEST_PROJECT_DIR/test-binary"
else
    print_error "Test project does not build"
    exit 1
fi

echo ""
echo "Step 4: Checking Neovim and test dependencies..."
echo "-------------------------------------------------"

if ! command -v nvim &> /dev/null; then
    print_error "Neovim is not installed or not in PATH"
    echo "Please install Neovim from https://neovim.io/"
    exit 1
fi

NVIM_VERSION=$(nvim --version | head -n1)
print_success "Neovim is installed: $NVIM_VERSION"

# Check for Plenary (will be auto-installed by minimal_init.lua if missing)
PLENARY_PATH="$HOME/.local/share/nvim/site/pack/test/start/plenary.nvim"
if [ -d "$PLENARY_PATH" ]; then
    print_success "Plenary.nvim is already installed"
else
    print_info "Plenary.nvim will be installed automatically when tests run"
fi

echo ""
echo "Step 5: Creating test runner script..."
echo "---------------------------------------"

# Create a test runner script
cat > "run_tests.sh" << 'EOF'
#!/bin/bash
# Run all tests for mtlog-analyzer Neovim plugin

# Ensure analyzer is available
export MTLOG_ANALYZER_PATH="${MTLOG_ANALYZER_PATH:-$(which mtlog-analyzer)}"
if [ -z "$MTLOG_ANALYZER_PATH" ] || [ ! -x "$MTLOG_ANALYZER_PATH" ]; then
    echo "ERROR: mtlog-analyzer not found. Please run setup_test_env.sh first"
    exit 1
fi

# Set test project directory
export MTLOG_TEST_PROJECT_DIR="${MTLOG_TEST_PROJECT_DIR:-/tmp/mtlog-test-project-$$}"

echo "Running mtlog-analyzer Neovim plugin tests..."
echo "Analyzer: $MTLOG_ANALYZER_PATH"
echo "Test project: $MTLOG_TEST_PROJECT_DIR"
echo ""

# Run tests
nvim --headless \
    -u tests/minimal_init.lua \
    -c "PlenaryBustedDirectory tests/spec/ { minimal_init = 'tests/minimal_init.lua' }"

# Check exit code
if [ $? -eq 0 ]; then
    echo ""
    echo "All tests passed!"
else
    echo ""
    echo "Some tests failed. Check the output above for details."
    exit 1
fi
EOF

chmod +x run_tests.sh
print_success "Created run_tests.sh script"

echo ""
echo "Step 6: Verifying everything works..."
echo "--------------------------------------"

# Export environment variables for test
export MTLOG_ANALYZER_PATH="$ANALYZER_PATH"
export MTLOG_TEST_PROJECT_DIR="$TEST_PROJECT_DIR"

# Try to run a simple test
print_info "Running verification test..."
VERIFY_OUTPUT=$(nvim --headless -u tests/minimal_init.lua -c "lua print('Test environment verified')" -c "qa!" 2>&1)

if echo "$VERIFY_OUTPUT" | grep -q "Test environment verified"; then
    print_success "Test environment is working correctly"
else
    print_warning "Could not verify test environment completely"
    echo "Output: $VERIFY_OUTPUT"
fi

echo ""
echo "==================================================================="
echo "Setup Complete!"
echo "==================================================================="
echo ""
echo "Environment variables set:"
echo "  MTLOG_ANALYZER_PATH=$ANALYZER_PATH"
echo "  MTLOG_TEST_PROJECT_DIR=$TEST_PROJECT_DIR"
echo ""
echo "To run tests:"
echo "  1. Export the environment variables above (or they'll be set automatically)"
echo "  2. Run: ./run_tests.sh"
echo ""
echo "To run specific test files:"
echo "  nvim --headless -u tests/minimal_init.lua \\"
echo "    -c \"PlenaryBustedFile tests/spec/analyzer_spec.lua\""
echo ""
print_success "Test environment is ready!"