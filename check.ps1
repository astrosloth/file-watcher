# PowerShell automated quality & test check script

Write-Host "==> Checking code formatting (go fmt)..." -ForegroundColor Cyan
$unformatted = gofmt -s -l .
if ($unformatted) {
    Write-Host "Formatting files:" -ForegroundColor Yellow
    $unformatted
    go fmt ./...
} else {
    Write-Host "All files cleanly formatted!" -ForegroundColor Green
}

Write-Host "`n==> Running static analysis (go vet)..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "go vet found issues!" -ForegroundColor Red
    exit 1
}
Write-Host "go vet clean!" -ForegroundColor Green

Write-Host "`n==> Running unit tests and measuring coverage..." -ForegroundColor Cyan
go test -coverprofile="coverage.out" ./pkg/...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed!" -ForegroundColor Red
    exit 1
}

Write-Host "`n==> Test Coverage Summary:" -ForegroundColor Cyan
go tool cover -func="coverage.out"

Write-Host "`n==> Building binary (file-watcher.exe)..." -ForegroundColor Cyan
go build -o file-watcher.exe ./cmd/file-watcher
if ($LASTEXITCODE -eq 0) {
    Write-Host "`nSuccess! Binary compiled and all quality checks passed cleanly." -ForegroundColor Green
}
