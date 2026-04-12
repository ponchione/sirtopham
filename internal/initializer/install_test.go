package initializer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSubstitutesAgentsDir(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	original := `agent_roles:
  coder:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
  planner:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/planner.md
`
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Install(InstallOptions{
		ConfigPath:         yamlPath,
		SodoryardAgentsDir: "/opt/yard/agents",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Substitutions != 2 {
		t.Errorf("expected 2 substitutions, got %d", result.Substitutions)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := `agent_roles:
  coder:
    system_prompt: /opt/yard/agents/coder.md
  planner:
    system_prompt: /opt/yard/agents/planner.md
`
	if string(got) != want {
		t.Errorf("substitution mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	if err := os.WriteFile(yamlPath, []byte("system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if result.Substitutions != 0 {
		t.Errorf("expected 0 substitutions on re-run, got %d", result.Substitutions)
	}
	second, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("file content changed across runs:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestInstallErrorsWhenConfigMissing(t *testing.T) {
	_, err := Install(InstallOptions{
		ConfigPath:         filepath.Join(t.TempDir(), "nonexistent.yaml"),
		SodoryardAgentsDir: "/opt/yard/agents",
	})
	if err == nil {
		t.Errorf("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "yard.yaml") && !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention the missing file, got: %v", err)
	}
}

func TestInstallErrorsWhenAgentsDirEmpty(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	if err := os.WriteFile(yamlPath, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: ""})
	if err == nil {
		t.Errorf("expected error for empty SodoryardAgentsDir, got nil")
	}
}

func TestInstallLeavesOtherPlaceholdersAlone(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	original := `project_root: /home/user/myapp
foo: {{SOME_OTHER_PLACEHOLDER}}
system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
`
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "{{SOME_OTHER_PLACEHOLDER}}") {
		t.Errorf("expected unrelated placeholder to be preserved, got:\n%s", got)
	}
	if !strings.Contains(string(got), "/opt/yard/agents/coder.md") {
		t.Errorf("expected agents-dir substitution, got:\n%s", got)
	}
}
