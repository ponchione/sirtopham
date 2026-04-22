package agent

import (
	stdctx "context"
	"encoding/json"
	"testing"
	"time"

	contextpkg "github.com/ponchione/sodoryard/internal/context"
	"github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/provider"
)

func TestPrepareRunTurnBuildsExecutionState(t *testing.T) {
	assembler := &loopContextAssemblerStub{
		pkg: &contextpkg.FullContextPackage{Content: "context", Frozen: true},
	}
	conversations := &loopConversationManagerStub{
		history: []db.Message{},
		seen:    loopSeenFilesStub{},
	}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    assembler,
		ConversationManager: conversations,
		ProviderRouter:      &providerRouterStub{},
		ToolExecutor:        &toolExecutorStub{},
		PromptBuilder:       NewPromptBuilder(nil),
		Config: AgentLoopConfig{
			ProviderName:           "default-provider",
			ModelName:              "default-model",
			LoopDetectionThreshold: 4,
		},
	})
	loop.now = func() time.Time { return time.Unix(1700000800, 0).UTC() }

	prepared, err := loop.prepareRunTurn(stdctx.Background(), RunTurnRequest{
		ConversationID:    "conv-start",
		TurnNumber:        2,
		Message:           "hello bootstrap",
		ModelContextLimit: 200000,
		Provider:          "override-provider",
		Model:             "override-model",
	})
	if err != nil {
		t.Fatalf("prepareRunTurn error: %v", err)
	}
	defer prepared.cleanup()

	if prepared == nil || prepared.exec == nil {
		t.Fatalf("prepared = %#v, want non-nil exec", prepared)
	}
	if prepared.ctx == nil {
		t.Fatal("prepared.ctx = nil")
	}
	if prepared.exec.effectiveProvider != "override-provider" {
		t.Fatalf("effectiveProvider = %q, want override-provider", prepared.exec.effectiveProvider)
	}
	if prepared.exec.effectiveModel != "override-model" {
		t.Fatalf("effectiveModel = %q, want override-model", prepared.exec.effectiveModel)
	}
	if prepared.exec.turnCtx == nil || prepared.exec.turnCtx.ContextPackage == nil {
		t.Fatalf("turnCtx = %#v, want prepared turn context", prepared.exec.turnCtx)
	}
	if len(prepared.exec.currentTurnMessages) != 1 {
		t.Fatalf("currentTurnMessages = %d, want 1", len(prepared.exec.currentTurnMessages))
	}
	if prepared.exec.currentTurnMessages[0].Role != provider.RoleUser {
		t.Fatalf("currentTurnMessages[0].Role = %q, want user", prepared.exec.currentTurnMessages[0].Role)
	}
	var text string
	if err := json.Unmarshal(prepared.exec.currentTurnMessages[0].Content, &text); err != nil {
		t.Fatalf("unmarshal user message content: %v", err)
	}
	if text != "hello bootstrap" {
		t.Fatalf("currentTurnMessages[0].Content = %q, want hello bootstrap", text)
	}
	if prepared.exec.detector == nil {
		t.Fatal("loop detector was not initialized")
	}
	if len(conversations.persistCalls) != 1 {
		t.Fatalf("PersistUserMessage calls = %d, want 1", len(conversations.persistCalls))
	}
}

func TestRunSingleIterationCompletesTextOnlyTurn(t *testing.T) {
	assembler := &loopContextAssemblerStub{
		pkg: &contextpkg.FullContextPackage{Content: "context", Frozen: true},
	}
	conversations := &loopConversationManagerStub{
		history: []db.Message{},
		seen:    loopSeenFilesStub{},
	}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    assembler,
		ConversationManager: conversations,
		ProviderRouter: &providerRouterStub{
			streamEvents: [][]provider.StreamEvent{
				textOnlyStream("done", 10, 5),
			},
		},
		ToolExecutor:  &toolExecutorStub{},
		PromptBuilder: NewPromptBuilder(nil),
	})
	loop.now = func() time.Time { return time.Unix(1700000801, 0).UTC() }

	turnCtx, err := loop.PrepareTurnContext(stdctx.Background(), "conv-iter", 1, "hello", 200000, 0)
	if err != nil {
		t.Fatalf("PrepareTurnContext error: %v", err)
	}
	turnExec := loop.newTurnExecution(RunTurnRequest{
		ConversationID:    "conv-iter",
		TurnNumber:        1,
		Message:           "hello",
		ModelContextLimit: 200000,
	}, turnCtx, loop.now())

	outcome, err := loop.runSingleIteration(stdctx.Background(), turnExec, 1)
	if err != nil {
		t.Fatalf("runSingleIteration error: %v", err)
	}
	if outcome == nil || !outcome.done || outcome.result == nil {
		t.Fatalf("outcome = %#v, want done result", outcome)
	}
	if outcome.result.FinalText != "done" {
		t.Fatalf("FinalText = %q, want done", outcome.result.FinalText)
	}
	if len(conversations.persistIterCalls) != 1 {
		t.Fatalf("PersistIteration calls = %d, want 1", len(conversations.persistIterCalls))
	}
}
