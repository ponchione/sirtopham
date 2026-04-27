package cmdutil

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appindex "github.com/ponchione/sodoryard/internal/index"
)

func TestCodeIndexCommandQuietSuppressesSummary(t *testing.T) {
	configPath := writeIndexCommandTestConfig(t)
	cmd := NewCodeIndexCommand("index", "Index", &configPath, func(context.Context, appindex.Options) (*appindex.Result, error) {
		return &appindex.Result{Mode: "incremental", ChunksWritten: 3}, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--quiet"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if out.String() != "" {
		t.Fatalf("quiet output = %q, want empty", out.String())
	}
}

func TestCodeIndexCommandJSONOverridesQuiet(t *testing.T) {
	configPath := writeIndexCommandTestConfig(t)
	cmd := NewCodeIndexCommand("index", "Index", &configPath, func(context.Context, appindex.Options) (*appindex.Result, error) {
		return &appindex.Result{Mode: "incremental", ChunksWritten: 3}, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--quiet", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"mode": "incremental"`) {
		t.Fatalf("json output = %q, want mode", out.String())
	}
}

func writeIndexCommandTestConfig(t *testing.T) string {
	t.Helper()
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".brain"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "yard.yaml")
	content := "project_root: " + projectRoot + "\n" +
		"brain:\n  enabled: true\n  vault_path: " + filepath.Join(projectRoot, ".brain") + "\n" +
		"local_services:\n  enabled: false\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return configPath
}
