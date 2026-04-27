package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ponchione/sodoryard/internal/provider"
)

const maxSSEScannerTokenSize = 1024 * 1024

func sendStreamEvent(ctx context.Context, ch chan<- provider.StreamEvent, event provider.StreamEvent) bool {
	select {
	case ch <- event:
		return true
	default:
	}

	select {
	case ch <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

// streamState tracks in-progress output items during SSE parsing.
type streamState struct {
	currentToolCallID   string
	currentToolCallName string
	toolCallArgs        strings.Builder
}

// SSE event data payload types.

type sseTextDelta struct {
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type sseReasoningDelta struct {
	ItemID string `json:"item_id"`
	Delta  string `json:"delta"`
}

type sseOutputItemAdded struct {
	OutputIndex int               `json:"output_index"`
	Item        sseOutputItemData `json:"item"`
}

type sseOutputItemDone struct {
	OutputIndex int               `json:"output_index"`
	Item        sseOutputItemData `json:"item"`
}

type sseOutputItemData struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	CallID           string `json:"call_id,omitempty"`
	Name             string `json:"name,omitempty"`
	Arguments        string `json:"arguments,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
}

type sseFuncArgDelta struct {
	ItemID string `json:"item_id"`
	Delta  string `json:"delta"`
}

type sseCompleted struct {
	Response sseCompletedResponse `json:"response"`
}

type sseCompletedResponse struct {
	ID     string              `json:"id"`
	Status string              `json:"status"`
	Usage  responsesUsage      `json:"usage"`
	Output []sseOutputItemData `json:"output,omitempty"`
}

// Stream sends a streaming request to the Responses API and returns a channel
// of unified StreamEvent values.
func (p *CodexProvider) Stream(ctx context.Context, req *provider.Request) (<-chan provider.StreamEvent, error) {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	model := codexRequestModel(req.Model)

	apiReq := buildResponsesRequest(model, req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, codexMarshalError(err)
	}

	httpReq, err := p.newResponsesHTTPRequest(ctx, body, token)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, codexRequestFailure(ctx, err)
	}

	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		return nil, codexStreamStatusFailure(resp.StatusCode, resp.Body)
	}

	ch := make(chan provider.StreamEvent, 64)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		state := &streamState{}
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), maxSSEScannerTokenSize)

		var eventType string

		for scanner.Scan() {
			if ctx.Err() != nil {
				sendStreamEvent(ctx, ch, provider.StreamError{
					Err:     ctx.Err(),
					Fatal:   true,
					Message: "stream cancelled",
				})
				return
			}

			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if !p.handleSSEEvent(ctx, eventType, []byte(data), state, ch) {
					return
				}
				eventType = ""
				continue
			}

			// Empty lines and other lines are ignored (SSE separator)
		}

		if err := scanner.Err(); err != nil {
			sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   true,
				Message: fmt.Sprintf("stream read error: %v", err),
			})
		}
	}()

	return ch, nil
}

// handleSSEEvent processes a single SSE event and emits unified StreamEvent
// values on the channel.
func (p *CodexProvider) handleSSEEvent(ctx context.Context, eventType string, data []byte, state *streamState, ch chan<- provider.StreamEvent) bool {
	switch eventType {
	case "response.output_text.delta":
		var delta sseTextDelta
		if err := json.Unmarshal(data, &delta); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}
		return sendStreamEvent(ctx, ch, provider.TokenDelta{Text: delta.Delta})

	case "response.reasoning.delta":
		var delta sseReasoningDelta
		if err := json.Unmarshal(data, &delta); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}
		return sendStreamEvent(ctx, ch, provider.ThinkingDelta{Thinking: delta.Delta})

	case "response.output_item.added":
		var added sseOutputItemAdded
		if err := json.Unmarshal(data, &added); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}
		if added.Item.Type == "function_call" {
			state.currentToolCallID = added.Item.CallID
			state.currentToolCallName = added.Item.Name
			state.toolCallArgs.Reset()
			return sendStreamEvent(ctx, ch, provider.ToolCallStart{
				ID:   added.Item.CallID,
				Name: added.Item.Name,
			})
		}
		return true

	case "response.function_call_arguments.delta":
		var delta sseFuncArgDelta
		if err := json.Unmarshal(data, &delta); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}
		if !sendStreamEvent(ctx, ch, provider.ToolCallDelta{
			ID:    state.currentToolCallID,
			Delta: delta.Delta,
		}) {
			return false
		}
		state.toolCallArgs.WriteString(delta.Delta)
		return true

	case "response.output_item.done":
		var done sseOutputItemDone
		if err := json.Unmarshal(data, &done); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}
		if done.Item.Type == "function_call" {
			if !sendStreamEvent(ctx, ch, provider.ToolCallEnd{
				ID:    done.Item.CallID,
				Input: json.RawMessage(state.toolCallArgs.String()),
			}) {
				return false
			}
			state.toolCallArgs.Reset()
		}
		return true

	case "response.completed":
		var completed sseCompleted
		if err := json.Unmarshal(data, &completed); err != nil {
			return sendStreamEvent(ctx, ch, provider.StreamError{
				Err:     err,
				Fatal:   false,
				Message: fmt.Sprintf("failed to parse stream event: %v", err),
			})
		}

		hasToolCall := false
		for _, item := range completed.Response.Output {
			if item.Type == "function_call" {
				hasToolCall = true
				break
			}
		}

		stopReason := provider.StopReasonEndTurn
		if hasToolCall {
			stopReason = provider.StopReasonToolUse
		}

		usage := provider.Usage{
			InputTokens:         completed.Response.Usage.InputTokens,
			OutputTokens:        completed.Response.Usage.OutputTokens,
			CacheReadTokens:     completed.Response.Usage.InputTokensDetails.CachedTokens,
			CacheCreationTokens: 0,
		}

		return sendStreamEvent(ctx, ch, provider.StreamDone{
			StopReason: stopReason,
			Usage:      usage,
		})

	case "response.content_part.added",
		"response.content_part.done",
		"response.created":
		return true

	default:
		return true
	}
}
