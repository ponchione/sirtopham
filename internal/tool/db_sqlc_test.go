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

// requireSqlc skips the test if sqlc is not available in PATH.
func requireSqlc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sqlc"); err != nil {
		t.Skip("sqlc not found in PATH, skipping sqlc tool tests")
	}
}

// setupSqlcProject creates a minimal sqlc project in a temp directory and
// runs `sqlc generate` to establish the initial generated state. Returns the
// project directory.
func setupSqlcProject(t *testing.T) string {
	t.Helper()
	requireSqlc(t)

	dir := t.TempDir()

	sqlcYaml := `version: "2"
sql:
  - engine: sqlite
    queries: query.sql
    schema: schema.sql
    gen:
      go:
        package: db
        out: db
`
	schemaSQL := `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`
	querySQL := `-- name: GetUser :one
SELECT id, name FROM users WHERE id = ?;
`

	if err := os.WriteFile(filepath.Join(dir, "sqlc.yaml"), []byte(sqlcYaml), 0o644); err != nil {
		t.Fatalf("write sqlc.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(schemaSQL), 0o644); err != nil {
		t.Fatalf("write schema.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "query.sql"), []byte(querySQL), 0o644); err != nil {
		t.Fatalf("write query.sql: %v", err)
	}

	// Run sqlc generate to establish initial state.
	cmd := exec.Command("sqlc", "generate")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initial sqlc generate failed: %v\n%s", err, out)
	}

	return dir
}

// --- schema and purity ---

func TestDbSqlcSchema(t *testing.T) {
	schema := DbSqlc{}.Schema()
	if !json.Valid(schema) {
		t.Fatal("Schema() is not valid JSON")
	}
	if !strings.Contains(string(schema), "db_sqlc") {
		t.Fatal("Schema() does not contain tool name 'db_sqlc'")
	}
}

func TestDbSqlcPurity(t *testing.T) {
	tool := DbSqlc{}
	if tool.ToolPurity() != Mutating {
		t.Fatal("expected DbSqlc to be Mutating")
	}
}

// --- error conditions ---

func TestDbSqlcNoSqlcYaml(t *testing.T) {
	dir := t.TempDir() // no sqlc config

	result, err := DbSqlc{}.Execute(context.Background(), dir, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when no sqlc config exists")
	}
	if !strings.Contains(result.Content, "sqlc.yaml") || !strings.Contains(result.Content, "sqlc.yml") {
		t.Fatalf("expected helpful error mentioning config files, got: %s", result.Content)
	}
}

func TestDbSqlcInvalidAction(t *testing.T) {
	dir := t.TempDir()
	// Write a dummy config so we get past the config check.
	os.WriteFile(filepath.Join(dir, "sqlc.yaml"), []byte("version: '2'\n"), 0o644)

	for _, bad := range []string{"drop", "migrate", "run", ""} {
		input, _ := json.Marshal(map[string]string{"action": bad})
		if bad == "" {
			// Empty string falls through to default "generate" — skip.
			continue
		}
		result, err := DbSqlc{}.Execute(context.Background(), dir, input)
		if err != nil {
			t.Fatalf("action %q: unexpected Go error: %v", bad, err)
		}
		if result.Success {
			t.Fatalf("action %q: expected failure for invalid action", bad)
		}
		if !strings.Contains(result.Content, bad) {
			t.Fatalf("action %q: expected action name in error, got: %s", bad, result.Content)
		}
	}
}

func TestDbSqlcPathTraversal(t *testing.T) {
	dir := t.TempDir()

	result, err := DbSqlc{}.Execute(context.Background(), dir,
		json.RawMessage(`{"path":"../../etc"}`))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for path traversal")
	}
	if !strings.Contains(result.Content, "escapes project root") {
		t.Fatalf("expected path traversal error, got: %s", result.Content)
	}
}

// --- functional tests (require sqlc in PATH) ---

func TestDbSqlcVetSuccess(t *testing.T) {
	dir := setupSqlcProject(t) // skips if sqlc unavailable

	result, err := DbSqlc{}.Execute(context.Background(), dir,
		json.RawMessage(`{"action":"vet"}`))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected vet success, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "no issues found") {
		t.Fatalf("expected 'no issues found', got: %s", result.Content)
	}
}

func TestDbSqlcDiff(t *testing.T) {
	dir := setupSqlcProject(t) // generates initial state, skips if no sqlc

	// After generate the working tree matches — diff should report in sync.
	result, err := DbSqlc{}.Execute(context.Background(), dir,
		json.RawMessage(`{"action":"diff"}`))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected diff success (even with differences), got: %s", result.Content)
	}
	// Should say "in sync" (empty diff output) or show a diff — either is acceptable,
	// but Success must be true.
	if !strings.Contains(result.Content, "sqlc diff") {
		t.Fatalf("expected 'sqlc diff' in output, got: %s", result.Content)
	}
}
