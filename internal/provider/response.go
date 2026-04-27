package provider

import "strings"

// StopReason indicates why an LLM response terminated.
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonCancelled StopReason = "cancelled"
)

// Usage tracks token consumption for an LLM call, including Anthropic prompt
// caching counters.
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
}

// Total returns the sum of input and output tokens.
func (u Usage) Total() int {
	return u.InputTokens + u.OutputTokens
}

// Add returns a new Usage with each field summed from u and other.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:         u.InputTokens + other.InputTokens,
		OutputTokens:        u.OutputTokens + other.OutputTokens,
		CacheReadTokens:     u.CacheReadTokens + other.CacheReadTokens,
		CacheCreationTokens: u.CacheCreationTokens + other.CacheCreationTokens,
	}
}

// Response is the unified result of an LLM call.
type Response struct {
	Content    []ContentBlock `json:"content"`
	Usage      Usage          `json:"usage"`
	Model      string         `json:"model"`
	StopReason StopReason     `json:"stop_reason"`
	LatencyMs  int64          `json:"-"`
}

func TextContent(response *Response) string {
	if response == nil {
		return ""
	}
	parts := make([]string, 0, len(response.Content))
	for _, block := range response.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
