package pattern_test

import (
	"testing"

	"file-watcher/pkg/pattern"
)

func TestResolveTarget(t *testing.T) {
	existing := map[string]bool{
		"file.txt":   true,
		"file_1.txt": true,
	}

	got := pattern.ResolveTarget("file.txt", func(name string) bool {
		return existing[name]
	})

	if got != "file_2.txt" {
		t.Errorf("ResolveTarget = %q; want %q", got, "file_2.txt")
	}
}
