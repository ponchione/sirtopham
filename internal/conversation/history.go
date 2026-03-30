package conversation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	contextpkg "github.com/ponchione/sirtopham/internal/context"
	"github.com/ponchione/sirtopham/internal/db"
)

// HistoryManager provides the first real conversation-history operations needed
// by the Layer 5 bootstrap path.
type HistoryManager struct {
	database *sql.DB
	queries  *db.Queries
	seen     *SeenFiles
	now      func() time.Time
}

// NewHistoryManager constructs a DB-backed history manager. If seen is nil, a
// fresh session-scoped tracker is created.
func NewHistoryManager(database *sql.DB, seen *SeenFiles) *HistoryManager {
	if seen == nil {
		seen = NewSeenFiles()
	}
	return &HistoryManager{
		database: database,
		queries:  db.New(database),
		seen:     seen,
		now:      time.Now,
	}
}

// SetNowForTest overrides the clock used for persisted timestamps.
func (m *HistoryManager) SetNowForTest(now func() time.Time) {
	if m == nil || now == nil {
		return
	}
	m.now = now
}

// PersistUserMessage inserts the initial user message row for a turn before
// context assembly begins.
func (m *HistoryManager) PersistUserMessage(ctx context.Context, conversationID string, turnNumber int, message string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.validate(); err != nil {
		return err
	}

	tx, err := m.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("conversation history: begin persist user message tx: %w", err)
	}
	defer tx.Rollback()

	q := m.queries.WithTx(tx)
	sequence, err := nextSequence(ctx, q, conversationID)
	if err != nil {
		return fmt.Errorf("conversation history: determine next sequence: %w", err)
	}

	timestamp := m.now().UTC().Format(time.RFC3339)
	if err := q.InsertUserMessage(ctx, db.InsertUserMessageParams{
		ConversationID: conversationID,
		Content:        sql.NullString{String: message, Valid: true},
		TurnNumber:     int64(turnNumber),
		Sequence:       sequence,
		CreatedAt:      timestamp,
	}); err != nil {
		return fmt.Errorf("conversation history: insert user message: %w", err)
	}
	if err := q.TouchConversationUpdatedAt(ctx, db.TouchConversationUpdatedAtParams{
		UpdatedAt: timestamp,
		ID:        conversationID,
	}); err != nil {
		return fmt.Errorf("conversation history: touch conversation updated_at: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("conversation history: commit persist user message tx: %w", err)
	}
	return nil
}

// ReconstructHistory returns the active message rows in provider order for the
// current conversation.
func (m *HistoryManager) ReconstructHistory(ctx context.Context, conversationID string) ([]db.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	messages, err := m.queries.ListActiveMessages(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation history: list active messages: %w", err)
	}
	return messages, nil
}

// SeenFiles exposes the session-scoped seen-files tracker used by Layer 3.
func (m *HistoryManager) SeenFiles(string) contextpkg.SeenFileLookup {
	if m == nil {
		return nil
	}
	return m.seen
}

func (m *HistoryManager) validate() error {
	if m == nil {
		return fmt.Errorf("conversation history: manager is nil")
	}
	if m.database == nil {
		return fmt.Errorf("conversation history: database is nil")
	}
	if m.queries == nil {
		return fmt.Errorf("conversation history: queries are nil")
	}
	if m.now == nil {
		return fmt.Errorf("conversation history: clock is nil")
	}
	return nil
}

func nextSequence(ctx context.Context, q *db.Queries, conversationID string) (float64, error) {
	value, err := q.NextMessageSequence(ctx, conversationID)
	if err != nil {
		return 0, err
	}
	switch v := value.(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case []byte:
		var parsed float64
		if _, err := fmt.Sscanf(string(v), "%f", &parsed); err != nil {
			return 0, fmt.Errorf("parse next sequence from bytes %q: %w", string(v), err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported next sequence type %T", value)
	}
}
