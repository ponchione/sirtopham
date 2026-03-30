package provider

import (
	"encoding/json"
	"fmt"
)

// Model describes an LLM model's capabilities. Returned by Provider.Models().
type Model struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ContextWindow    int    `json:"context_window"`
	SupportsTools    bool   `json:"supports_tools"`
	SupportsThinking bool   `json:"supports_thinking"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult is the response to a ToolCall.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ProviderError is a structured error type for provider failures. It carries
// the HTTP status code, retry eligibility, and originating provider name.
type ProviderError struct {
	Provider   string
	StatusCode int
	Message    string
	Retriable  bool
	Err        error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s: %s (status %d)", e.Provider, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

// Unwrap returns the underlying error for errors.Is/errors.As chain traversal.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a ProviderError with automatic Retriable determination.
// Retriable is true for status codes 429, 500, 502, 503, or when statusCode is 0
// and err is non-nil (network error). False otherwise.
func NewProviderError(provider string, statusCode int, message string, err error) *ProviderError {
	retriable := false
	switch statusCode {
	case 429, 500, 502, 503:
		retriable = true
	case 0:
		retriable = err != nil
	}
	return &ProviderError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Retriable:  retriable,
		Err:        err,
	}
}
