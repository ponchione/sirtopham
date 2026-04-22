package agent

import (
	stdctx "context"
	"fmt"

	"github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/provider"
)

type iterationExecution struct {
	number       int
	history      []db.Message
	disableTools bool
	promptReq    *provider.Request
}

func (l *AgentLoop) prepareIteration(ctx stdctx.Context, turnExec *turnExecution, iteration int) (*iterationExecution, error) {
	history, err := l.conversationManager.ReconstructHistory(ctx, turnExec.req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("agent loop: reconstruct history for iteration %d: %w", iteration, err)
	}

	iterExec := &iterationExecution{
		number:       iteration,
		history:      history,
		disableTools: l.cfg.MaxIterations > 0 && iteration >= l.cfg.MaxIterations,
	}
	if iterExec.disableTools {
		turnExec.currentTurnMessages = append(turnExec.currentTurnMessages, provider.NewUserMessage(loopDirectiveMessage))
		l.logger.Warn("final iteration reached, disabling tools",
			"conversation_id", turnExec.req.ConversationID,
			"turn", turnExec.req.TurnNumber,
			"iteration", iteration,
			"max_iterations", l.cfg.MaxIterations,
		)
	}

	promptReq, err := l.buildIterationRequest(ctx, turnExec, iterExec)
	if err != nil {
		return nil, err
	}
	iterExec.promptReq = promptReq
	return iterExec, nil
}

func (l *AgentLoop) buildIterationRequest(ctx stdctx.Context, turnExec *turnExecution, iterExec *iterationExecution) (*provider.Request, error) {
	promptReq, err := l.promptBuilder.BuildPrompt(l.buildPromptConfig(
		turnExec.turnCtx.ContextPackage,
		iterExec.history,
		turnExec.currentTurnMessages,
		turnExec.effectiveProvider,
		turnExec.effectiveModel,
		turnExec.req.ModelContextLimit,
		iterExec.disableTools,
		turnExec.req.ConversationID,
		turnExec.req.TurnNumber,
		iterExec.number,
	))
	if err != nil {
		return nil, fmt.Errorf("agent loop: build prompt for iteration %d: %w", iterExec.number, err)
	}

	if !l.tryPreflightCompression(ctx, turnExec.req.ConversationID, promptReq, turnExec.req.ModelContextLimit) {
		return promptReq, nil
	}

	history, err := l.conversationManager.ReconstructHistory(ctx, turnExec.req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("agent loop: reconstruct history after compression in iteration %d: %w", iterExec.number, err)
	}
	iterExec.history = history

	promptReq, err = l.promptBuilder.BuildPrompt(l.buildPromptConfig(
		turnExec.turnCtx.ContextPackage,
		iterExec.history,
		turnExec.currentTurnMessages,
		turnExec.effectiveProvider,
		turnExec.effectiveModel,
		turnExec.req.ModelContextLimit,
		iterExec.disableTools,
		turnExec.req.ConversationID,
		turnExec.req.TurnNumber,
		iterExec.number,
	))
	if err != nil {
		return nil, fmt.Errorf("agent loop: rebuild prompt after compression in iteration %d: %w", iterExec.number, err)
	}
	return promptReq, nil
}

func (l *AgentLoop) runProviderIteration(ctx stdctx.Context, turnExec *turnExecution, iterExec *iterationExecution) (*streamResult, error) {
	l.emit(StatusEvent{State: StateWaitingForLLM, Time: l.now()})

	result, err := l.streamWithRetry(ctx, iterExec.promptReq, iterExec.number, turnExec.req.ConversationID)
	if err != nil {
		if isCancelled(ctx) {
			if result != nil {
				cleanupTurn := cleanupInflightTurnBase(turnExec, iterExec.number)
				cleanupTurn.AssistantResponseStarted = result.TextContent != "" || len(result.ContentBlocks) > 0
				cleanupTurn.AssistantMessageContent = assistantContentJSONForCleanup(result)
				return nil, l.handleTurnCancellation(cleanupTurn, ctx.Err())
			}
			return nil, l.handleIterationSetupCancellation(turnExec.req.ConversationID, turnExec.req.TurnNumber, iterExec.number, turnExec.completedIterations, ctx.Err())
		}

		if l.isContextOverflowError(err) {
			if retryResult, retryErr := l.tryEmergencyCompression(ctx, turnExec.req, turnExec.turnCtx, turnExec.currentTurnMessages, iterExec.number, iterExec.disableTools); retryResult != nil || retryErr != nil {
				if retryErr != nil {
					return nil, retryErr
				}
				result = retryResult
				err = nil
			}
		}
		if err != nil {
			if result != nil && (result.TextContent != "" || len(result.ContentBlocks) > 0) {
				cleanupTurn := cleanupInflightTurnBase(turnExec, iterExec.number)
				cleanupTurn.AssistantResponseStarted = true
				cleanupTurn.AssistantMessageContent = assistantContentJSONForCleanup(result)
				return nil, l.handleTurnStreamFailure(cleanupTurn, err)
			}
			return nil, err
		}
	}

	turnExec.totalUsage = turnExec.totalUsage.Add(result.Usage)
	l.tryPostResponseCompression(ctx, turnExec.req.ConversationID, result.Usage.InputTokens, turnExec.req.ModelContextLimit)
	return result, nil
}

func assistantContentJSONForCleanup(result *streamResult) string {
	if result == nil || len(result.ContentBlocks) == 0 {
		return ""
	}
	assistantContentJSON, _ := contentBlocksToJSON(sanitizeContentBlocks(result.ContentBlocks))
	return assistantContentJSON
}
