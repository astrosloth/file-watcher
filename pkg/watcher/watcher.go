package watcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"file-watcher/pkg/archive"
	"file-watcher/pkg/pattern"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors a specified directory for new or updated files, matching them
// against a pattern and moving or extracting them to a destination directory.
type Watcher struct {
	opts       Options
	mu         sync.Mutex
	pending    map[string]*time.Timer
	processing map[string]bool
	failed     map[string]time.Time
}

// New initializes and validates a new Watcher instance with default fallback settings for missing options.
func New(opts Options) (*Watcher, error) {
	if opts.WatchDir == "" {
		return nil, fmt.Errorf("watch directory cannot be empty")
	}

	opts.WatchDir = filepath.Clean(opts.WatchDir)

	if opts.PollInterval <= 0 {
		opts.PollInterval = 5 * time.Second
	}
	if opts.DebounceDelay <= 0 {
		opts.DebounceDelay = 500 * time.Millisecond
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &Watcher{
		opts:       opts,
		pending:    make(map[string]*time.Timer),
		processing: make(map[string]bool),
		failed:     make(map[string]time.Time),
	}, nil
}

// Start initiates the directory watching process using event-driven notification (fsnotify)
// or periodic polling depending on configuration options. Blocks until the provided context is canceled.
func (w *Watcher) Start(ctx context.Context) error {
	if w.opts.DestDir != "" {
		if err := os.MkdirAll(w.opts.DestDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	w.opts.Logger.Info("Starting watcher",
		"dir", w.opts.WatchDir,
		"pattern", w.opts.Pattern,
		"dest", w.opts.DestDir,
		"use_polling", w.opts.UsePolling,
	)

	w.processExistingFiles(ctx)

	if w.opts.UsePolling {
		return w.runPolling(ctx)
	}
	return w.runFSNotify(ctx)
}

// stopPendingTimers cancels all active debouncing timers on shutdown.
func (w *Watcher) stopPendingTimers() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for p, t := range w.pending {
		t.Stop()
		delete(w.pending, p)
	}
}

// processExistingFiles scans the watch directory for pre-existing files matching the pattern.
func (w *Watcher) processExistingFiles(ctx context.Context) {
	w.pruneFailures()

	entries, err := os.ReadDir(w.opts.WatchDir)
	if err != nil {
		w.opts.Logger.Error("Failed to read watch dir for existing files", "error", err)
		return
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if entry.IsDir() {
			continue
		}

		path := filepath.Join(w.opts.WatchDir, entry.Name())
		_ = w.handleFile(path)
	}
}

func (w *Watcher) pruneFailures() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	for p, t := range w.failed {
		if now.Sub(t) > 5*time.Minute {
			delete(w.failed, p)
		}
	}
}

// handleFile processes an individual file, handling pattern matching, archive extraction,
// filename collision resolution, and moving the file to the destination directory.
func (w *Watcher) handleFile(path string) error {
	w.mu.Lock()
	if w.processing[path] {
		w.mu.Unlock()
		return nil
	}
	if lastErrTime, failedBefore := w.failed[path]; failedBefore && time.Since(lastErrTime) < 30*time.Second {
		w.mu.Unlock()
		return nil
	}
	w.processing[path] = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.processing, path)
		w.mu.Unlock()
	}()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	baseName := filepath.Base(path)

	// Check pattern match
	matched, err := pattern.Match(w.opts.Pattern, baseName)
	if err != nil {
		return err
	}

	// 1. Single-file archive extraction
	if w.opts.ExtractArchives && archive.IsSupported(path) {
		extracted, destPath, err := archive.InspectAndExtractSingleFile(path, w.opts.Pattern, w.opts.DestDir)
		if err != nil {
			w.opts.Logger.Error("Failed processing archive", "archive", baseName, "error", err)
			w.recordFailure(path)
			return err
		}
		if extracted {
			w.opts.Logger.Info("Extracted single archive file", "archive", baseName, "dest", destPath)
			w.clearFailure(path)
			return nil
		}
		// If archive was not extracted, fall through to check if the archive itself matches pattern
	}

	// 2. Skip non-matching files
	if !matched {
		return nil
	}

	// 3. Resolve target duplicate filename using pattern package helper
	targetName := pattern.ResolveTarget(baseName, func(name string) bool {
		_, err := os.Stat(filepath.Join(w.opts.DestDir, name))
		return err == nil
	})

	destPath := filepath.Join(w.opts.DestDir, targetName)

	if err := os.MkdirAll(w.opts.DestDir, 0755); err != nil {
		w.recordFailure(path)
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 4. Move file to destination
	if err := os.Rename(path, destPath); err != nil {
		if err := copyAndRemove(path, destPath); err != nil {
			w.opts.Logger.Error("Failed to move file", "source", path, "dest", destPath, "error", err)
			w.recordFailure(path)
			return err
		}
	}

	w.clearFailure(path)
	w.opts.Logger.Info("Moved file", "file", baseName, "dest", destPath)
	return nil
}

func (w *Watcher) recordFailure(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failed[path] = time.Now()
}

func (w *Watcher) clearFailure(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.failed, path)
}

// runFSNotify runs the event-driven watcher loop using operating system inotify/fsnotify events.
func (w *Watcher) runFSNotify(ctx context.Context) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.opts.Logger.Warn("fsnotify creation failed, falling back to polling", "error", err)
		return w.runPolling(ctx)
	}
	defer fsWatcher.Close()
	defer w.stopPendingTimers()

	if err := fsWatcher.Add(w.opts.WatchDir); err != nil {
		w.opts.Logger.Warn("fsnotify watch add failed, falling back to polling", "error", err)
		return w.runPolling(ctx)
	}

	w.opts.Logger.Info("Inotify / Event-driven watcher active", "dir", w.opts.WatchDir)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				fi, err := os.Stat(event.Name)
				if err != nil || fi.IsDir() {
					continue
				}

				targetPath := event.Name
				w.scheduleDebounced(targetPath)
			}
		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.opts.Logger.Error("fsnotify watcher error", "error", err)
		}
	}
}

// scheduleDebounced schedules or resets a debouncing timer for targetPath to prevent race conditions.
func (w *Watcher) scheduleDebounced(targetPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.pending[targetPath]; ok {
		t.Stop()
	}

	w.pending[targetPath] = time.AfterFunc(w.opts.DebounceDelay, func() {
		w.mu.Lock()
		delete(w.pending, targetPath)
		w.mu.Unlock()

		if _, err := os.Stat(targetPath); err == nil {
			_ = w.handleFile(targetPath)
		}
	})
}

// runPolling runs a fallback polling loop scanning the watch directory at fixed intervals.
func (w *Watcher) runPolling(ctx context.Context) error {
	ticker := time.NewTicker(w.opts.PollInterval)
	defer ticker.Stop()

	w.opts.Logger.Info("Polling watcher active", "interval", w.opts.PollInterval)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-ticker.C:
			w.processExistingFiles(ctx)
		}
	}
}

// copyAndRemove copies a file from src to dst and removes src upon success.
// It uses a temporary .tmp extension on destination to ensure atomic file placement,
// flushes write buffers via Sync(), and retries transient failures before removing src.
func copyAndRemove(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	tmpDst := dst + ".tmp"
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = atomicCopyFile(src, tmpDst, dst, info.Mode())
		if err == nil {
			return os.Remove(src)
		}
		lastErr = err
		_ = os.Remove(tmpDst)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}
	}

	return fmt.Errorf("failed to copy file after %d attempts: %w", maxRetries, lastErr)
}

func atomicCopyFile(src, tmpDst, finalDst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(tmpDst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	return os.Rename(tmpDst, finalDst)
}
