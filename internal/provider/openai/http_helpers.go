package openai

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ponchione/sodoryard/internal/provider"
)

func (p *OpenAIProvider) newChatCompletionRequest(ctx context.Context, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("OpenAI-compatible provider '%s': failed to create request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	return httpReq, nil
}

func (p *OpenAIProvider) requestFailure(ctx context.Context, err error) *provider.ProviderError {
	if isConnectionError(err) {
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: 0,
			Message:    fmt.Sprintf("OpenAI-compatible provider '%s' at %s is not reachable. Is the model server running?", p.name, p.baseURL),
			Retriable:  false,
			Err:        err,
		}
	}
	if ctx.Err() != nil {
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: 0,
			Message:    ctx.Err().Error(),
			Retriable:  false,
			Err:        ctx.Err(),
		}
	}
	return &provider.ProviderError{
		Provider:   p.name,
		StatusCode: 0,
		Message:    fmt.Sprintf("OpenAI-compatible provider '%s': request failed: %s", p.name, err),
		Retriable:  true,
		Err:        err,
	}
}

func (p *OpenAIProvider) statusFailure(status int, retryAfter time.Duration, exhaustedRetries bool) *provider.ProviderError {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: status,
			Message:    fmt.Sprintf("OpenAI-compatible provider '%s' authentication failed. Check API key configuration.", p.name),
			Retriable:  false,
		}
	case http.StatusTooManyRequests:
		message := fmt.Sprintf("OpenAI-compatible provider '%s': rate limited", p.name)
		if exhaustedRetries {
			message = fmt.Sprintf("%s after %d attempts", message, maxRetryAttempts)
		}
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: status,
			Message:    message,
			Retriable:  true,
			RetryAfter: retryAfter,
		}
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		message := fmt.Sprintf("OpenAI-compatible provider '%s': server error (HTTP %d)", p.name, status)
		if exhaustedRetries {
			message = fmt.Sprintf("%s after %d attempts", message, maxRetryAttempts)
		}
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: status,
			Message:    message,
			Retriable:  true,
			RetryAfter: retryAfter,
		}
	default:
		return &provider.ProviderError{
			Provider:   p.name,
			StatusCode: status,
			Message:    fmt.Sprintf("OpenAI-compatible provider '%s': unexpected HTTP status %d", p.name, status),
			Retriable:  false,
		}
	}
}
