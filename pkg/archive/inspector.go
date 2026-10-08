// Package archive provides functionality for inspecting archive files (.zip, .tar, .tar.gz, .tar.bz2, .tar.xz, .7z, .rar)
// and extracting single-file payloads matching target pattern specifications.
package archive

import (
	"errors"
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
	if !IsSupported(archivePath) {
		return false, "", nil
	}
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

// MaxExtractEntrySize is the maximum allowed size (500 MB) when extracting a single archive entry to prevent zip bombs.
const MaxExtractEntrySize = 500 * 1024 * 1024

// cleanEntryBaseName sanitizes entryName by normalizing separators and extracting the clean base filename.
func cleanEntryBaseName(entryName string) string {
	// Replace backslashes with forward slashes for cross-platform normalization
	normalized := strings.ReplaceAll(entryName, "\\", "/")
	base := filepath.Base(normalized)
	if base == "." || base == "/" || base == ".." {
		return ""
	}
	return base
}

// matchSinglePayload validates if the archive contains exactly one file entry matching targetPattern.
func matchSinglePayload(entryName string, fileCount int, targetPattern string) (bool, string, error) {
	if fileCount != 1 || entryName == "" {
		return false, "", nil
	}
	baseName := cleanEntryBaseName(entryName)
	if baseName == "" {
		return false, "", nil
	}
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

type randomAccessEntry struct {
	Name  string
	IsDir bool
	Open  func() (io.ReadCloser, error)
}

// inspectAndExtractRandomAccess handles streaming inspection and extraction for random-access archives (.zip, .7z).
func inspectAndExtractRandomAccess(archivePath, targetPattern, destDir string, entries []randomAccessEntry, closeArchive func() error) (bool, string, error) {
	var targetEntry *randomAccessEntry
	fileCount := 0

	for i := range entries {
		e := &entries[i]
		if e.IsDir || strings.HasSuffix(e.Name, "/") || strings.HasSuffix(e.Name, "\\") {
			continue
		}
		fileCount++
		if fileCount > 1 {
			if closeArchive != nil {
				_ = closeArchive()
			}
			return false, "", nil
		}
		targetEntry = e
	}

	var singleEntryName string
	if targetEntry != nil {
		singleEntryName = targetEntry.Name
	}

	ok, baseName, err := matchSinglePayload(singleEntryName, fileCount, targetPattern)
	if !ok || err != nil {
		if closeArchive != nil {
			_ = closeArchive()
		}
		return ok, "", err
	}

	rc, err := targetEntry.Open()
	if err != nil {
		if closeArchive != nil {
			_ = closeArchive()
		}
		return false, "", fmt.Errorf("failed to open archive entry: %w", err)
	}

	destPath, err := extractStream(rc, baseName, destDir)
	_ = rc.Close()

	if closeArchive != nil {
		_ = closeArchive()
	}

	return finalizeExtraction(archivePath, destPath, err)
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

// extractStream copies from an entry stream reader r to destDir, resolving filename collisions,
// bounded by MaxExtractEntrySize to protect against zip bombs.
func extractStream(r io.Reader, baseName, destDir string) (string, error) {
	return ExtractStreamBounded(r, baseName, destDir, MaxExtractEntrySize)
}

// ExtractStreamBounded copies from an entry stream reader r to destDir, resolving filename collisions,
// bounded by maxEntrySize to protect against archive/zip bombs.
func ExtractStreamBounded(r io.Reader, baseName, destDir string, maxEntrySize int64) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination dir: %w", err)
	}

	root, err := os.OpenRoot(destDir)
	if err != nil {
		return "", fmt.Errorf("failed to open root destination dir: %w", err)
	}
	defer root.Close()

	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	var outFile *os.File
	var candidate string
	for counter := 0; ; counter++ {
		if counter == 0 {
			candidate = baseName
		} else if ext != "" {
			candidate = fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		} else {
			candidate = fmt.Sprintf("%s_%d", nameWithoutExt, counter)
		}

		f, err := root.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			outFile = f
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("failed to create destination file: %w", err)
		}
	}
	destPath := filepath.Join(destDir, candidate)
	cleanup := func() {
		if outFile != nil {
			_ = outFile.Close()
			outFile = nil
		}
		_ = root.Remove(candidate)
	}

	limitedReader := io.LimitReader(r, maxEntrySize+1)
	n, err := io.Copy(outFile, limitedReader)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("failed to copy content: %w", err)
	}

	if n > maxEntrySize {
		cleanup()
		return "", fmt.Errorf("archive entry exceeded maximum allowed size (%d bytes)", maxEntrySize)
	}

	closeErr := outFile.Close()
	outFile = nil
	if closeErr != nil {
		_ = root.Remove(candidate)
		return "", fmt.Errorf("failed to close destination file: %w", closeErr)
	}

	return destPath, nil
}

