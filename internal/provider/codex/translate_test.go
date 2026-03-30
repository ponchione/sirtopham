package codex

import (
	"encoding/json"
	"testing"

	"github.com/ponchione/sirtopham/internal/provider"
)

func TestBuildResponsesRequest_SystemPromptConcatenation(t *testing.T) {
	req := &provider.Request{
		SystemBlocks: []provider.SystemBlock{
			{Text: "You are a coding assistant."},
			{Text: "Project context: Go backend"},
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) < 1 {
		t.Fatal("expected at least one input item")
	}
	item := rr.Input[0]
	if item.Role != "system" {
		t.Fatalf("expected role %q, got %q", "system", item.Role)
	}

	content, ok := item.Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", item.Content)
	}
	expected := "You are a coding assistant.\n\nProject context: Go backend"
	if content != expected {
		t.Errorf("expected content %q, got %q", expected, content)
	}
}

func TestBuildResponsesRequest_EmptySystemBlocks(t *testing.T) {
	req := &provider.Request{
		Messages: []provider.Message{
			provider.NewUserMessage("hello"),
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) == 0 {
		t.Fatal("expected at least one input item")
	}
	if rr.Input[0].Role == "system" {
		t.Error("no system input item should be emitted when SystemBlocks is nil")
	}
	if rr.Input[0].Role != "user" {
		t.Errorf("expected first item role %q, got %q", "user", rr.Input[0].Role)
	}
}

func TestBuildResponsesRequest_UserMessage(t *testing.T) {
	req := &provider.Request{
		Messages: []provider.Message{
			provider.NewUserMessage("Fix the auth bug"),
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(rr.Input))
	}
	item := rr.Input[0]
	if item.Role != "user" {
		t.Fatalf("expected role %q, got %q", "user", item.Role)
	}
	content, ok := item.Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", item.Content)
	}
	if content != "Fix the auth bug" {
		t.Errorf("expected content %q, got %q", "Fix the auth bug", content)
	}
}

func TestBuildResponsesRequest_AssistantMessageTextOnly(t *testing.T) {
	blocks := []provider.ContentBlock{
		{Type: "text", Text: "I'll check the code."},
	}
	raw, _ := json.Marshal(blocks)
	req := &provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: raw},
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(rr.Input))
	}
	item := rr.Input[0]
	if item.Role != "assistant" {
		t.Fatalf("expected role %q, got %q", "assistant", item.Role)
	}

	contentBlocks, ok := item.Content.([]responsesContentBlock)
	if !ok {
		t.Fatalf("expected []responsesContentBlock, got %T", item.Content)
	}
	if len(contentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contentBlocks))
	}
	if contentBlocks[0].Type != "text" || contentBlocks[0].Text != "I'll check the code." {
		t.Errorf("expected text block with text %q, got type=%q text=%q", "I'll check the code.", contentBlocks[0].Type, contentBlocks[0].Text)
	}
}

func TestBuildResponsesRequest_AssistantMessageWithToolUse(t *testing.T) {
	blocks := []provider.ContentBlock{
		{Type: "text", Text: "Let me read that."},
		{Type: "tool_use", ID: "tc_1", Name: "file_read", Input: json.RawMessage(`{"path":"auth.go"}`)},
	}
	raw, _ := json.Marshal(blocks)
	req := &provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: raw},
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(rr.Input))
	}

	contentBlocks, ok := rr.Input[0].Content.([]responsesContentBlock)
	if !ok {
		t.Fatalf("expected []responsesContentBlock, got %T", rr.Input[0].Content)
	}
	if len(contentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(contentBlocks))
	}

	fc := contentBlocks[1]
	if fc.Type != "function_call" {
		t.Errorf("expected type %q, got %q", "function_call", fc.Type)
	}
	if fc.ID != "fc_tc_1" {
		t.Errorf("expected ID %q, got %q", "fc_tc_1", fc.ID)
	}
	if fc.CallID != "tc_1" {
		t.Errorf("expected CallID %q, got %q", "tc_1", fc.CallID)
	}
	if fc.Name != "file_read" {
		t.Errorf("expected Name %q, got %q", "file_read", fc.Name)
	}
	if fc.Arguments != `{"path":"auth.go"}` {
		t.Errorf("expected Arguments %q, got %q", `{"path":"auth.go"}`, fc.Arguments)
	}
}

func TestBuildResponsesRequest_ToolResultMessage(t *testing.T) {
	req := &provider.Request{
		Messages: []provider.Message{
			provider.NewToolResultMessage("tc_1", "file_read", "package auth..."),
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(rr.Input))
	}
	item := rr.Input[0]
	if item.Role != "user" {
		t.Fatalf("expected role %q, got %q", "user", item.Role)
	}

	contentBlocks, ok := item.Content.([]responsesContentBlock)
	if !ok {
		t.Fatalf("expected []responsesContentBlock, got %T", item.Content)
	}
	if len(contentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contentBlocks))
	}

	fco := contentBlocks[0]
	if fco.Type != "function_call_output" {
		t.Errorf("expected type %q, got %q", "function_call_output", fco.Type)
	}
	if fco.CallID != "tc_1" {
		t.Errorf("expected CallID %q, got %q", "tc_1", fco.CallID)
	}
	if fco.Output != "package auth..." {
		t.Errorf("expected Output %q, got %q", "package auth...", fco.Output)
	}
}

func TestBuildResponsesRequest_ToolDefinitions(t *testing.T) {
	req := &provider.Request{
		Tools: []provider.ToolDefinition{
			{
				Name:        "file_read",
				Description: "Read a file",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	if len(rr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(rr.Tools))
	}
	tool := rr.Tools[0]
	if tool.Type != "function" {
		t.Errorf("expected type %q, got %q", "function", tool.Type)
	}
	if tool.Name != "file_read" {
		t.Errorf("expected name %q, got %q", "file_read", tool.Name)
	}
	if tool.Description != "Read a file" {
		t.Errorf("expected description %q, got %q", "Read a file", tool.Description)
	}

	var params map[string]interface{}
	if err := json.Unmarshal(tool.Parameters, &params); err != nil {
		t.Fatalf("failed to unmarshal parameters: %v", err)
	}
	if params["type"] != "object" {
		t.Errorf("expected parameters.type %q, got %v", "object", params["type"])
	}
}

func TestBuildResponsesRequest_ReasoningForO3(t *testing.T) {
	req := &provider.Request{}
	rr := buildResponsesRequest("o3", req, false)

	if rr.Reasoning == nil {
		t.Fatal("expected reasoning config for o3")
	}
	if rr.Reasoning.Effort != "high" {
		t.Errorf("expected effort %q, got %q", "high", rr.Reasoning.Effort)
	}
	if rr.Reasoning.EncryptedContent != "retain" {
		t.Errorf("expected encrypted_content %q, got %q", "retain", rr.Reasoning.EncryptedContent)
	}
}

func TestBuildResponsesRequest_ReasoningForO4Mini(t *testing.T) {
	req := &provider.Request{}
	rr := buildResponsesRequest("o4-mini", req, false)

	if rr.Reasoning == nil {
		t.Fatal("expected reasoning config for o4-mini")
	}
	if rr.Reasoning.Effort != "high" {
		t.Errorf("expected effort %q, got %q", "high", rr.Reasoning.Effort)
	}
}

func TestBuildResponsesRequest_NoReasoningForGPT41(t *testing.T) {
	req := &provider.Request{}
	rr := buildResponsesRequest("gpt-4.1", req, false)

	if rr.Reasoning != nil {
		t.Error("expected no reasoning config for gpt-4.1")
	}
}

func TestBuildResponsesRequest_StreamFlag(t *testing.T) {
	req := &provider.Request{}
	rrStream := buildResponsesRequest("o3", req, true)
	if !rrStream.Stream {
		t.Error("expected stream=true")
	}

	rrNoStream := buildResponsesRequest("o3", req, false)
	if rrNoStream.Stream {
		t.Error("expected stream=false")
	}
}

func TestBuildResponsesRequest_JSONOutput(t *testing.T) {
	req := &provider.Request{
		SystemBlocks: []provider.SystemBlock{
			{Text: "You are helpful."},
		},
		Messages: []provider.Message{
			provider.NewUserMessage("hello"),
		},
	}
	rr := buildResponsesRequest("o3", req, false)

	data, err := json.Marshal(rr)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if raw["model"] != "o3" {
		t.Errorf("expected model %q, got %v", "o3", raw["model"])
	}
	if raw["stream"] != false {
		t.Errorf("expected stream false, got %v", raw["stream"])
	}

	input, ok := raw["input"].([]interface{})
	if !ok {
		t.Fatalf("expected input array, got %T", raw["input"])
	}
	if len(input) != 2 {
		t.Fatalf("expected 2 input items, got %d", len(input))
	}
}
