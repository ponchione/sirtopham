package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRunHeadlessWrapsSharedValidationErrorsAsRunExitError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetContext(context.Background())
	_, err := runHeadless(cmd, filepath.Join(t.TempDir(), "yard.yaml"), runFlags{
		Role:     "coder",
		Task:     "inline",
		TaskFile: filepath.Join(t.TempDir(), "task.txt"),
		Timeout:  time.Minute,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var exitErr runExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %T, want runExitError", err)
	}
	if exitErr.ExitCode() != runExitInfrastructure {
		t.Fatalf("exit code = %d, want %d", exitErr.ExitCode(), runExitInfrastructure)
	}
}
