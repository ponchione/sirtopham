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
	baseURL      string       // default: "https://chatgpt.com/backend-api/codex"
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
		baseURL:    "https://chatgpt.com/backend-api/codex",
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

// Compile-time assertion that CodexProvider satisfies provider.Provider.
var _ provider.Provider = (*CodexProvider)(nil)

// Models returns the static list of models supported by the Codex provider.
func (p *CodexProvider) Models(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "gpt-5.1-codex-mini", Name: "gpt-5.1-codex-mini", ContextWindow: 400000, SupportsTools: true, SupportsThinking: false},
		{ID: "o3", Name: "o3", ContextWindow: 200000, SupportsTools: true, SupportsThinking: false},
		{ID: "o4-mini", Name: "o4-mini", ContextWindow: 200000, SupportsTools: true, SupportsThinking: false},
		{ID: "gpt-4.1", Name: "GPT-4.1", ContextWindow: 1000000, SupportsTools: true, SupportsThinking: false},
	}, nil
}

func (p *CodexProvider) responsesEndpointURL() string {
	base := strings.TrimRight(p.baseURL, "/")
	if strings.Contains(base, "chatgpt.com/backend-api/codex") || strings.HasSuffix(base, "/codex") {
		return base + "/responses"
	}
	return base + "/v1/responses"
}

func (p *CodexProvider) usesChatGPTCodexEndpoint() bool {
	base := strings.TrimRight(p.baseURL, "/")
	return strings.Contains(base, "chatgpt.com/backend-api/codex") || strings.HasSuffix(base, "/codex")
}
