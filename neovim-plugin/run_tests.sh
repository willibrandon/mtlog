#!/bin/bash
# Automated test runner for mtlog.nvim plugin

set -e

echo "================================"
echo "Running mtlog.nvim test suite"
echo "================================"

# Check if Neovim is installed
if ! command -v nvim &> /dev/null; then
    echo "Error: Neovim is not installed"
    exit 1
fi

# Check if Plenary is installed (will be handled by minimal_init.lua)
echo "Setting up test environment..."

# Run all tests
echo "Running tests..."
nvim --headless --noplugin -u tests/minimal_init.lua \
  -c "lua require('plenary.busted')" \
  -c "lua require('plenary.test_harness').test_directory('tests/spec', { minimal_init = 'tests/minimal_init.lua', sequential = true })" \
  +qa

# Check exit code
if [ $? -eq 0 ]; then
    echo "================================"
    echo "All tests passed successfully!"
    echo "================================"
    exit 0
else
    echo "================================"
    echo "Test failures detected"
    echo "================================"
    exit 1
fi