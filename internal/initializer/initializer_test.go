//go:build sqlite_fts5
// +build sqlite_fts5

package initializer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitializesEmptyDirectory(t *testing.T) {
	projectRoot := t.TempDir()

	report, err := Run(context.Background(), Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report == nil {
		t.Fatalf("Run returned nil report")
	}

	wantPaths := []string{
		"yard.yaml",
		".yard",
		".yard/yard.db",
		".yard/lancedb/code",
		".yard/lancedb/brain",
		".brain",
		".brain/.obsidian/app.json",
		".brain/notes",
		".brain/specs/.gitkeep",
		".brain/architecture/.gitkeep",
		".brain/epics/.gitkeep",
		".brain/tasks/.gitkeep",
		".brain/plans/.gitkeep",
		".brain/receipts/.gitkeep",
		".brain/logs/.gitkeep",
		".brain/conventions/.gitkeep",
		".gitignore",
	}
	for _, p := range wantPaths {
		full := filepath.Join(projectRoot, p)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}

	configData, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile yard.yaml: %v", err)
	}
	got := string(configData)

	if !strings.Contains(got, "project_root: \""+projectRoot+"\"") {
		t.Errorf("expected project_root substituted to %s, got:\n%s", projectRoot, got)
	}
	wantName := filepath.Base(projectRoot)
	if !strings.Contains(got, "Project: "+wantName) {
		t.Errorf("expected PROJECT_NAME substituted to %s, got:\n%s", wantName, got)
	}
	if !strings.Contains(got, "system_prompt: \"builtin:coder\"") {
		t.Errorf("expected builtin coder marker in generated config")
	}
	if strings.Contains(got, "{{SODORYARD_AGENTS_DIR}}") {
		t.Errorf("expected generated config to avoid {{SODORYARD_AGENTS_DIR}} placeholder")
	}
	for _, want := range []string{"yard index --config yard.yaml", "yard chain start --config yard.yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected generated config to contain %q", want)
		}
	}
	for _, stale := range []string{"tidmouth index", "sirtopham chain"} {
		if strings.Contains(got, stale) {
			t.Errorf("expected generated config to avoid stale command %q", stale)
		}
	}
	wantRoles := []string{
		"orchestrator:", "coder:", "planner:", "test-writer:", "resolver:",
		"correctness-auditor:", "integration-auditor:", "performance-auditor:",
		"security-auditor:", "quality-auditor:", "docs-arbiter:",
		"epic-decomposer:", "task-decomposer:",
	}
	for _, role := range wantRoles {
		if !strings.Contains(got, "  "+role) {
			t.Errorf("expected agent_roles to contain %q", role)
		}
	}

	gitignoreData, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore: %v", err)
	}
	for _, want := range []string{".yard/", ".brain/"} {
		if !strings.Contains(string(gitignoreData), want) {
			t.Errorf("expected .gitignore to contain %q", want)
		}
	}
}

func TestRunIsIdempotent(t *testing.T) {
	projectRoot := t.TempDir()

	if _, err := Run(context.Background(), Options{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	firstYaml, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile after first run: %v", err)
	}
	firstGitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore after first run: %v", err)
	}

	report, err := Run(context.Background(), Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if report == nil {
		t.Fatal("nil report on re-run")
	}

	secondYaml, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile after second run: %v", err)
	}
	if string(firstYaml) != string(secondYaml) {
		t.Errorf("yard.yaml content changed across runs")
	}
	secondGitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore after second run: %v", err)
	}
	if string(firstGitignore) != string(secondGitignore) {
		t.Errorf(".gitignore content changed across runs")
	}
}

func TestRunRequiresProjectRoot(t *testing.T) {
	if _, err := Run(context.Background(), Options{}); err == nil {
		t.Errorf("expected error for empty ProjectRoot, got nil")
	}
}
