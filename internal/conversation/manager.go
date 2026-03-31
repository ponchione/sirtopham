package conversation

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/ponchione/sirtopham/internal/db"
	sid "github.com/ponchione/sirtopham/internal/id"
)

// Conversation is the application-level representation of a conversation row.
// It converts sql.NullString fields to Go-native pointer types.
type Conversation struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Title     *string    `json:"title,omitempty"`
	Model     *string    `json:"model,omitempty"`
	Provider  *string    `json:"provider,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ConversationSummary is a lightweight projection for list views.
type ConversationSummary struct {
	ID        string    `json:"id"`
	Title     *string   `json:"title,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateOptions carries optional fields for conversation creation.
type CreateOptions struct {
	Title    *string
	Model    *string
	Provider *string
}

// CreateOption is a functional option for Create.
type CreateOption func(*CreateOptions)

// WithTitle sets an initial title on the new conversation.
func WithTitle(title string) CreateOption {
	return func(o *CreateOptions) { o.Title = &title }
}

// WithModel sets the model on the new conversation.
func WithModel(model string) CreateOption {
	return func(o *CreateOptions) { o.Model = &model }
}

// WithProvider sets the provider on the new conversation.
func WithProvider(provider string) CreateOption {
	return func(o *CreateOptions) { o.Provider = &provider }
}

// Manager provides the full conversation lifecycle: CRUD operations plus
// the history management operations needed by the agent loop. It embeds
// HistoryManager to satisfy the agent.ConversationManager interface while
// adding conversation-level lifecycle methods.
//
// Manager lives in internal/conversation/ (not internal/agent/) so that the
// REST API layer can use it without importing the agent loop.
type Manager struct {
	*HistoryManager
	queries *db.Queries
	logger  *slog.Logger
	newID   func() string // injectable for testing
}

// NewManager constructs a Manager backed by the given database. The seen
// tracker is optional — nil creates a fresh session-scoped tracker.
func NewManager(database *sql.DB, seen *SeenFiles, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		HistoryManager: NewHistoryManager(database, seen),
		queries:        db.New(database),
		logger:         logger,
		newID:          sid.New,
	}
}

// Create inserts a new conversation with a UUIDv7 ID. Functional options
// allow setting an initial title, model, and provider.
func (m *Manager) Create(ctx context.Context, projectID string, opts ...CreateOption) (*Conversation, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	options := &CreateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	id := m.newID()
	now := m.now().UTC()
	timestamp := now.Format(time.RFC3339)

	params := db.InsertConversationParams{
		ID:        id,
		ProjectID: projectID,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}
	if options.Title != nil {
		params.Title = sql.NullString{String: *options.Title, Valid: true}
	}
	if options.Model != nil {
		params.Model = sql.NullString{String: *options.Model, Valid: true}
	}
	if options.Provider != nil {
		params.Provider = sql.NullString{String: *options.Provider, Valid: true}
	}

	if err := m.queries.InsertConversation(ctx, params); err != nil {
		return nil, fmt.Errorf("conversation manager: create: %w", err)
	}

	return &Conversation{
		ID:        id,
		ProjectID: projectID,
		Title:     options.Title,
		Model:     options.Model,
		Provider:  options.Provider,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Get loads a single conversation by ID. Returns an error wrapping sql.ErrNoRows
// if not found.
func (m *Manager) Get(ctx context.Context, conversationID string) (*Conversation, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	row, err := m.queries.GetConversation(ctx, conversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation manager: get %q: %w", conversationID, err)
		}
		return nil, fmt.Errorf("conversation manager: get: %w", err)
	}

	return dbConversationToConversation(row), nil
}

// List returns conversations for a project ordered by updated_at DESC.
func (m *Manager) List(ctx context.Context, projectID string, limit, offset int) ([]ConversationSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := m.queries.ListConversations(ctx, db.ListConversationsParams{
		ProjectID: projectID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("conversation manager: list: %w", err)
	}

	summaries := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		s := ConversationSummary{
			ID: row.ID,
		}
		if row.Title.Valid {
			t := row.Title.String
			s.Title = &t
		}
		if parsed, err := time.Parse(time.RFC3339, row.UpdatedAt); err == nil {
			s.UpdatedAt = parsed
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// Delete removes a conversation and all related records. SQLite foreign key
// CASCADE handles messages, tool_executions, and sub_calls.
func (m *Manager) Delete(ctx context.Context, conversationID string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := m.queries.DeleteConversation(ctx, conversationID); err != nil {
		return fmt.Errorf("conversation manager: delete %q: %w", conversationID, err)
	}
	return nil
}

// SetTitle updates the conversation's title and updated_at timestamp.
func (m *Manager) SetTitle(ctx context.Context, conversationID, title string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	timestamp := m.now().UTC().Format(time.RFC3339)
	if err := m.queries.SetConversationTitle(ctx, db.SetConversationTitleParams{
		Title:     sql.NullString{String: title, Valid: true},
		UpdatedAt: timestamp,
		ID:        conversationID,
	}); err != nil {
		return fmt.Errorf("conversation manager: set title: %w", err)
	}
	return nil
}

// Count returns the total number of conversations for a project.
func (m *Manager) Count(ctx context.Context, projectID string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return m.queries.CountConversations(ctx, projectID)
}

// dbConversationToConversation converts a sqlc-generated db.Conversation to
// the application-level Conversation type.
func dbConversationToConversation(row db.Conversation) *Conversation {
	c := &Conversation{
		ID:        row.ID,
		ProjectID: row.ProjectID,
	}
	if row.Title.Valid {
		t := row.Title.String
		c.Title = &t
	}
	if row.Model.Valid {
		m := row.Model.String
		c.Model = &m
	}
	if row.Provider.Valid {
		p := row.Provider.String
		c.Provider = &p
	}
	if parsed, err := time.Parse(time.RFC3339, row.CreatedAt); err == nil {
		c.CreatedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339, row.UpdatedAt); err == nil {
		c.UpdatedAt = parsed
	}
	return c
}
