#!/usr/bin/env fish
# Automated quality & test check script for Fish shell

set -l CYAN (set_color cyan)
set -l YELLOW (set_color yellow)
set -l GREEN (set_color green)
set -l RED (set_color red)
set -l NORMAL (set_color normal)

echo "$CYAN==> Checking code formatting (go fmt)...$NORMAL"
set -l unformatted (gofmt -s -l .)
if test -n "$unformatted"
    echo "$YELLOWFormatting files:$NORMAL"
    for file in $unformatted
        echo $file
    end
    go fmt ./...
else
    echo "$GREENAll files cleanly formatted!$NORMAL"
end

echo ""
echo "$CYAN==> Running static analysis (go vet)...$NORMAL"
go vet ./...
if test $status -ne 0
    echo "$REDgo vet found issues!$NORMAL"
    exit 1
end
echo "$GREENgo vet clean!$NORMAL"

echo ""
echo "$CYAN==> Running unit tests and measuring coverage...$NORMAL"
go test -coverprofile="coverage.out" ./pkg/...
if test $status -ne 0
    echo "$REDTests failed!$NORMAL"
    exit 1
end

echo ""
echo "$CYAN==> Test Coverage Summary:$NORMAL"
go tool cover -func="coverage.out"

echo ""
echo "$CYAN==> Building binary (file-watcher)...$NORMAL"
go build -o file-watcher ./cmd/file-watcher
if test $status -eq 0
    echo ""
    echo "$GREENSuccess! Binary compiled and all quality checks passed cleanly.$NORMAL"
end
