package pattern

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveTarget generates a unique filename if targetFilename already exists in destination
// by appending an incrementing integer suffix before the file extension (e.g. "doc_1.pdf").
func ResolveTarget(targetFilename string, exists func(string) bool) string {
	if !exists(targetFilename) {
		return targetFilename
	}

	ext := filepath.Ext(targetFilename)
	name := targetFilename
	if ext != "" {
		name = strings.TrimSuffix(targetFilename, ext)
	}

	counter := 1
	for {
		var candidate string
		if ext != "" {
			candidate = fmt.Sprintf("%s_%d%s", name, counter, ext)
		} else {
			candidate = fmt.Sprintf("%s_%d", name, counter)
		}

		if !exists(candidate) {
			return candidate
		}
		counter++
	}
}
