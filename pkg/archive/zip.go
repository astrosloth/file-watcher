package archive

import (
	"archive/zip"
	"fmt"
	"strings"
)

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

	var singleEntryName string
	if targetFile != nil {
		singleEntryName = targetFile.Name
	}

	ok, baseName, err := matchSinglePayload(singleEntryName, fileCount, targetPattern)
	if !ok || err != nil {
		_ = r.Close()
		return ok, "", err
	}

	rc, err := targetFile.Open()
	if err != nil {
		_ = r.Close()
		return false, "", fmt.Errorf("failed to open zip entry: %w", err)
	}

	destPath, err := extractStream(rc, baseName, destDir)
	_ = rc.Close()
	_ = r.Close()

	return finalizeExtraction(archivePath, destPath, err)
}
