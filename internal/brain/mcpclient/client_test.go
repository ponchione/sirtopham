package mcpclient

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectProvidesBackendOperations(t *testing.T) {
	ctx := context.Background()
	vaultPath := t.TempDir()

	client, err := Connect(ctx, vaultPath)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	content := "---\ntags: [architecture]\n---\n# Design\nPipeline notes"
	if err := client.WriteDocument(ctx, "notes/design.md", content); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	got, err := client.ReadDocument(ctx, "notes/design.md")
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}
	if got != content {
		t.Fatalf("ReadDocument mismatch\n got: %q\nwant: %q", got, content)
	}

	hits, err := client.SearchKeyword(ctx, "pipeline")
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(hits) != 1 || hits[0].Path != "notes/design.md" {
		t.Fatalf("SearchKeyword = %#v, want notes/design.md hit", hits)
	}

	files, err := client.ListDocuments(ctx, "notes")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(files) != 1 || files[0] != "notes/design.md" {
		t.Fatalf("ListDocuments = %#v, want [notes/design.md]", files)
	}

	if err := client.PatchDocument(ctx, "notes/design.md", "append", "## Appendix\n\nMore notes."); err != nil {
		t.Fatalf("PatchDocument: %v", err)
	}
	got, err = client.ReadDocument(ctx, "notes/design.md")
	if err != nil {
		t.Fatalf("ReadDocument after patch: %v", err)
	}
	if want := "## Appendix\n\nMore notes."; !strings.Contains(got, want) {
		t.Fatalf("patched document missing %q:\n%s", want, got)
	}
}

func TestSearchKeywordLimitPassesLimitToVaultSearch(t *testing.T) {
	ctx := context.Background()
	vaultPath := t.TempDir()

	client, err := Connect(ctx, vaultPath)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	for _, path := range []string{"notes/a.md", "notes/b.md", "notes/c.md"} {
		if err := client.WriteDocument(ctx, path, "pipeline notes"); err != nil {
			t.Fatalf("WriteDocument(%s): %v", path, err)
		}
	}

	hits, err := client.SearchKeywordLimit(ctx, "pipeline", 2)
	if err != nil {
		t.Fatalf("SearchKeywordLimit: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("SearchKeywordLimit returned %d hits, want 2: %#v", len(hits), hits)
	}
}

func TestConnectRejectsMissingVaultPath(t *testing.T) {
	_, err := Connect(context.Background(), filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for missing vault path")
	}
}
