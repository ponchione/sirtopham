package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewAgentLoopUsesConfiguredToolResultStoreRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "persisted-tool-results")

	loop := NewAgentLoop(AgentLoopDeps{
		Config: AgentLoopConfig{
			ToolResultStoreRoot: root,
		},
	})

	store, ok := loop.toolResultStore.(*FileToolResultStore)
	if !ok {
		t.Fatalf("toolResultStore type = %T, want *FileToolResultStore", loop.toolResultStore)
	}
	if store.rootDir != root {
		t.Fatalf("store.rootDir = %q, want %q", store.rootDir, root)
	}
}

func TestNewFileToolResultStoreUsesSodoryardTempRootByDefault(t *testing.T) {
	store := NewFileToolResultStore("")

	want := filepath.Join(os.TempDir(), "sodoryard-tool-results")
	if store.rootDir != want {
		t.Fatalf("rootDir = %q, want %q", store.rootDir, want)
	}
}

func TestFileToolResultStorePersistsPrivateArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not stable on windows")
	}
	root := filepath.Join(t.TempDir(), "tool-results")
	store := NewFileToolResultStore(root)

	path, err := store.PersistToolResult(context.Background(), "tool/use:1", "shell run", "secret output")
	if err != nil {
		t.Fatalf("PersistToolResult returned error: %v", err)
	}

	dirInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("root mode = %o, want 700", got)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat persisted result: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want 600", got)
	}
}

func TestFileToolResultStoreTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not stable on windows")
	}
	root := filepath.Join(t.TempDir(), "tool-results")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	path := filepath.Join(root, "shell-tool-1.txt")
	if err := os.WriteFile(path, []byte("old output"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	store := NewFileToolResultStore(root)
	if _, err := store.PersistToolResult(context.Background(), "tool-1", "shell", "new output"); err != nil {
		t.Fatalf("PersistToolResult returned error: %v", err)
	}

	dirInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("root mode = %o, want 700", got)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat persisted result: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want 600", got)
	}
}
