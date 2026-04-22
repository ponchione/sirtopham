package agent

import (
	stdctx "context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/provider"
)

func TestCompleteToolIterationPersistsMessagesAndAdvancesState(t *testing.T) {
	conversations := &loopConversationManagerStub{}
	loop := NewAgentLoop(AgentLoopDeps{
		ConversationManager: conversations,
		ToolExecutor:        &toolExecutorStub{},
		Config: AgentLoopConfig{
			LoopDetectionThreshold: 4,
		},
	})
	loop.now = func() time.Time { return time.Unix(1700000900, 0).UTC() }

	turnExec := loop.newTurnExecution(RunTurnRequest{
		ConversationID:    "conv-tool-iter",
		TurnNumber:        3,
		Message:           "run pwd",
		ModelContextLimit: 200000,
	}, &TurnStartResult{}, loop.now())

	assistantContentJSON := `[{"type":"tool_use","id":"t1","name":"shell","input":{"cmd":"pwd"}}]`
	toolCalls := []provider.ToolCall{{
		ID:    "t1",
		Name:  "shell",
		Input: json.RawMessage(`{"cmd":"pwd"}`),
	}}
	toolResults := []provider.ToolResult{{
		ToolUseID: "t1",
		Content:   "pwd output",
	}}

	if err := loop.completeToolIteration(stdctx.Background(), turnExec, 2, assistantContentJSON, toolCalls, toolResults); err != nil {
		t.Fatalf("completeToolIteration error: %v", err)
	}

	if turnExec.completedIterations != 2 {
		t.Fatalf("completedIterations = %d, want 2", turnExec.completedIterations)
	}
	if len(conversations.persistIterCalls) != 1 {
		t.Fatalf("PersistIteration calls = %d, want 1", len(conversations.persistIterCalls))
	}

	persisted := conversations.persistIterCalls[0]
	if persisted.conversationID != "conv-tool-iter" || persisted.turnNumber != 3 || persisted.iteration != 2 {
		t.Fatalf("PersistIteration call = %+v, want conv-tool-iter/3/2", persisted)
	}
	if len(persisted.messages) != 2 {
		t.Fatalf("persisted messages = %d, want 2", len(persisted.messages))
	}

	if persisted.messages[0] != (conversation.IterationMessage{Role: "assistant", Content: assistantContentJSON}) {
		t.Fatalf("assistant persisted message = %#v, want assistant content %q", persisted.messages[0], assistantContentJSON)
	}
	if persisted.messages[1] != (conversation.IterationMessage{
		Role:      "tool",
		Content:   "pwd output",
		ToolUseID: "t1",
		ToolName:  "shell",
	}) {
		t.Fatalf("tool persisted message = %#v, want tool payload", persisted.messages[1])
	}

	if len(turnExec.currentTurnMessages) != 3 {
		t.Fatalf("currentTurnMessages = %d, want 3 (user, assistant, tool)", len(turnExec.currentTurnMessages))
	}
	if turnExec.currentTurnMessages[1].Role != provider.RoleAssistant {
		t.Fatalf("assistant role = %q, want assistant", turnExec.currentTurnMessages[1].Role)
	}
	if string(turnExec.currentTurnMessages[1].Content) != assistantContentJSON {
		t.Fatalf("assistant content = %s, want %s", turnExec.currentTurnMessages[1].Content, assistantContentJSON)
	}

	wantToolMessage := provider.NewToolResultMessage("t1", "shell", "pwd output")
	gotToolMessage := turnExec.currentTurnMessages[2]
	if gotToolMessage.Role != wantToolMessage.Role || gotToolMessage.ToolUseID != wantToolMessage.ToolUseID || gotToolMessage.ToolName != wantToolMessage.ToolName || string(gotToolMessage.Content) != string(wantToolMessage.Content) {
		t.Fatalf("tool message = %#v, want %#v", gotToolMessage, wantToolMessage)
	}
}
