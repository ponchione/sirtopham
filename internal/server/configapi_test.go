package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ponchione/sirtopham/internal/config"
	"github.com/ponchione/sirtopham/internal/provider"
	"github.com/ponchione/sirtopham/internal/server"
)

func TestGetConfigIncludesToolOutputLimitAndStoreRoot(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := config.Default()
	cfg.ProjectRoot = projectRoot
	cfg.Brain.Enabled = false
	cfg.Agent.ToolResultStoreRoot = filepath.Join(projectRoot, ".artifacts", "tool-results")
	cfg.Agent.MaxIterationsPerTurn = 42
	cfg.Agent.ExtendedThinking = false

	srv := server.New(server.Config{Host: "127.0.0.1", Port: 0}, newTestLogger())
	server.NewConfigHandler(srv, cfg, nil, newTestLogger())
	_, base := startServer(t, srv)

	resp, err := http.Get(base + "/api/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Agent struct {
			MaxIterations       int    `json:"max_iterations"`
			ExtendedThinking    bool   `json:"extended_thinking"`
			ToolOutputMaxTokens int    `json:"tool_output_max_tokens"`
			ToolResultStoreRoot string `json:"tool_result_store_root"`
		} `json:"agent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Agent.MaxIterations != 42 {
		t.Fatalf("agent.max_iterations = %d, want 42", body.Agent.MaxIterations)
	}
	if body.Agent.ExtendedThinking {
		t.Fatal("agent.extended_thinking = true, want false")
	}
	if body.Agent.ToolOutputMaxTokens != cfg.Agent.ToolOutputMaxTokens {
		t.Fatalf("agent.tool_output_max_tokens = %d, want %d", body.Agent.ToolOutputMaxTokens, cfg.Agent.ToolOutputMaxTokens)
	}
	if body.Agent.ToolResultStoreRoot != cfg.Agent.ToolResultStoreRoot {
		t.Fatalf("agent.tool_result_store_root = %q, want %q", body.Agent.ToolResultStoreRoot, cfg.Agent.ToolResultStoreRoot)
	}
}

// stubModelLister returns a fixed set of models for testing.
type stubModelLister struct {
	models []provider.Model
}

func (s *stubModelLister) Models(_ context.Context) ([]provider.Model, error) {
	return s.models, nil
}

func TestProvidersEndpointGroupsModelsByProvider(t *testing.T) {
	cfg := config.Default()
	cfg.ProjectRoot = t.TempDir()
	cfg.Brain.Enabled = false
	cfg.Providers = map[string]config.ProviderConfig{
		"anthropic": {Type: "anthropic", Model: "claude-sonnet-4-20250514"},
		"openai":    {Type: "openai", Model: "gpt-4o"},
	}

	models := &stubModelLister{models: []provider.Model{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", ContextWindow: 200000, SupportsTools: true},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextWindow: 128000, SupportsTools: true},
	}}

	srv := server.New(server.Config{Host: "127.0.0.1", Port: 0}, newTestLogger())
	server.NewConfigHandler(srv, cfg, models, newTestLogger())
	_, base := startServer(t, srv)

	resp, err := http.Get(base + "/api/providers")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result []struct {
		Name   string           `json:"name"`
		Models []provider.Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Sort for deterministic ordering.
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}

	// Anthropic should have only Claude models.
	if result[0].Name != "anthropic" {
		t.Fatalf("expected first provider = anthropic, got %q", result[0].Name)
	}
	if len(result[0].Models) != 1 || result[0].Models[0].ID != "claude-sonnet-4-20250514" {
		t.Fatalf("anthropic models: %+v", result[0].Models)
	}

	// OpenAI should have only GPT models.
	if result[1].Name != "openai" {
		t.Fatalf("expected second provider = openai, got %q", result[1].Name)
	}
	if len(result[1].Models) != 1 || result[1].Models[0].ID != "gpt-4o" {
		t.Fatalf("openai models: %+v", result[1].Models)
	}
}

func TestConfigEndpointGroupsModelsByProvider(t *testing.T) {
	cfg := config.Default()
	cfg.ProjectRoot = t.TempDir()
	cfg.Brain.Enabled = false
	cfg.Routing.Default.Provider = "anthropic"
	cfg.Routing.Default.Model = "claude-sonnet-4-20250514"
	cfg.Providers = map[string]config.ProviderConfig{
		"anthropic": {Type: "anthropic", Model: "claude-sonnet-4-20250514"},
		"openai":    {Type: "openai", Model: "gpt-4o"},
	}

	models := &stubModelLister{models: []provider.Model{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", ContextWindow: 200000},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextWindow: 128000},
	}}

	srv := server.New(server.Config{Host: "127.0.0.1", Port: 0}, newTestLogger())
	server.NewConfigHandler(srv, cfg, models, newTestLogger())
	_, base := startServer(t, srv)

	resp, err := http.Get(base + "/api/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Providers []struct {
			Name   string   `json:"name"`
			Models []string `json:"models"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	sort.Slice(body.Providers, func(i, j int) bool { return body.Providers[i].Name < body.Providers[j].Name })

	if len(body.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(body.Providers))
	}

	// Anthropic should only have claude model.
	if len(body.Providers[0].Models) != 1 || body.Providers[0].Models[0] != "claude-sonnet-4-20250514" {
		t.Fatalf("anthropic models in /api/config: %v", body.Providers[0].Models)
	}

	// OpenAI should only have gpt model.
	if len(body.Providers[1].Models) != 1 || body.Providers[1].Models[0] != "gpt-4o" {
		t.Fatalf("openai models in /api/config: %v", body.Providers[1].Models)
	}
}
