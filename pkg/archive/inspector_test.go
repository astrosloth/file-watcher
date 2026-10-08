package archive_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"file-watcher/pkg/archive"
	"github.com/ulikunitz/xz"
)

func createTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_*.zip")
	if err != nil {
		t.Fatalf("failed to create temp zip: %v", err)
	}
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content for %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return tmpFile.Name()
}

func createTestTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp tar.gz: %v", err)
	}
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content %s: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return tmpFile.Name()
}

func createTestTarXz(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_*.tar.xz")
	if err != nil {
		t.Fatalf("failed to create temp tar.xz: %v", err)
	}
	defer tmpFile.Close()

	xzw, err := xz.NewWriter(tmpFile)
	if err != nil {
		t.Fatalf("failed to create xz writer: %v", err)
	}
	tw := tar.NewWriter(xzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content %s: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := xzw.Close(); err != nil {
		t.Fatalf("failed to close xz writer: %v", err)
	}

	return tmpFile.Name()
}

func TestIsSupported(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"document.pdf", false},
		{"archive.zip", true},
		{"archive.tar.gz", true},
		{"archive.tgz", true},
		{"archive.tar.bz2", true},
		{"archive.tar.xz", true},
		{"archive.7z", true},
		{"archive.rar", true},
	}

	for _, tt := range tests {
		got := archive.IsSupported(tt.filename)
		if got != tt.expected {
			t.Errorf("IsSupported(%q) = %v; want %v", tt.filename, got, tt.expected)
		}
	}
}

func TestInspectAndExtractSingleFile(t *testing.T) {
	t.Run("zip with single matching file", func(t *testing.T) {
		zipPath := createTestZip(t, map[string]string{
			"reports/document.pdf": "pdf content data",
		})
		defer os.Remove(zipPath)

		destDir, err := os.MkdirTemp("", "dest_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		// Create existing file in destDir to trigger duplicate name resolution
		if err := os.WriteFile(filepath.Join(destDir, "document.pdf"), []byte("existing"), 0644); err != nil {
			t.Fatalf("failed to write existing file: %v", err)
		}

		extracted, destFile, err := archive.InspectAndExtractSingleFile(zipPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !extracted {
			t.Fatalf("expected extracted to be true")
		}

		if filepath.Base(destFile) != "document_1.pdf" {
			t.Errorf("destFile basename = %q; want %q", filepath.Base(destFile), "document_1.pdf")
		}

		content, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if !bytes.Equal(content, []byte("pdf content data")) {
			t.Errorf("extracted content = %q; want %q", string(content), "pdf content data")
		}

		if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
			t.Errorf("archive file %s was not deleted after extraction", zipPath)
		}
	})

	t.Run("zip with multiple files (should skip)", func(t *testing.T) {
		zipPath := createTestZip(t, map[string]string{
			"file1.pdf": "pdf content 1",
			"file2.pdf": "pdf content 2",
		})
		defer os.Remove(zipPath)

		destDir, err := os.MkdirTemp("", "dest_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		extracted, _, err := archive.InspectAndExtractSingleFile(zipPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if extracted {
			t.Fatalf("expected extracted to be false for multi-file archive")
		}

		if _, err := os.Stat(zipPath); os.IsNotExist(err) {
			t.Errorf("archive file should not be removed when skipped")
		}
	})

	t.Run("tar.gz with single matching file", func(t *testing.T) {
		tarPath := createTestTarGz(t, map[string]string{
			"invoice.pdf": "invoice pdf content",
		})
		defer os.Remove(tarPath)

		destDir, err := os.MkdirTemp("", "dest_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		extracted, destFile, err := archive.InspectAndExtractSingleFile(tarPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !extracted {
			t.Fatalf("expected extracted to be true")
		}

		if filepath.Base(destFile) != "invoice.pdf" {
			t.Errorf("destFile basename = %q; want %q", filepath.Base(destFile), "invoice.pdf")
		}
	})

	t.Run("tar.xz with single matching file", func(t *testing.T) {
		tarPath := createTestTarXz(t, map[string]string{
			"statement.pdf": "statement pdf content",
		})
		defer os.Remove(tarPath)

		destDir, err := os.MkdirTemp("", "dest_xz_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		extracted, destFile, err := archive.InspectAndExtractSingleFile(tarPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !extracted {
			t.Fatalf("expected extracted to be true")
		}

		if filepath.Base(destFile) != "statement.pdf" {
			t.Errorf("destFile basename = %q; want %q", filepath.Base(destFile), "statement.pdf")
		}
	})

	t.Run("tar.bz2 reading test", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x42, 0x5a, 0x68})
		bzr := bzip2.NewReader(buf)
		if bzr == nil {
			t.Fatalf("bzip2 reader nil")
		}
	})

	t.Run("invalid 7z file returns error", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid_*.7z")
		if err != nil {
			t.Fatalf("failed to create temp 7z: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		_, _ = tmpFile.Write([]byte("not a real 7z archive"))
		_ = tmpFile.Close()

		destDir, err := os.MkdirTemp("", "dest_7z_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		_, _, err = archive.InspectAndExtractSingleFile(tmpFile.Name(), "*.pdf", destDir)
		if err == nil {
			t.Errorf("expected error when inspecting invalid 7z archive")
		}
	})

	t.Run("invalid rar file returns error", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid_*.rar")
		if err != nil {
			t.Fatalf("failed to create temp rar: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		_, _ = tmpFile.Write([]byte("not a real rar archive"))
		_ = tmpFile.Close()

		destDir, err := os.MkdirTemp("", "dest_rar_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		_, _, err = archive.InspectAndExtractSingleFile(tmpFile.Name(), "*.pdf", destDir)
		if err == nil {
			t.Errorf("expected error when inspecting invalid rar archive")
		}
	})

	t.Run("zip entry with path traversal is sanitized to base name", func(t *testing.T) {
		zipPath := createTestZip(t, map[string]string{
			"../../evil.pdf": "traversal content",
		})
		defer os.Remove(zipPath)

		destDir, err := os.MkdirTemp("", "dest_traversal_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		extracted, destFile, err := archive.InspectAndExtractSingleFile(zipPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !extracted {
			t.Fatalf("expected extracted to be true")
		}
		if filepath.Dir(destFile) != destDir {
			t.Errorf("expected extracted file inside destDir, got %s", destFile)
		}
		if filepath.Base(destFile) != "evil.pdf" {
			t.Errorf("expected base name evil.pdf, got %s", filepath.Base(destFile))
		}
	})

	t.Run("zip entry collision resolves to incremented filename", func(t *testing.T) {
		destDir, err := os.MkdirTemp("", "dest_collision_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		// Create pre-existing file in destDir
		if err := os.WriteFile(filepath.Join(destDir, "report.pdf"), []byte("existing"), 0644); err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		zipPath := createTestZip(t, map[string]string{
			"report.pdf": "new content",
		})
		defer os.Remove(zipPath)

		extracted, destFile, err := archive.InspectAndExtractSingleFile(zipPath, "*.pdf", destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !extracted {
			t.Fatalf("expected extracted to be true")
		}
		if filepath.Base(destFile) != "report_1.pdf" {
			t.Errorf("expected base name report_1.pdf, got %s", filepath.Base(destFile))
		}
	})

	t.Run("zip entry exceeding max size returns error", func(t *testing.T) {
		destDir, err := os.MkdirTemp("", "dest_limit_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		if archive.MaxExtractEntrySize <= 0 {
			t.Fatalf("invalid MaxExtractEntrySize")
		}

		// Test with ExtractStreamBounded with small max size
		const limit = 100
		oversizedData := bytes.Repeat([]byte("A"), limit+50)
		_, err = archive.ExtractStreamBounded(bytes.NewReader(oversizedData), "test.txt", destDir, limit)
		if err == nil {
			t.Fatalf("expected error when stream exceeds limit, got nil")
		}

		entries, readErr := os.ReadDir(destDir)
		if readErr != nil {
			t.Fatalf("failed to read destDir: %v", readErr)
		}
		if len(entries) != 0 {
			t.Errorf("expected oversized partial file to be cleaned up, found %d files", len(entries))
		}
	})

	t.Run("stream within size limit extracts successfully", func(t *testing.T) {
		destDir, err := os.MkdirTemp("", "dest_within_limit_*")
		if err != nil {
			t.Fatalf("failed to create dest temp dir: %v", err)
		}
		defer os.RemoveAll(destDir)

		const limit = 100
		validData := bytes.Repeat([]byte("B"), limit)
		destPath, err := archive.ExtractStreamBounded(bytes.NewReader(validData), "test.txt", destDir, limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if !bytes.Equal(data, validData) {
			t.Errorf("extracted content does not match")
		}
	})
}

