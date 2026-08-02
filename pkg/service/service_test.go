package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	customConfigPath := filepath.Join(tempDir, "custom.conf")

	path, err := EnsureDefaultConfig(customConfigPath)
	if err != nil {
		t.Fatalf("EnsureDefaultConfig failed: %v", err)
	}

	if path != customConfigPath {
		t.Errorf("expected path %s, got %s", customConfigPath, path)
	}

	content, err := os.ReadFile(customConfigPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("created config content is empty")
	}

	// Calling again should not overwrite existing file
	if err := os.WriteFile(customConfigPath, []byte("custom content"), 0644); err != nil {
		t.Fatalf("failed to overwrite config file: %v", err)
	}

	_, err = EnsureDefaultConfig(customConfigPath)
	if err != nil {
		t.Fatalf("second EnsureDefaultConfig call failed: %v", err)
	}

	updatedContent, _ := os.ReadFile(customConfigPath)
	if string(updatedContent) != "custom content" {
		t.Errorf("expected existing file to be preserved, but got: %s", string(updatedContent))
	}
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "src.txt")
	dstPath := filepath.Join(tempDir, "sub", "dst.txt")

	if err := os.WriteFile(srcPath, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}

	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(content))
	}
}
