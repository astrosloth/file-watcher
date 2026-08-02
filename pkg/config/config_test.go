package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"file-watcher/pkg/config"
)

func TestParseConfig(t *testing.T) {
	configContent := `
# Sample File Watcher Config

[watch:pdfs]
dir = /tmp/watch_downloads
pattern = *.pdf
dest = /tmp/watch_pdfs
extract-archives = true

[watch:images]
dir = /tmp/watch_downloads
pattern = *.{jpg,png}
dest = /tmp/watch_images
poll-interval = 10
use-polling = true
`
	tmpFile, err := os.CreateTemp("", "test_config_*.conf")
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	defer os.RemoveAll(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.Watches) != 2 {
		t.Fatalf("expected 2 watches, got %d", len(cfg.Watches))
	}

	pdfWatch, ok := cfg.Watches["pdfs"]
	if !ok {
		t.Fatalf("watch 'pdfs' not found")
	}
	if pdfWatch.Name != "pdfs" {
		t.Errorf("Name = %q; want %q", pdfWatch.Name, "pdfs")
	}
	if pdfWatch.Dir != filepath.Clean("/tmp/watch_downloads") {
		t.Errorf("Dir = %q; want %q", pdfWatch.Dir, filepath.Clean("/tmp/watch_downloads"))
	}
	if pdfWatch.Pattern != "*.pdf" {
		t.Errorf("Pattern = %q; want %q", pdfWatch.Pattern, "*.pdf")
	}
	if pdfWatch.Dest != filepath.Clean("/tmp/watch_pdfs") {
		t.Errorf("Dest = %q; want %q", pdfWatch.Dest, filepath.Clean("/tmp/watch_pdfs"))
	}
	if !pdfWatch.ExtractArchives {
		t.Errorf("ExtractArchives = false; want true")
	}
	if pdfWatch.PollInterval != 5 {
		t.Errorf("PollInterval = %d; want 5 (default)", pdfWatch.PollInterval)
	}
	if pdfWatch.UsePolling {
		t.Errorf("UsePolling = true; want false")
	}

	imgWatch, ok := cfg.Watches["images"]
	if !ok {
		t.Fatalf("watch 'images' not found")
	}
	if imgWatch.PollInterval != 10 {
		t.Errorf("PollInterval = %d; want 10", imgWatch.PollInterval)
	}
	if !imgWatch.UsePolling {
		t.Errorf("UsePolling = false; want true")
	}
}

func TestInvalidConfig(t *testing.T) {
	invalidContent := `
[watch:invalid]
dir = /tmp/watch
pattern = *.pdf
`
	tmpFile, err := os.CreateTemp("", "invalid_*.conf")
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, _ = tmpFile.WriteString(invalidContent)
	tmpFile.Close()

	_, err = config.LoadConfig(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for missing dest field, got nil")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping home dir test: no home directory found")
	}

	expanded := config.ExpandPath("~/Documents")
	expected := filepath.Join(home, "Documents")
	if expanded != expected {
		t.Errorf("ExpandPath(~/Documents) = %q; want %q", expanded, expected)
	}

	expandedWin := config.ExpandPath("~\\Documents")
	if expandedWin != expected {
		t.Errorf("ExpandPath(~\\Documents) = %q; want %q", expandedWin, expected)
	}

	t.Setenv("TEST_VAR", "my_folder")
	expandedEnv := config.ExpandPath("%TEST_VAR%/sub")
	expectedEnv := filepath.Clean("my_folder/sub")
	if expandedEnv != expectedEnv {
		t.Errorf("ExpandPath(%%TEST_VAR%%/sub) = %q; want %q", expandedEnv, expectedEnv)
	}

	// Undefined env var should remain unexpanded
	expandedUnset := config.ExpandPath("%NONEXISTENT_VAR_999%/sub")
	expectedUnset := filepath.Clean("%NONEXISTENT_VAR_999%/sub")
	if expandedUnset != expectedUnset {
		t.Errorf("ExpandPath(%%NONEXISTENT_VAR_999%%/sub) = %q; want %q", expandedUnset, expectedUnset)
	}

	// Self-referencing env var should expand in a single pass without infinite looping
	t.Setenv("RECURSIVE_VAR", "%RECURSIVE_VAR%")
	expandedRec := config.ExpandPath("%RECURSIVE_VAR%")
	if expandedRec != "%RECURSIVE_VAR%" {
		t.Errorf("ExpandPath(%%RECURSIVE_VAR%%) = %q; want %%RECURSIVE_VAR%%", expandedRec)
	}

	// Multiple env vars in path
	t.Setenv("VAR_A", "path_a")
	t.Setenv("VAR_B", "path_b")
	expandedMulti := config.ExpandPath("%VAR_A%/%VAR_B%")
	expectedMulti := filepath.Clean("path_a/path_b")
	if expandedMulti != expectedMulti {
		t.Errorf("ExpandPath(%%VAR_A%%/%%VAR_B%%) = %q; want %q", expandedMulti, expectedMulti)
	}

	// Unmatched % sign
	expandedUnmatched := config.ExpandPath("100%_done")
	expectedUnmatched := filepath.Clean("100%_done")
	if expandedUnmatched != expectedUnmatched {
		t.Errorf("ExpandPath(100%%_done) = %q; want %q", expandedUnmatched, expectedUnmatched)
	}
}
