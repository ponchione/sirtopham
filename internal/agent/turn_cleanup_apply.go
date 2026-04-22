package agent

import (
	stdctx "context"
	"fmt"
)

func (l *AgentLoop) applyCleanupPlan(ctx stdctx.Context, turn inflightTurn, plan cleanupPlan) error {
	for _, action := range plan.Actions {
		if err := l.applyCleanupAction(ctx, turn, action); err != nil {
			return err
		}
	}
	return nil
}

func (l *AgentLoop) applyCleanupAction(ctx stdctx.Context, turn inflightTurn, action cleanupAction) error {
	switch action.Kind {
	case cleanupActionCancelIteration:
		if err := l.conversationManager.CancelIteration(ctx, turn.ConversationID, turn.TurnNumber, action.Iteration); err != nil {
			return fmt.Errorf("cancel iteration %d: %w", action.Iteration, err)
		}
	case cleanupActionPersistIteration:
		if err := l.conversationManager.PersistIteration(ctx, turn.ConversationID, turn.TurnNumber, action.Iteration, action.Messages); err != nil {
			return fmt.Errorf("persist interrupted iteration %d: %w", action.Iteration, err)
		}
	default:
		return fmt.Errorf("unknown cleanup action kind %q", action.Kind)
	}
	return nil
}
