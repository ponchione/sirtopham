package codex

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	p := &CodexProvider{}
	if p.Name() != "codex" {
		t.Errorf("expected %q, got %q", "codex", p.Name())
	}
}

func TestModels(t *testing.T) {
	p := &CodexProvider{}
	models, err := p.Models(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	expectedModels := []struct {
		id            string
		contextWindow int
	}{
		{"o3", 200000},
		{"o4-mini", 200000},
		{"gpt-4.1", 1000000},
	}

	for i, exp := range expectedModels {
		if models[i].ID != exp.id {
			t.Errorf("model %d: expected ID %q, got %q", i, exp.id, models[i].ID)
		}
		if models[i].ContextWindow != exp.contextWindow {
			t.Errorf("model %d: expected context window %d, got %d", i, exp.contextWindow, models[i].ContextWindow)
		}
		if !models[i].SupportsTools {
			t.Errorf("model %d: expected SupportsTools=true", i)
		}
		if models[i].SupportsThinking {
			t.Errorf("model %d: expected SupportsThinking=false", i)
		}
	}
}

func TestWithBaseURL_TrimsTrailingSlash(t *testing.T) {
	p := &CodexProvider{}
	opt := WithBaseURL("https://api.openai.com/")
	opt(p)
	if p.baseURL != "https://api.openai.com" {
		t.Errorf("expected %q, got %q", "https://api.openai.com", p.baseURL)
	}
}

func TestNewCodexProvider_CodexNotOnPath(t *testing.T) {
	// Save original PATH and set to empty temp dir
	tmpDir := t.TempDir()
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	_, err := NewCodexProvider()
	if err == nil {
		t.Fatal("expected error when codex is not on PATH")
	}
	if !strings.Contains(err.Error(), "Codex CLI not found on PATH") {
		t.Errorf("expected error containing %q, got %q", "Codex CLI not found on PATH", err.Error())
	}
}
