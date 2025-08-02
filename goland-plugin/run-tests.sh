#!/bin/bash
# Shell script to run GoLand plugin tests locally

echo -e "\033[32mRunning GoLand Plugin Integration Tests\033[0m"
echo -e "\033[32m=======================================\033[0m"

# Check if mtlog-analyzer is installed
if ! command -v mtlog-analyzer &> /dev/null; then
    echo -e "\033[31mERROR: mtlog-analyzer not found in PATH\033[0m"
    echo -e "\033[33mPlease install it first:\033[0m"
    echo -e "\033[33m  go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest\033[0m"
    exit 1
fi

echo -e "\033[36mFound mtlog-analyzer at: $(which mtlog-analyzer)\033[0m"

# Run the tests
echo -e "\n\033[32mRunning tests...\033[0m"
./gradlew test --info

if [ $? -eq 0 ]; then
    echo -e "\n\033[32mAll tests passed!\033[0m"
else
    echo -e "\n\033[31mTests failed!\033[0m"
    echo -e "\033[33mCheck build/reports/tests/test/index.html for details\033[0m"
fi

# Option to open test report
read -p "Open test report in browser? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        open build/reports/tests/test/index.html
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        xdg-open build/reports/tests/test/index.html
    fi
fi