package agent

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/provider"
)

func TestBuildCleanupPlanSkipsCompletedIteration(t *testing.T) {
	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                1,
		CompletedIterations:      1,
		AssistantResponseStarted: true,
	}, cleanupReasonCancel)
	if len(plan.Actions) != 0 {
		t.Fatalf("cleanup actions = %#v, want none", plan.Actions)
	}
}

func TestBuildCleanupPlanSkipsUnmaterializedIterationSetupCancellation(t *testing.T) {
	plan := buildCleanupPlan(inflightTurn{
		ConversationID:      "conv-1",
		TurnNumber:          2,
		Iteration:           1,
		CompletedIterations: 0,
	}, cleanupReasonCancel)
	if len(plan.Actions) != 0 {
		t.Fatalf("cleanup actions = %#v, want none for cancellation before any assistant/tool state existed", plan.Actions)
	}
}

func TestBuildCleanupPlanPersistsInterruptedAssistantMessage(t *testing.T) {
	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                3,
		CompletedIterations:      2,
		AssistantResponseStarted: true,
		AssistantMessageContent:  `[{"type":"text","text":"partial"}]`,
	}, cleanupReasonInterrupt)
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != cleanupActionPersistIteration {
		t.Fatalf("cleanup actions = %#v, want one persist_iteration", plan.Actions)
	}
	if len(plan.Actions[0].Messages) != 1 || plan.Actions[0].Messages[0].Role != "assistant" {
		t.Fatalf("persisted messages = %#v, want one assistant message", plan.Actions[0].Messages)
	}
	if !strings.Contains(plan.Actions[0].Messages[0].Content, "[interrupted_assistant]") {
		t.Fatalf("assistant tombstone content = %q, want interrupted assistant marker", plan.Actions[0].Messages[0].Content)
	}
}

func TestBuildCleanupPlanPersistsFailedAssistantMessageForStreamFailure(t *testing.T) {
	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                3,
		CompletedIterations:      2,
		AssistantResponseStarted: true,
		AssistantMessageContent:  `[{"type":"text","text":"partial"}]`,
	}, cleanupReasonStreamFailure)
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != cleanupActionPersistIteration {
		t.Fatalf("cleanup actions = %#v, want one persist_iteration", plan.Actions)
	}
	if !strings.Contains(plan.Actions[0].Messages[0].Content, "[failed_assistant]") {
		t.Fatalf("assistant tombstone content = %q, want failed assistant marker", plan.Actions[0].Messages[0].Content)
	}
	if !strings.Contains(plan.Actions[0].Messages[0].Content, "reason=stream_failure") {
		t.Fatalf("assistant tombstone content = %q, want stream_failure reason", plan.Actions[0].Messages[0].Content)
	}
}

func TestBuildCleanupPlanPersistsInterruptedAssistantMessageWhenFirstBlockIsThinking(t *testing.T) {
	raw, err := json.Marshal([]provider.ContentBlock{
		provider.NewThinkingBlock("reasoning"),
		provider.NewTextBlock("partial"),
	})
	if err != nil {
		t.Fatalf("marshal content blocks: %v", err)
	}

	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                3,
		CompletedIterations:      2,
		AssistantResponseStarted: true,
		AssistantMessageContent:  string(raw),
	}, cleanupReasonInterrupt)
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != cleanupActionPersistIteration {
		t.Fatalf("cleanup actions = %#v, want one persist_iteration", plan.Actions)
	}
	blocks, err := provider.ContentBlocksFromRaw(json.RawMessage(plan.Actions[0].Messages[0].Content))
	if err != nil {
		t.Fatalf("ContentBlocksFromRaw: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("persisted assistant blocks = %#v, want 2", blocks)
	}
	if blocks[0].Type != "thinking" || blocks[0].Thinking != "reasoning" {
		t.Fatalf("first block = %#v, want original thinking block preserved", blocks[0])
	}
	if blocks[1].Type != "text" || !strings.Contains(blocks[1].Text, "[interrupted_assistant]") || !strings.Contains(blocks[1].Text, "partial_text=partial") {
		t.Fatalf("second block = %#v, want interrupted tombstone text preserving partial text", blocks[1])
	}
}

func TestBuildCleanupPlanPersistsInterruptedAssistantMessageWhenOnlyThinkingBlockExists(t *testing.T) {
	raw, err := json.Marshal([]provider.ContentBlock{
		provider.NewThinkingBlock("reasoning"),
	})
	if err != nil {
		t.Fatalf("marshal content blocks: %v", err)
	}

	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                3,
		CompletedIterations:      2,
		AssistantResponseStarted: true,
		AssistantMessageContent:  string(raw),
	}, cleanupReasonInterrupt)
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != cleanupActionPersistIteration {
		t.Fatalf("cleanup actions = %#v, want one persist_iteration", plan.Actions)
	}
	blocks, err := provider.ContentBlocksFromRaw(json.RawMessage(plan.Actions[0].Messages[0].Content))
	if err != nil {
		t.Fatalf("ContentBlocksFromRaw: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("persisted assistant blocks = %#v, want original thinking plus appended tombstone text", blocks)
	}
	if blocks[0].Type != "thinking" || blocks[0].Thinking != "reasoning" {
		t.Fatalf("first block = %#v, want original thinking block preserved", blocks[0])
	}
	if blocks[1].Type != "text" || !strings.Contains(blocks[1].Text, "[interrupted_assistant]") || strings.Contains(blocks[1].Text, "partial_text=") {
		t.Fatalf("second block = %#v, want appended interrupted tombstone text without partial_text", blocks[1])
	}
}

func TestBuildCleanupPlanPersistsInterruptedToolResults(t *testing.T) {
	plan := buildCleanupPlan(inflightTurn{
		ConversationID:           "conv-1",
		TurnNumber:               2,
		Iteration:                3,
		CompletedIterations:      2,
		AssistantResponseStarted: true,
		AssistantMessageContent:  `[{"type":"tool_use","id":"tool-1","name":"shell","input":{}}]`,
		ToolCalls: []inflightToolCall{{
			ToolCallID: "tool-1",
			ToolName:   "shell",
			Started:    true,
		}},
	}, cleanupReasonInterrupt)
	if plan.Reason != cleanupReasonInterrupt {
		t.Fatalf("plan reason = %q, want %q", plan.Reason, cleanupReasonInterrupt)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("cleanup action count = %d, want 1", len(plan.Actions))
	}
	if plan.Actions[0].Kind != cleanupActionPersistIteration || plan.Actions[0].Iteration != 3 {
		t.Fatalf("cleanup action = %#v, want persist_iteration for iter 3", plan.Actions[0])
	}
	if len(plan.Actions[0].Messages) != 2 {
		t.Fatalf("persisted message count = %d, want 2", len(plan.Actions[0].Messages))
	}
	if plan.Actions[0].Messages[1].ToolUseID != "tool-1" {
		t.Fatalf("tool result = %#v, want tool_use_id tool-1", plan.Actions[0].Messages[1])
	}
}

func TestApplyCleanupPlanCancelsInflightIteration(t *testing.T) {
	conversations := &loopConversationManagerStub{}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})

	turn := inflightTurn{ConversationID: "conv-2", TurnNumber: 4, Iteration: 2}
	plan := cleanupPlan{
		Reason: cleanupReasonCancel,
		Actions: []cleanupAction{{
			Kind:      cleanupActionCancelIteration,
			Iteration: 2,
		}},
	}
	if err := loop.applyCleanupPlan(stdctx.Background(), turn, plan); err != nil {
		t.Fatalf("applyCleanupPlan returned error: %v", err)
	}
	if len(conversations.cancelIterCalls) != 1 {
		t.Fatalf("CancelIteration calls = %d, want 1", len(conversations.cancelIterCalls))
	}
	if got := conversations.cancelIterCalls[0]; got.conversationID != "conv-2" || got.turnNumber != 4 || got.iteration != 2 {
		t.Fatalf("CancelIteration call = %+v, want conv-2/4/2", got)
	}
}

func TestApplyCleanupPlanPersistsInterruptedIteration(t *testing.T) {
	conversations := &loopConversationManagerStub{}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})

	turn := inflightTurn{ConversationID: "conv-3", TurnNumber: 5, Iteration: 2}
	plan := cleanupPlan{
		Reason: cleanupReasonInterrupt,
		Actions: []cleanupAction{{
			Kind:      cleanupActionPersistIteration,
			Iteration: 2,
			Messages:  []conversation.IterationMessage{{Role: "assistant", Content: `[{"type":"tool_use"}]`}, {Role: "tool", Content: "[interrupted_tool_result]", ToolUseID: "tool-1", ToolName: "shell"}},
		}},
	}
	if err := loop.applyCleanupPlan(stdctx.Background(), turn, plan); err != nil {
		t.Fatalf("applyCleanupPlan returned error: %v", err)
	}
	if len(conversations.persistIterCalls) != 1 {
		t.Fatalf("PersistIteration calls = %d, want 1", len(conversations.persistIterCalls))
	}
	if got := conversations.persistIterCalls[0]; got.conversationID != "conv-3" || got.turnNumber != 5 || got.iteration != 2 || len(got.messages) != 2 {
		t.Fatalf("PersistIteration call = %+v, want conv-3/5/2 with 2 messages", got)
	}
}

func TestCancellationReasonUsesInterruptForLoopCancel(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: &loopConversationManagerStub{},
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})
	loop.interruptRequested = true

	if got := loop.cancellationReason(stdctx.Canceled); got != cleanupReasonInterrupt {
		t.Fatalf("cancellationReason = %q, want %q", got, cleanupReasonInterrupt)
	}
}

func TestCleanupReasonEventValue(t *testing.T) {
	tests := []struct {
		name   string
		reason turnCleanupReason
		want   string
	}{
		{name: "interrupt", reason: cleanupReasonInterrupt, want: "user_interrupted"},
		{name: "deadline", reason: cleanupReasonDeadlineExceeded, want: "context_deadline_exceeded"},
		{name: "stream failure", reason: cleanupReasonStreamFailure, want: "stream_failure"},
		{name: "cancel default", reason: cleanupReasonCancel, want: "user_cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanupReasonEventValue(tt.reason); got != tt.want {
				t.Fatalf("cleanupReasonEventValue(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func TestApplyCleanupPlanReturnsUnknownActionError(t *testing.T) {
	conversations := &loopConversationManagerStub{}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})

	err := loop.applyCleanupPlan(stdctx.Background(), inflightTurn{ConversationID: "conv-4", TurnNumber: 6, Iteration: 3}, cleanupPlan{
		Reason: cleanupReasonCancel,
		Actions: []cleanupAction{{
			Kind:      "wat",
			Iteration: 3,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), `unknown cleanup action kind "wat"`) {
		t.Fatalf("applyCleanupPlan error = %v, want unknown action error", err)
	}
}

func TestHandleTurnCleanupEmitsCancelledThenIdleAndReturnsWrappedCause(t *testing.T) {
	sink := NewChannelSink(8)
	conversations := &loopConversationManagerStub{}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
		EventSink:           sink,
	})
	loop.now = func() time.Time { return time.Unix(1700000700, 0).UTC() }

	cause := errors.New("context canceled")
	err := loop.handleTurnCleanup(inflightTurn{
		ConversationID:           "conv-5",
		TurnNumber:               7,
		Iteration:                2,
		CompletedIterations:      1,
		AssistantResponseStarted: true,
		AssistantMessageContent:  `[{"type":"text","text":"partial"}]`,
	}, cleanupReasonCancel, cause)
	if !errors.Is(err, ErrTurnCancelled) {
		t.Fatalf("handleTurnCleanup error = %v, want ErrTurnCancelled", err)
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("handleTurnCleanup error = %v, want wrapped cause text %q", err, cause.Error())
	}

	events := drainCleanupEvents(sink)
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	cancelled, ok := events[0].(TurnCancelledEvent)
	if !ok {
		t.Fatalf("first event = %T, want TurnCancelledEvent", events[0])
	}
	if cancelled.Reason != "user_cancelled" {
		t.Fatalf("TurnCancelledEvent reason = %q, want user_cancelled", cancelled.Reason)
	}
	status, ok := events[1].(StatusEvent)
	if !ok {
		t.Fatalf("second event = %T, want StatusEvent", events[1])
	}
	if status.State != StateIdle {
		t.Fatalf("StatusEvent state = %q, want %q", status.State, StateIdle)
	}
}

func TestHandleTurnCleanupStillEmitsEventsWhenCleanupApplyFails(t *testing.T) {
	sink := NewChannelSink(8)
	conversations := &loopConversationManagerStub{persistIterErr: errors.New("boom")}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
		EventSink:           sink,
	})
	loop.now = func() time.Time { return time.Unix(1700000800, 0).UTC() }

	cause := errors.New("stop now")
	err := loop.handleTurnCleanup(inflightTurn{
		ConversationID:           "conv-6",
		TurnNumber:               8,
		Iteration:                2,
		CompletedIterations:      1,
		AssistantResponseStarted: true,
		AssistantMessageContent:  `[{"type":"text","text":"partial"}]`,
	}, cleanupReasonInterrupt, cause)
	if !errors.Is(err, ErrTurnCancelled) {
		t.Fatalf("handleTurnCleanup error = %v, want ErrTurnCancelled", err)
	}
	if len(conversations.persistIterCalls) != 1 {
		t.Fatalf("PersistIteration calls = %d, want 1 despite failure", len(conversations.persistIterCalls))
	}

	events := drainCleanupEvents(sink)
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	if _, ok := events[0].(TurnCancelledEvent); !ok {
		t.Fatalf("first event = %T, want TurnCancelledEvent", events[0])
	}
	status, ok := events[1].(StatusEvent)
	if !ok {
		t.Fatalf("second event = %T, want StatusEvent", events[1])
	}
	if status.State != StateIdle {
		t.Fatalf("StatusEvent state = %q, want %q", status.State, StateIdle)
	}
}

func TestHandleTurnCleanupReturnsErrTurnCancelledWithoutCause(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: &loopConversationManagerStub{},
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})

	err := loop.handleTurnCleanup(inflightTurn{}, cleanupReasonCancel, nil)
	if !errors.Is(err, ErrTurnCancelled) {
		t.Fatalf("handleTurnCleanup error = %v, want ErrTurnCancelled", err)
	}
	if err != ErrTurnCancelled {
		t.Fatalf("handleTurnCleanup error = %#v, want bare ErrTurnCancelled", err)
	}
}

func drainCleanupEvents(sink *ChannelSink) []Event {
	var events []Event
	for {
		select {
		case event := <-sink.Events():
			if event == nil {
				return events
			}
			events = append(events, event)
		default:
			return events
		}
	}
}
