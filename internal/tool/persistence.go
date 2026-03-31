package tool

import (
	"context"
	"database/sql"
	"time"

	"github.com/ponchione/sirtopham/internal/db"
)

// ExecutionMeta carries the conversation context needed for tool_executions
// persistence. Passed to ExecuteWithMeta to record analytics rows.
type ExecutionMeta struct {
	ConversationID string
	TurnNumber     int
	Iteration      int
}

// ToolExecutionRecorder persists tool execution analytics. When nil on the
// Executor, persistence is silently skipped (useful for testing).
type ToolExecutionRecorder struct {
	queries *db.Queries
}

// NewToolExecutionRecorder creates a recorder backed by sqlc queries.
func NewToolExecutionRecorder(queries *db.Queries) *ToolExecutionRecorder {
	if queries == nil {
		return nil
	}
	return &ToolExecutionRecorder{queries: queries}
}

// Record inserts a tool_executions row. Errors are returned to the caller
// (the executor logs and swallows them).
func (r *ToolExecutionRecorder) Record(ctx context.Context, call ToolCall, result ToolResult, meta ExecutionMeta, now time.Time) error {
	if r == nil || r.queries == nil {
		return nil
	}

	var errStr sql.NullString
	if result.Error != "" {
		errStr = sql.NullString{String: result.Error, Valid: true}
	}

	var inputStr sql.NullString
	if len(call.Arguments) > 0 {
		inputStr = sql.NullString{String: string(call.Arguments), Valid: true}
	}

	var success int64
	if result.Success {
		success = 1
	}

	return r.queries.InsertToolExecution(ctx, db.InsertToolExecutionParams{
		ConversationID: meta.ConversationID,
		TurnNumber:     int64(meta.TurnNumber),
		Iteration:      int64(meta.Iteration),
		ToolUseID:      call.ID,
		ToolName:       call.Name,
		Input:          inputStr,
		OutputSize:     sql.NullInt64{Int64: int64(len(result.Content)), Valid: true},
		Error:          errStr,
		Success:        success,
		DurationMs:     result.DurationMs,
		CreatedAt:      now.UTC().Format(time.RFC3339),
	})
}
