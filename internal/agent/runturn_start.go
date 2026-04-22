package agent

import (
	stdctx "context"
	"fmt"
	"time"
)

func (l *AgentLoop) prepareRunTurn(ctx stdctx.Context, req RunTurnRequest) (_ *preparedTurn, err error) {
	if err := l.validate(); err != nil {
		return nil, err
	}
	if l.providerRouter == nil {
		return nil, fmt.Errorf("agent loop: provider router is nil")
	}
	if l.toolExecutor == nil {
		return nil, fmt.Errorf("agent loop: tool executor is nil")
	}
	if err := validateRunTurnRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := stdctx.WithCancel(ctx)
	l.setCancel(cancel)
	cleanup := func() {
		cancel()
		l.clearCancel()
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	if err := l.persistInitialUserMessage(ctx, req); err != nil {
		return nil, err
	}

	turnExec, err := l.prepareTurnExecution(ctx, req, l.now())
	if err != nil {
		return nil, err
	}

	return &preparedTurn{
		ctx:     ctx,
		exec:    turnExec,
		cleanup: cleanup,
	}, nil
}

func (l *AgentLoop) persistInitialUserMessage(ctx stdctx.Context, req RunTurnRequest) error {
	if err := l.conversationManager.PersistUserMessage(ctx, req.ConversationID, req.TurnNumber, req.Message); err != nil {
		if isCancelled(ctx) {
			return l.handleCancellation(req.ConversationID, req.TurnNumber, 0, 0, ctx.Err())
		}
		wrapped := fmt.Errorf("agent loop: persist user message: %w", err)
		l.emit(ErrorEvent{
			ErrorCode:   "persist_user_message_failed",
			Message:     wrapped.Error(),
			Recoverable: false,
			Time:        l.now(),
		})
		return wrapped
	}
	return nil
}

func (l *AgentLoop) prepareTurnExecution(ctx stdctx.Context, req RunTurnRequest, turnStart time.Time) (*turnExecution, error) {
	turnCtx, err := l.PrepareTurnContext(
		ctx,
		req.ConversationID,
		req.TurnNumber,
		req.Message,
		req.ModelContextLimit,
		req.HistoryTokenCount,
	)
	if err != nil {
		if isCancelled(ctx) {
			return nil, l.handleCancellation(req.ConversationID, req.TurnNumber, 0, 0, ctx.Err())
		}
		return nil, err
	}
	return l.newTurnExecution(req, turnCtx, turnStart), nil
}
