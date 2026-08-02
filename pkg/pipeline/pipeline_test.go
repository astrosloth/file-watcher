package pipeline

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"file-watcher/pkg/logging"
)

func createZipFile(t *testing.T, dir, filename string, files map[string]string) string {
	t.Helper()
	zipPath := filepath.Join(dir, filename)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	return zipPath
}

func TestPipelineFileMove(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_*")
	if err != nil {
		t.Fatalf("failed to create watch temp dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_*")
	if err != nil {
		t.Fatalf("failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	srcFile := filepath.Join(watchDir, "report.pdf")
	if err := os.WriteFile(srcFile, []byte("pdf report content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	logger := logging.NewConsoleLogger(nil)
	proc := BuildPipeline(
		destDir,
		"*.pdf",
		false,
		logger,
	)

	fi, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("failed to stat src file: %v", err)
	}

	ev := Event{
		Path:     srcFile,
		Basename: "report.pdf",
		FileInfo: fi,
	}

	if err := proc(context.Background(), ev); err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	destPath := filepath.Join(destDir, "report.pdf")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("file was not moved to destination: %s", destPath)
	}

	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("original source file still exists: %s", srcFile)
	}
}

func TestPipelineFilterNonMatching(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_*")
	if err != nil {
		t.Fatalf("failed to create watch temp dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_*")
	if err != nil {
		t.Fatalf("failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	srcFile := filepath.Join(watchDir, "data.txt")
	if err := os.WriteFile(srcFile, []byte("txt content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	logger := logging.NewConsoleLogger(nil)
	proc := BuildPipeline(
		destDir,
		"*.pdf",
		false,
		logger,
	)

	fi, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("failed to stat src file: %v", err)
	}

	ev := Event{
		Path:     srcFile,
		Basename: "data.txt",
		FileInfo: fi,
	}

	if err := proc(context.Background(), ev); err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Errorf("source file should not have been moved")
	}
}

func TestPipelineArchiveExtraction(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "watch_arc_*")
	if err != nil {
		t.Fatalf("failed to create watch temp dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	destDir, err := os.MkdirTemp("", "dest_arc_*")
	if err != nil {
		t.Fatalf("failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	zipPath := createZipFile(t, watchDir, "archive.zip", map[string]string{
		"extracted.pdf": "zip pdf content",
	})

	logger := logging.NewConsoleLogger(nil)
	proc := BuildPipeline(
		destDir,
		"*.pdf",
		true, // enable archive extraction
		logger,
	)

	fi, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("failed to stat zip: %v", err)
	}

	ev := Event{
		Path:     zipPath,
		Basename: "archive.zip",
		FileInfo: fi,
	}

	if err := proc(context.Background(), ev); err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	extractedFile := filepath.Join(destDir, "extracted.pdf")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Errorf("archive single file was not extracted: %s", extractedFile)
	}
}

func TestCopyAndRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copy_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := []byte("hello world copy test data")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	if err := copyAndRemove(src, dst); err != nil {
		t.Fatalf("copyAndRemove failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src file should be deleted")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("dst content = %q; want %q", string(got), string(content))
	}
}
