//go:build sqlite_fts5
// +build sqlite_fts5

package conversation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/ponchione/sirtopham/internal/db"
	sid "github.com/ponchione/sirtopham/internal/id"
)

func TestHistoryManagerPersistUserMessageAssignsSequenceAndTouchesConversation(t *testing.T) {
	ctx := context.Background()
	database := newHistoryTestDB(t)
	queries := db.New(database)
	conversationID := seedHistoryConversation(t, database)

	manager := NewHistoryManager(database, nil)
	manager.now = func() time.Time { return time.Unix(1700000800, 0).UTC() }

	if err := manager.PersistUserMessage(ctx, conversationID, 1, "fix auth"); err != nil {
		t.Fatalf("PersistUserMessage returned error: %v", err)
	}
	if err := manager.PersistUserMessage(ctx, conversationID, 2, "add tests"); err != nil {
		t.Fatalf("second PersistUserMessage returned error: %v", err)
	}

	rows, err := queries.ListTurnMessages(ctx, conversationID)
	if err != nil {
		t.Fatalf("ListTurnMessages returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[0].Sequence != 0.0 || rows[1].Sequence != 1.0 {
		t.Fatalf("sequences = (%v, %v), want (0.0, 1.0)", rows[0].Sequence, rows[1].Sequence)
	}
	if rows[0].Iteration != 1 || rows[1].Iteration != 1 {
		t.Fatalf("iterations = (%d, %d), want (1, 1)", rows[0].Iteration, rows[1].Iteration)
	}
	if rows[0].Content.String != "fix auth" || rows[1].Content.String != "add tests" {
		t.Fatalf("contents = (%q, %q), want (fix auth, add tests)", rows[0].Content.String, rows[1].Content.String)
	}

	var updatedAt string
	if err := database.QueryRowContext(ctx, `SELECT updated_at FROM conversations WHERE id = ?`, conversationID).Scan(&updatedAt); err != nil {
		t.Fatalf("query updated_at returned error: %v", err)
	}
	wantUpdatedAt := manager.now().Format(time.RFC3339)
	if updatedAt != wantUpdatedAt {
		t.Fatalf("updated_at = %q, want %q", updatedAt, wantUpdatedAt)
	}
}

func TestHistoryManagerReconstructHistoryReturnsOnlyActiveMessages(t *testing.T) {
	ctx := context.Background()
	database := newHistoryTestDB(t)
	conversationID := seedHistoryConversation(t, database)
	createdAt := time.Unix(1700000900, 0).UTC().Format(time.RFC3339)

	mustExecHistory(t, database, `INSERT INTO messages(conversation_id, role, content, turn_number, iteration, sequence, created_at)
		VALUES (?, 'user', ?, 1, 1, 0.0, ?)`, conversationID, "fix auth", createdAt)
	mustExecHistory(t, database, `INSERT INTO messages(conversation_id, role, content, turn_number, iteration, sequence, created_at)
		VALUES (?, 'assistant', ?, 1, 1, 1.0, ?)`, conversationID, `[{"type":"text","text":"checking"}]`, createdAt)
	mustExecHistory(t, database, `INSERT INTO messages(conversation_id, role, content, turn_number, iteration, sequence, is_compressed, is_summary, compressed_turn_start, compressed_turn_end, created_at)
		VALUES (?, 'assistant', ?, 9, 1, 1.5, 1, 1, 1, 8, ?)`, conversationID, `[{"type":"text","text":"summary"}]`, createdAt)
	mustExecHistory(t, database, `INSERT INTO messages(conversation_id, role, content, tool_use_id, tool_name, turn_number, iteration, sequence, created_at)
		VALUES (?, 'tool', ?, ?, ?, 1, 1, 2.0, ?)`, conversationID, "file contents", "toolu_1", "file_read", createdAt)

	manager := NewHistoryManager(database, nil)
	history, err := manager.ReconstructHistory(ctx, conversationID)
	if err != nil {
		t.Fatalf("ReconstructHistory returned error: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}
	if history[0].Role != "user" || history[0].Sequence != 0.0 {
		t.Fatalf("history[0] = %#v, want user seq 0.0", history[0])
	}
	if history[1].Role != "assistant" || history[1].Sequence != 1.0 {
		t.Fatalf("history[1] = %#v, want assistant seq 1.0", history[1])
	}
	if history[2].Role != "tool" || history[2].ToolUseID.String != "toolu_1" || history[2].Sequence != 2.0 {
		t.Fatalf("history[2] = %#v, want tool seq 2.0", history[2])
	}
}

func newHistoryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "conversation-history.db")
	database, err := db.OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	if err := db.Init(ctx, database); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seedHistoryConversation(t *testing.T, database *sql.DB) string {
	t.Helper()
	projectID := sid.New()
	conversationID := sid.New()
	createdAt := time.Unix(1700000700, 0).UTC().Format(time.RFC3339)

	mustExecHistory(t, database, `INSERT INTO projects(id, name, root_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, projectID, "proj", "/tmp/proj", createdAt, createdAt)
	mustExecHistory(t, database, `INSERT INTO conversations(id, project_id, title, model, provider, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, conversationID, projectID, "Test", "claude", "anthropic", createdAt, createdAt)
	return conversationID
}

func mustExecHistory(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("exec failed for %q: %v", query, err)
	}
}
