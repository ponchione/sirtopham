//go:build sqlite_fts5
// +build sqlite_fts5

package context

import (
	stdctx "context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ponchione/sirtopham/internal/config"
	dbpkg "github.com/ponchione/sirtopham/internal/db"
	"github.com/ponchione/sirtopham/internal/provider"
)

type compressionProviderStub struct {
	responseText string
	err          error
	requests     []*provider.Request
}

func (s *compressionProviderStub) Complete(_ stdctx.Context, req *provider.Request) (*provider.Response, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	return &provider.Response{
		Content: []provider.ContentBlock{provider.NewTextBlock(s.responseText)},
	}, nil
}

func (s *compressionProviderStub) Stream(stdctx.Context, *provider.Request) (<-chan provider.StreamEvent, error) {
	return nil, errors.New("not implemented")
}

func (s *compressionProviderStub) Models(stdctx.Context) ([]provider.Model, error) {
	return nil, nil
}

func (s *compressionProviderStub) Name() string {
	return "stub"
}

func TestCompressionTriggerChecks(t *testing.T) {
	cfg := config.ContextConfig{CompressionThreshold: 0.5}

	if !NeedsCompressionPreflight(500000, 200000, cfg) {
		t.Fatal("NeedsCompressionPreflight = false, want true")
	}
	if NeedsCompressionPreflight(1000, 200000, cfg) {
		t.Fatal("NeedsCompressionPreflight = true, want false")
	}
	if !NeedsCompressionPostResponse(100001, 200000, cfg) {
		t.Fatal("NeedsCompressionPostResponse = false, want true")
	}
	if NeedsCompressionPostResponse(99999, 200000, cfg) {
		t.Fatal("NeedsCompressionPostResponse = true, want false")
	}
	if !NeedsCompressionAfterProviderError(413, nil) {
		t.Fatal("NeedsCompressionAfterProviderError(413) = false, want true")
	}
	if !NeedsCompressionAfterProviderError(400, errors.New("provider failed: context_length_exceeded")) {
		t.Fatal("NeedsCompressionAfterProviderError(400 context_length_exceeded) = false, want true")
	}
	if NeedsCompressionAfterProviderError(400, errors.New("other bad request")) {
		t.Fatal("NeedsCompressionAfterProviderError(other 400) = true, want false")
	}
}

func TestCompressionEngineSummarizesMiddleSanitizesOrphansAndInvalidatesCache(t *testing.T) {
	db := newCompressionTestDB(t)
	conversationID := seedCompressionConversation(t, db)
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 1, role: "user", content: "start", turn: 1, iteration: 1})
	insertCompressionMessage(t, db, compressionSeedMessage{
		sequence: 2,
		role:     "assistant",
		content: assistantJSON(t,
			provider.NewTextBlock("I'll inspect the auth file."),
			provider.NewToolUseBlock("toolu-old", "file_read", json.RawMessage(`{"path":"auth.go"}`)),
		),
		turn:      1,
		iteration: 1,
	})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 3, role: "tool", content: "package auth", toolUseID: "toolu-old", toolName: "file_read", turn: 1, iteration: 1})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 4, role: "assistant", content: assistantJSON(t, provider.NewTextBlock("Found the issue.")), turn: 1, iteration: 2})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 5, role: "user", content: "continue", turn: 2, iteration: 1})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 6, role: "assistant", content: assistantJSON(t, provider.NewTextBlock("working")), turn: 2, iteration: 1})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 7, role: "user", content: "tail user", turn: 3, iteration: 1})
	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 8, role: "assistant", content: assistantJSON(t, provider.NewTextBlock("tail assistant")), turn: 3, iteration: 1})

	providerStub := &compressionProviderStub{responseText: "- Kept auth work moving\n- Mentioned auth.go"}
	engine := NewCompressionEngine(db, providerStub)
	result, err := engine.Compress(stdctx.Background(), conversationID, config.ContextConfig{
		CompressionHeadPreserve: 2,
		CompressionTailPreserve: 4,
		CompressionModel:        "local",
	})
	if err != nil {
		t.Fatalf("Compress returned error: %v", err)
	}
	if !result.Compressed {
		t.Fatal("Compressed = false, want true")
	}
	if !result.SummaryInserted {
		t.Fatal("SummaryInserted = false, want true")
	}
	if result.FallbackUsed {
		t.Fatal("FallbackUsed = true, want false")
	}
	if !result.CacheInvalidated {
		t.Fatal("CacheInvalidated = false, want true")
	}
	if result.CompressedTurnStart != 1 || result.CompressedTurnEnd != 1 {
		t.Fatalf("compressed turn range = %d-%d, want 1-1", result.CompressedTurnStart, result.CompressedTurnEnd)
	}
	if len(providerStub.requests) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(providerStub.requests))
	}
	if providerStub.requests[0].Purpose != "compression" {
		t.Fatalf("provider purpose = %q, want compression", providerStub.requests[0].Purpose)
	}
	if providerStub.requests[0].Model != "local" {
		t.Fatalf("provider model = %q, want local", providerStub.requests[0].Model)
	}

	messages := listCompressionMessages(t, db, conversationID)
	active := activeCompressionMessages(messages)
	if len(active) != 7 {
		t.Fatalf("active message count = %d, want 7", len(active))
	}
	if active[2].Role != "user" || active[2].IsSummary != 1 {
		t.Fatalf("active[2] = %+v, want summary user message", active[2])
	}
	if active[2].Sequence != 3.5 {
		t.Fatalf("summary sequence = %v, want 3.5", active[2].Sequence)
	}
	if !strings.HasPrefix(active[2].Content, "[CONTEXT COMPACTION]\n") {
		t.Fatalf("summary content = %q, want [CONTEXT COMPACTION] prefix", active[2].Content)
	}

	assistantBlocks := assistantBlocks(t, active[1].Content)
	if len(assistantBlocks) != 1 || assistantBlocks[0].Type != "text" {
		t.Fatalf("assistant blocks after sanitization = %+v, want text-only", assistantBlocks)
	}
	if strings.Contains(active[1].Content, "tool_use") {
		t.Fatalf("assistant content still contains tool_use block: %s", active[1].Content)
	}
	if ftsCountForQuery(t, db, "file_read") != 0 {
		t.Fatal("messages_fts still matched removed tool_use content")
	}

	reconstructed := reconstructActiveHistory(t, db, conversationID)
	if len(reconstructed) != 7 {
		t.Fatalf("reconstructed history count = %d, want 7", len(reconstructed))
	}
}

func TestCompressionEngineFallsBackWithoutSummary(t *testing.T) {
	db := newCompressionTestDB(t)
	conversationID := seedCompressionConversation(t, db)
	for i := 1; i <= 8; i++ {
		role := "user"
		content := "message"
		if i%2 == 0 {
			role = "assistant"
			content = assistantJSON(t, provider.NewTextBlock("assistant"))
		}
		insertCompressionMessage(t, db, compressionSeedMessage{sequence: float64(i), role: role, content: content, turn: (i + 1) / 2, iteration: 1})
	}

	engine := NewCompressionEngine(db, &compressionProviderStub{err: errors.New("compression model unavailable")})
	result, err := engine.Compress(stdctx.Background(), conversationID, config.ContextConfig{
		CompressionHeadPreserve: 2,
		CompressionTailPreserve: 4,
		CompressionModel:        "local",
	})
	if err != nil {
		t.Fatalf("Compress returned error: %v", err)
	}
	if !result.Compressed {
		t.Fatal("Compressed = false, want true")
	}
	if result.SummaryInserted {
		t.Fatal("SummaryInserted = true, want false")
	}
	if !result.FallbackUsed {
		t.Fatal("FallbackUsed = false, want true")
	}
	if !result.CacheInvalidated {
		t.Fatal("CacheInvalidated = false, want true")
	}

	messages := listCompressionMessages(t, db, conversationID)
	active := activeCompressionMessages(messages)
	if len(active) != 6 {
		t.Fatalf("active message count = %d, want 6", len(active))
	}
	for _, msg := range active {
		if msg.IsSummary == 1 {
			t.Fatalf("unexpected summary row in fallback path: %+v", msg)
		}
	}
}

func TestCompressionEngineBisectsSummarySequenceWhenRawMidpointCollides(t *testing.T) {
	db := newCompressionTestDB(t)
	conversationID := seedCompressionConversation(t, db)
	for i := 1; i <= 20; i++ {
		role := "user"
		content := "user"
		if i%2 == 0 {
			role = "assistant"
			content = assistantJSON(t, provider.NewTextBlock("assistant"))
		}
		insertCompressionMessage(t, db, compressionSeedMessage{sequence: float64(i), role: role, content: content, turn: (i + 1) / 2, iteration: 1})
	}

	engine := NewCompressionEngine(db, &compressionProviderStub{responseText: "- compacted"})
	result, err := engine.Compress(stdctx.Background(), conversationID, config.ContextConfig{
		CompressionHeadPreserve: 3,
		CompressionTailPreserve: 4,
		CompressionModel:        "local",
	})
	if err != nil {
		t.Fatalf("Compress returned error: %v", err)
	}
	if !result.SummaryInserted {
		t.Fatal("SummaryInserted = false, want true")
	}

	messages := listCompressionMessages(t, db, conversationID)
	active := activeCompressionMessages(messages)
	if len(active) != 8 {
		t.Fatalf("active message count = %d, want 8", len(active))
	}
	summary := active[3]
	if summary.IsSummary != 1 {
		t.Fatalf("summary row = %+v, want is_summary=1", summary)
	}
	if summary.Sequence == 10.0 {
		t.Fatalf("summary sequence = %v, want non-colliding bisected value", summary.Sequence)
	}
	if summary.Sequence <= 3.0 || summary.Sequence >= 17.0 {
		t.Fatalf("summary sequence = %v, want to fall between 3 and 17", summary.Sequence)
	}

	insertCompressionMessage(t, db, compressionSeedMessage{sequence: 21, role: "user", content: "after compression", turn: 11, iteration: 1})
}

type compressionSeedMessage struct {
	sequence  float64
	role      string
	content   string
	toolUseID string
	toolName  string
	turn      int
	iteration int
}

type compressionMessageRow struct {
	ID                  int64
	Role                string
	Content             string
	ToolUseID           string
	ToolName            string
	TurnNumber          int
	Iteration           int
	Sequence            float64
	IsCompressed        int
	IsSummary           int
	CompressedTurnStart sql.NullInt64
	CompressedTurnEnd   sql.NullInt64
}

func newCompressionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := stdctx.Background()
	dbPath := filepath.Join(t.TempDir(), "compression.db")
	sqlDB, err := dbpkg.OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	if err := dbpkg.Init(ctx, sqlDB); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func seedCompressionConversation(t *testing.T, sqlDB *sql.DB) string {
	t.Helper()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	projectID := "project-1"
	conversationID := "conversation-1"
	mustExecCompression(t, sqlDB, `INSERT INTO projects(id, name, root_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, projectID, "sirtopham", filepath.Join(t.TempDir(), "project"), createdAt, createdAt)
	mustExecCompression(t, sqlDB, `INSERT INTO conversations(id, project_id, title, model, provider, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, conversationID, projectID, "Compression", "claude", "anthropic", createdAt, createdAt)
	return conversationID
}

func insertCompressionMessage(t *testing.T, sqlDB *sql.DB, msg compressionSeedMessage) {
	t.Helper()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	mustExecCompression(t, sqlDB, `
		INSERT INTO messages(
			conversation_id, role, content, tool_use_id, tool_name, turn_number, iteration, sequence, created_at
		) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)
	`, "conversation-1", msg.role, msg.content, msg.toolUseID, msg.toolName, msg.turn, msg.iteration, msg.sequence, createdAt)
}

func listCompressionMessages(t *testing.T, sqlDB *sql.DB, conversationID string) []compressionMessageRow {
	t.Helper()
	rows, err := sqlDB.Query(`
		SELECT id, role, content, COALESCE(tool_use_id, ''), COALESCE(tool_name, ''), turn_number, iteration, sequence,
		       is_compressed, is_summary, compressed_turn_start, compressed_turn_end
		FROM messages
		WHERE conversation_id = ?
		ORDER BY sequence
	`, conversationID)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	defer rows.Close()

	var messages []compressionMessageRow
	for rows.Next() {
		var msg compressionMessageRow
		if err := rows.Scan(
			&msg.ID,
			&msg.Role,
			&msg.Content,
			&msg.ToolUseID,
			&msg.ToolName,
			&msg.TurnNumber,
			&msg.Iteration,
			&msg.Sequence,
			&msg.IsCompressed,
			&msg.IsSummary,
			&msg.CompressedTurnStart,
			&msg.CompressedTurnEnd,
		); err != nil {
			t.Fatalf("scan message: %v", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate messages: %v", err)
	}
	return messages
}

func activeCompressionMessages(messages []compressionMessageRow) []compressionMessageRow {
	active := make([]compressionMessageRow, 0, len(messages))
	for _, msg := range messages {
		if msg.IsCompressed == 0 {
			active = append(active, msg)
		}
	}
	return active
}

func reconstructActiveHistory(t *testing.T, sqlDB *sql.DB, conversationID string) []dbpkg.ReconstructConversationHistoryRow {
	t.Helper()
	queries := dbpkg.New(sqlDB)
	rows, err := queries.ReconstructConversationHistory(stdctx.Background(), conversationID)
	if err != nil {
		t.Fatalf("ReconstructConversationHistory returned error: %v", err)
	}
	return rows
}

func assistantBlocks(t *testing.T, raw string) []provider.ContentBlock {
	t.Helper()
	blocks, err := provider.ContentBlocksFromRaw(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("ContentBlocksFromRaw returned error: %v", err)
	}
	return blocks
}

func assistantJSON(t *testing.T, blocks ...provider.ContentBlock) string {
	t.Helper()
	raw, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal assistant blocks: %v", err)
	}
	return string(raw)
}

func ftsCountForQuery(t *testing.T, sqlDB *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts.content MATCH ?`, query).Scan(&count); err != nil {
		t.Fatalf("fts count query failed: %v", err)
	}
	return count
}

func mustExecCompression(t *testing.T, sqlDB *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := sqlDB.Exec(query, args...); err != nil {
		t.Fatalf("exec failed for %q: %v", strings.TrimSpace(query), err)
	}
}
