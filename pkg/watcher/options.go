// Package watcher provides file system monitoring capabilities using both event-driven (fsnotify)
// and polling strategies, with support for glob pattern matching, debouncing, and archive extraction.
package watcher

import (
	"log/slog"
	"time"
)

// Options specifies configuration settings for directory watching operations.
type Options struct {
	// WatchDir is the absolute or relative directory path to monitor for incoming files.
	WatchDir string

	// Pattern is the glob pattern used to match target files (e.g. "*.pdf", "*.{jpg,png}").
	Pattern string

	// DestDir is the directory path where matched files will be moved or extracted.
	DestDir string

	// ExtractArchives enables automatic single-file archive inspection and extraction.
	ExtractArchives bool

	// PollInterval sets the frequency at which directory contents are scanned when polling mode is active.
	PollInterval time.Duration

	// UsePolling forces polling mode instead of using native OS event notifications (fsnotify).
	UsePolling bool

	// DebounceDelay is the duration to wait after a file system event before processing the file.
	DebounceDelay time.Duration

	// Logger is the structured logger instance used for reporting status, progress, and errors.
	Logger *slog.Logger
}
