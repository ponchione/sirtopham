// Package runtime provides shared runtime construction helpers used by
// cmd/yard, cmd/tidmouth, and cmd/sirtopham. It exists because Go does
// not allow importing main packages across binaries.
package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

// ChainCleanup extends a teardown chain without falling into the closure
// capture-by-reference trap. Each call captures prev as a value parameter,
// so later extensions get a fresh copy rather than sharing one variable that
// eventually points at the final extension and self-recurses.
func ChainCleanup(prev func(), next func()) func() {
	return func() {
		next()
		if prev != nil {
			prev()
		}
	}
}

// EnsureProjectRecord upserts the project row in the projects table so
// that downstream queries referencing project_id can join against it.
func EnsureProjectRecord(ctx context.Context, database *sql.DB, cfg *appconfig.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	name := filepath.Base(cfg.ProjectRoot)
	_, err := database.ExecContext(ctx, `
INSERT INTO projects(id, name, root_path, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	root_path = excluded.root_path,
	updated_at = excluded.updated_at
`, cfg.ProjectRoot, name, cfg.ProjectRoot, now, now)
	return err
}

// LoadRoleSystemPrompt reads and returns the system prompt file content
// for an agent role, resolving the path relative to the project root.
func LoadRoleSystemPrompt(projectRoot string, promptPath string) (string, error) {
	cfg := &appconfig.Config{ProjectRoot: projectRoot}
	resolved := cfg.ResolveAgentRoleSystemPromptPath(promptPath)
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("role system_prompt is required")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read role system prompt %s: %w", resolved, err)
	}
	return string(data), nil
}

// ResolveModelContextLimit returns the context window size for a provider,
// either from explicit config or from built-in defaults per provider type.
func ResolveModelContextLimit(cfg *appconfig.Config, providerName string) (int, error) {
	if cfg == nil {
		return 0, fmt.Errorf("config is required")
	}
	providerCfg, ok := cfg.Providers[providerName]
	if !ok {
		return 0, fmt.Errorf("unknown provider: %s", providerName)
	}
	if providerCfg.ContextLength > 0 {
		return providerCfg.ContextLength, nil
	}
	switch providerCfg.Type {
	case "anthropic", "codex":
		return 200000, nil
	case "openai-compatible":
		return 32768, nil
	default:
		return 0, fmt.Errorf("provider %s has no positive context_length configured", providerName)
	}
}
