package codex

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/ponchione/sodoryard/internal/provider"
)

func codexCancelledError() *provider.ProviderError {
	return &provider.ProviderError{
		Provider:   "codex",
		StatusCode: 0,
		Message:    "request cancelled",
		Retriable:  false,
	}
}

func codexMarshalError(err error) *provider.ProviderError {
	return &provider.ProviderError{
		Provider:   "codex",
		StatusCode: 0,
		Message:    fmt.Sprintf("failed to marshal request: %v", err),
		Retriable:  false,
	}
}

func (p *CodexProvider) newResponsesHTTPRequest(ctx context.Context, body []byte, token string) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.responsesEndpointURL(), bytes.NewReader(body))
	if err != nil {
		return nil, &provider.ProviderError{
			Provider:   "codex",
			StatusCode: 0,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Retriable:  false,
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func codexRequestFailure(ctx context.Context, err error) *provider.ProviderError {
	if ctx.Err() != nil {
		return codexCancelledError()
	}
	return &provider.ProviderError{
		Provider:   "codex",
		StatusCode: 0,
		Message:    fmt.Sprintf("request failed: %v", err),
		Retriable:  true,
		Err:        err,
	}
}

func codexStreamStatusFailure(status int, body io.Reader) error {
	errBody, _ := io.ReadAll(io.LimitReader(body, 1024))
	bodyStr := string(errBody)

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return provider.NewAuthProviderError("codex", provider.AuthInvalidCredentials, status, "Codex authentication failed.", codexAuthRemediation(), nil)
	case http.StatusTooManyRequests:
		return &provider.ProviderError{
			Provider:   "codex",
			StatusCode: http.StatusTooManyRequests,
			Message:    "rate limited",
			Retriable:  true,
		}
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return &provider.ProviderError{
			Provider:   "codex",
			StatusCode: status,
			Message:    "server error: " + truncateBody(bodyStr, 512),
			Retriable:  true,
		}
	default:
		return &provider.ProviderError{
			Provider:   "codex",
			StatusCode: status,
			Message:    fmt.Sprintf("unexpected status %d: %s", status, bodyStr),
			Retriable:  false,
		}
	}
}

func truncateBody(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
