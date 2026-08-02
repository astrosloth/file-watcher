// Package service provides cross-platform functionality for installing, uninstalling,
// and checking the status of file-watcher as a background user autostart service/task.
package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"file-watcher/pkg/config"
)

// DefaultConfigTemplate contains starter configuration examples (commented out by default to avoid unexpected file moves).
const DefaultConfigTemplate = `# File Watcher Configuration
# 
# Define watches by adding [watch:name] sections.
# Options per watch:
#   dir              - Directory to watch (required)
#   pattern          - Glob pattern to match (required, e.g. *.pdf, *.{jpg,png})
#   dest             - Destination directory (required)
#   extract-archives - Extract from single-file archives (true/false, default: false)
#   poll-interval    - Polling interval in seconds (default: 5)
#   use-polling      - Force polling mode (true/false, default: false)
#
# Note: Paths starting with ~ expand to your user home directory.
# After editing this file, restart file-watcher for changes to take effect:
#   file-watcher install   (or restart the Scheduled Task / Service)

# Example 1: Automatically organize PDFs from Downloads to Documents/PDFs
# [watch:pdfs]
# dir = ~/Downloads
# pattern = *.pdf
# dest = ~/Documents/PDFs
# extract-archives = true

# Example 2: Automatically organize image files
# [watch:images]
# dir = ~/Downloads
# pattern = *.{jpg,jpeg,png,gif,webp}
# dest = ~/Pictures
`

// copyFile copies a source file to a destination file path.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create target binary: %w", err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	return out.Close()
}

// EnsureDefaultConfig creates a default configuration file if targetPath does not exist.
func EnsureDefaultConfig(targetPath string) (string, error) {
	if targetPath == "" {
		targetPath = config.ExpandPath("~/.config/file-watcher/file-watcher.conf")
	} else {
		targetPath = config.ExpandPath(targetPath)
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(DefaultConfigTemplate), 0644); err != nil {
			return "", fmt.Errorf("failed to write default config: %w", err)
		}
	}
	return targetPath, nil
}
