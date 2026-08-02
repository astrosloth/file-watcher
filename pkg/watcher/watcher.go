package watcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"file-watcher/pkg/archive"
	"file-watcher/pkg/pattern"
	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	opts Options
}

// New creates a new Watcher instance with the specified Options struct.
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

	return &Watcher{opts: opts}, nil
}

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

func (w *Watcher) processExistingFiles(ctx context.Context) {
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

func (w *Watcher) handleFile(path string) error {
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
	if w.opts.ExtractArchives && archive.IsArchive(path) {
		extracted, destPath, err := archive.InspectAndExtractSingleFile(path, w.opts.Pattern, w.opts.DestDir)
		if err != nil {
			w.opts.Logger.Error("Failed processing archive", "archive", baseName, "error", err)
			return err
		}
		if extracted {
			w.opts.Logger.Info("Extracted single archive file", "archive", baseName, "dest", destPath)
			return nil
		}
		return nil
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
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 4. Move file to destination
	if err := os.Rename(path, destPath); err != nil {
		if err := copyAndRemove(path, destPath); err != nil {
			w.opts.Logger.Error("Failed to move file", "source", path, "dest", destPath, "error", err)
			return err
		}
	}

	w.opts.Logger.Info("Moved file", "file", baseName, "dest", destPath)
	return nil
}

func (w *Watcher) runFSNotify(ctx context.Context) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.opts.Logger.Warn("fsnotify creation failed, falling back to polling", "error", err)
		return w.runPolling(ctx)
	}
	defer fsWatcher.Close()

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
				go func(p string) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(w.opts.DebounceDelay):
					}

					if _, err := os.Stat(p); err == nil {
						_ = w.handleFile(p)
					}
				}(targetPath)
			}
		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.opts.Logger.Error("fsnotify watcher error", "error", err)
		}
	}
}

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

func copyAndRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	_ = in.Close()
	_ = out.Close()

	return os.Remove(src)
}
