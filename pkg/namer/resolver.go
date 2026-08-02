package namer

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ExistFunc checks if a given filename exists in the target location.
type ExistFunc func(filename string) bool

// ResolveTarget generates a unique filename if targetFilename already exists in destination.
func ResolveTarget(targetFilename string, exists ExistFunc) string {
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
