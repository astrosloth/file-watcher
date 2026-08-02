// Package archive provides functionality for inspecting archive files (.zip, .tar, .tar.gz, .tar.bz2, .tar.xz, .7z, .rar)
// and extracting single-file payloads matching target pattern specifications.
package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"file-watcher/pkg/pattern"
)

// IsSupported checks if the file path has a supported archive extension.
func IsSupported(path string) bool {
	lower := strings.ToLower(path)
	extensions := []string{
		".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz", ".tbz2", ".tar.xz", ".txz",
		".7z", ".rar",
	}
	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// InspectAndExtractSingleFile checks if the archive at archivePath contains exactly one file entry matching targetPattern.
// If matched, it streams the file to destDir (resolving duplicate filenames) and deletes the original archive file.
// Returns a boolean indicating whether extraction took place, the final extracted file path, and any error encountered.
func InspectAndExtractSingleFile(archivePath, targetPattern, destDir string) (bool, string, error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return inspectAndExtractZip(archivePath, targetPattern, destDir)
	}
	if strings.HasSuffix(lower, ".7z") {
		return inspectAndExtract7z(archivePath, targetPattern, destDir)
	}
	if strings.HasSuffix(lower, ".rar") {
		return inspectAndExtractRar(archivePath, targetPattern, destDir)
	}
	return inspectAndExtractTar(archivePath, targetPattern, destDir)
}

// matchSinglePayload validates if the archive contains exactly one file entry matching targetPattern.
func matchSinglePayload(entryName string, fileCount int, targetPattern string) (bool, string, error) {
	if fileCount != 1 || entryName == "" {
		return false, "", nil
	}
	baseName := filepath.Base(entryName)
	matched, err := pattern.Match(targetPattern, baseName)
	if err != nil || !matched {
		return false, "", err
	}
	return true, baseName, nil
}

// finalizeExtraction removes the original archive file upon successful extraction and returns extraction results.
func finalizeExtraction(archivePath, destPath string, err error) (bool, string, error) {
	if err != nil {
		return false, "", err
	}
	_ = os.Remove(archivePath)
	return true, destPath, nil
}

type entryItem struct {
	Name string
}

type archiveIterator interface {
	Next() (*entryItem, io.Reader, error)
	Close() error
}

// inspectAndExtractSequential handles two-pass inspection and extraction for stream-based archives (e.g. TAR, RAR).
func inspectAndExtractSequential(archivePath, targetPattern, destDir string, openIter func() (archiveIterator, error)) (bool, string, error) {
	// Pass 1: Count files and record single entry name
	iter, err := openIter()
	if err != nil {
		return false, "", err
	}

	var singleEntryName string
	fileCount := 0

	for {
		item, _, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = iter.Close()
			return false, "", err
		}

		fileCount++
		if fileCount > 1 {
			break
		}
		singleEntryName = item.Name
	}
	_ = iter.Close()

	ok, baseName, err := matchSinglePayload(singleEntryName, fileCount, targetPattern)
	if !ok || err != nil {
		return ok, "", err
	}

	// Pass 2: Extract target entry
	iter2, err := openIter()
	if err != nil {
		return false, "", err
	}
	defer iter2.Close()

	var destPath string
	var extractErr error

	for {
		item, r, err := iter2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			extractErr = err
			break
		}
		if item.Name == singleEntryName {
			destPath, extractErr = extractStream(r, baseName, destDir)
			break
		}
	}

	return finalizeExtraction(archivePath, destPath, extractErr)
}

// extractStream copies from an entry stream reader r to destDir, resolving filename collisions.
func extractStream(r io.Reader, baseName, destDir string) (string, error) {
	targetName := pattern.ResolveTarget(baseName, func(name string) bool {
		_, err := os.Stat(filepath.Join(destDir, name))
		return err == nil
	})

	destPath := filepath.Join(destDir, targetName)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination dir: %w", err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}

	if _, err := io.Copy(outFile, r); err != nil {
		_ = outFile.Close()
		_ = os.Remove(destPath)
		return "", fmt.Errorf("failed to copy content: %w", err)
	}

	if err := outFile.Close(); err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("failed to close destination file: %w", err)
	}

	return destPath, nil
}
