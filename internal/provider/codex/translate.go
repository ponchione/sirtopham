package codex

import (
	"encoding/json"
	"strings"

	"github.com/ponchione/sirtopham/internal/provider"
)

// responsesRequest is the top-level JSON body for POST /v1/responses.
type responsesRequest struct {
	Model     string              `json:"model"`
	Input     []responsesInput    `json:"input"`
	Tools     []responsesTool     `json:"tools,omitempty"`
	Stream    bool                `json:"stream"`
	Reasoning *responsesReasoning `json:"reasoning,omitempty"`
}

// responsesInput represents one item in the input array.
// For system/user/assistant roles, this is a message with either string
// content or an array of content blocks.
type responsesInput struct {
	Role    string      `json:"role"`    // "system", "user", "assistant"
	Content interface{} `json:"content"` // string or []responsesContentBlock
}

// responsesContentBlock represents a typed content block within a message.
type responsesContentBlock struct {
	Type      string `json:"type"`                // "text", "function_call", "function_call_output"
	Text      string `json:"text,omitempty"`      // for type="text"
	ID        string `json:"id,omitempty"`        // for type="function_call"
	CallID    string `json:"call_id,omitempty"`   // for type="function_call" and "function_call_output"
	Name      string `json:"name,omitempty"`      // for type="function_call"
	Arguments string `json:"arguments,omitempty"` // for type="function_call" (JSON string)
	Output    string `json:"output,omitempty"`    // for type="function_call_output"
}

// responsesTool represents a tool definition in the tools array.
type responsesTool struct {
	Type        string          `json:"type"`                  // always "function"
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // JSON Schema object
}

// responsesReasoning controls reasoning behavior.
type responsesReasoning struct {
	Effort           string `json:"effort"`            // "high", "medium", "low"
	EncryptedContent string `json:"encrypted_content"` // "retain"
}

// buildResponsesRequest translates a unified Request into the Responses API
// request body. The model parameter comes from the provider config or request.
func buildResponsesRequest(model string, req *provider.Request, stream bool) responsesRequest {
	rr := responsesRequest{
		Model:  model,
		Stream: stream,
	}

	// System prompt handling: concatenate all system blocks
	if len(req.SystemBlocks) > 0 {
		var parts []string
		for _, sb := range req.SystemBlocks {
			parts = append(parts, sb.Text)
		}
		rr.Input = append(rr.Input, responsesInput{
			Role:    "system",
			Content: strings.Join(parts, "\n\n"),
		})
	}

	// Message translation
	for _, msg := range req.Messages {
		switch msg.Role {
		case provider.RoleUser:
			var text string
			_ = json.Unmarshal(msg.Content, &text)
			rr.Input = append(rr.Input, responsesInput{
				Role:    "user",
				Content: text,
			})

		case provider.RoleAssistant:
			blocks, err := provider.ContentBlocksFromRaw(msg.Content)
			if err != nil {
				// If we can't parse content blocks, try as string
				var text string
				_ = json.Unmarshal(msg.Content, &text)
				rr.Input = append(rr.Input, responsesInput{
					Role:    "assistant",
					Content: text,
				})
				continue
			}

			var contentBlocks []responsesContentBlock
			for _, block := range blocks {
				switch block.Type {
				case "text":
					contentBlocks = append(contentBlocks, responsesContentBlock{
						Type: "text",
						Text: block.Text,
					})
				case "tool_use":
					contentBlocks = append(contentBlocks, responsesContentBlock{
						Type:      "function_call",
						ID:        "fc_" + block.ID,
						CallID:    block.ID,
						Name:      block.Name,
						Arguments: string(block.Input),
					})
				case "thinking":
					// Skip: Responses API uses encrypted reasoning, not plaintext thinking
				}
			}
			rr.Input = append(rr.Input, responsesInput{
				Role:    "assistant",
				Content: contentBlocks,
			})

		case provider.RoleTool:
			var text string
			_ = json.Unmarshal(msg.Content, &text)
			rr.Input = append(rr.Input, responsesInput{
				Role: "user",
				Content: []responsesContentBlock{{
					Type:   "function_call_output",
					CallID: msg.ToolUseID,
					Output: text,
				}},
			})
		}
	}

	// Tool definitions
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			rr.Tools = append(rr.Tools, responsesTool{
				Type:        "function",
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
	}

	// Reasoning configuration for reasoning models
	if model == "o3" || model == "o4-mini" {
		rr.Reasoning = &responsesReasoning{
			Effort:           "high",
			EncryptedContent: "retain",
		}
	}

	return rr
}
