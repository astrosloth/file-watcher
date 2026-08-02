// Package pattern provides string pattern matching algorithms and glob brace expansion utilities.
package pattern

import (
	"path/filepath"
	"strings"
)

// ExpandBraces expands a pattern containing brace expressions like "*.{jpg,png}" into a slice of patterns.
// For example: "*.{jpg,png}" -> ["*.jpg", "*.png"]. Supports recursive nested brace expressions.
func ExpandBraces(pat string) []string {
	start := strings.IndexByte(pat, '{')
	if start == -1 {
		return []string{pat}
	}

	end := strings.IndexByte(pat[start:], '}')
	if end == -1 {
		return []string{pat}
	}
	end += start

	prefix := pat[:start]
	suffix := pat[end+1:]
	options := strings.Split(pat[start+1:end], ",")

	var results []string
	for _, opt := range options {
		expanded := prefix + opt + suffix
		// Recursively expand any remaining braces
		results = append(results, ExpandBraces(expanded)...)
	}

	return results
}

// Match checks whether a filename matches a glob pattern, supporting brace expansion (e.g. "*.{jpg,png}").
func Match(pattern, filename string) (bool, error) {
	subPatterns := ExpandBraces(pattern)
	for _, pat := range subPatterns {
		matched, err := filepath.Match(pat, filename)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
