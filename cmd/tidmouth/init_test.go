//go:build sqlite_fts5
// +build sqlite_fts5

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) returned error: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory to %q: %v", wd, err)
		}
	})
}

func TestRunInitUsesCanonicalYardPaths(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "eyebox")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot): %v", err)
	}
	withWorkingDir(t, projectRoot)

	if err := runInit(context.Background(), ""); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	for _, path := range []string{
		filepath.Join(projectRoot, "yard.yaml"),
		filepath.Join(projectRoot, ".yard"),
		filepath.Join(projectRoot, ".yard", "yard.db"),
		filepath.Join(projectRoot, ".yard", "lancedb", "code"),
		filepath.Join(projectRoot, ".yard", "lancedb", "brain"),
		filepath.Join(projectRoot, ".brain", ".obsidian", "app.json"),
		filepath.Join(projectRoot, ".brain", "notes"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	configData, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(yard.yaml): %v", err)
	}
	if !strings.Contains(string(configData), "**/.yard/**") {
		t.Fatalf("expected yard.yaml to exclude .yard, got:\n%s", string(configData))
	}
	for _, want := range []string{"local_services:", "compose_file: ./ops/llm/docker-compose.yml", "base_url: http://localhost:12434", "base_url: http://localhost:12435"} {
		if !strings.Contains(string(configData), want) {
			t.Fatalf("expected yard.yaml to contain %q, got:\n%s", want, string(configData))
		}
	}

	gitignoreData, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore): %v", err)
	}
	if !strings.Contains(string(gitignoreData), ".yard/") {
		t.Fatalf("expected .gitignore to contain .yard/, got:\n%s", string(gitignoreData))
	}
}
