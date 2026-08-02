// Package archive provides functionality for inspecting archive files (.zip, .tar, .tar.gz, .tar.bz2, .tar.xz)
// and extracting single-file payloads matching target pattern specifications.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"file-watcher/pkg/pattern"
	"github.com/ulikunitz/xz"
)

// IsArchive checks if the file path has a supported archive extension.
func IsArchive(path string) bool {
	lower := strings.ToLower(path)
	extensions := []string{
		".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz", ".tbz2", ".tar.xz", ".txz",
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
	return inspectAndExtractTar(archivePath, targetPattern, destDir)
}

// inspectAndExtractZip handles streaming inspection and extraction of zip archives.
func inspectAndExtractZip(archivePath, targetPattern, destDir string) (bool, string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open zip: %w", err)
	}

	var targetFile *zip.File
	fileCount := 0

	for _, f := range r.File {
		fInfo := f.FileInfo()
		if fInfo.IsDir() || strings.HasSuffix(f.Name, "/") {
			continue
		}
		fileCount++
		if fileCount > 1 {
			_ = r.Close()
			return false, "", nil
		}
		targetFile = f
	}

	if fileCount != 1 || targetFile == nil {
		_ = r.Close()
		return false, "", nil
	}

	baseName := filepath.Base(targetFile.Name)
	matched, err := pattern.Match(targetPattern, baseName)
	if err != nil || !matched {
		_ = r.Close()
		return false, "", err
	}

	rc, err := targetFile.Open()
	if err != nil {
		_ = r.Close()
		return false, "", fmt.Errorf("failed to open zip entry: %w", err)
	}

	destPath, err := extractStream(rc, baseName, destDir)
	_ = rc.Close()
	_ = r.Close()

	if err != nil {
		return false, "", err
	}

	_ = os.Remove(archivePath)
	return true, destPath, nil
}

// inspectAndExtractTar handles two-pass streaming inspection and extraction of tar-based archives (.tar, .tar.gz, .tar.bz2, .tar.xz).
func inspectAndExtractTar(archivePath, targetPattern, destDir string) (bool, string, error) {
	// Pass 1: Count files and check for single file match
	file, err := os.Open(archivePath)
	if err != nil {
		return false, "", err
	}

	decomp, err := openDecompressor(file, archivePath)
	if err != nil {
		_ = file.Close()
		return false, "", err
	}

	tr := tar.NewReader(decomp)
	var singleEntryName string
	fileCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			closeDecompressor(decomp)
			_ = file.Close()
			return false, "", err
		}

		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") {
			continue
		}

		fileCount++
		if fileCount > 1 {
			break
		}
		singleEntryName = hdr.Name
	}

	closeDecompressor(decomp)
	_ = file.Close()

	if fileCount != 1 {
		return false, "", nil
	}

	baseName := filepath.Base(singleEntryName)
	matched, err := pattern.Match(targetPattern, baseName)
	if err != nil || !matched {
		return false, "", err
	}

	// Pass 2: Extract the single target entry
	file2, err := os.Open(archivePath)
	if err != nil {
		return false, "", err
	}

	decomp2, err := openDecompressor(file2, archivePath)
	if err != nil {
		_ = file2.Close()
		return false, "", err
	}

	tr2 := tar.NewReader(decomp2)
	var destPath string
	var extractErr error

	for {
		hdr, err := tr2.Next()
		if err != nil {
			extractErr = err
			break
		}
		if hdr.Name == singleEntryName {
			destPath, extractErr = extractStream(tr2, baseName, destDir)
			break
		}
	}

	closeDecompressor(decomp2)
	_ = file2.Close()

	if extractErr != nil {
		return false, "", extractErr
	}

	_ = os.Remove(archivePath)
	return true, destPath, nil
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

// openDecompressor wraps r in a decompression reader (gzip, bzip2, xz) based on the archive file extension.
func openDecompressor(r io.Reader, path string) (io.Reader, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return gzip.NewReader(r)
	}
	if strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz") || strings.HasSuffix(lower, ".tbz2") {
		return bzip2.NewReader(r), nil
	}
	if strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
		return xz.NewReader(r)
	}
	return r, nil
}

// closeDecompressor closes r if it implements io.Closer.
func closeDecompressor(r io.Reader) {
	if cl, ok := r.(io.Closer); ok {
		_ = cl.Close()
	}
}
