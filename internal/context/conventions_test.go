package context

import (
	stdctx "context"
	"os"
	"path/filepath"
	"testing"
)

func TestBrainConventionSourceLoadReturnsEmptyWhenDirectoryMissing(t *testing.T) {
	source := NewBrainConventionSource(t.TempDir())
	text, err := source.Load(stdctx.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("Load = %q, want empty string", text)
	}
}

func TestBrainConventionSourceLoadExtractsBulletsAndParagraphSummaries(t *testing.T) {
	vault := t.TempDir()
	mustWriteConventionFile(t, vault, "conventions/testing-patterns.md", "---\ntags: [convention]\n---\n\n# Testing patterns\n\n- Prefer table-driven tests\n- Keep fixtures local to each test file\n")
	mustWriteConventionFile(t, vault, "conventions/error-handling.md", "# Error handling\n\nAlways wrap errors with operation context and preserve the original cause.\n\n```go\nfmt.Errorf(\"load config: %w\", err)\n```\n")

	source := NewBrainConventionSource(vault)
	text, err := source.Load(stdctx.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := "Error handling: Always wrap errors with operation context and preserve the original cause.\nPrefer table-driven tests\nKeep fixtures local to each test file"
	if text != want {
		t.Fatalf("Load mismatch\n got: %q\nwant: %q", text, want)
	}
}

func TestBrainConventionSourceLoadDeduplicatesAndRespectsLimit(t *testing.T) {
	vault := t.TempDir()
	mustWriteConventionFile(t, vault, "conventions/a.md", "- Same rule\n- Same rule\n- Rule A\n")
	mustWriteConventionFile(t, vault, "conventions/b.md", "- Rule B\n")
	source := NewBrainConventionSource(vault)
	source.bulletLimit = 2

	text, err := source.Load(stdctx.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if text != "Same rule\nRule A" {
		t.Fatalf("Load = %q, want first two unique bullets", text)
	}
}

func TestExtractConventionBulletsFallsBackToFilenameWhenHeadingMissing(t *testing.T) {
	got := extractConventionBullets("anti-patterns.md", "Never patch production data manually.\n")
	if len(got) != 1 || got[0] != "anti patterns: Never patch production data manually." {
		t.Fatalf("extractConventionBullets = %#v, want filename-derived summary", got)
	}
}

func mustWriteConventionFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", fullPath, err)
	}
}
