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
