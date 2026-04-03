package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	appindex "github.com/ponchione/sirtopham/internal/index"
)

func TestIndexCommandPassesFlagsToService(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "sirtopham.yaml")
	configYAML := "project_root: " + projectRoot + "\nbrain:\n  enabled: false\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	original := runIndexService
	defer func() { runIndexService = original }()

	var gotOpts appindex.Options
	runIndexService = func(_ context.Context, opts appindex.Options) (*appindex.Result, error) {
		gotOpts = opts
		return &appindex.Result{Mode: "full", Duration: time.Second}, nil
	}

	configFlag := configPath
	cmd := newIndexCmd(&configFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--full"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !gotOpts.Full {
		t.Fatal("expected Full=true")
	}
	if !gotOpts.IncludeDirty {
		t.Fatal("expected IncludeDirty=true")
	}
	if gotOpts.Config == nil || gotOpts.Config.ProjectRoot != projectRoot {
		t.Fatalf("Config.ProjectRoot = %v, want %s", gotOpts.Config, projectRoot)
	}
}

func TestIndexCommandJSONOutput(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "sirtopham.yaml")
	configYAML := "project_root: " + projectRoot + "\nbrain:\n  enabled: false\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	original := runIndexService
	defer func() { runIndexService = original }()

	runIndexService = func(context.Context, appindex.Options) (*appindex.Result, error) {
		return &appindex.Result{Mode: "incremental", FilesChanged: 2}, nil
	}

	configFlag := configPath
	cmd := newIndexCmd(&configFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result appindex.Result
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, buf.String())
	}
	if result.Mode != "incremental" || result.FilesChanged != 2 {
		t.Fatalf("result = %+v, want incremental/2", result)
	}
}
