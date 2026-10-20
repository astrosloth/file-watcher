#!/usr/bin/env bash
# Automated installer for file-watcher (Linux / macOS)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXE_PATH="$SCRIPT_DIR/file-watcher"

if [ ! -f "$EXE_PATH" ]; then
    echo "file-watcher binary not found in $SCRIPT_DIR. Building binary..."
    go -C "$SCRIPT_DIR" build -o "$EXE_PATH" ./cmd/file-watcher
fi

echo "Running file-watcher installation..."
"$EXE_PATH" install
