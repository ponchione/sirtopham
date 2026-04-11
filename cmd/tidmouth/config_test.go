package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandDefaultsConfigPathToYardYAML(t *testing.T) {
	cmd := newRootCmd()
	flag := cmd.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("config flag missing")
	}
	if got := flag.DefValue; got != "yard.yaml" {
		t.Fatalf("config default = %q, want yard.yaml", got)
	}
}

func TestConfigCommandPrintsEffectiveConfig(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "sirtopham.yaml")
	configYAML := strings.Join([]string{
		"project_root: " + projectRoot,
		"server:",
		"  host: 127.0.0.1",
		"  port: 9001",
		"routing:",
		"  default:",
		"    provider: anthropic",
		"    model: claude-sonnet-4-6-20250514",
		"  fallback:",
		"    provider: openrouter",
		"    model: anthropic/claude-sonnet-4",
		"embedding:",
		"  base_url: http://localhost:12435",
		"brain:",
		"  enabled: false",
		"local_services:",
		"  enabled: true",
		"  mode: manual",
		"  compose_file: ./ops/llm/docker-compose.yml",
		"  project_dir: ./ops/llm",
	}, "\n") + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configFlag := configPath
	cmd := newConfigCmd(&configFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	stateDir := filepath.Join(projectRoot, ".yard")
	for _, want := range []string{
		"config: valid",
		"project_root: " + projectRoot,
		"server_address: 127.0.0.1:9001",
		"default_provider: anthropic",
		"default_model: claude-sonnet-4-6-20250514",
		"fallback_provider: openrouter",
		"fallback_model: anthropic/claude-sonnet-4",
		"database_path: " + filepath.Join(stateDir, "yard.db"),
		"code_index_path: " + filepath.Join(stateDir, "lancedb", "code"),
		"brain_vault_path: <disabled>",
		"embedding_base_url: http://localhost:12435",
		"local_services_enabled: true",
		"local_services_mode: manual",
		"local_services_compose_file: " + filepath.Join(projectRoot, "ops", "llm", "docker-compose.yml"),
		"local_services_project_dir: " + filepath.Join(projectRoot, "ops", "llm"),
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q\noutput=%s", want, output)
		}
	}
}

func TestConfigCommandReturnsErrorForInvalidConfig(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "sirtopham.yaml")
	configYAML := strings.Join([]string{
		"project_root: " + projectRoot,
		"server:",
		"  port: 70000",
		"brain:",
		"  enabled: false",
	}, "\n") + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configFlag := configPath
	cmd := newConfigCmd(&configFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected Execute to fail")
	}
	if !strings.Contains(err.Error(), "invalid field server.port") {
		t.Fatalf("error = %v, want invalid server.port", err)
	}
}
