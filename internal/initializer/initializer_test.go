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

	// Files that must exist after init.
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

	// yard.yaml content checks.
	configData, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile yard.yaml: %v", err)
	}
	got := string(configData)

	// PROJECT_ROOT was substituted (quoted in YAML for template validity).
	if !strings.Contains(got, "project_root: \""+projectRoot+"\"") {
		t.Errorf("expected project_root substituted to %s, got:\n%s", projectRoot, got)
	}
	// PROJECT_NAME (basename) was substituted.
	wantName := filepath.Base(projectRoot)
	if !strings.Contains(got, "Project: "+wantName) {
		t.Errorf("expected PROJECT_NAME substituted to %s, got:\n%s", wantName, got)
	}
	// SODORYARD_AGENTS_DIR placeholder is preserved.
	if !strings.Contains(got, "{{SODORYARD_AGENTS_DIR}}/thomas.md") {
		t.Errorf("expected {{SODORYARD_AGENTS_DIR}} placeholder to be preserved")
	}
	// All 13 roles are present in agent_roles.
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

	// .gitignore has the railway entries.
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

	// Capture file contents after first run.
	firstYaml, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile after first run: %v", err)
	}
	firstGitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore after first run: %v", err)
	}

	// Re-run.
	report, err := Run(context.Background(), Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if report == nil {
		t.Fatal("nil report on re-run")
	}

	// Content of files must not change.
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
