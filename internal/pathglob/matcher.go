package pathglob

import (
	"path/filepath"
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
)

func Match(pattern, relPath string) bool {
	pattern = normalize(pattern)
	relPath = normalize(relPath)
	if pattern == "" || relPath == "" {
		return false
	}
	matched, err := doublestar.Match(pattern, relPath)
	if err != nil {
		return false
	}
	return matched
}

func MatchAny(patterns []string, relPath string) bool {
	for _, pattern := range patterns {
		if Match(pattern, relPath) {
			return true
		}
	}
	return false
}

func normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		return ""
	}
	return value
}
