package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestYardRunHeadlessWrapsSharedValidationErrorsAsYardRunExitError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetContext(context.Background())
	_, err := yardRunHeadless(cmd, filepath.Join(t.TempDir(), "yard.yaml"), yardRunFlags{
		Role:     "coder",
		Task:     "inline",
		TaskFile: filepath.Join(t.TempDir(), "task.txt"),
		Timeout:  time.Minute,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var exitErr yardRunExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %T, want yardRunExitError", err)
	}
	if exitErr.ExitCode() != yardRunExitInfrastructure {
		t.Fatalf("exit code = %d, want %d", exitErr.ExitCode(), yardRunExitInfrastructure)
	}
}
