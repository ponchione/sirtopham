package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	appconfig "github.com/ponchione/sirtopham/internal/config"
)

func TestBuildProviderSupportsCodex(t *testing.T) {
	binDir := t.TempDir()
	codexPath := filepath.Join(binDir, "codex")
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		codexPath += ".bat"
		script = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(codexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(codex stub): %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := buildProvider("codex", appconfig.ProviderConfig{Type: "codex"})
	if err != nil {
		t.Fatalf("buildProvider(codex) error = %v, want nil", err)
	}
	if got := provider.Name(); got != "codex" {
		t.Fatalf("provider.Name() = %q, want codex", got)
	}
}
