package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"file-watcher/pkg/logging"
	"file-watcher/pkg/watcher"
)

func TestWatcherPollingMode(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_poll_*")
	if err != nil {
		t.Fatalf("failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_poll_*")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	logger := logging.NewConsoleLogger(nil)
	w, err := watcher.New(watcher.Options{
		WatchDir:     watchDir,
		Pattern:      "*.pdf",
		DestDir:      destDir,
		PollInterval: 100 * time.Millisecond,
		UsePolling:   true,
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Start(ctx)
	}()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create test file
	testFile := filepath.Join(watchDir, "doc.pdf")
	if err := os.WriteFile(testFile, []byte("pdf content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for polling loop to process file
	time.Sleep(300 * time.Millisecond)

	destFile := filepath.Join(destDir, "doc.pdf")
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("file was not moved to destination: %s", destFile)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("watcher returned error on stop: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("watcher failed to stop within timeout")
	}
}

func TestWatcherFSNotifyMode(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_fsnotify_*")
	if err != nil {
		t.Fatalf("failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_fsnotify_*")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	logger := logging.NewConsoleLogger(nil)
	w, err := watcher.New(watcher.Options{
		WatchDir:      watchDir,
		Pattern:       "*.png",
		DestDir:       destDir,
		DebounceDelay: 50 * time.Millisecond,
		UsePolling:    false,
		Logger:        logger,
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	testFile := filepath.Join(watchDir, "image.png")
	if err := os.WriteFile(testFile, []byte("png image data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	destFile := filepath.Join(destDir, "image.png")
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("file was not moved to destination via fsnotify: %s", destFile)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("watcher returned error on stop: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("watcher failed to stop within timeout")
	}
}
