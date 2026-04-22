package agent

import (
	stdctx "context"
	"encoding/json"
	"fmt"

	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/provider"
)

func (l *AgentLoop) applyToolResultBudget(ctx stdctx.Context, req RunTurnRequest, iteration int, toolResults []provider.ToolResult, toolCalls []provider.ToolCall) []provider.ToolResult {
	managedToolResults := l.toolOutputManager.ApplyAggregateBudget(ctx, toolResults, toolCalls, l.cfg.MaxToolResultsPerMessageChars)
	toolResults = managedToolResults.Results
	budgetReport := managedToolResults.Report
	if budgetReport.ReplacedResults > 0 {
		l.logger.Debug("aggregate tool-result budget applied",
			"conversation_id", req.ConversationID,
			"turn", req.TurnNumber,
			"iteration", iteration,
			"replaced_results", budgetReport.ReplacedResults,
			"persisted_results", budgetReport.PersistedResults,
			"inline_shrunk_results", budgetReport.InlineShrunkResults,
			"chars_saved", budgetReport.CharsSaved,
			"original_chars", budgetReport.OriginalChars,
			"final_chars", budgetReport.FinalChars,
			"max_chars", budgetReport.MaxChars,
		)
	}
	return toolResults
}

func buildIterationPersistMessages(assistantContentJSON string, toolResults []provider.ToolResult, toolCalls []provider.ToolCall) []conversation.IterationMessage {
	persistMessages := []conversation.IterationMessage{{
		Role:    "assistant",
		Content: assistantContentJSON,
	}}
	for _, tr := range toolResults {
		persistMessages = append(persistMessages, conversation.IterationMessage{
			Role:      "tool",
			Content:   tr.Content,
			ToolUseID: tr.ToolUseID,
			ToolName:  toolNameFromResults(toolCalls, tr.ToolUseID),
		})
	}
	return persistMessages
}

func (l *AgentLoop) persistToolIteration(ctx stdctx.Context, turnExec *turnExecution, iteration int, assistantContentJSON string, persistMessages []conversation.IterationMessage) error {
	cleanupTurn := cleanupInflightTurnBase(turnExec, iteration)
	cleanupTurn.AssistantResponseStarted = true
	cleanupTurn.AssistantMessageContent = assistantContentJSON
	cleanupTurn.ToolMessages = append([]conversation.IterationMessage(nil), persistMessages[1:]...)
	if err := l.conversationManager.PersistIteration(ctx, turnExec.req.ConversationID, turnExec.req.TurnNumber, iteration, persistMessages); err != nil {
		if isCancelled(ctx) {
			return l.handleTurnCancellation(cleanupTurn, ctx.Err())
		}
		return fmt.Errorf("agent loop: persist iteration %d: %w", iteration, err)
	}
	return nil
}

func (l *AgentLoop) completeToolIteration(
	ctx stdctx.Context,
	turnExec *turnExecution,
	iteration int,
	assistantContentJSON string,
	toolCalls []provider.ToolCall,
	toolResults []provider.ToolResult,
) error {
	toolResults = l.applyToolResultBudget(ctx, turnExec.req, iteration, toolResults, toolCalls)
	persistMessages := buildIterationPersistMessages(assistantContentJSON, toolResults, toolCalls)
	if err := l.persistToolIteration(ctx, turnExec, iteration, assistantContentJSON, persistMessages); err != nil {
		return err
	}
	turnExec.completedIterations = iteration
	appendIterationMessages(turnExec, assistantContentJSON, toolResults, toolCalls)
	l.injectLoopNudgeIfNeeded(turnExec, iteration, toolCalls)
	return nil
}

func appendIterationMessages(turnExec *turnExecution, assistantContentJSON string, toolResults []provider.ToolResult, toolCalls []provider.ToolCall) {
	assistantMsg := provider.Message{
		Role:    provider.RoleAssistant,
		Content: json.RawMessage(assistantContentJSON),
	}
	turnExec.currentTurnMessages = append(turnExec.currentTurnMessages, assistantMsg)
	for _, tr := range toolResults {
		turnExec.currentTurnMessages = append(turnExec.currentTurnMessages, provider.NewToolResultMessage(
			tr.ToolUseID,
			toolNameFromResults(toolCalls, tr.ToolUseID),
			tr.Content,
		))
	}
}

func (l *AgentLoop) injectLoopNudgeIfNeeded(turnExec *turnExecution, iteration int, toolCalls []provider.ToolCall) {
	turnExec.detector.record(toolCalls)
	if !turnExec.detector.isLooping() {
		return
	}
	l.logger.Warn("loop detected — injecting nudge",
		"conversation_id", turnExec.req.ConversationID,
		"turn", turnExec.req.TurnNumber,
		"iteration", iteration,
		"threshold", l.cfg.LoopDetectionThreshold,
	)
	turnExec.currentTurnMessages = append(turnExec.currentTurnMessages, provider.NewUserMessage(loopNudgeMessage))
}
