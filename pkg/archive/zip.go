package archive

import (
	"archive/zip"
	"fmt"
	"io"
)

// inspectAndExtractZip handles streaming inspection and extraction of zip archives.
func inspectAndExtractZip(archivePath, targetPattern, destDir string) (bool, string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open zip: %w", err)
	}

	entries := make([]randomAccessEntry, len(r.File))
	for i, f := range r.File {
		f := f
		entries[i] = randomAccessEntry{
			Name:  f.Name,
			IsDir: f.FileInfo().IsDir(),
			Open:  func() (io.ReadCloser, error) { return f.Open() },
		}
	}

	return inspectAndExtractRandomAccess(archivePath, targetPattern, destDir, entries, r.Close)
}
