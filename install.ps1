# PowerShell Automated Installer for file-watcher

$ErrorActionPreference = "Stop"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "      file-watcher Installer (Windows)    " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$exePath = Join-Path $scriptDir "file-watcher.exe"

if (-not (Test-Path $exePath)) {
    Write-Host "file-watcher.exe not found in $scriptDir. Building binary..." -ForegroundColor Yellow
    go build -o $exePath ./cmd/file-watcher
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed! Please ensure Go is installed." -ForegroundColor Red
        exit 1
    }
}

Write-Host "`nRunning file-watcher installation..." -ForegroundColor Cyan
& $exePath install

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nInstallation complete! file-watcher is now configured as a user autostart task." -ForegroundColor Green
    Write-Host "To check status: & '$exePath' status" -ForegroundColor Gray
    Write-Host "To uninstall:    & '$exePath' uninstall" -ForegroundColor Gray
} else {
    Write-Host "Installation failed." -ForegroundColor Red
    exit 1
}
