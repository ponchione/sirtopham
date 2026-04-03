package main

import "testing"

func TestNewBrainServeCmdHasVaultFlag(t *testing.T) {
	cmd := newBrainServeCmd()
	if cmd.Use != "brain-serve" {
		t.Fatalf("Use = %q, want brain-serve", cmd.Use)
	}
	flag := cmd.Flags().Lookup("vault")
	if flag == nil {
		t.Fatal("expected --vault flag")
	}
}
