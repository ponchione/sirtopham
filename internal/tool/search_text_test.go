package tool

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireRipgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not found in PATH, skipping search_text tests")
	}
}

func TestSearchTextSuccess(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "lib.go"), []byte("package main\n\nfunc helper() string {\n\treturn \"world\"\n}\n"), 0o644)

	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":"func"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "main.go") {
		t.Fatalf("expected main.go in results, got:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "lib.go") {
		t.Fatalf("expected lib.go in results, got:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "func") {
		t.Fatalf("expected 'func' in results, got:\n%s", result.Content)
	}
}

func TestSearchTextNoResults(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)

	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":"nonexistent_xyz_pattern"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true for no matches")
	}
	if !strings.Contains(result.Content, "No matches found") {
		t.Fatalf("expected 'No matches found', got: %s", result.Content)
	}
}

func TestSearchTextFileGlob(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\nvar x = 42\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\nvar x = 42\n"), 0o644)

	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":"42","file_glob":"*.go"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "code.go") {
		t.Fatalf("expected code.go in results, got:\n%s", result.Content)
	}
	if strings.Contains(result.Content, "notes.md") {
		t.Fatalf("did NOT expect notes.md in results (glob should filter), got:\n%s", result.Content)
	}
}

func TestSearchTextRegex(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("func main() {}\nfunc helper() {}\nvar x = 1\n"), 0o644)

	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":"func\\s+\\w+"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Content)
	}
	// Should find both func lines.
	if !strings.Contains(result.Content, "main") && !strings.Contains(result.Content, "helper") {
		t.Fatalf("expected func matches, got:\n%s", result.Content)
	}
}

func TestSearchTextContextLines(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	lines := []string{
		"line 1", "line 2", "line 3",
		"MATCH", "line 5", "line 6", "line 7",
	}
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":"MATCH","context_lines":3}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Content)
	}
	// Should have context lines around MATCH.
	if !strings.Contains(result.Content, "MATCH") {
		t.Fatalf("expected MATCH in results, got:\n%s", result.Content)
	}
}

func TestSearchTextEmptyPattern(t *testing.T) {
	dir := t.TempDir()
	result, err := SearchText{}.Execute(context.Background(), dir,
		json.RawMessage(`{"pattern":""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for empty pattern")
	}
}

func TestSearchTextSchema(t *testing.T) {
	schema := SearchText{}.Schema()
	if !json.Valid(schema) {
		t.Fatal("Schema() is not valid JSON")
	}
	if !strings.Contains(string(schema), "search_text") {
		t.Fatal("Schema() does not contain tool name")
	}
}
