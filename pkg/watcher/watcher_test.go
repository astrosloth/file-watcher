package watcher

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"file-watcher/pkg/logger"
)

func createZipFile(t *testing.T, dir, filename string, files map[string]string) string {
	t.Helper()
	zipPath := filepath.Join(dir, filename)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	return zipPath
}

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

	l := logger.NewConsoleLogger(nil)
	w, err := New(Options{
		WatchDir:     watchDir,
		Pattern:      "*.pdf",
		DestDir:      destDir,
		PollInterval: 100 * time.Millisecond,
		UsePolling:   true,
		Logger:       l,
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

	testFile := filepath.Join(watchDir, "doc.pdf")
	if err := os.WriteFile(testFile, []byte("pdf content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

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

	l := logger.NewConsoleLogger(nil)
	w, err := New(Options{
		WatchDir:      watchDir,
		Pattern:       "*.png",
		DestDir:       destDir,
		DebounceDelay: 50 * time.Millisecond,
		UsePolling:    false,
		Logger:        l,
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

func TestWatcherArchiveExtraction(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_archive_*")
	if err != nil {
		t.Fatalf("failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_archive_*")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	zipPath := createZipFile(t, watchDir, "test.zip", map[string]string{
		"sample.pdf": "zip pdf content",
	})

	l := logger.NewConsoleLogger(nil)
	w, err := New(Options{
		WatchDir:        watchDir,
		Pattern:         "*.pdf",
		DestDir:         destDir,
		ExtractArchives: true,
		PollInterval:    100 * time.Millisecond,
		UsePolling:      true,
		Logger:          l,
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	extractedFile := filepath.Join(destDir, "sample.pdf")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Errorf("single archive file was not extracted: %s", extractedFile)
	}

	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Errorf("archive file should have been removed: %s", zipPath)
	}
}

func TestWatcherArchiveFallbackMove(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_archive_fallback_*")
	if err != nil {
		t.Fatalf("failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_archive_fallback_*")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create a zip file containing MULTIPLE files so it won't be extracted
	zipPath := createZipFile(t, watchDir, "archive.zip", map[string]string{
		"file1.txt": "content 1",
		"file2.txt": "content 2",
	})

	l := logger.NewConsoleLogger(nil)
	w, err := New(Options{
		WatchDir:        watchDir,
		Pattern:         "*.zip",
		DestDir:         destDir,
		ExtractArchives: true,
		PollInterval:    100 * time.Millisecond,
		UsePolling:      true,
		Logger:          l,
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	// Because archive contained 2 files, single-file extraction did not occur,
	// but because pattern "*.zip" matched "archive.zip", archive.zip should be moved to destDir.
	movedZip := filepath.Join(destDir, "archive.zip")
	if _, err := os.Stat(movedZip); os.IsNotExist(err) {
		t.Errorf("archive.zip was not moved to destination on fallback: %s", movedZip)
	}

	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Errorf("original archive should no longer exist in watchDir: %s", zipPath)
	}
}

func TestCopyAndRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copy_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := []byte("hello world copy test data")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	if err := copyAndRemove(src, dst); err != nil {
		t.Fatalf("copyAndRemove failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src file should be deleted")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("dst content = %q; want %q", string(got), string(content))
	}
}

func TestCopyAndRemoveFailureCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copy_fail_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "nonexistent.txt")
	dst := filepath.Join(tmpDir, "dst_fail.txt")

	err = copyAndRemove(src, dst)
	if err == nil {
		t.Errorf("expected error for non-existent source, got nil")
	}

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("destination file should not exist after failed copy")
	}
}
