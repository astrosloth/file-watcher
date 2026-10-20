#!/usr/bin/env bash
# Automated quality & test check script for Linux / macOS / POSIX shells

set -e

# Color definitions
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${CYAN}==> Checking code formatting (go fmt)...${NC}"
UNFORMATTED=$(gofmt -s -l .)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${YELLOW}Formatting files:${NC}"
    echo "$UNFORMATTED"
    go fmt ./...
else
    echo -e "${GREEN}All files cleanly formatted!${NC}"
fi

echo -e "\n${CYAN}==> Running static analysis (go vet)...${NC}"
if ! go vet ./...; then
    echo -e "${RED}go vet found issues!${NC}"
    exit 1
fi
echo -e "${GREEN}go vet clean!${NC}"

echo -e "\n${CYAN}==> Running unit tests and measuring coverage...${NC}"
if ! go test -coverprofile="coverage.out" ./pkg/...; then
    echo -e "${RED}Tests failed!${NC}"
    exit 1
fi

echo -e "\n${CYAN}==> Test Coverage Summary:${NC}"
go tool cover -func="coverage.out"

echo -e "\n${CYAN}==> Building binary (file-watcher)...${NC}"
if go build -o file-watcher ./cmd/file-watcher; then
    echo -e "\n${GREEN}Success! Binary compiled and all quality checks passed cleanly.${NC}"
fi
