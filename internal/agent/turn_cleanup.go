package agent

import "github.com/ponchione/sodoryard/internal/conversation"

type turnCleanupReason string

const (
	cleanupReasonCancel           turnCleanupReason = "cancel"
	cleanupReasonInterrupt        turnCleanupReason = "interrupt"
	cleanupReasonDeadlineExceeded turnCleanupReason = "context_deadline_exceeded"
	cleanupReasonStreamFailure    turnCleanupReason = "stream_failure"
)

const (
	cleanupActionCancelIteration  = "cancel_iteration"
	cleanupActionPersistIteration = "persist_iteration"
)

type inflightToolCall struct {
	ToolCallID   string
	ToolName     string
	Started      bool
	Completed    bool
	ResultStored bool
}

type inflightTurn struct {
	ConversationID           string
	TurnNumber               int
	Iteration                int
	CompletedIterations      int
	AssistantResponseStarted bool
	AssistantResponseStored  bool
	AssistantMessageContent  string
	ToolCalls                []inflightToolCall
	ToolMessages             []conversation.IterationMessage
}

type cleanupAction struct {
	Kind      string
	Iteration int
	Messages  []conversation.IterationMessage
}

type cleanupPlan struct {
	Reason  turnCleanupReason
	Actions []cleanupAction
}
