package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/ponchione/sirtopham/internal/provider"
)

// codexAuthFile represents the JSON structure of ~/.codex/auth.json.
type codexAuthFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at"` // RFC3339 format, e.g. "2026-03-28T16:00:00Z"`
	LastRefresh  string `json:"last_refresh"`
	Tokens       struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
	} `json:"tokens"`
}

type codexRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Error        string `json:"error,omitempty"`
	Description  string `json:"error_description,omitempty"`
	Message      string `json:"message,omitempty"`
}

type codexAuthState struct {
	path   string
	auth   codexAuthFile
	token  string
	expiry time.Time
}

type jwtClaims struct {
	Exp int64 `json:"exp"`
}

// getAccessToken obtains a valid access token, refreshing if needed.
// It uses a read-lock fast path when the cached token is still valid,
// and a write-lock slow path with double-check to avoid redundant refreshes.
func (p *CodexProvider) getAccessToken(ctx context.Context) (string, error) {
	// Fast path: read lock
	p.mu.RLock()
	if p.cachedToken != "" && time.Until(p.tokenExpiry) > 120*time.Second {
		token := p.cachedToken
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	// Slow path: write lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we waited
	if p.cachedToken != "" && time.Until(p.tokenExpiry) > 120*time.Second {
		return p.cachedToken, nil
	}

	// Try reading the auth file first (it may already have a valid token).
	token, expiry, err := p.readAuthFile()
	if err == nil && token != "" && time.Until(expiry) > 30*time.Second {
		p.cachedToken = token
		p.tokenExpiry = expiry
		return token, nil
	}

	// Fall back to CLI refresh only when the auth file is missing/expired.
	if refreshErr := p.refreshToken(ctx); refreshErr != nil {
		return "", refreshErr
	}

	// Read the updated auth file.
	token, expiry, err = p.readAuthFile()
	if err != nil {
		return "", err
	}
	p.cachedToken = token
	p.tokenExpiry = expiry
	return token, nil
}


// authFilePath is a package-level variable to allow tests to override the home directory.
var homeDir = os.UserHomeDir

var stdinIsTerminal = func() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

var codexOAuthClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
var codexOAuthTokenURL = "https://auth.openai.com/oauth/token"

func (p *CodexProvider) readAuthState() (*codexAuthState, error) {
	home, err := homeDir()
	if err != nil {
		return nil, fmt.Errorf("codex: cannot determine home directory: %w", err)
	}

	path := home + "/.codex/auth.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("codex: auth file not found at %s. Run `codex auth` to authenticate.", path)
		}
		return nil, fmt.Errorf("codex: cannot read auth file: %w", err)
	}

	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("codex: invalid auth file format: %w", err)
	}

	token := auth.AccessToken
	if token == "" {
		token = auth.Tokens.AccessToken
	}
	if token == "" {
		return nil, fmt.Errorf("codex: auth file contains empty access_token. Run `codex auth` to re-authenticate.")
	}

	var expiry time.Time
	if auth.ExpiresAt != "" {
		expiry, err = time.Parse(time.RFC3339, auth.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("codex: invalid expires_at timestamp in auth file: %w", err)
		}
	} else {
		expiry, err = jwtExpiry(token)
		if err != nil {
			return nil, fmt.Errorf("codex: auth file missing expires_at and token exp claim: %w", err)
		}
	}

	return &codexAuthState{path: path, auth: auth, token: token, expiry: expiry}, nil
}

// readAuthFile reads and parses ~/.codex/auth.json.
func (p *CodexProvider) readAuthFile() (string, time.Time, error) {
	state, err := p.readAuthState()
	if err != nil {
		return "", time.Time{}, err
	}
	return state.token, state.expiry, nil
}

func jwtExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("parse JWT payload: %w", err)
	}
	if claims.Exp <= 0 {
		return time.Time{}, fmt.Errorf("missing exp claim")
	}
	return time.Unix(claims.Exp, 0).UTC(), nil
}

// refreshToken refreshes Codex OAuth credentials via the token endpoint and persists them back to ~/.codex/auth.json.
func (p *CodexProvider) refreshToken(ctx context.Context) error {
	home, err := homeDir()
	if err != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("codex: cannot determine home directory: %v", err), Retriable: false, Err: err}
	}
	path := home + "/.codex/auth.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("codex: auth file not found at %s. Run `codex auth` to authenticate.", path), Retriable: false, Err: err}
		}
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("codex: cannot read auth file: %v", err), Retriable: false, Err: err}
	}
	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("codex: invalid auth file format: %v", err), Retriable: false, Err: err}
	}
	state := &codexAuthState{path: path, auth: auth}

	refreshToken := strings.TrimSpace(state.auth.Tokens.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(state.auth.RefreshToken)
	}
	if refreshToken == "" {
		return &provider.ProviderError{
			Provider:   "codex",
			StatusCode: 0,
			Message:    "Codex auth file is missing refresh_token. Run `codex auth` in a terminal to re-authenticate.",
			Retriable:  false,
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", codexOAuthClientID)

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, codexOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("Codex credential refresh request build failed: %v", err), Retriable: false}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		message := fmt.Sprintf("Codex credential refresh failed: %v", err)
		retriable := timeoutCtx.Err() != nil
		if timeoutCtx.Err() != nil {
			message = "Codex credential refresh timed out after 30s"
		}
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: message, Retriable: retriable, Err: err}
	}
	defer resp.Body.Close()

	var payload codexRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: resp.StatusCode, Message: fmt.Sprintf("Codex credential refresh returned invalid JSON: %v", err), Retriable: false, Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		message := fmt.Sprintf("Codex token refresh failed with status %d.", resp.StatusCode)
		if payload.Description != "" {
			message = fmt.Sprintf("Codex token refresh failed: %s", payload.Description)
		} else if payload.Message != "" {
			message = fmt.Sprintf("Codex token refresh failed: %s", payload.Message)
		}
		return &provider.ProviderError{Provider: "codex", StatusCode: resp.StatusCode, Message: message, Retriable: false}
	}

	if strings.TrimSpace(payload.AccessToken) == "" {
		return &provider.ProviderError{Provider: "codex", StatusCode: resp.StatusCode, Message: "Codex token refresh response was missing access_token.", Retriable: false}
	}

	state.auth.AccessToken = payload.AccessToken
	state.auth.Tokens.AccessToken = payload.AccessToken
	if strings.TrimSpace(payload.RefreshToken) != "" {
		state.auth.RefreshToken = payload.RefreshToken
		state.auth.Tokens.RefreshToken = payload.RefreshToken
		refreshToken = payload.RefreshToken
	}
	state.auth.LastRefresh = time.Now().UTC().Format(time.RFC3339)
	if expiry, err := jwtExpiry(payload.AccessToken); err == nil {
		state.auth.ExpiresAt = expiry.Format(time.RFC3339)
		state.expiry = expiry
	} else {
		state.auth.ExpiresAt = ""
		state.expiry = time.Time{}
	}
	state.token = payload.AccessToken

	data, marshalErr := json.MarshalIndent(state.auth, "", "  ")
	if marshalErr != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("Codex credential refresh could not serialize auth state: %v", marshalErr), Retriable: false, Err: marshalErr}
	}
	if err := os.WriteFile(state.path, data, 0o600); err != nil {
		return &provider.ProviderError{Provider: "codex", StatusCode: 0, Message: fmt.Sprintf("Codex credential refresh could not persist auth state: %v", err), Retriable: false, Err: err}
	}

	p.cachedToken = payload.AccessToken
	p.tokenExpiry = state.expiry
	_ = refreshToken
	return nil
}
