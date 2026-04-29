package tool

import (
	"encoding/json"
	"strings"

	"github.com/ponchione/sodoryard/internal/provider"
)

func newFileMutationDetails(fields map[string]any) json.RawMessage {
	return provider.NewToolResultDetails("file_mutation", fields)
}

func fileMutationDetailFields(operation, path string, created bool, changed bool, diff string, bytesBefore, bytesAfter int) map[string]any {
	return map[string]any{
		"operation":       operation,
		"path":            path,
		"created":         created,
		"changed":         changed,
		"diff_format":     "unified",
		"diff_line_count": detailLineCount(diff),
		"diff_truncated":  false,
		"bytes_before":    bytesBefore,
		"bytes_after":     bytesAfter,
	}
}

func detailLineCount(text string) int {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func firstChangedLine(content, needle string) int {
	if needle == "" {
		return 0
	}
	idx := strings.Index(content, needle)
	if idx < 0 {
		return 0
	}
	return strings.Count(content[:idx], "\n") + 1
}
