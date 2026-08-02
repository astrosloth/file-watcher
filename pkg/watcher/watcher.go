package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"file-watcher/pkg/pipeline"
	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	opts      Options
	processor pipeline.Processor
	seen      sync.Map
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

	proc := pipeline.BuildPipeline(
		opts.DestDir,
		opts.Pattern,
		opts.ExtractArchives,
		opts.Logger,
	)

	return &Watcher{
		opts:      opts,
		processor: proc,
	}, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	// Create destination directory if missing
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

	// Process existing files first
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
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		ev := pipeline.Event{
			Path:     path,
			Basename: entry.Name(),
			FileInfo: fi,
		}

		_ = w.processor(ctx, ev)
	}
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

				baseName := filepath.Base(event.Name)
				ev := pipeline.Event{
					Path:     event.Name,
					Basename: baseName,
					FileInfo: fi,
				}

				// Debounce to allow write to stabilize
				go func(ev pipeline.Event) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(w.opts.DebounceDelay):
					}

					// Verify file still exists after debounce
					if _, err := os.Stat(ev.Path); err == nil {
						_ = w.processor(ctx, ev)
					}
				}(ev)
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
