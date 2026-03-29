package searcher

import (
	"context"
	"testing"

	"github.com/ponchione/sirtopham/internal/codeintel"
)

// fakeEmbedder returns a fixed vector for any query.
type fakeEmbedder struct {
	vec []float32
}

func (f *fakeEmbedder) EmbedTexts(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}

func (f *fakeEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	return f.vec, nil
}

// fakeStore returns pre-configured results.
type fakeStore struct {
	searchResults []codeintel.SearchResult
	byName        map[string][]codeintel.Chunk
}

func (f *fakeStore) Upsert(_ context.Context, _ []codeintel.Chunk) error { return nil }
func (f *fakeStore) VectorSearch(_ context.Context, _ []float32, topK int, _ codeintel.Filter) ([]codeintel.SearchResult, error) {
	if topK < len(f.searchResults) {
		return f.searchResults[:topK], nil
	}
	return f.searchResults, nil
}
func (f *fakeStore) GetByFilePath(_ context.Context, _ string) ([]codeintel.Chunk, error) {
	return nil, nil
}
func (f *fakeStore) GetByName(_ context.Context, name string) ([]codeintel.Chunk, error) {
	return f.byName[name], nil
}
func (f *fakeStore) DeleteByFilePath(_ context.Context, _ string) error { return nil }
func (f *fakeStore) Close() error                                       { return nil }

func TestSearch_SingleQuery(t *testing.T) {
	store := &fakeStore{
		searchResults: []codeintel.SearchResult{
			{Chunk: codeintel.Chunk{ID: "a", Name: "FuncA"}, Score: 0.9},
			{Chunk: codeintel.Chunk{ID: "b", Name: "FuncB"}, Score: 0.8},
		},
	}
	embedder := &fakeEmbedder{vec: make([]float32, 10)}

	s := New(store, embedder)

	results, err := s.Search(context.Background(), []string{"find auth"}, codeintel.SearchOptions{
		TopK:       10,
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Chunk.Name != "FuncA" {
		t.Errorf("results[0].Name = %q, want FuncA", results[0].Chunk.Name)
	}
}

func TestSearch_MultiQueryDedup(t *testing.T) {
	// Same chunk returned for both queries — should be deduped with hitCount=2.
	store := &fakeStore{
		searchResults: []codeintel.SearchResult{
			{Chunk: codeintel.Chunk{ID: "a", Name: "FuncA"}, Score: 0.9},
		},
	}
	embedder := &fakeEmbedder{vec: make([]float32, 10)}

	s := New(store, embedder)

	results, err := s.Search(context.Background(), []string{"query1", "query2"}, codeintel.SearchOptions{
		TopK:       10,
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (deduped)", len(results))
	}
	if results[0].HitCount != 2 {
		t.Errorf("HitCount = %d, want 2", results[0].HitCount)
	}
}

func TestSearch_HopExpansion(t *testing.T) {
	store := &fakeStore{
		searchResults: []codeintel.SearchResult{
			{Chunk: codeintel.Chunk{
				ID:   "a",
				Name: "FuncA",
				Calls: []codeintel.FuncRef{{Name: "HelperB", Package: "pkg"}},
			}, Score: 0.9},
		},
		byName: map[string][]codeintel.Chunk{
			"HelperB": {{ID: "b", Name: "HelperB"}},
		},
	}
	embedder := &fakeEmbedder{vec: make([]float32, 10)}

	s := New(store, embedder)

	results, err := s.Search(context.Background(), []string{"find auth"}, codeintel.SearchOptions{
		TopK:              10,
		MaxResults:        10,
		EnableHopExpansion: true,
		HopBudgetFraction: 0.5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("got %d results, want >= 2 (with hop)", len(results))
	}

	var foundHop bool
	for _, r := range results {
		if r.FromHop && r.Chunk.Name == "HelperB" {
			foundHop = true
		}
	}
	if !foundHop {
		t.Error("expected HelperB as a hop result")
	}
}

func TestSearch_EmptyQueries(t *testing.T) {
	s := New(&fakeStore{}, &fakeEmbedder{})

	results, err := s.Search(context.Background(), nil, codeintel.SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestSearcherImplementsInterface(t *testing.T) {
	var _ codeintel.Searcher = (*Searcher)(nil)
}
