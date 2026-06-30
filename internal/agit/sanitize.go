package agit

import (
	"path/filepath"
	"strings"
)

// sanitizeAssertionPath validates and cleans a file path for use in assertions.
// Returns the cleaned path and true if safe, or empty string and false if unsafe.
func sanitizeAssertionPath(path string) (string, bool) {
	clean := filepath.Clean(path)
	// Reject absolute paths and traversal
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", false
	}
	// Reject paths that still contain traversal segments after cleaning
	if strings.Contains(clean, "..") {
		return "", false
	}
	// Reject null bytes
	if strings.ContainsRune(clean, 0) {
		return "", false
	}
	return clean, true
}
