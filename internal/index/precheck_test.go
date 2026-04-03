package index

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ponchione/sirtopham/internal/config"
)

func TestRunIndexPrecheckPassesWhenBothServicesHealthy(t *testing.T) {
	oldBaseURL := describerBaseURL
	qwen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer qwen.Close()

	embed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"nomic"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer embed.Close()

	describerBaseURL = qwen.URL
	defer func() { describerBaseURL = oldBaseURL }()

	cfg := config.Default()
	cfg.Brain.Enabled = false
	cfg.Embedding.BaseURL = embed.URL
	if err := runIndexPrecheck(context.Background(), cfg); err != nil {
		t.Fatalf("runIndexPrecheck: %v", err)
	}
}

func TestRunIndexPrecheckFailsWhenEmbeddingServiceMissing(t *testing.T) {
	oldBaseURL := describerBaseURL
	qwen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer qwen.Close()
	describerBaseURL = qwen.URL
	defer func() { describerBaseURL = oldBaseURL }()

	cfg := config.Default()
	cfg.Brain.Enabled = false
	cfg.Embedding.BaseURL = "http://127.0.0.1:1"
	if err := runIndexPrecheck(context.Background(), cfg); err == nil {
		t.Fatal("expected precheck error")
	} else if got := err.Error(); !containsAll(got, []string{"nomic-embed", "not reachable"}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
