package archive

import (
	"fmt"
	"io"

	"github.com/bodgit/sevenzip"
)

// inspectAndExtract7z handles streaming inspection and extraction of 7z archives.
func inspectAndExtract7z(archivePath, targetPattern, destDir string) (bool, string, error) {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open 7z: %w", err)
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
