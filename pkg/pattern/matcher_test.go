package pattern_test

import (
	"testing"

	"file-watcher/pkg/pattern"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		pat      string
		filename string
		expected bool
	}{
		{
			name:     "simple glob match",
			pat:      "*.pdf",
			filename: "document.pdf",
			expected: true,
		},
		{
			name:     "simple glob no match",
			pat:      "*.pdf",
			filename: "document.txt",
			expected: false,
		},
		{
			name:     "brace expansion match 1",
			pat:      "*.{jpg,jpeg,png,gif,webp}",
			filename: "photo.jpg",
			expected: true,
		},
		{
			name:     "brace expansion match 2",
			pat:      "*.{jpg,jpeg,png,gif,webp}",
			filename: "photo.png",
			expected: true,
		},
		{
			name:     "brace expansion no match",
			pat:      "*.{jpg,jpeg,png,gif,webp}",
			filename: "photo.pdf",
			expected: false,
		},
		{
			name:     "prefix glob match",
			pat:      "report_*.csv",
			filename: "report_2026.csv",
			expected: true,
		},
		{
			name:     "prefix glob no match",
			pat:      "report_*.csv",
			filename: "summary_2026.csv",
			expected: false,
		},
		{
			name:     "exact match",
			pat:      "readme.txt",
			filename: "readme.txt",
			expected: true,
		},
		{
			name:     "nested braces expansion",
			pat:      "file_.{a,b}.{txt,doc}",
			filename: "file_.a.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pattern.Match(tt.pat, tt.filename)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("Match(%q, %q) = %v; want %v", tt.pat, tt.filename, got, tt.expected)
			}
		})
	}
}
