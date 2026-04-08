package index

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRuntimeDescriberLLMCompletesUsingDiscoveredModel(t *testing.T) {
	var modelsCalls atomic.Int32
	var chatCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			modelsCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "qwen-test"}}})
		case "/v1/chat/completions":
			chatCalls.Add(1)
			var req chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Model != "qwen-test" {
				t.Fatalf("request model = %q, want qwen-test", req.Model)
			}
			if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
				t.Fatalf("request messages = %#v, want system+user", req.Messages)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]string{"content": `[{"name":"Example","description":"semantic"}]`},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	llm := &runtimeDescriberLLM{
		baseURL:    server.URL,
		modelsURL:  server.URL + "/v1/models",
		httpClient: server.Client(),
	}

	first, err := llm.Complete(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("first Complete: %v", err)
	}
	second, err := llm.Complete(context.Background(), "system prompt", "user prompt 2")
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if !strings.Contains(first, "semantic") || !strings.Contains(second, "semantic") {
		t.Fatalf("unexpected responses: %q / %q", first, second)
	}
	if got := modelsCalls.Load(); got != 1 {
		t.Fatalf("models endpoint calls = %d, want 1", got)
	}
	if got := chatCalls.Load(); got != 2 {
		t.Fatalf("chat completion calls = %d, want 2", got)
	}
}

func TestRuntimeDescriberLLMReturnsErrorWhenModelsEndpointEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{}})
	}))
	defer server.Close()

	llm := &runtimeDescriberLLM{
		baseURL:    server.URL,
		modelsURL:  server.URL,
		httpClient: server.Client(),
	}

	_, err := llm.Complete(context.Background(), "system prompt", "user prompt")
	if err == nil || !strings.Contains(err.Error(), "no models") {
		t.Fatalf("Complete error = %v, want no models error", err)
	}
}
