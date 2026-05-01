package main

import "testing"

func TestRootCommandRegistersTUI(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"tui"})
	if err != nil {
		t.Fatalf("Find(tui) returned error: %v", err)
	}
	if cmd == nil || cmd.Use != "tui" {
		t.Fatalf("Find(tui) = %#v, want tui command", cmd)
	}
}

func TestRootCommandDoesNotRegisterPublicRun(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "run" {
			t.Fatal("root command still registers public run command")
		}
	}
}
