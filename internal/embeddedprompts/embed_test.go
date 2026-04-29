package embeddedprompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeysIncludesAllBuiltInRoles(t *testing.T) {
	want := []string{
		"coder",
		"correctness-auditor",
		"docs-arbiter",
		"epic-decomposer",
		"integration-auditor",
		"orchestrator",
		"performance-auditor",
		"planner",
		"quality-auditor",
		"resolver",
		"security-auditor",
		"task-decomposer",
		"test-writer",
	}
	got := Keys()
	if len(got) != len(want) {
		t.Fatalf("Keys() returned %d keys, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("Keys()[%d] = %q, want %q (all=%v)", i, got[i], w, got)
		}
		if !Has(w) {
			t.Fatalf("Has(%q) = false, want true", w)
		}
		content, ok := Get(w)
		if !ok {
			t.Fatalf("Get(%q) ok = false, want true", w)
		}
		if content == "" {
			t.Fatalf("Get(%q) returned empty content", w)
		}
	}
}

func TestGetUnknownRoleReturnsFalse(t *testing.T) {
	if got, ok := Get("not-a-role"); ok || got != "" {
		t.Fatalf("Get(unknown) = (%q, %t), want (\"\", false)", got, ok)
	}
	if Has("not-a-role") {
		t.Fatal("Has(unknown) = true, want false")
	}
}

func TestEmbeddedPromptsMatchRepoRootAgents(t *testing.T) {
	for role, filename := range roleToAsset {
		embedded, ok := Get(role)
		if !ok {
			t.Fatalf("Get(%q) ok = false", role)
		}
		repoPath := filepath.Join("..", "..", "agents", filename)
		data, err := os.ReadFile(repoPath)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", repoPath, err)
		}
		if embedded != string(data) {
			t.Fatalf("embedded prompt %q does not match %s", role, repoPath)
		}
	}
}

func TestEmbeddedPromptsUseRuntimeToolNamesAndCleanMarkdown(t *testing.T) {
	for role := range roleToAsset {
		content, ok := Get(role)
		if !ok {
			t.Fatalf("Get(%q) ok = false", role)
		}
		if !strings.HasPrefix(content, "# ") {
			t.Fatalf("prompt %q starts with %q, want markdown heading", role, content[:min(len(content), 20)])
		}
		if strings.Contains(content, "spawn_engine") {
			t.Fatalf("prompt %q references obsolete spawn_engine tool name", role)
		}
	}
}
