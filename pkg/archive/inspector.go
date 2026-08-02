package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"file-watcher/pkg/namer"
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

type ArchiveEntry struct {
	Name   string
	IsFile bool
	Open   func() (io.ReadCloser, error)
}

// InspectAndExtractSingleFile checks if archive contains exactly one file matching targetPattern.
// If matched, extracts it to destDir (handling duplicate filenames) and removes the archive file.
func InspectAndExtractSingleFile(archivePath, targetPattern, destDir string) (bool, string, error) {
	entries, err := listArchiveEntries(archivePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to list archive entries: %w", err)
	}

	var files []ArchiveEntry
	for _, entry := range entries {
		if entry.IsFile {
			files = append(files, entry)
		}
	}

	if len(files) != 1 {
		// Needs to contain exactly one file
		return false, "", nil
	}

	singleFile := files[0]
	baseName := filepath.Base(singleFile.Name)

	matched, err := pattern.Match(targetPattern, baseName)
	if err != nil || !matched {
		return false, "", err
	}

	// Read content
	reader, err := singleFile.Open()
	if err != nil {
		return false, "", fmt.Errorf("failed to open entry reader: %w", err)
	}
	defer reader.Close()

	// Resolve destination file path
	targetName := namer.ResolveTarget(baseName, func(name string) bool {
		_, err := os.Stat(filepath.Join(destDir, name))
		return err == nil
	})

	destPath := filepath.Join(destDir, targetName)

	// Ensure target directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return false, "", fmt.Errorf("failed to create destination dir: %w", err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return false, "", fmt.Errorf("failed to copy content: %w", err)
	}

	// Close outfile before removing archive
	_ = outFile.Close()

	// Remove original archive file
	if err := os.Remove(archivePath); err != nil {
		// Log warning or return error if remove fails
	}

	return true, destPath, nil
}

func listArchiveEntries(archivePath string) ([]ArchiveEntry, error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return listZipEntries(archivePath)
	}
	return listTarEntries(archivePath)
}

func listZipEntries(archivePath string) ([]ArchiveEntry, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}

	var entries []ArchiveEntry
	for _, f := range r.File {
		fInfo := f.FileInfo()
		if fInfo.IsDir() || strings.HasSuffix(f.Name, "/") {
			continue
		}
		fileObj := f
		entries = append(entries, ArchiveEntry{
			Name:   f.Name,
			IsFile: true,
			Open: func() (io.ReadCloser, error) {
				return fileObj.Open()
			},
		})
	}
	// Note: We close Zip reader after reading entries, so for Zip we buffer entry bytes or re-open
	// To be safe for zip reader lifetime:
	var buffered []ArchiveEntry
	for _, e := range entries {
		rc, err := e.Open()
		if err != nil {
			r.Close()
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			r.Close()
			return nil, err
		}
		name := e.Name
		buffered = append(buffered, ArchiveEntry{
			Name:   name,
			IsFile: true,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		})
	}
	r.Close()
	return buffered, nil
}

func listTarEntries(archivePath string) ([]ArchiveEntry, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file
	lower := strings.ToLower(archivePath)

	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		reader = gzr
	} else if strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz") || strings.HasSuffix(lower, ".tbz2") {
		reader = bzip2.NewReader(file)
	} else if strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
		xzr, err := xz.NewReader(file)
		if err != nil {
			return nil, err
		}
		reader = xzr
	}

	tr := tar.NewReader(reader)
	var entries []ArchiveEntry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") {
			continue
		}

		// Read content into memory for single file
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, err
		}

		data := buf.Bytes()
		name := hdr.Name
		entries = append(entries, ArchiveEntry{
			Name:   name,
			IsFile: true,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		})
	}

	return entries, nil
}
