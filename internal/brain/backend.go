package brain

import (
	"context"
	"path/filepath"
	"strings"
)

// SearchHit is a keyword search result returned by a brain backend.
type SearchHit struct {
	Path    string  `json:"path"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// Backend defines the operations brain tools need from their backing store.
type Backend interface {
	ReadDocument(ctx context.Context, path string) (string, error)
	WriteDocument(ctx context.Context, path string, content string) error
	PatchDocument(ctx context.Context, path string, operation string, content string) error
	SearchKeyword(ctx context.Context, query string) ([]SearchHit, error)
	ListDocuments(ctx context.Context, directory string) ([]string, error)
}

// IsOperationalDocument returns true for brain documents that are operational
// bookkeeping (e.g. _log.md) rather than real knowledge notes. These should be
// excluded from proactive retrieval and search results.
func IsOperationalDocument(path string) bool {
	cleaned := strings.Trim(filepath.ToSlash(strings.TrimSpace(path)), "/")
	if cleaned == "" {
		return false
	}
	return filepath.Base(cleaned) == "_log.md"
}
