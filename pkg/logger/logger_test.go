package logger_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-watcher/pkg/logger"
)

func TestRotatingLogger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "log_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")
	// Small max size to trigger rotation (100 bytes)
	l, err := logger.NewFileLogger(logFile, 100)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	// Write enough log messages to exceed 100 bytes
	for i := 0; i < 10; i++ {
		l.Info("This is a test log line meant to exceed size limit", "index", i)
	}

	// Check if backup log file test.log.1 was created
	backupFile := logFile + ".1"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Errorf("expected backup file %s to exist after log rotation", backupFile)
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read current log file: %v", err)
	}
	if !strings.Contains(string(content), "This is a test log line") {
		t.Errorf("log file content missing expected text")
	}
}

func TestConsoleLogger(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewConsoleLogger(&buf)
	l.Info("console test message", "key", "val")

	if !strings.Contains(buf.String(), "console test message") {
		t.Errorf("console logger output missing expected string: %s", buf.String())
	}
}

func TestRotatingWriterClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "writer_close_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "close.log")
	rw, err := logger.NewRotatingWriter(logFile, 1024)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}

	_, err = rw.Write([]byte("some data\n"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if err := rw.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Double close should be safe
	if err := rw.Close(); err != nil {
		t.Fatalf("double close failed: %v", err)
	}
}
