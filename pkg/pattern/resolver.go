package pattern

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CandidateName formats a filename with an optional collision suffix (e.g. index 0 -> "doc.pdf", index 1 -> "doc_1.pdf").
func CandidateName(targetFilename string, index int) string {
	if index <= 0 {
		return targetFilename
	}
	ext := filepath.Ext(targetFilename)
	name := targetFilename
	if ext != "" {
		name = strings.TrimSuffix(targetFilename, ext)
	}
	if ext != "" {
		return fmt.Sprintf("%s_%d%s", name, index, ext)
	}
	return fmt.Sprintf("%s_%d", name, index)
}

// ResolveTarget generates a unique filename if targetFilename already exists in destination
// by appending an incrementing integer suffix before the file extension (e.g. "doc_1.pdf").
func ResolveTarget(targetFilename string, exists func(string) bool) string {
	for counter := 0; ; counter++ {
		candidate := CandidateName(targetFilename, counter)
		if !exists(candidate) {
			return candidate
		}
	}
}
