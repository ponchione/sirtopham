package agent

import (
	stdctx "context"
	"fmt"
)

func cleanupReasonEventValue(reason turnCleanupReason) string {
	switch reason {
	case cleanupReasonInterrupt:
		return "user_interrupted"
	case cleanupReasonDeadlineExceeded:
		return "context_deadline_exceeded"
	case cleanupReasonStreamFailure:
		return "stream_failure"
	default:
		return "user_cancelled"
	}
}

// handleCancellation performs post-cancellation cleanup for any in-flight
// assistant/tool state, emits TurnCancelledEvent and StatusEvent(StateIdle),
// and returns ErrTurnCancelled wrapping the underlying cause.
func (l *AgentLoop) handleCancellation(conversationID string, turnNumber, currentIteration, completedIterations int, cause error) error {
	cleanupTurn := cleanupInflightTurn(conversationID, turnNumber, currentIteration, completedIterations)
	cleanupTurn.AssistantResponseStarted = currentIteration > 0
	return l.handleTurnCancellation(cleanupTurn, cause)
}

func (l *AgentLoop) handleIterationSetupCancellation(conversationID string, turnNumber, currentIteration, completedIterations int, cause error) error {
	return l.handleTurnCancellation(cleanupInflightTurn(conversationID, turnNumber, currentIteration, completedIterations), cause)
}

func (l *AgentLoop) handleTurnCancellation(turn inflightTurn, cause error) error {
	cleanupReason := l.cancellationReason(cause)
	return l.handleTurnCleanup(turn, cleanupReason, cause)
}

func (l *AgentLoop) handleTurnStreamFailure(turn inflightTurn, cause error) error {
	return l.handleTurnCleanup(turn, cleanupReasonStreamFailure, cause)
}

func (l *AgentLoop) handleTurnCleanup(turn inflightTurn, cleanupReason turnCleanupReason, cause error) error {
	plan := buildCleanupPlan(turn, cleanupReason)
	reason := cleanupReasonEventValue(plan.Reason)

	l.logTurnCleanup(turn, reason, plan)
	l.applyCleanupPlanBestEffort(turn, plan)
	l.emitTurnCleanupEvents(turn, reason)
	return cleanupReturnError(cause)
}

func (l *AgentLoop) logTurnCleanup(turn inflightTurn, reason string, plan cleanupPlan) {
	l.logger.Warn("turn cancelled",
		"conversation_id", turn.ConversationID,
		"turn", turn.TurnNumber,
		"current_iteration", turn.Iteration,
		"completed_iterations", turn.CompletedIterations,
		"reason", reason,
		"cleanup_actions", len(plan.Actions),
	)
}

func (l *AgentLoop) applyCleanupPlanBestEffort(turn inflightTurn, plan cleanupPlan) {
	if len(plan.Actions) == 0 {
		return
	}
	cleanupCtx := stdctx.Background()
	if err := l.applyCleanupPlan(cleanupCtx, turn, plan); err != nil {
		l.logger.Error("failed to apply cancellation cleanup plan",
			"conversation_id", turn.ConversationID,
			"turn", turn.TurnNumber,
			"iteration", turn.Iteration,
			"error", err,
		)
	}
}

func (l *AgentLoop) emitTurnCleanupEvents(turn inflightTurn, reason string) {
	l.emit(TurnCancelledEvent{
		TurnNumber:          turn.TurnNumber,
		CompletedIterations: turn.CompletedIterations,
		Reason:              reason,
		Time:                l.now(),
	})
	l.emit(StatusEvent{State: StateIdle, Time: l.now()})
}

func cleanupReturnError(cause error) error {
	if cause != nil {
		return fmt.Errorf("%w: %v", ErrTurnCancelled, cause)
	}
	return ErrTurnCancelled
}
