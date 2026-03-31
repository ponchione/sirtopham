package conversation

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ponchione/sirtopham/internal/provider"
)

const titleSystemPrompt = `Generate a short, descriptive title (5-8 words) for a conversation that starts with the following user message. Return only the title text, no quotes or formatting.`

const titleMaxTokens = 50

// TitleProvider is the narrow interface needed by title generation — a
// non-streaming LLM call. The provider.Router satisfies this via its
// Complete method, or any single-provider can be used directly.
type TitleProvider interface {
	Complete(ctx context.Context, req *provider.Request) (*provider.Response, error)
}

// TitleGen generates conversation titles via a lightweight LLM call.
// It satisfies the agent.TitleGenerator interface:
//
//	GenerateTitle(ctx context.Context, conversationID string)
//
// The generator is fire-and-forget: errors are logged but never propagated.
type TitleGen struct {
	manager  *Manager
	provider TitleProvider
	logger   *slog.Logger
	model    string
}

// NewTitleGen constructs a title generator. The model parameter selects which
// model to use for title generation (typically a fast, cheap model).
func NewTitleGen(manager *Manager, provider TitleProvider, model string, logger *slog.Logger) *TitleGen {
	if logger == nil {
		logger = slog.Default()
	}
	return &TitleGen{
		manager:  manager,
		provider: provider,
		logger:   logger,
		model:    model,
	}
}

// GenerateTitle implements agent.TitleGenerator. It makes a non-streaming LLM
// call to generate a title from the first user message in the conversation,
// then persists it via SetTitle. Errors are logged, never propagated.
func (g *TitleGen) GenerateTitle(ctx context.Context, conversationID string) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Reconstruct history to find the first user message.
	messages, err := g.manager.ReconstructHistory(ctx, conversationID)
	if err != nil {
		g.logger.Warn("title generation: failed to reconstruct history",
			"conversation_id", conversationID,
			"error", err,
		)
		return
	}

	// Find the first user message.
	var firstMessage string
	for _, msg := range messages {
		if msg.Role == "user" && msg.Content.Valid {
			firstMessage = msg.Content.String
			break
		}
	}
	if firstMessage == "" {
		g.logger.Warn("title generation: no user message found",
			"conversation_id", conversationID,
		)
		return
	}

	// Make a lightweight LLM call.
	req := &provider.Request{
		SystemBlocks: []provider.SystemBlock{
			{Text: titleSystemPrompt},
		},
		Messages: []provider.Message{
			provider.NewUserMessage(firstMessage),
		},
		Model:          g.model,
		MaxTokens:      titleMaxTokens,
		Purpose:        "title_generation",
		ConversationID: conversationID,
	}

	resp, err := g.provider.Complete(ctx, req)
	if err != nil {
		g.logger.Warn("title generation: LLM call failed",
			"conversation_id", conversationID,
			"error", err,
		)
		return
	}

	title := cleanTitle(extractText(resp))
	if title == "" {
		g.logger.Warn("title generation: empty title returned",
			"conversation_id", conversationID,
		)
		return
	}

	if err := g.manager.SetTitle(ctx, conversationID, title); err != nil {
		g.logger.Warn("title generation: failed to persist title",
			"conversation_id", conversationID,
			"title", title,
			"error", err,
		)
		return
	}

	g.logger.Info("title generated",
		"conversation_id", conversationID,
		"title", title,
	)
}

// cleanTitle trims whitespace and surrounding quotes from a generated title.
func cleanTitle(raw string) string {
	title := strings.TrimSpace(raw)
	// Strip surrounding quotes (models often wrap titles in them).
	for _, q := range []string{`"`, `'`, "`"} {
		title = strings.TrimPrefix(title, q)
		title = strings.TrimSuffix(title, q)
	}
	title = strings.TrimSpace(title)

	// Truncate overly long titles.
	if len(title) > 100 {
		title = title[:100]
		if lastSpace := strings.LastIndex(title, " "); lastSpace > 50 {
			title = title[:lastSpace]
		}
	}
	return title
}

// extractText concatenates all text content blocks from a response.
func extractText(resp *provider.Response) string {
	if resp == nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}
