package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ponchione/sirtopham/internal/config"
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
