package codeintel

import "context"

// Parser extracts top-level declarations from source or document content.
//
// Implementations exist for Go (AST-based), TypeScript/TSX (tree-sitter),
// Python (tree-sitter), Markdown (heading splitter), and a fallback sliding
// window parser with 40-line windows and 20-line overlap.
type Parser interface {
	// Parse extracts top-level declarations from the given file content.
	// filePath is used for error messages and chunk metadata, not for reading
	// the file — content is passed directly.
	// Returns an empty slice (not nil) if no chunks are found.
	// Returns an error if parsing fails.
	Parse(filePath string, content []byte) ([]RawChunk, error)
}

// Store persists chunks, performs vector search, and supports metadata lookups.
type Store interface {
	// Upsert inserts or updates chunks in the vector store.
	// Implementation strategy is delete-by-ID then insert because LanceDB has
	// no native upsert support. Callers provide native Go slices on Chunk and
	// the store handles any serialization required by the backing schema.
	Upsert(ctx context.Context, chunks []Chunk) error

	// VectorSearch performs cosine similarity search against stored embeddings.
	// The zero-value Filter applies no metadata constraints.
	VectorSearch(ctx context.Context, queryEmbedding []float32, topK int, filter Filter) ([]SearchResult, error)

	// GetByFilePath returns all chunks stored for a given file path.
	GetByFilePath(ctx context.Context, filePath string) ([]Chunk, error)

	// GetByName returns all chunks matching a symbol name.
	GetByName(ctx context.Context, name string) ([]Chunk, error)

	// DeleteByFilePath removes all chunks associated with a file path.
	DeleteByFilePath(ctx context.Context, filePath string) error

	// Close releases store-held resources.
	Close() error
}
