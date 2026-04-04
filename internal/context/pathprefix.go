package context

import (
	"path"
	"strings"
)

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	common := splitNonEmptyPath(paths[0])
	for _, value := range paths[1:] {
		common = sharedPrefix(common, splitNonEmptyPath(value))
		if len(common) == 0 {
			return ""
		}
	}
	return joinPathParts(common)
}

func splitNonEmptyPath(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func joinPathParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return path.Clean(path.Join(parts...))
}

func sharedPrefix(left []string, right []string) []string {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	prefix := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			break
		}
		prefix = append(prefix, left[i])
	}
	return prefix
}
