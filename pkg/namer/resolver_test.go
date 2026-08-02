package namer_test

import (
	"testing"

	"file-watcher/pkg/namer"
)

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name           string
		targetFilename string
		existingFiles  map[string]bool
		expected       string
	}{
		{
			name:           "no collision",
			targetFilename: "document.pdf",
			existingFiles:  map[string]bool{},
			expected:       "document.pdf",
		},
		{
			name:           "single collision",
			targetFilename: "document.pdf",
			existingFiles: map[string]bool{
				"document.pdf": true,
			},
			expected: "document_1.pdf",
		},
		{
			name:           "multiple collisions",
			targetFilename: "document.pdf",
			existingFiles: map[string]bool{
				"document.pdf":   true,
				"document_1.pdf": true,
				"document_2.pdf": true,
			},
			expected: "document_3.pdf",
		},
		{
			name:           "filename with double extension",
			targetFilename: "archive.tar.gz",
			existingFiles: map[string]bool{
				"archive.tar.gz": true,
			},
			expected: "archive.tar_1.gz",
		},
		{
			name:           "filename without extension",
			targetFilename: "Dockerfile",
			existingFiles: map[string]bool{
				"Dockerfile": true,
			},
			expected: "Dockerfile_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existsFn := func(name string) bool {
				return tt.existingFiles[name]
			}
			got := namer.ResolveTarget(tt.targetFilename, existsFn)
			if got != tt.expected {
				t.Errorf("ResolveTarget(%q) = %q; want %q", tt.targetFilename, got, tt.expected)
			}
		})
	}
}
