package watcher

import (
	"log/slog"
	"time"
)

// Options specifies configuration settings for directory watching.
type Options struct {
	WatchDir        string
	Pattern         string
	DestDir         string
	ExtractArchives bool
	PollInterval    time.Duration
	UsePolling      bool
	DebounceDelay   time.Duration
	Logger          *slog.Logger
}
