package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"file-watcher/pkg/config"
)

func TestRunWatchStopsOnCancel(t *testing.T) {
	watchDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

	wc := config.WatchConfig{
		Name:         "test",
		Dir:          watchDir,
		Pattern:      "*.txt",
		Dest:         destDir,
		PollInterval: 1,
		UsePolling:   true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runWatch(ctx, wc, slog.New(slog.NewTextHandler(os.Stderr, nil)))
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runWatch did not return after context cancel")
	}
}
