package agent

import (
	stdctx "context"
	"errors"
	"fmt"
	"time"

	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/provider"
	toolpkg "github.com/ponchione/sodoryard/internal/tool"
)

type validatedToolCalls struct {
	toolResults    []provider.ToolResult
	validCalls     []provider.ToolCall
	validIndices   []int
	toolsCancelled bool
}

func newInflightToolTurn(req RunTurnRequest, iteration, completedIterations int, result *streamResult, assistantContentJSON string) inflightTurn {
	inflight := inflightTurn{
		ConversationID:           req.ConversationID,
		TurnNumber:               req.TurnNumber,
		Iteration:                iteration,
		CompletedIterations:      completedIterations,
		AssistantResponseStarted: true,
		AssistantMessageContent:  assistantContentJSON,
		ToolCalls:                make([]inflightToolCall, len(result.ToolCalls)),
	}
	for i, tc := range result.ToolCalls {
		inflight.ToolCalls[i] = inflightToolCall{ToolCallID: tc.ID, ToolName: tc.Name}
	}
	return inflight
}

func (l *AgentLoop) validateToolCalls(ctx stdctx.Context, turnExec *turnExecution, iteration int, result *streamResult, inflight *inflightTurn) validatedToolCalls {
	validated := validatedToolCalls{
		toolResults:  make([]provider.ToolResult, 0, len(result.ToolCalls)),
		validCalls:   make([]provider.ToolCall, 0, len(result.ToolCalls)),
		validIndices: make([]int, 0, len(result.ToolCalls)),
	}
	for idx, tc := range result.ToolCalls {
		if isCancelled(ctx) {
			validated.toolsCancelled = true
			break
		}

		turnExec.allToolCalls = append(turnExec.allToolCalls, completedToolCall{ToolName: tc.Name, Arguments: tc.Input})
		validation := validateToolCallAgainstSchema(tc, l.toolDefinitions)
		if !validation.Valid {
			l.logger.Warn("malformed tool call",
				"conversation_id", turnExec.req.ConversationID,
				"turn", turnExec.req.TurnNumber,
				"iteration", iteration,
				"tool_name", tc.Name,
				"tool_call_id", tc.ID,
				"error", validation.ErrorMessage,
			)
			l.emit(ErrorEvent{
				ErrorCode:   ErrorCodeMalformedToolCall,
				Message:     validation.ErrorMessage,
				Recoverable: true,
				Time:        l.now(),
			})
			validated.toolResults = append(validated.toolResults, provider.ToolResult{
				ToolUseID: tc.ID,
				Content:   validation.ErrorMessage,
				IsError:   true,
			})
			l.emit(ToolCallEndEvent{
				ToolCallID: tc.ID,
				Result:     validation.ErrorMessage,
				Duration:   0,
				Success:    false,
				Time:       l.now(),
			})
			continue
		}

		l.emit(ToolCallStartEvent{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Arguments:  tc.Input,
			Time:       l.now(),
		})
		inflight.ToolCalls[idx].Started = true
		validated.validCalls = append(validated.validCalls, tc)
		validated.validIndices = append(validated.validIndices, idx)
	}
	return validated
}

func (l *AgentLoop) executeValidToolCalls(
	ctx stdctx.Context,
	turnExec *turnExecution,
	iteration int,
	result *streamResult,
	inflight *inflightTurn,
	validated validatedToolCalls,
) ([]provider.ToolResult, *TurnResult, error) {
	toolResults := append([]provider.ToolResult(nil), validated.toolResults...)
	if len(validated.validCalls) == 0 {
		return toolResults, nil, nil
	}

	execCtx := toolpkg.ContextWithExecutionMeta(ctx, toolpkg.ExecutionMeta{
		ConversationID: turnExec.req.ConversationID,
		TurnNumber:     turnExec.req.TurnNumber,
		Iteration:      iteration,
	})

	batchDuration := time.Duration(0)
	var batchResults []provider.ToolResult
	var batchErr error
	if batchExecutor, ok := l.toolExecutor.(BatchToolExecutor); ok {
		batchStart := l.now()
		batchResults, batchErr = batchExecutor.ExecuteBatch(execCtx, validated.validCalls)
		batchDuration = l.now().Sub(batchStart)
	} else {
		batchResults = make([]provider.ToolResult, 0, len(validated.validCalls))
		for _, tc := range validated.validCalls {
			toolStart := l.now()
			toolResult, toolErr := l.toolExecutor.Execute(execCtx, tc)
			batchDuration = l.now().Sub(toolStart)
			if toolErr != nil {
				if errors.Is(toolErr, toolpkg.ErrChainComplete) {
					return nil, finalTurnResult(turnExec.turnCtx, result.TextContent, iteration, turnExec.totalUsage, l.now().Sub(turnExec.turnStart)), nil
				}
				enrichedMsg := enrichToolError(tc.Name, toolErr)
				toolResult = &provider.ToolResult{
					ToolUseID: tc.ID,
					Content:   enrichedMsg,
					IsError:   true,
				}
				l.emit(ErrorEvent{
					ErrorCode:   ErrorCodeToolExecution,
					Message:     enrichedMsg,
					Recoverable: true,
					Time:        l.now(),
				})
			}
			batchResults = append(batchResults, *toolResult)
			if isCancelled(ctx) {
				return nil, nil, l.handleTurnCancellation(*inflight, ctx.Err())
			}
		}
	}

	if isCancelled(ctx) {
		return nil, nil, l.handleTurnCancellation(*inflight, ctx.Err())
	}
	if batchErr == nil && len(batchResults) != len(validated.validCalls) {
		batchErr = fmt.Errorf("tool executor returned %d batch results for %d calls", len(batchResults), len(validated.validCalls))
	}

	if batchErr != nil {
		if errors.Is(batchErr, toolpkg.ErrChainComplete) {
			return nil, finalTurnResult(turnExec.turnCtx, result.TextContent, iteration, turnExec.totalUsage, l.now().Sub(turnExec.turnStart)), nil
		}
		for i, tc := range validated.validCalls {
			enrichedMsg := enrichToolError(tc.Name, batchErr)
			toolResult := provider.ToolResult{ToolUseID: tc.ID, Content: enrichedMsg, IsError: true}
			toolResults = append(toolResults, toolResult)
			idx := validated.validIndices[i]
			inflight.ToolCalls[idx].Completed = true
			inflight.ToolCalls[idx].ResultStored = true
			inflight.ToolMessages = append(inflight.ToolMessages, conversation.IterationMessage{
				Role:      "tool",
				Content:   toolResult.Content,
				ToolUseID: tc.ID,
				ToolName:  tc.Name,
			})
			l.emit(ErrorEvent{
				ErrorCode:   ErrorCodeToolExecution,
				Message:     enrichedMsg,
				Recoverable: true,
				Time:        l.now(),
			})
			l.emit(ToolCallOutputEvent{ToolCallID: tc.ID, Output: toolResult.Content, Time: l.now()})
			l.emit(ToolCallEndEvent{ToolCallID: tc.ID, Result: toolResult.Content, Duration: batchDuration, Success: false, Time: l.now()})
		}
		return toolResults, nil, nil
	}

	for i, tc := range validated.validCalls {
		toolResult := batchResults[i]
		toolResult.ToolUseID = tc.ID
		toolResults = append(toolResults, toolResult)
		idx := validated.validIndices[i]
		inflight.ToolCalls[idx].Completed = true
		inflight.ToolCalls[idx].ResultStored = true
		inflight.ToolMessages = append(inflight.ToolMessages, conversation.IterationMessage{
			Role:      "tool",
			Content:   toolResult.Content,
			ToolUseID: tc.ID,
			ToolName:  tc.Name,
		})
		l.emit(ToolCallOutputEvent{ToolCallID: tc.ID, Output: toolResult.Content, Time: l.now()})
		l.emit(ToolCallEndEvent{ToolCallID: tc.ID, Result: toolResult.Content, Duration: batchDuration, Success: !toolResult.IsError, Time: l.now()})
	}
	return toolResults, nil, nil
}
