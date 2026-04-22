package agent

import (
	"testing"

	"github.com/ponchione/sodoryard/internal/provider"
)

func TestNewInflightToolTurnBuildsBaseAndToolMetadata(t *testing.T) {
	req := RunTurnRequest{ConversationID: "conv-tools", TurnNumber: 5}
	iteration := 3
	completedIterations := 2
	assistantContentJSON := `[{"type":"text","text":"partial"}]`
	result := &streamResult{ToolCalls: []provider.ToolCall{{ID: "tool-1", Name: "read_file"}, {ID: "tool-2", Name: "search_text"}}}

	inflight := newInflightToolTurn(req, iteration, completedIterations, result, assistantContentJSON)

	if inflight.ConversationID != "conv-tools" {
		t.Fatalf("ConversationID = %q, want conv-tools", inflight.ConversationID)
	}
	if inflight.TurnNumber != 5 {
		t.Fatalf("TurnNumber = %d, want 5", inflight.TurnNumber)
	}
	if inflight.Iteration != 3 {
		t.Fatalf("Iteration = %d, want 3", inflight.Iteration)
	}
	if inflight.CompletedIterations != 2 {
		t.Fatalf("CompletedIterations = %d, want 2", inflight.CompletedIterations)
	}
	if !inflight.AssistantResponseStarted {
		t.Fatal("AssistantResponseStarted = false, want true")
	}
	if inflight.AssistantMessageContent != assistantContentJSON {
		t.Fatalf("AssistantMessageContent = %q, want %q", inflight.AssistantMessageContent, assistantContentJSON)
	}
	if len(inflight.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2", len(inflight.ToolCalls))
	}
	if inflight.ToolCalls[0] != (inflightToolCall{ToolCallID: "tool-1", ToolName: "read_file"}) {
		t.Fatalf("ToolCalls[0] = %+v, want tool-1/read_file with zero-value flags", inflight.ToolCalls[0])
	}
	if inflight.ToolCalls[1] != (inflightToolCall{ToolCallID: "tool-2", ToolName: "search_text"}) {
		t.Fatalf("ToolCalls[1] = %+v, want tool-2/search_text with zero-value flags", inflight.ToolCalls[1])
	}
}
