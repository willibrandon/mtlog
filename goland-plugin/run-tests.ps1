# PowerShell script to run GoLand plugin tests locally

Write-Host "Running GoLand Plugin Integration Tests" -ForegroundColor Green
Write-Host "=======================================" -ForegroundColor Green

# Check if mtlog-analyzer is installed
$analyzer = Get-Command mtlog-analyzer -ErrorAction SilentlyContinue
if (-not $analyzer) {
    Write-Host "ERROR: mtlog-analyzer not found in PATH" -ForegroundColor Red
    Write-Host "Please install it first:" -ForegroundColor Yellow
    Write-Host "  go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest" -ForegroundColor Yellow
    exit 1
}

Write-Host "Found mtlog-analyzer at: $($analyzer.Path)" -ForegroundColor Cyan

# Run the tests
Write-Host "`nRunning tests..." -ForegroundColor Green
.\gradlew.bat test --info

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nAll tests passed!" -ForegroundColor Green
} else {
    Write-Host "`nTests failed!" -ForegroundColor Red
    Write-Host "Check build/reports/tests/test/index.html for details" -ForegroundColor Yellow
}

# Option to open test report
$openReport = Read-Host "`nOpen test report in browser? (y/n)"
if ($openReport -eq 'y') {
    Start-Process "build\reports\tests\test\index.html"
}