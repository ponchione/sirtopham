package embedder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ponchione/sirtopham/internal/codeintel"
	"github.com/ponchione/sirtopham/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.Embedding{
		BaseURL:        "http://localhost:8081",
		Model:          "nomic-embed-code",
		BatchSize:      32,
		TimeoutSeconds: 15,
		QueryPrefix:    "query: ",
	}

	c := New(cfg)

	if c.baseURL != cfg.BaseURL {
		t.Fatalf("baseURL = %q, want %q", c.baseURL, cfg.BaseURL)
	}
	if c.model != cfg.Model {
		t.Fatalf("model = %q, want %q", c.model, cfg.Model)
	}
	if c.batchSize != cfg.BatchSize {
		t.Fatalf("batchSize = %d, want %d", c.batchSize, cfg.BatchSize)
	}
	if c.queryPrefix != cfg.QueryPrefix {
		t.Fatalf("queryPrefix = %q, want %q", c.queryPrefix, cfg.QueryPrefix)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	if c.httpClient.Timeout != 15*time.Second {
		t.Fatalf("httpClient.Timeout = %v, want %v", c.httpClient.Timeout, 15*time.Second)
	}
}

func TestSendBatch_Success(t *testing.T) {
	dims := codeintel.DefaultEmbeddingDims

	// Build a fake embedding response with 2 items (returned out of order by index).
	resp := embeddingResponse{
		Data: []embeddingData{
			{Embedding: makeVec(dims, 0.2), Index: 1},
			{Embedding: makeVec(dims, 0.1), Index: 0},
		},
	}

	var gotReq embeddingRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(config.Embedding{
		BaseURL:        srv.URL,
		Model:          "test-model",
		BatchSize:      32,
		TimeoutSeconds: 5,
	})

	vecs, err := c.sendBatch(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("sendBatch error: %v", err)
	}

	// Verify request body.
	if len(gotReq.Input) != 2 {
		t.Fatalf("request input len = %d, want 2", len(gotReq.Input))
	}
	if gotReq.Model != "test-model" {
		t.Fatalf("request model = %q, want %q", gotReq.Model, "test-model")
	}

	// Verify vectors are re-ordered by index.
	if len(vecs) != 2 {
		t.Fatalf("vecs len = %d, want 2", len(vecs))
	}
	if vecs[0][0] != 0.1 {
		t.Errorf("vecs[0][0] = %f, want 0.1 (reordered by index)", vecs[0][0])
	}
	if vecs[1][0] != 0.2 {
		t.Errorf("vecs[1][0] = %f, want 0.2 (reordered by index)", vecs[1][0])
	}
}

func TestSendBatch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer srv.Close()

	c := New(config.Embedding{
		BaseURL:        srv.URL,
		Model:          "test-model",
		BatchSize:      32,
		TimeoutSeconds: 5,
	})

	_, err := c.sendBatch(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestSendBatch_WrongCount(t *testing.T) {
	dims := codeintel.DefaultEmbeddingDims
	// Return 1 embedding for 2 inputs.
	resp := embeddingResponse{
		Data: []embeddingData{
			{Embedding: makeVec(dims, 0.1), Index: 0},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(config.Embedding{
		BaseURL:        srv.URL,
		Model:          "test-model",
		BatchSize:      32,
		TimeoutSeconds: 5,
	})

	_, err := c.sendBatch(context.Background(), []string{"hello", "world"})
	if err == nil {
		t.Fatal("expected error for count mismatch, got nil")
	}
}

func TestSendBatch_WrongDims(t *testing.T) {
	// Return embeddings with wrong dimensionality.
	resp := embeddingResponse{
		Data: []embeddingData{
			{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(config.Embedding{
		BaseURL:        srv.URL,
		Model:          "test-model",
		BatchSize:      32,
		TimeoutSeconds: 5,
	})

	_, err := c.sendBatch(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for dimension mismatch, got nil")
	}
}

func TestSendBatch_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	c := New(config.Embedding{
		BaseURL:        srv.URL,
		Model:          "test-model",
		BatchSize:      32,
		TimeoutSeconds: 30,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.sendBatch(ctx, []string{"hello"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// makeVec returns a float32 slice of the given length filled with val.
func makeVec(dims int, val float32) []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = val
	}
	return v
}
