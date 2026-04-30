package agent

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	contextpkg "github.com/ponchione/sodoryard/internal/context"
	"github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/provider"
)

func TestNormalizeOverflowRecoveryUsesEmergencyCompressionRetryResult(t *testing.T) {
	conversations := &loopConversationManagerStub{
		history: []db.Message{},
		seen:    loopSeenFilesStub{},
	}
	compression := &compressionEngineStub{
		result: &contextpkg.CompressionResult{
			Compressed:         true,
			CompressedMessages: 3,
		},
	}
	overflowErr := &provider.ProviderError{
		Provider:   "anthropic",
		StatusCode: 400,
		Message:    "context_length_exceeded",
	}
	router := &providerRouterStub{
		streamEvents: [][]provider.StreamEvent{
			textOnlyStream("recovered", 50, 10),
		},
	}

	loop := NewAgentLoop(AgentLoopDeps{
		ConversationManager: conversations,
		ProviderRouter:      router,
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
		CompressionEngine:   compression,
	})

	turnExec := loop.newTurnExecution(RunTurnRequest{
		ConversationID:    "conv-overflow-helper",
		TurnNumber:        2,
		Message:           "help",
		ModelContextLimit: 200000,
	}, &TurnStartResult{ContextPackage: &contextpkg.FullContextPackage{Content: "ctx", Frozen: true}}, time.Unix(1700001000, 0).UTC())
	iterExec := &iterationExecution{
		number:    1,
		promptReq: &provider.Request{},
	}

	gotResult, gotErr := loop.normalizeOverflowRecovery(stdctx.Background(), turnExec, iterExec, nil, overflowErr)
	if gotErr != nil {
		t.Fatalf("normalizeOverflowRecovery error: %v", gotErr)
	}
	if gotResult == nil {
		t.Fatal("normalizeOverflowRecovery result = nil, want retry result")
	}
	if gotResult.TextContent != "recovered" {
		t.Fatalf("TextContent = %q, want %q", gotResult.TextContent, "recovered")
	}
	if len(conversations.reconstructCalls) != 1 || conversations.reconstructCalls[0] != "conv-overflow-helper" {
		t.Fatalf("ReconstructHistory calls = %#v, want conv-overflow-helper once", conversations.reconstructCalls)
	}
}

func TestPrepareIterationUsesCachedTurnHistory(t *testing.T) {
	cachedHistory := []db.Message{{Role: "user", Content: nullStr("cached history")}}
	conversations := &loopConversationManagerStub{
		reconstructFn: func(ctx stdctx.Context, conversationID string) ([]db.Message, error) {
			t.Fatalf("ReconstructHistory should not be called for cached iteration history")
			return nil, nil
		},
	}
	loop := NewAgentLoop(AgentLoopDeps{
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
	})

	turnExec := loop.newTurnExecution(RunTurnRequest{
		ConversationID:    "conv-cached-history",
		TurnNumber:        1,
		Message:           "help",
		ModelContextLimit: 200000,
	}, &TurnStartResult{
		History:        cachedHistory,
		ContextPackage: &contextpkg.FullContextPackage{Content: "ctx", Frozen: true},
	}, time.Unix(1700001000, 0).UTC())

	iterExec, err := loop.prepareIteration(stdctx.Background(), turnExec, 1)
	if err != nil {
		t.Fatalf("prepareIteration error: %v", err)
	}
	if len(iterExec.history) != 1 || iterExec.history[0].Content.String != "cached history" {
		t.Fatalf("iteration history = %#v, want cached history", iterExec.history)
	}
	if len(conversations.reconstructCalls) != 0 {
		t.Fatalf("ReconstructHistory calls = %#v, want none", conversations.reconstructCalls)
	}
}

func TestNormalizeOverflowRecoveryLeavesNonOverflowErrorUntouched(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ToolExecutor:  &toolExecutorStub{},
		PromptBuilder: NewPromptBuilder(nil),
	})
	originalResult := &streamResult{TextContent: "partial"}
	originalErr := errors.New("provider unavailable")

	gotResult, gotErr := loop.normalizeOverflowRecovery(stdctx.Background(), &turnExecution{}, &iterationExecution{}, originalResult, originalErr)
	if gotErr != originalErr {
		t.Fatalf("error = %v, want original %v", gotErr, originalErr)
	}
	if gotResult != originalResult {
		t.Fatalf("result = %#v, want original %#v", gotResult, originalResult)
	}
}

func TestNormalizeIterationSetupErrorReturnsCancellationCleanup(t *testing.T) {
	sink := NewChannelSink(8)
	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	cancel()

	loop := NewAgentLoop(AgentLoopDeps{
		ConversationManager: &loopConversationManagerStub{},
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
		EventSink:           sink,
	})
	loop.now = func() time.Time { return time.Unix(1700001100, 0).UTC() }

	turnExec := &turnExecution{
		req:                 RunTurnRequest{ConversationID: "conv-setup-cancel", TurnNumber: 3},
		completedIterations: 1,
	}

	gotErr := loop.normalizeIterationSetupError(ctx, turnExec, 2, ctx.Err())
	if !errors.Is(gotErr, ErrTurnCancelled) {
		t.Fatalf("error = %v, want ErrTurnCancelled", gotErr)
	}

	events := drainEvents(sink, 8)
	got := eventTypes(events)
	if len(got) < 2 || got[len(got)-2] != "turn_cancelled" || got[len(got)-1] != "status:idle" {
		t.Fatalf("event sequence = %v, want ... turn_cancelled, status:idle", got)
	}
}

func TestNormalizeIterationSetupErrorLeavesNonCancellationUntouched(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ToolExecutor:  &toolExecutorStub{},
		PromptBuilder: NewPromptBuilder(nil),
	})
	turnExec := &turnExecution{
		req: RunTurnRequest{ConversationID: "conv-setup-plain", TurnNumber: 1},
	}
	originalErr := errors.New("prompt build failed")

	gotErr := loop.normalizeIterationSetupError(stdctx.Background(), turnExec, 1, originalErr)
	if gotErr != originalErr {
		t.Fatalf("error = %v, want original %v", gotErr, originalErr)
	}
}
