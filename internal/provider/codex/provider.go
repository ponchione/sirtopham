package codex

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ponchione/sirtopham/internal/provider"
)

// CodexProvider implements the unified Provider interface for OpenAI's
// Responses API, using credentials delegated to the codex CLI binary.
type CodexProvider struct {
	httpClient   *http.Client
	baseURL      string       // default: "https://api.openai.com"
	mu           sync.RWMutex // guards cachedToken and tokenExpiry
	cachedToken  string
	tokenExpiry  time.Time
	codexBinPath string // resolved path from exec.LookPath
}

// ProviderOption is a functional option for configuring CodexProvider.
type ProviderOption func(*CodexProvider)

// WithHTTPClient sets the HTTP client used for API requests.
func WithHTTPClient(c *http.Client) ProviderOption {
	return func(p *CodexProvider) {
		p.httpClient = c
	}
}

// WithBaseURL sets the base URL for the Responses API endpoint.
// Any trailing slash is stripped.
func WithBaseURL(url string) ProviderOption {
	return func(p *CodexProvider) {
		p.baseURL = strings.TrimRight(url, "/")
	}
}

// NewCodexProvider creates a new CodexProvider after verifying that the codex
// CLI binary is available on PATH.
func NewCodexProvider(opts ...ProviderOption) (*CodexProvider, error) {
	p := &CodexProvider{
		baseURL:    "https://api.openai.com",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}

	binPath, err := exec.LookPath("codex")
	if err != nil {
		return nil, &provider.ProviderError{
			Provider:   "codex",
			StatusCode: 0,
			Message:    "Codex CLI not found on PATH. Install from https://openai.com/codex and run `codex auth`.",
			Retriable:  false,
		}
	}
	p.codexBinPath = binPath

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Name returns the provider name.
func (p *CodexProvider) Name() string {
	return "codex"
}

// Models returns the static list of models supported by the Codex provider.
func (p *CodexProvider) Models(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "o3", Name: "o3", ContextWindow: 200000, SupportsTools: true, SupportsThinking: false},
		{ID: "o4-mini", Name: "o4-mini", ContextWindow: 200000, SupportsTools: true, SupportsThinking: false},
		{ID: "gpt-4.1", Name: "GPT-4.1", ContextWindow: 1000000, SupportsTools: true, SupportsThinking: false},
	}, nil
}
