package archive

import (
	"fmt"
	"strings"

	"github.com/bodgit/sevenzip"
)

// inspectAndExtract7z handles streaming inspection and extraction of 7z archives.
func inspectAndExtract7z(archivePath, targetPattern, destDir string) (bool, string, error) {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open 7z: %w", err)
	}
	defer r.Close()

	var targetFile *sevenzip.File
	fileCount := 0

	for _, f := range r.File {
		fInfo := f.FileInfo()
		if fInfo.IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, "\\") {
			continue
		}
		fileCount++
		if fileCount > 1 {
			return false, "", nil
		}
		targetFile = f
	}

	var singleEntryName string
	if targetFile != nil {
		singleEntryName = targetFile.Name
	}

	ok, baseName, err := matchSinglePayload(singleEntryName, fileCount, targetPattern)
	if !ok || err != nil {
		return ok, "", err
	}

	rc, err := targetFile.Open()
	if err != nil {
		return false, "", fmt.Errorf("failed to open 7z entry: %w", err)
	}

	destPath, err := extractStream(rc, baseName, destDir)
	_ = rc.Close()

	return finalizeExtraction(archivePath, destPath, err)
}
