package codeintel

import (
	"context"
	"testing"
)

type parserStub struct{}

type storeStub struct{}

type embedderStub struct{}

type describerStub struct{}

type searcherStub struct{}

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

func (embedderStub) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2}}, nil
}

func (embedderStub) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return []float32{0.3, 0.4}, nil
}

func (describerStub) DescribeFile(ctx context.Context, fileContent string, relationshipContext string) ([]Description, error) {
	return []Description{{Name: "ValidateToken", Description: "Validates a token."}}, nil
}

func (searcherStub) Search(ctx context.Context, queries []string, opts SearchOptions) ([]SearchResult, error) {
	return []SearchResult{{
		Chunk: Chunk{ID: "chunk-id", Name: "ValidateToken", ChunkType: ChunkTypeFunction},
		Score: 0.9,
	}}, nil
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

func TestEmbedderInterfaceIsSatisfied(t *testing.T) {
	var embedder Embedder = embedderStub{}
	ctx := context.Background()

	vectors, err := embedder.EmbedTexts(ctx, []string{"func ValidateToken() error\nValidates a token."})
	if err != nil {
		t.Fatalf("EmbedTexts returned unexpected error: %v", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 2 {
		t.Fatalf("EmbedTexts returned %#v, want one embedding vector", vectors)
	}

	queryVector, err := embedder.EmbedQuery(ctx, "auth middleware")
	if err != nil {
		t.Fatalf("EmbedQuery returned unexpected error: %v", err)
	}
	if len(queryVector) != 2 {
		t.Fatalf("EmbedQuery returned %#v, want one embedding vector", queryVector)
	}
}

func TestDescriberInterfaceIsSatisfied(t *testing.T) {
	var describer Describer = describerStub{}
	ctx := context.Background()

	descriptions, err := describer.DescribeFile(ctx, "func ValidateToken() error { return nil }", "calls: auth.ParseToken")
	if err != nil {
		t.Fatalf("DescribeFile returned unexpected error: %v", err)
	}
	if len(descriptions) != 1 {
		t.Fatalf("DescribeFile returned %#v, want one description", descriptions)
	}
	if descriptions[0].Name != "ValidateToken" {
		t.Fatalf("Description name = %q, want %q", descriptions[0].Name, "ValidateToken")
	}
	if descriptions[0].Description == "" {
		t.Fatal("Description text is empty, want semantic summary")
	}
}

func TestSearcherInterfaceIsSatisfied(t *testing.T) {
	var searcher Searcher = searcherStub{}
	ctx := context.Background()

	results, err := searcher.Search(ctx, []string{"auth middleware", "jwt validator"}, SearchOptions{
		TopK:               10,
		Filter:             Filter{Language: "go"},
		MaxResults:         30,
		EnableHopExpansion: true,
		HopBudgetFraction:  0.4,
		HopDepth:           1,
	})
	if err != nil {
		t.Fatalf("Search returned unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %#v, want one result", results)
	}
	if results[0].Chunk.ID != "chunk-id" {
		t.Fatalf("Search result chunk ID = %q, want %q", results[0].Chunk.ID, "chunk-id")
	}
	if results[0].Score != 0.9 {
		t.Fatalf("Search result score = %v, want 0.9", results[0].Score)
	}
}
