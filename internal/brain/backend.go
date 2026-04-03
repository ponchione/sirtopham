package brain

import "context"

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
