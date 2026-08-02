// Package logging provides size-rotating file logger capabilities and structured console logging.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// RotatingWriter is a thread-safe io.WriteCloser that writes log entries to a file,
// rotating the file when it exceeds the configured maximum byte size.
type RotatingWriter struct {
	mu         sync.Mutex
	filePath   string
	maxSize    int64
	file       *os.File
	currentLen int64
}

// NewRotatingWriter creates and opens a new RotatingWriter target file at filePath.
// If maxSize is less than or equal to 0, defaults to 1MB (1,048,576 bytes).
func NewRotatingWriter(filePath string, maxSize int64) (*RotatingWriter, error) {
	if maxSize <= 0 {
		maxSize = 1048576 // default 1MB
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	rw := &RotatingWriter{
		filePath: filePath,
		maxSize:  maxSize,
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// openFile opens or creates the underlying log file in append mode.
func (rw *RotatingWriter) openFile() error {
	info, err := os.Stat(rw.filePath)
	if err == nil {
		rw.currentLen = info.Size()
	} else {
		rw.currentLen = 0
	}

	f, err := os.OpenFile(rw.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	rw.file = f
	return nil
}

// Write writes data p to the active log file, triggering a rotation if the written bytes exceed maxSize.
func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.currentLen+int64(len(p)) >= rw.maxSize {
		rw.rotate()
	}

	if rw.file == nil {
		if err := rw.openFile(); err != nil {
			return 0, err
		}
	}

	n, err = rw.file.Write(p)
	rw.currentLen += int64(n)
	return n, err
}

// rotate closes the current log file and renames it to <filePath>.1, replacing any existing backup.
func (rw *RotatingWriter) rotate() {
	if rw.file != nil {
		_ = rw.file.Close()
		rw.file = nil
	}

	backup := rw.filePath + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(rw.filePath, backup)
	rw.currentLen = 0
}

// Close closes the underlying open log file.
func (rw *RotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file != nil {
		err := rw.file.Close()
		rw.file = nil
		return err
	}
	return nil
}

// NewFileLogger creates a slog.Logger writing to a size-rotated log file.
func NewFileLogger(filePath string, maxSize int64) (*slog.Logger, error) {
	rw, err := NewRotatingWriter(filePath, maxSize)
	if err != nil {
		return nil, err
	}
	handler := slog.NewTextHandler(rw, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler), nil
}

// NewConsoleLogger creates a slog.Logger writing structured text logs to stdout or the provided writer.
func NewConsoleLogger(w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}
