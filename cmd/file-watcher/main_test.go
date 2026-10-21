package main

import (
	"context"
	"file-watcher/pkg/watcher"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunWatchGroupStopsOnCancel(t *testing.T) {
	watchDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

	rules := []watcher.Rule{{
		Name:    "test",
		Pattern: "*.txt",
		DestDir: destDir,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runWatchGroup(ctx, watchDir, rules, 1, true, slog.New(slog.NewTextHandler(os.Stderr, nil)))
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runWatchGroup did not return after context cancel")
	}
}
