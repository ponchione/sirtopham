package codeintel

import (
	"context"
	"testing"
)

type parserStub struct{}

type storeStub struct{}

func (parserStub) Parse(filePath string, content []byte) ([]RawChunk, error) {
	return []RawChunk{}, nil
}

func (storeStub) Upsert(ctx context.Context, chunks []Chunk) error {
	return nil
}

func (storeStub) VectorSearch(ctx context.Context, queryEmbedding []float32, topK int, filter Filter) ([]SearchResult, error) {
	return []SearchResult{}, nil
}

func (storeStub) GetByFilePath(ctx context.Context, filePath string) ([]Chunk, error) {
	return []Chunk{}, nil
}

func (storeStub) GetByName(ctx context.Context, name string) ([]Chunk, error) {
	return []Chunk{}, nil
}

func (storeStub) DeleteByFilePath(ctx context.Context, filePath string) error {
	return nil
}

func (storeStub) Close() error {
	return nil
}

func TestParserInterfaceIsSatisfied(t *testing.T) {
	var parser Parser = parserStub{}

	chunks, err := parser.Parse("main.go", []byte("package main"))
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if chunks == nil {
		t.Fatal("Parse returned nil slice, want empty slice")
	}
}

func TestStoreInterfaceIsSatisfied(t *testing.T) {
	var store Store = storeStub{}
	ctx := context.Background()

	if err := store.Upsert(ctx, []Chunk{{ID: "chunk-id"}}); err != nil {
		t.Fatalf("Upsert returned unexpected error: %v", err)
	}

	results, err := store.VectorSearch(ctx, []float32{0.1, 0.2}, 5, Filter{Language: "go"})
	if err != nil {
		t.Fatalf("VectorSearch returned unexpected error: %v", err)
	}
	if results == nil {
		t.Fatal("VectorSearch returned nil slice, want empty slice")
	}

	chunksByPath, err := store.GetByFilePath(ctx, "main.go")
	if err != nil {
		t.Fatalf("GetByFilePath returned unexpected error: %v", err)
	}
	if chunksByPath == nil {
		t.Fatal("GetByFilePath returned nil slice, want empty slice")
	}

	chunksByName, err := store.GetByName(ctx, "main")
	if err != nil {
		t.Fatalf("GetByName returned unexpected error: %v", err)
	}
	if chunksByName == nil {
		t.Fatal("GetByName returned nil slice, want empty slice")
	}

	if err := store.DeleteByFilePath(ctx, "main.go"); err != nil {
		t.Fatalf("DeleteByFilePath returned unexpected error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}
}
