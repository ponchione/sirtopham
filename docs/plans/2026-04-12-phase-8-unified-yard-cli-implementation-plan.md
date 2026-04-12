# Phase 8 Unified Yard CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate all 19 operator-facing commands under the `yard` binary. Today three CLIs (`yard`, `tidmouth`, `sirtopham`) spread commands across three entry points with no discoverability between them. After Phase 8, `yard` owns the entire operator surface. Internal binaries (`tidmouth`, `sirtopham`) continue building for subprocess use but are not part of the documented operator surface.

**Architecture:** Extract runtime builders and shared helpers from `cmd/tidmouth/` and `cmd/sirtopham/` into a new `internal/runtime/` package (Go forbids importing `main` packages across binaries). Update the legacy binaries to call the extracted code. Wire new cobra commands in `cmd/yard/` that delegate to the same extracted runtime.

**Tech Stack:** Go 1.25+, cobra, `internal/runtime/` (new), all existing `internal/` packages. No new third-party dependencies.

**Spec:** [`docs/specs/18-unified-yard-cli.md`](../specs/18-unified-yard-cli.md)

---

## Required reading before starting

Read these in order:

1. `AGENTS.md` -- repo conventions and hard rules
2. `docs/specs/18-unified-yard-cli.md` -- the design spec this plan implements
3. `cmd/tidmouth/runtime.go` -- `buildAppRuntime()`, `appRuntime` struct, `chainCleanup()`, `buildBrainBackend()`, `buildGraphStore()`, `buildConventionSource()`, `ensureProjectRecord()`
4. `cmd/sirtopham/runtime.go` -- `buildOrchestratorRuntime()`, `orchestratorRuntime` struct, `noopContextAssembler`, `registryToolExecutor`, `buildOrchestratorRegistry()`, `buildProvider()`, `resolveProviderAPIKey()`, `ensureProjectRecord()`
5. `cmd/tidmouth/serve.go` -- `buildProvider()`, `resolveProviderAPIKey()`, `withProviderAlias()`, `aliasedProvider`, `logProviderAuthStatus()` (via `provider_auth_logging.go`)
6. `cmd/tidmouth/run.go` -- `loadRoleSystemPrompt()`, `resolveModelContextLimit()`, receipt/progress helpers
7. `cmd/sirtopham/chain.go` -- `loadRoleSystemPrompt()`, `resolveModelContextLimit()` (duplicates of tidmouth's)
8. `cmd/yard/main.go` -- current root command with `init` + `install`
9. `Makefile` -- build targets for all 4 binaries

After reading, run `make test` to confirm the baseline is green before touching anything.

---

## Locked decisions (do not re-litigate)

These are fixed for Phase 8. If implementation reveals one is wrong, stop and ask before changing.

1. All extracted types in `internal/runtime/` use **PascalCase** (Go exported names).
2. `tidmouth`'s `buildBrainBackend` signature `(brain.Backend, func(), error)` wins over sirtopham's `(brain.Backend, error)`. The extracted version returns 3 values.
3. `tidmouth`'s `buildProvider` (with `withProviderAlias` + `logProviderAuthStatus`) wins over sirtopham's simpler version.
4. `ensureProjectRecord` is extracted once; both tidmouth and sirtopham use `filepath.Base()` for the project name (tidmouth's version).
5. `loadRoleSystemPrompt` and `resolveModelContextLimit` are extracted once from their duplicated copies.
6. The sirtopham runtime's `cfg.ProviderNamesForSurfaces()` loop for provider registration (R6 fix) is preserved exactly.
7. The sirtopham runtime's `provRouter.DrainTracking()` before DB close (R5 fix) is preserved exactly.
8. `cmd/yard/chain.go` uses `yard chain start` (cobra `Use: "start"`), not `yard chain` as the run subcommand.
9. The spawn subprocess binary stays `"tidmouth"` -- `yard chain start` calls the orchestrator runtime which spawns `tidmouth run` subprocesses.
10. No Makefile changes needed -- the existing `yard:` target already builds `cmd/yard/`.
11. Legacy `tidmouth` and `sirtopham` binaries continue to work unchanged after extraction.
12. `cmd/yard/serve.go` must import `webfs` for the embedded frontend, same as `cmd/tidmouth/serve.go`.

---

## File structure

**New files:**

```
internal/runtime/
  helpers.go              # EnsureProjectRecord, ChainCleanup, LoadRoleSystemPrompt, ResolveModelContextLimit
  provider.go             # BuildProvider, ResolveProviderAPIKey, WithProviderAlias, AliasedProvider, LogProviderAuthStatus, ErrorAsProviderError
  engine.go               # EngineRuntime struct + BuildEngineRuntime(), BuildBrainBackend (3-return), BuildGraphStore, BuildConventionSource
  orchestrator.go         # OrchestratorRuntime struct + BuildOrchestratorRuntime(), NoopContextAssembler, RegistryToolExecutor, BuildOrchestratorRegistry

cmd/yard/
  serve.go                # yard serve
  run.go                  # yard run
  index.go                # yard index + yard index (code only, no brain subcommand here)
  auth.go                 # yard auth + yard auth status + yard doctor
  config_cmd.go           # yard config
  llm.go                  # yard llm + status/up/down/logs
  brain.go                # yard brain + index/serve subcommands
  chain.go                # yard chain + start/status/logs/receipt/cancel/pause/resume
  run_helpers.go          # receipt, progress sink, readTask, etc. (shared helpers for run command)
```

**Modified files:**

```
cmd/yard/main.go          # add --config persistent flag, register all new subcommands
cmd/tidmouth/runtime.go   # thin wrapper calling internal/runtime
cmd/tidmouth/serve.go     # remove buildProvider, resolveProviderAPIKey, withProviderAlias, aliasedProvider (use internal/runtime)
cmd/tidmouth/run.go       # remove loadRoleSystemPrompt, resolveModelContextLimit (use internal/runtime)
cmd/tidmouth/auth.go      # remove errorAsProviderError (use internal/runtime)
cmd/tidmouth/index.go     # update buildBrainIndexBackend to use internal/runtime.BuildBrainBackend
cmd/tidmouth/provider_auth_logging.go  # remove logProviderAuthStatus (use internal/runtime)
cmd/sirtopham/runtime.go  # thin wrapper calling internal/runtime
cmd/sirtopham/chain.go    # remove loadRoleSystemPrompt, resolveModelContextLimit (use internal/runtime)
```

**Unchanged:**

- `cmd/yard/init.go` -- stays as-is
- `cmd/yard/install.go` -- stays as-is
- `Makefile` -- no changes needed (already builds all 4 binaries)
- All other `internal/` packages
- Web frontend, Docker infrastructure, templates, agent prompts, specs

---

## Checkpoints

| Checkpoint | Tasks | Proof |
|---|---|---|
| CP1: Helpers + provider extracted | 1, 2 | `internal/runtime/` compiles, `make test` passes |
| CP2: Engine runtime extracted | 3, 4 | `BuildEngineRuntime` callable from both `cmd/tidmouth` and `cmd/yard`; `make test` passes |
| CP3: Orchestrator runtime extracted | 5, 6 | `BuildOrchestratorRuntime` callable from both `cmd/sirtopham` and `cmd/yard`; `make test` passes |
| CP4: Legacy binaries updated | 7 | `cmd/tidmouth` and `cmd/sirtopham` use `internal/runtime/`; `make test` passes; no duplicate helper functions |
| CP5: Yard CLI wired (engine commands) | 8, 9, 10, 11, 12, 13 | `yard serve`, `yard run`, `yard index`, `yard auth`, `yard config`, `yard llm` all compile |
| CP6: Yard CLI wired (orchestrator + brain) | 14, 15 | `yard chain start`, `yard brain index`, `yard brain serve` all compile |
| CP7: Full verification + tag | 16 | `make all` green, `make test` green, `yard --help` shows all commands, tag `v0.8-unified-cli` |

If you finish a session mid-checkpoint, update `NEXT_SESSION_HANDOFF.md` with the current checkpoint, the failing command/test, and the next unresolved sub-step.

---

## Task 1: Create `internal/runtime/helpers.go` -- shared helpers

**Files:**
- Create: `internal/runtime/helpers.go`

**Background:** Four helper functions are duplicated between `cmd/tidmouth/` and `cmd/sirtopham/`. Extract them into a shared package. `ChainCleanup` comes from `cmd/tidmouth/runtime.go`. `EnsureProjectRecord` exists in both `cmd/tidmouth/runtime.go` and `cmd/sirtopham/runtime.go` (slightly different implementations -- use tidmouth's which uses `filepath.Base()`). `LoadRoleSystemPrompt` exists in both `cmd/tidmouth/run.go` and `cmd/sirtopham/chain.go`. `ResolveModelContextLimit` exists in both `cmd/tidmouth/run.go` and `cmd/sirtopham/chain.go`.

- [ ] **Step 1.1: Create the helpers file**

Create `internal/runtime/helpers.go` with the following content:

```go
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
```

- [ ] **Step 1.2: Verify the new file compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./internal/runtime/
```

---

## Task 2: Create `internal/runtime/provider.go` -- provider construction

**Files:**
- Create: `internal/runtime/provider.go`

**Background:** Provider construction is duplicated between `cmd/tidmouth/serve.go` and `cmd/sirtopham/runtime.go`. The tidmouth version is more complete (includes `withProviderAlias`, `logProviderAuthStatus`, `errorAsProviderError`). Extract the tidmouth version. The `AliasedProvider` type must implement all four interfaces: `provider.Provider`, `provider.Pinger`, `provider.AuthStatusReporter` (via method delegation).

- [ ] **Step 2.1: Create the provider file**

Create `internal/runtime/provider.go` with the following content:

```go
package runtime

import (
	"context"
	"errors"
	"log/slog"
	"os"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/provider"
	"github.com/ponchione/sodoryard/internal/provider/anthropic"
	"github.com/ponchione/sodoryard/internal/provider/codex"
	"github.com/ponchione/sodoryard/internal/provider/openai"
)

// ResolveProviderAPIKey returns the API key for a provider config, checking
// the direct APIKey field first, then the APIKeyEnv environment variable.
func ResolveProviderAPIKey(cfg appconfig.ProviderConfig) string {
	if cfg.APIKey != "" {
		return cfg.APIKey
	}
	if cfg.APIKeyEnv != "" {
		return os.Getenv(cfg.APIKeyEnv)
	}
	return ""
}

// BuildProvider constructs a provider.Provider from config. It applies
// name aliasing when the constructed provider's internal name differs
// from the config key.
func BuildProvider(name string, cfg appconfig.ProviderConfig) (provider.Provider, error) {
	apiKey := ResolveProviderAPIKey(cfg)

	switch cfg.Type {
	case "anthropic":
		var credOpts []anthropic.CredentialOption
		if apiKey != "" {
			credOpts = append(credOpts, anthropic.WithAPIKey(apiKey))
		}
		creds, err := anthropic.NewCredentialManager(credOpts...)
		if err != nil {
			return nil, err
		}
		return WithProviderAlias(name, anthropic.NewAnthropicProvider(creds)), nil

	case "openai-compatible":
		return openai.NewOpenAIProvider(openai.OpenAIConfig{
			Name:          name,
			BaseURL:       cfg.BaseURL,
			APIKey:        apiKey,
			Model:         cfg.Model,
			ContextLength: cfg.ContextLength,
		})

	case "codex":
		var opts []codex.ProviderOption
		if cfg.BaseURL != "" {
			opts = append(opts, codex.WithBaseURL(cfg.BaseURL))
		}
		p, err := codex.NewCodexProvider(opts...)
		if err != nil {
			return nil, err
		}
		return WithProviderAlias(name, p), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %q", cfg.Type)
	}
}

// WithProviderAlias wraps a provider to override its Name() if the
// config-level name differs from the provider's built-in name.
func WithProviderAlias(name string, inner provider.Provider) provider.Provider {
	if inner == nil || name == "" || inner.Name() == name {
		return inner
	}
	return AliasedProvider{Name_: name, Inner: inner}
}

// AliasedProvider wraps a provider.Provider, overriding Name() with a
// config-level alias while delegating all other methods to the inner
// provider.
type AliasedProvider struct {
	Name_ string
	Inner provider.Provider
}

func (p AliasedProvider) Name() string {
	return p.Name_
}

func (p AliasedProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return p.Inner.Complete(ctx, req)
}

func (p AliasedProvider) Stream(ctx context.Context, req *provider.Request) (<-chan provider.StreamEvent, error) {
	return p.Inner.Stream(ctx, req)
}

func (p AliasedProvider) Models(ctx context.Context) ([]provider.Model, error) {
	return p.Inner.Models(ctx)
}

func (p AliasedProvider) Ping(ctx context.Context) error {
	pinger, ok := p.Inner.(provider.Pinger)
	if !ok {
		return nil
	}
	return pinger.Ping(ctx)
}

func (p AliasedProvider) AuthStatus(ctx context.Context) (*provider.AuthStatus, error) {
	reporter, ok := p.Inner.(provider.AuthStatusReporter)
	if !ok {
		return nil, nil
	}
	status, err := reporter.AuthStatus(ctx)
	if err != nil || status == nil {
		return status, err
	}
	cloned := *status
	cloned.Provider = p.Name_
	return &cloned, nil
}

// LogProviderAuthStatus logs the authentication status of a provider
// at registration time for operator diagnostics.
func LogProviderAuthStatus(ctx context.Context, logger *slog.Logger, name string, cfg appconfig.ProviderConfig, p provider.Provider) {
	attrs := []any{"name", name, "type", cfg.Type}
	reporter, ok := p.(provider.AuthStatusReporter)
	if !ok {
		logger.Info("registered provider", attrs...)
		return
	}
	status, err := reporter.AuthStatus(ctx)
	if err != nil {
		attrs = append(attrs, "auth_status_error", err.Error())
		var pe *provider.ProviderError
		if ErrorAsProviderError(err, &pe) && pe.Remediation != "" {
			attrs = append(attrs, "auth_remediation", pe.Remediation)
		}
		logger.Info("registered provider", attrs...)
		return
	}
	attrs = append(attrs,
		"auth_mode", status.Mode,
		"auth_source", status.Source,
		"auth_store", status.StorePath,
		"auth_source_path", status.SourcePath,
		"auth_has_refresh", status.HasRefreshToken,
	)
	if !status.ExpiresAt.IsZero() {
		attrs = append(attrs, "auth_expires_at", status.ExpiresAt)
	}
	if !status.LastRefresh.IsZero() {
		attrs = append(attrs, "auth_last_refresh", status.LastRefresh)
	}
	logger.Info("registered provider", attrs...)
}

// ErrorAsProviderError is a helper that wraps errors.As for provider.ProviderError.
func ErrorAsProviderError(err error, out **provider.ProviderError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, out)
}
```

- [ ] **Step 2.2: Add the missing `fmt` import**

Note: the `BuildProvider` function uses `fmt.Errorf`. Ensure the import block includes `"fmt"`. (The code above already includes it implicitly in the switch default case -- verify the import block is correct.)

Update the import block in `internal/runtime/provider.go` to include `"fmt"`:

```go
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/provider"
	"github.com/ponchione/sodoryard/internal/provider/anthropic"
	"github.com/ponchione/sodoryard/internal/provider/codex"
	"github.com/ponchione/sodoryard/internal/provider/openai"
)
```

- [ ] **Step 2.3: Verify the package compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./internal/runtime/
```

- [ ] **Step 2.4: Commit Phase A foundation**

```bash
cd /home/gernsback/source/sodoryard
git add internal/runtime/helpers.go internal/runtime/provider.go
git commit -m "feat(runtime): extract shared helpers and provider construction into internal/runtime

Phase 8 step: move ChainCleanup, EnsureProjectRecord, LoadRoleSystemPrompt,
ResolveModelContextLimit, BuildProvider, AliasedProvider, and
LogProviderAuthStatus from cmd/tidmouth and cmd/sirtopham into a shared
internal/runtime package. This enables cmd/yard to import the same runtime
builders without duplicating code."
```

---

## Task 3: Create `internal/runtime/engine.go` -- engine runtime

**Files:**
- Create: `internal/runtime/engine.go`

**Background:** The `EngineRuntime` struct and `BuildEngineRuntime` function are extracted from `cmd/tidmouth/runtime.go`'s `appRuntime` and `buildAppRuntime`. This is the most complex extraction -- it constructs the full engine harness runtime including database, provider router, brain backend, code vectorstore, graph store, convention source, context assembler, and conversation manager.

- [ ] **Step 3.1: Create the engine runtime file**

Create `internal/runtime/engine.go` with the following content:

```go
package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ponchione/sodoryard/internal/brain"
	"github.com/ponchione/sodoryard/internal/brain/mcpclient"
	"github.com/ponchione/sodoryard/internal/codeintel/embedder"
	codegraph "github.com/ponchione/sodoryard/internal/codeintel/graph"
	codesearcher "github.com/ponchione/sodoryard/internal/codeintel/searcher"
	"github.com/ponchione/sodoryard/internal/codestore"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	contextpkg "github.com/ponchione/sodoryard/internal/context"
	"github.com/ponchione/sodoryard/internal/conversation"
	appdb "github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/logging"
	"github.com/ponchione/sodoryard/internal/provider/router"
	"github.com/ponchione/sodoryard/internal/provider/tracking"
)

// EngineRuntime holds the fully-constructed engine harness runtime.
// It is the exported equivalent of cmd/tidmouth's appRuntime.
type EngineRuntime struct {
	Config              *appconfig.Config
	Logger              *slog.Logger
	Database            *sql.DB
	Queries             *appdb.Queries
	ProviderRouter      *router.Router
	BrainBackend        brain.Backend
	SemanticSearcher    *codesearcher.Searcher
	BrainSearcher       *contextpkg.HybridBrainSearcher
	ConversationManager *conversation.Manager
	ContextAssembler    *contextpkg.ContextAssembler
	Cleanup             func()
}

// BuildEngineRuntime constructs the full engine harness runtime from config.
// This is the exported equivalent of cmd/tidmouth's buildAppRuntime.
func BuildEngineRuntime(ctx context.Context, cfg *appconfig.Config) (*EngineRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("runtime config is required")
	}

	logger, err := logging.Init(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return nil, fmt.Errorf("init logging: %w", err)
	}

	database, err := appdb.OpenDB(ctx, cfg.DatabasePath())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() {
		_ = database.Close()
	}
	closeOnError := func(err error) (*EngineRuntime, error) {
		cleanup()
		return nil, err
	}

	if _, err := appdb.InitIfNeeded(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("init database schema: %w", err))
	}
	if err := appdb.EnsureMessageSearchIndexesIncludeTools(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("upgrade message search indexes: %w", err))
	}
	if err := appdb.EnsureContextReportsIncludeTokenBudget(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("upgrade context report token budget storage: %w", err))
	}
	if err := appdb.EnsureChainSchema(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("ensure chain schema: %w", err))
	}
	queries := appdb.New(database)
	if err := EnsureProjectRecord(ctx, database, cfg); err != nil {
		return closeOnError(fmt.Errorf("ensure project record: %w", err))
	}

	routerCfg := router.RouterConfig{
		Default: router.RouteTarget{
			Provider: cfg.Routing.Default.Provider,
			Model:    cfg.Routing.Default.Model,
		},
		Fallback: router.RouteTarget{
			Provider: cfg.Routing.Fallback.Provider,
			Model:    cfg.Routing.Fallback.Model,
		},
	}
	provRouter, err := router.NewRouter(routerCfg, tracking.NewSQLiteSubCallStore(queries), logger)
	if err != nil {
		return closeOnError(fmt.Errorf("create router: %w", err))
	}
	for name, provCfg := range cfg.Providers {
		p, err := BuildProvider(name, provCfg)
		if err != nil {
			return closeOnError(fmt.Errorf("build provider %q: %w", name, err))
		}
		if err := provRouter.RegisterProvider(p); err != nil {
			return closeOnError(fmt.Errorf("register provider %q: %w", name, err))
		}
		LogProviderAuthStatus(ctx, logger, name, provCfg, p)
	}
	if err := provRouter.Validate(ctx); err != nil {
		return closeOnError(fmt.Errorf("validate providers: %w", err))
	}

	codeStore, err := codestore.Open(ctx, cfg.CodeLanceDBPath())
	if err != nil {
		return closeOnError(fmt.Errorf("open code vectorstore: %w", err))
	}
	cleanup = ChainCleanup(cleanup, func() { _ = codeStore.Close() })
	semanticEmbedder := embedder.New(cfg.Embedding)
	semanticSearcher := codesearcher.New(codeStore, semanticEmbedder)

	brainStore, err := codestore.Open(ctx, cfg.BrainLanceDBPath())
	if err != nil {
		return closeOnError(fmt.Errorf("open brain vectorstore: %w", err))
	}
	cleanup = ChainCleanup(cleanup, func() { _ = brainStore.Close() })

	brainBackend, closeBrainBackend, err := BuildBrainBackend(ctx, cfg.Brain, logger)
	if err != nil {
		return closeOnError(fmt.Errorf("build brain backend: %w", err))
	}
	cleanup = ChainCleanup(cleanup, closeBrainBackend)

	graphStore, closeGraphStore, err := BuildGraphStore(cfg)
	if err != nil {
		return closeOnError(fmt.Errorf("build graph store: %w", err))
	}
	cleanup = ChainCleanup(cleanup, closeGraphStore)

	conventionSource := BuildConventionSource(cfg)
	brainSearcher := contextpkg.NewHybridBrainSearcher(brainBackend, brainStore, semanticEmbedder, queries, cfg.ProjectRoot)
	retrievalOrchestrator := contextpkg.NewRetrievalOrchestrator(semanticSearcher, graphStore, conventionSource, brainSearcher, cfg.ProjectRoot)
	retrievalOrchestrator.SetLogBrainQueries(cfg.Brain.LogBrainQueries)
	retrievalOrchestrator.SetBrainConfig(cfg.Brain)
	budgetManager := contextpkg.PriorityBudgetManager{}
	budgetManager.SetBrainConfig(cfg.Brain)

	convManager := conversation.NewManager(database, nil, logger)
	contextAssembler := contextpkg.NewContextAssembler(
		contextpkg.RuleBasedAnalyzer{},
		contextpkg.HeuristicQueryExtractor{},
		contextpkg.HistoryMomentumTracker{},
		retrievalOrchestrator,
		budgetManager,
		contextpkg.MarkdownSerializer{},
		cfg.Context,
		database,
	)

	return &EngineRuntime{
		Config:              cfg,
		Logger:              logger,
		Database:            database,
		Queries:             queries,
		ProviderRouter:      provRouter,
		BrainBackend:        brainBackend,
		SemanticSearcher:    semanticSearcher,
		BrainSearcher:       brainSearcher,
		ConversationManager: convManager,
		ContextAssembler:    contextAssembler,
		Cleanup:             cleanup,
	}, nil
}

// BuildBrainBackend creates the brain MCP client backend if brain is enabled.
// Returns (backend, cleanup, error). The cleanup function is always safe to call.
func BuildBrainBackend(ctx context.Context, cfg appconfig.BrainConfig, logger *slog.Logger) (brain.Backend, func(), error) {
	if !cfg.Enabled {
		return nil, func() {}, nil
	}
	client, err := mcpclient.Connect(ctx, cfg.VaultPath)
	if err != nil {
		return nil, func() {}, err
	}
	logger.Info("brain backend: MCP (in-process)", "vault", cfg.VaultPath)
	return client, func() { _ = client.Close() }, nil
}

// BuildGraphStore opens the code graph SQLite store.
// Returns (store, cleanup, error). The cleanup function is always safe to call.
func BuildGraphStore(cfg *appconfig.Config) (*codegraph.Store, func(), error) {
	if err := os.MkdirAll(filepath.Dir(cfg.GraphDBPath()), 0o755); err != nil {
		return nil, func() {}, err
	}
	store, err := codegraph.NewStore(cfg.GraphDBPath())
	if err != nil {
		return nil, func() {}, err
	}
	return store, func() { _ = store.Close() }, nil
}

// BuildConventionSource creates the brain-backed convention source for
// context assembly.
func BuildConventionSource(cfg *appconfig.Config) contextpkg.ConventionSource {
	return contextpkg.NewBrainConventionSource(cfg.BrainVaultPath())
}
```

- [ ] **Step 3.2: Verify the package compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./internal/runtime/
```

- [ ] **Step 3.3: Commit engine runtime extraction**

```bash
cd /home/gernsback/source/sodoryard
git add internal/runtime/engine.go
git commit -m "feat(runtime): extract engine runtime builder into internal/runtime

Move BuildEngineRuntime (from tidmouth's buildAppRuntime), BuildBrainBackend,
BuildGraphStore, and BuildConventionSource into internal/runtime/engine.go.
The EngineRuntime struct mirrors tidmouth's appRuntime with exported fields."
```

---

## Task 4: Create `internal/runtime/orchestrator.go` -- orchestrator runtime

**Files:**
- Create: `internal/runtime/orchestrator.go`

**Background:** The `OrchestratorRuntime` struct and `BuildOrchestratorRuntime` function are extracted from `cmd/sirtopham/runtime.go`. Key differences from the engine runtime: (1) uses `cfg.ProviderNamesForSurfaces()` for provider registration (R6 fix), (2) calls `provRouter.DrainTracking()` before DB close (R5 fix), (3) uses a simpler brain backend (no cleanup function), (4) includes `ChainStore`, (5) uses `NoopContextAssembler` instead of the full context assembly pipeline.

- [ ] **Step 4.1: Create the orchestrator runtime file**

Create `internal/runtime/orchestrator.go` with the following content:

```go
package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/ponchione/sodoryard/internal/agent"
	"github.com/ponchione/sodoryard/internal/brain"
	"github.com/ponchione/sodoryard/internal/brain/mcpclient"
	"github.com/ponchione/sodoryard/internal/chain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	contextpkg "github.com/ponchione/sodoryard/internal/context"
	"github.com/ponchione/sodoryard/internal/conversation"
	appdb "github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/logging"
	"github.com/ponchione/sodoryard/internal/provider"
	"github.com/ponchione/sodoryard/internal/provider/router"
	"github.com/ponchione/sodoryard/internal/provider/tracking"
	"github.com/ponchione/sodoryard/internal/role"
	spawnpkg "github.com/ponchione/sodoryard/internal/spawn"
	"github.com/ponchione/sodoryard/internal/tool"
)

// OrchestratorRuntime holds the fully-constructed chain orchestrator runtime.
// It is the exported equivalent of cmd/sirtopham's orchestratorRuntime.
type OrchestratorRuntime struct {
	Config              *appconfig.Config
	Logger              *slog.Logger
	Database            *sql.DB
	Queries             *appdb.Queries
	ProviderRouter      *router.Router
	BrainBackend        brain.Backend
	ConversationManager *conversation.Manager
	ContextAssembler    agent.ContextAssembler
	ChainStore          *chain.Store
	Cleanup             func()
}

// NoopContextAssembler is a context assembler that returns an empty frozen
// context package. The orchestrator does not need RAG context -- it delegates
// that to the spawned engine subprocesses.
type NoopContextAssembler struct{}

func (NoopContextAssembler) Assemble(ctx context.Context, message string, history []appdb.Message, scope contextpkg.AssemblyScope, modelContextLimit int, historyTokenCount int) (*contextpkg.FullContextPackage, bool, error) {
	return &contextpkg.FullContextPackage{Content: "", Frozen: true, Report: &contextpkg.ContextAssemblyReport{TurnNumber: scope.TurnNumber}}, false, nil
}

func (NoopContextAssembler) UpdateQuality(context.Context, string, int, bool, []string) error {
	return nil
}

// RegistryToolExecutor adapts a tool.Registry into the agent.ToolExecutor
// interface for the orchestrator's agent loop.
type RegistryToolExecutor struct {
	Registry    *tool.Registry
	ProjectRoot string
}

func (e *RegistryToolExecutor) Execute(ctx context.Context, call provider.ToolCall) (*provider.ToolResult, error) {
	t, ok := e.Registry.Get(call.Name)
	if !ok {
		return &provider.ToolResult{ToolUseID: call.ID, Content: fmt.Sprintf("Unknown tool: %s", call.Name), IsError: true}, nil
	}
	result, err := t.Execute(ctx, e.ProjectRoot, call.Input)
	if err != nil {
		return nil, err
	}
	result.CallID = call.ID
	return &provider.ToolResult{ToolUseID: call.ID, Content: result.Content, IsError: !result.Success}, nil
}

// BuildOrchestratorRuntime constructs the chain orchestrator runtime from config.
// This is the exported equivalent of cmd/sirtopham's buildOrchestratorRuntime.
func BuildOrchestratorRuntime(ctx context.Context, cfg *appconfig.Config) (*OrchestratorRuntime, error) {
	logger, err := logging.Init(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return nil, fmt.Errorf("init logging: %w", err)
	}
	database, err := appdb.OpenDB(ctx, cfg.DatabasePath())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() { _ = database.Close() }
	if _, err := appdb.InitIfNeeded(ctx, database); err != nil {
		cleanup()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	for _, fn := range []func(context.Context, *sql.DB) error{appdb.EnsureMessageSearchIndexesIncludeTools, appdb.EnsureContextReportsIncludeTokenBudget, appdb.EnsureChainSchema} {
		if err := fn(ctx, database); err != nil {
			cleanup()
			return nil, err
		}
	}
	queries := appdb.New(database)
	if err := EnsureProjectRecord(ctx, database, cfg); err != nil {
		cleanup()
		return nil, fmt.Errorf("ensure project record: %w", err)
	}
	routerCfg := router.RouterConfig{Default: router.RouteTarget{Provider: cfg.Routing.Default.Provider, Model: cfg.Routing.Default.Model}, Fallback: router.RouteTarget{Provider: cfg.Routing.Fallback.Provider, Model: cfg.Routing.Fallback.Model}}
	provRouter, err := router.NewRouter(routerCfg, tracking.NewSQLiteSubCallStore(queries), logger)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create router: %w", err)
	}
	// Only register providers the YAML explicitly listed. This avoids
	// registering Default() providers (anthropic, openrouter) that the
	// operator's config never asked for (TECH-DEBT R6).
	providerNames := cfg.ProviderNamesForSurfaces()
	for _, name := range providerNames {
		provCfg, ok := cfg.Providers[name]
		if !ok {
			continue
		}
		p, err := BuildProvider(name, provCfg)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("build provider %q: %w", name, err)
		}
		if err := provRouter.RegisterProvider(p); err != nil {
			cleanup()
			return nil, fmt.Errorf("register provider %q: %w", name, err)
		}
	}
	if err := provRouter.Validate(ctx); err != nil {
		cleanup()
		return nil, fmt.Errorf("validate providers: %w", err)
	}
	brainBackend, err := buildOrchestratorBrainBackend(ctx, cfg.Brain)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("build brain backend: %w", err)
	}
	convManager := conversation.NewManager(database, nil, logger)
	return &OrchestratorRuntime{Config: cfg, Logger: logger, Database: database, Queries: queries, ProviderRouter: provRouter, BrainBackend: brainBackend, ConversationManager: convManager, ContextAssembler: NoopContextAssembler{}, ChainStore: chain.NewStore(database), Cleanup: func() {
		// Drain in-flight sub-call writes before closing the DB so stream
		// goroutines don't race against database.Close() (TECH-DEBT R5).
		provRouter.DrainTracking()
		if brainBackend != nil {
			if c, ok := brainBackend.(interface{ Close() error }); ok {
				_ = c.Close()
			}
		}
		cleanup()
	}}, nil
}

// buildOrchestratorBrainBackend creates the brain backend for the orchestrator.
// Unlike the engine's BuildBrainBackend, this returns only (Backend, error)
// because the orchestrator manages brain cleanup in its own Cleanup function.
func buildOrchestratorBrainBackend(ctx context.Context, cfg appconfig.BrainConfig) (brain.Backend, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return mcpclient.Connect(ctx, cfg.VaultPath)
}

// BuildOrchestratorRegistry creates the tool registry for the chain orchestrator,
// including spawn_agent and chain_complete tools.
func BuildOrchestratorRegistry(rt *OrchestratorRuntime, roleCfg appconfig.AgentRoleConfig, chainID string) (*tool.Registry, error) {
	factory := map[string]func() tool.Tool{
		"spawn_agent": func() tool.Tool {
			return spawnpkg.NewSpawnAgentTool(spawnpkg.SpawnAgentDeps{Store: rt.ChainStore, Backend: rt.BrainBackend, Config: rt.Config, ChainID: chainID, EngineBinary: "tidmouth", ProjectRoot: rt.Config.ProjectRoot})
		},
		"chain_complete": func() tool.Tool { return spawnpkg.NewChainCompleteTool(rt.ChainStore, rt.BrainBackend, chainID) },
	}
	registry, _, err := role.BuildRegistry(rt.Config, roleCfg, role.BuilderDeps{BrainBackend: rt.BrainBackend, ProviderRuntime: rt.ProviderRouter, Queries: rt.Queries, ProjectID: rt.Config.ProjectRoot, CustomToolFactory: factory})
	if err != nil {
		return nil, err
	}
	return registry, nil
}
```

- [ ] **Step 4.2: Verify the package compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./internal/runtime/
```

- [ ] **Step 4.3: Commit orchestrator runtime extraction**

```bash
cd /home/gernsback/source/sodoryard
git add internal/runtime/orchestrator.go
git commit -m "feat(runtime): extract orchestrator runtime builder into internal/runtime

Move BuildOrchestratorRuntime, NoopContextAssembler, RegistryToolExecutor,
and BuildOrchestratorRegistry from cmd/sirtopham into internal/runtime.
Preserves R5 DrainTracking and R6 ProviderNamesForSurfaces behavior."
```

---

## Task 5: Update `cmd/tidmouth/runtime.go` to use `internal/runtime/`

**Files:**
- Modify: `cmd/tidmouth/runtime.go`

**Background:** Replace the entire `buildAppRuntime` body with a call to `runtime.BuildEngineRuntime`, and map the returned `EngineRuntime` back to the local `appRuntime` struct. Remove duplicated functions (`chainCleanup`, `ensureProjectRecord`, `buildBrainBackend`, `buildGraphStore`, `buildConventionSource`). Keep the `appRuntime` struct definition since all existing cmd/tidmouth command files reference it.

- [ ] **Step 5.1: Rewrite `cmd/tidmouth/runtime.go`**

Replace the entire content of `cmd/tidmouth/runtime.go` with:

```go
package main

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/ponchione/sodoryard/internal/brain"
	codesearcher "github.com/ponchione/sodoryard/internal/codeintel/searcher"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	contextpkg "github.com/ponchione/sodoryard/internal/context"
	"github.com/ponchione/sodoryard/internal/conversation"
	appdb "github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/provider/router"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
)

type appRuntime struct {
	Config              *appconfig.Config
	Logger              *slog.Logger
	Database            *sql.DB
	Queries             *appdb.Queries
	ProviderRouter      *router.Router
	BrainBackend        brain.Backend
	SemanticSearcher    *codesearcher.Searcher
	BrainSearcher       *contextpkg.HybridBrainSearcher
	ConversationManager *conversation.Manager
	ContextAssembler    *contextpkg.ContextAssembler
	Cleanup             func()
}

func buildAppRuntime(ctx context.Context, cfg *appconfig.Config) (*appRuntime, error) {
	rt, err := rtpkg.BuildEngineRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &appRuntime{
		Config:              rt.Config,
		Logger:              rt.Logger,
		Database:            rt.Database,
		Queries:             rt.Queries,
		ProviderRouter:      rt.ProviderRouter,
		BrainBackend:        rt.BrainBackend,
		SemanticSearcher:    rt.SemanticSearcher,
		BrainSearcher:       rt.BrainSearcher,
		ConversationManager: rt.ConversationManager,
		ContextAssembler:    rt.ContextAssembler,
		Cleanup:             rt.Cleanup,
	}, nil
}
```

Note: `chainCleanup`, `ensureProjectRecord`, `buildBrainBackend`, `buildGraphStore`, `buildConventionSource` are all removed -- they now live in `internal/runtime/`.

- [ ] **Step 5.2: Verify `cmd/tidmouth` compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./cmd/tidmouth/
```

This will fail if any other file in `cmd/tidmouth/` still references the deleted functions. Proceed to the next task to fix those references.

---

## Task 6: Update remaining `cmd/tidmouth/` files to use `internal/runtime/`

**Files:**
- Modify: `cmd/tidmouth/serve.go`
- Modify: `cmd/tidmouth/run.go`
- Modify: `cmd/tidmouth/auth.go`
- Modify: `cmd/tidmouth/index.go`
- Modify: `cmd/tidmouth/provider_auth_logging.go`

**Background:** Several `cmd/tidmouth/` files define functions that are now in `internal/runtime/`. Remove the local copies and replace references with calls to the `internal/runtime/` package. Some files (serve.go, auth.go) define local `buildProvider` / `resolveProviderAPIKey` / `withProviderAlias` / `aliasedProvider` / `logProviderAuthStatus` / `errorAsProviderError` that must be removed and replaced.

- [ ] **Step 6.1: Update `cmd/tidmouth/serve.go`**

Remove these functions from `cmd/tidmouth/serve.go`:
- `resolveProviderAPIKey` (lines 159-167)
- `buildProvider` (lines 170-208)
- `withProviderAlias` (lines 210-215)
- `aliasedProvider` type and all its methods (lines 217-258)

Keep `newServeCmd`, `runServe`, and `launchBrowser`. In `runServe`, the `buildAppRuntime` call stays (it's the local wrapper from task 5).

The resulting `cmd/tidmouth/serve.go` should be:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/agent"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/server"
	"github.com/ponchione/sodoryard/internal/tool"
	"github.com/ponchione/sodoryard/webfs"
)

func newServeCmd(configPath *string) *cobra.Command {
	var (
		portOverride int
		hostOverride string
		devMode      bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the tidmouth server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd, *configPath, portOverride, hostOverride, devMode)
		},
	}

	cmd.Flags().IntVar(&portOverride, "port", 0, "Override server port")
	cmd.Flags().StringVar(&hostOverride, "host", "", "Override server host")
	cmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode")

	return cmd
}

func runServe(cmd *cobra.Command, configPath string, portOverride int, hostOverride string, devMode bool) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if portOverride > 0 {
		cfg.Server.Port = portOverride
	}
	if hostOverride != "" {
		cfg.Server.Host = hostOverride
	}
	if devMode {
		cfg.Server.DevMode = true
	}

	runtimeBundle, err := buildAppRuntime(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer runtimeBundle.Cleanup()

	logger := runtimeBundle.Logger
	projectID := cfg.ProjectRoot
	logger.Info("tidmouth starting",
		"version", version,
		"project", cfg.ProjectRoot,
		"listen", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
		"dev_mode", cfg.Server.DevMode,
	)

	registry := tool.NewRegistry()
	tool.RegisterFileTools(registry)
	tool.RegisterGitTools(registry)
	tool.RegisterShellTool(registry, tool.ShellConfig{
		TimeoutSeconds: cfg.Agent.ShellTimeoutSeconds,
		Denylist:       cfg.Agent.ShellDenylist,
	})
	tool.RegisterBrainToolsWithProviderRuntimeAndIndex(registry, runtimeBundle.BrainBackend, runtimeBundle.BrainSearcher, cfg.Brain, runtimeBundle.ProviderRouter, runtimeBundle.Queries, cfg.ProjectRoot)
	tool.RegisterSearchTools(registry, runtimeBundle.SemanticSearcher)

	executor := tool.NewExecutor(registry, tool.ExecutorConfig{
		MaxOutputTokens: cfg.Agent.ToolOutputMaxTokens,
		ProjectRoot:     cfg.ProjectRoot,
	}, logger)
	executor.SetRecorder(tool.NewToolExecutionRecorder(runtimeBundle.Queries))
	adapter := tool.NewAgentLoopAdapter(executor)
	titleGen := conversation.NewTitleGen(runtimeBundle.ConversationManager, runtimeBundle.ProviderRouter, cfg.Routing.Default.Model, logger)

	agentLoop := agent.NewAgentLoop(agent.AgentLoopDeps{
		ContextAssembler:    runtimeBundle.ContextAssembler,
		ConversationManager: runtimeBundle.ConversationManager,
		ProviderRouter:      runtimeBundle.ProviderRouter,
		ToolExecutor:        adapter,
		ToolDefinitions:     registry.ToolDefinitions(),
		PromptBuilder:       agent.NewPromptBuilder(logger),
		TitleGenerator:      titleGen,
		Config: agent.AgentLoopConfig{
			MaxIterations:              cfg.Agent.MaxIterationsPerTurn,
			LoopDetectionThreshold:     cfg.Agent.LoopDetectionThreshold,
			ExtendedThinking:           cfg.Agent.ExtendedThinking,
			ProviderName:               cfg.Routing.Default.Provider,
			ModelName:                  cfg.Routing.Default.Model,
			EmitContextDebug:           cfg.Context.EmitContextDebug,
			ContextConfig:              cfg.Context,
			ToolResultStoreRoot:        cfg.Agent.ToolResultStoreRoot,
			CacheSystemPrompt:          cfg.Agent.CacheSystemPrompt,
			CacheAssembledContext:      cfg.Agent.CacheAssembledContext,
			CacheConversationHistory:   cfg.Agent.CacheConversationHistory,
			CompressHistoricalResults:  cfg.Agent.CompressHistoricalResults,
			StripHistoricalLineNumbers: cfg.Agent.StripHistoricalLineNumbers,
			ElideDuplicateReads:        cfg.Agent.ElideDuplicateReads,
			HistorySummarizeAfterTurns: cfg.Agent.HistorySummarizeAfterTurns,
		},
		Logger: logger,
	})
	defer agentLoop.Close()

	serverCfg := server.Config{Host: cfg.Server.Host, Port: cfg.Server.Port, DevMode: cfg.Server.DevMode}
	if !cfg.Server.DevMode {
		frontendFS, err := webfs.FS()
		if err != nil {
			logger.Warn("embedded frontend not available", "error", err)
		} else {
			serverCfg.FrontendFS = frontendFS
		}
	}

	srv := server.New(serverCfg, logger)
	runtimeDefaults := server.NewRuntimeDefaults(cfg)
	server.NewConversationHandler(srv, runtimeBundle.ConversationManager, projectID, logger)
	server.NewWebSocketHandler(srv, agentLoop, runtimeBundle.ConversationManager, cfg, runtimeDefaults, logger)
	server.NewProjectHandler(srv, cfg, logger)
	server.NewConfigHandler(srv, cfg, runtimeBundle.ProviderRouter, runtimeDefaults, logger)
	server.NewMetricsHandler(srv, runtimeBundle.Queries, logger)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.Server.OpenBrowser && !cfg.Server.DevMode {
		go launchBrowser(fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port), logger)
	}
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	logger.Info("shutting down")
	agentLoop.Cancel()
	logger.Info("shutdown complete")
	return nil
}

func launchBrowser(url string, logger *slog.Logger) {
	time.Sleep(500 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		logger.Debug("failed to open browser", "error", err)
	}
}
```

Note: The import of `"os"` is removed (was only used by `resolveProviderAPIKey`). The imports of `"github.com/ponchione/sodoryard/internal/provider"`, `"github.com/ponchione/sodoryard/internal/provider/anthropic"`, `"github.com/ponchione/sodoryard/internal/provider/codex"`, and `"github.com/ponchione/sodoryard/internal/provider/openai"` are removed. The `context` import stays because `launchBrowser` is not using it but `signal.NotifyContext` is in the `os/signal` package. Actually `context` import is no longer needed directly -- remove it. The `os/signal` import stays. The `os` import is needed by `signal.NotifyContext` -- wait, `signal.NotifyContext` takes a `context.Context` as first arg so we do still need `context` -- no, `cmd.Context()` returns one. Actually we don't import `context` directly -- we use `cmd.Context()` which returns a `context.Context`. The `context` package is not directly referenced. Remove the `context` import.

Wait -- let me re-examine. The original `serve.go` imports `context` but it's used by `aliasedProvider.Complete` etc. Since those are removed, check if `context` is still needed. `signal.NotifyContext` returns `context.Context` but is from `os/signal`. The function `runServe` uses `cmd.Context()` which is from cobra. So `context` is no longer directly imported. Remove it.

Actually -- looking more carefully at the original, `os` is imported and used by `signal.NotifyContext` via `os.Interrupt`. But we replaced `os.Interrupt` with `syscall.SIGINT`. Let me keep `os` imported because... no, `os.Interrupt` was in the original. Let me match the original pattern but with the removed functions. The original line is:

```go
ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
```

So `os` IS used for `os.Interrupt`. Keep `os` in the import. Remove `context` from the import since it's no longer directly referenced.

Let me correct the file. Replace the `signal.NotifyContext` line to preserve `os.Interrupt`:

```go
ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
```

And the import block should be:

```go
import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/agent"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/server"
	"github.com/ponchione/sodoryard/internal/tool"
	"github.com/ponchione/sodoryard/webfs"
)
```

- [ ] **Step 6.2: Update `cmd/tidmouth/run.go`**

Remove these functions from `cmd/tidmouth/run.go`:
- `loadRoleSystemPrompt` (lines 310-321)
- `resolveModelContextLimit` (lines 323-342)

Replace calls to them with `rtpkg.LoadRoleSystemPrompt` and `rtpkg.ResolveModelContextLimit`.

Add the import `rtpkg "github.com/ponchione/sodoryard/internal/runtime"` to the import block.

Change line 154 from:
```go
	systemPrompt, err := loadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
```
to:
```go
	systemPrompt, err := rtpkg.LoadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
```

Change line 243 from:
```go
	modelContextLimit, err := resolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
```
to:
```go
	modelContextLimit, err := rtpkg.ResolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
```

Remove the `loadRoleSystemPrompt` function definition (lines 310-321) and the `resolveModelContextLimit` function definition (lines 323-342).

- [ ] **Step 6.3: Update `cmd/tidmouth/auth.go`**

Remove the `errorAsProviderError` function (lines 159-164) from `cmd/tidmouth/auth.go`.

Add the import `rtpkg "github.com/ponchione/sodoryard/internal/runtime"` to the import block.

Replace the `buildProviderForAuthReports` variable assignment. Change line 28 from:
```go
var buildProviderForAuthReports = buildProvider
```
to:
```go
var buildProviderForAuthReports = rtpkg.BuildProvider
```

Replace calls to `errorAsProviderError` with `rtpkg.ErrorAsProviderError`. There are two call sites:
1. In `collectProviderAuthReports` around line 126: change `errorAsProviderError` to `rtpkg.ErrorAsProviderError`
2. In `collectProviderAuthReports` around line 148: change `errorAsProviderError` to `rtpkg.ErrorAsProviderError`

- [ ] **Step 6.4: Update `cmd/tidmouth/provider_auth_logging.go`**

Delete the entire file `cmd/tidmouth/provider_auth_logging.go`. The `logProviderAuthStatus` function now lives in `internal/runtime/provider.go` as `LogProviderAuthStatus`.

The only caller of `logProviderAuthStatus` was `cmd/tidmouth/runtime.go` line 120, but that entire function body has been replaced by the thin wrapper in Task 5 (which calls `rtpkg.BuildEngineRuntime` which internally calls `LogProviderAuthStatus`). So there is no remaining reference to the deleted function.

- [ ] **Step 6.5: Update `cmd/tidmouth/index.go`**

The `buildBrainIndexBackend` variable on line 27 references the local `buildBrainBackend`. Since `buildBrainBackend` was removed from `runtime.go` in Task 5, update it to reference the extracted version.

Change line 27 from:
```go
var buildBrainIndexBackend = buildBrainBackend
```
to:
```go
var buildBrainIndexBackend = rtpkg.BuildBrainBackend
```

Add the import `rtpkg "github.com/ponchione/sodoryard/internal/runtime"` to the import block.

Also, `ensureProjectRecord` on line 124 now needs to call the runtime version. Change:
```go
	if err := ensureProjectRecord(ctx, database, cfg); err != nil {
```
to:
```go
	if err := rtpkg.EnsureProjectRecord(ctx, database, cfg); err != nil {
```

- [ ] **Step 6.6: Verify `cmd/tidmouth` compiles and tests pass**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./cmd/tidmouth/
```

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" go test -tags 'sqlite_fts5' ./cmd/tidmouth/...
```

- [ ] **Step 6.7: Commit tidmouth migration**

```bash
cd /home/gernsback/source/sodoryard
git add cmd/tidmouth/runtime.go cmd/tidmouth/serve.go cmd/tidmouth/run.go cmd/tidmouth/auth.go cmd/tidmouth/index.go
git rm cmd/tidmouth/provider_auth_logging.go
git commit -m "refactor(tidmouth): delegate to internal/runtime for shared helpers

cmd/tidmouth now imports internal/runtime for BuildProvider, EnsureProjectRecord,
LoadRoleSystemPrompt, ResolveModelContextLimit, ErrorAsProviderError, and
BuildBrainBackend. Local copies removed. provider_auth_logging.go deleted
(LogProviderAuthStatus now in internal/runtime/provider.go)."
```

---

## Task 7: Update `cmd/sirtopham/` to use `internal/runtime/`

**Files:**
- Modify: `cmd/sirtopham/runtime.go`
- Modify: `cmd/sirtopham/chain.go`
- Modify: `cmd/sirtopham/status.go`
- Modify: `cmd/sirtopham/logs.go`
- Modify: `cmd/sirtopham/receipt.go`
- Modify: `cmd/sirtopham/cancel.go`
- Modify: `cmd/sirtopham/pause_resume.go`

**Background:** Replace `cmd/sirtopham/runtime.go`'s full implementation with a thin wrapper that calls `internal/runtime/`. Remove duplicated functions from `chain.go`. Update all command files that call `buildOrchestratorRuntime` to use the local wrapper. Also update `buildOrchestratorRegistry` references.

- [ ] **Step 7.1: Rewrite `cmd/sirtopham/runtime.go`**

Replace the entire content of `cmd/sirtopham/runtime.go` with:

```go
package main

import (
	"context"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/tool"
)

// orchestratorRuntime is a local alias for the extracted OrchestratorRuntime.
// Command files reference this type so we keep the thin wrapper.
type orchestratorRuntime = rtpkg.OrchestratorRuntime

func buildOrchestratorRuntime(ctx context.Context, cfg *appconfig.Config) (*orchestratorRuntime, error) {
	return rtpkg.BuildOrchestratorRuntime(ctx, cfg)
}

func buildOrchestratorRegistry(rt *orchestratorRuntime, roleCfg appconfig.AgentRoleConfig, chainID string) (*tool.Registry, error) {
	return rtpkg.BuildOrchestratorRegistry(rt, roleCfg, chainID)
}
```

- [ ] **Step 7.2: Update `cmd/sirtopham/chain.go`**

Remove the duplicated functions from `cmd/sirtopham/chain.go`:
- `loadRoleSystemPrompt` (lines 136-144)
- `resolveModelContextLimit` (lines 146-162)

Add the import `rtpkg "github.com/ponchione/sodoryard/internal/runtime"` to the import block.

Change the call to `loadRoleSystemPrompt` on line 62:
```go
	systemPrompt, err := loadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
```
to:
```go
	systemPrompt, err := rtpkg.LoadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
```

Change the call to `resolveModelContextLimit` on line 90:
```go
	limit, err := resolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
```
to:
```go
	limit, err := rtpkg.ResolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
```

Also update the `registryToolExecutor` reference in chain.go line 94. Since `registryToolExecutor` was defined in the old `runtime.go` and is now in `internal/runtime`, change:
```go
	ToolExecutor: &registryToolExecutor{registry: registry, projectRoot: cfg.ProjectRoot},
```
to:
```go
	ToolExecutor: &rtpkg.RegistryToolExecutor{Registry: registry, ProjectRoot: cfg.ProjectRoot},
```

Also remove the `noopContextAssembler` reference if present. Actually -- `noopContextAssembler` was in the old runtime.go. The new runtime.go uses a type alias `orchestratorRuntime = rtpkg.OrchestratorRuntime`, so `rt.ContextAssembler` already holds a `NoopContextAssembler` from `BuildOrchestratorRuntime`. No explicit reference to the local `noopContextAssembler` in chain.go -- it's only used inside `buildOrchestratorRuntime` which is now in `internal/runtime/`.

- [ ] **Step 7.3: Verify `cmd/sirtopham` compiles and tests pass**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./cmd/sirtopham/
```

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" go test -tags 'sqlite_fts5' ./cmd/sirtopham/...
```

- [ ] **Step 7.4: Run the full test suite**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" go test -tags 'sqlite_fts5' ./...
```

- [ ] **Step 7.5: Commit sirtopham migration**

```bash
cd /home/gernsback/source/sodoryard
git add cmd/sirtopham/runtime.go cmd/sirtopham/chain.go
git commit -m "refactor(sirtopham): delegate to internal/runtime for shared helpers

cmd/sirtopham now imports internal/runtime for BuildOrchestratorRuntime,
BuildOrchestratorRegistry, LoadRoleSystemPrompt, ResolveModelContextLimit,
and RegistryToolExecutor. Local copies removed."
```

---

## Task 8: Update `cmd/yard/main.go` -- add `--config` and register all subcommands

**Files:**
- Modify: `cmd/yard/main.go`

**Background:** The existing `cmd/yard/main.go` registers `init` and `install`. Add a `--config` persistent flag (defaulting to `yard.yaml`) and register all new subcommands. The `configPath` variable is shared by pointer with all subcommand constructors, matching the pattern used by `cmd/tidmouth/main.go` and `cmd/sirtopham/main.go`.

- [ ] **Step 8.1: Rewrite `cmd/yard/main.go`**

Replace the entire content of `cmd/yard/main.go` with:

```go
// Command yard is the unified operator-facing CLI for railway projects.
// It consolidates all operator commands from tidmouth and sirtopham under
// a single binary with a single --help.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	var configPath string

	rootCmd := &cobra.Command{
		Use:          "yard",
		Short:        "Yard — railway project operator CLI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "yard %s\n", version)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", appconfig.ConfigFilename, "Path to yard.yaml config file")

	rootCmd.AddCommand(
		newInitCmd(),
		newInstallCmd(),
		newYardServeCmd(&configPath),
		newYardRunCmd(&configPath),
		newYardIndexCmd(&configPath),
		newYardAuthCmd(&configPath),
		newYardDoctorCmd(&configPath),
		newYardConfigCmd(&configPath),
		newYardLLMCmd(&configPath),
		newYardBrainCmd(&configPath),
		newYardChainCmd(&configPath),
	)
	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if coded, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(coded.ExitCode())
		}
		os.Exit(1)
	}
}
```

- [ ] **Step 8.2: Verify the file compiles (it won't yet -- the new*Cmd functions don't exist)**

This step is expected to fail compilation. Proceed to the next tasks to create the subcommand files.

---

## Task 9: Create `cmd/yard/serve.go` -- yard serve

**Files:**
- Create: `cmd/yard/serve.go`

**Background:** `yard serve` mirrors `tidmouth serve` exactly. It loads config, builds the engine runtime, constructs the agent loop and tool registry, and starts the HTTP server with the embedded frontend.

- [ ] **Step 9.1: Create the serve command file**

Create `cmd/yard/serve.go` with the following content:

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/agent"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/server"
	"github.com/ponchione/sodoryard/internal/tool"
	"github.com/ponchione/sodoryard/webfs"
)

func newYardServeCmd(configPath *string) *cobra.Command {
	var (
		portOverride int
		hostOverride string
		devMode      bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI and API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runYardServe(cmd, *configPath, portOverride, hostOverride, devMode)
		},
	}

	cmd.Flags().IntVar(&portOverride, "port", 0, "Override server port")
	cmd.Flags().StringVar(&hostOverride, "host", "", "Override server host")
	cmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode")

	return cmd
}

func runYardServe(cmd *cobra.Command, configPath string, portOverride int, hostOverride string, devMode bool) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if portOverride > 0 {
		cfg.Server.Port = portOverride
	}
	if hostOverride != "" {
		cfg.Server.Host = hostOverride
	}
	if devMode {
		cfg.Server.DevMode = true
	}

	rt, err := rtpkg.BuildEngineRuntime(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer rt.Cleanup()

	logger := rt.Logger
	projectID := cfg.ProjectRoot
	logger.Info("yard serve starting",
		"version", version,
		"project", cfg.ProjectRoot,
		"listen", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
		"dev_mode", cfg.Server.DevMode,
	)

	registry := tool.NewRegistry()
	tool.RegisterFileTools(registry)
	tool.RegisterGitTools(registry)
	tool.RegisterShellTool(registry, tool.ShellConfig{
		TimeoutSeconds: cfg.Agent.ShellTimeoutSeconds,
		Denylist:       cfg.Agent.ShellDenylist,
	})
	tool.RegisterBrainToolsWithProviderRuntimeAndIndex(registry, rt.BrainBackend, rt.BrainSearcher, cfg.Brain, rt.ProviderRouter, rt.Queries, cfg.ProjectRoot)
	tool.RegisterSearchTools(registry, rt.SemanticSearcher)

	executor := tool.NewExecutor(registry, tool.ExecutorConfig{
		MaxOutputTokens: cfg.Agent.ToolOutputMaxTokens,
		ProjectRoot:     cfg.ProjectRoot,
	}, logger)
	executor.SetRecorder(tool.NewToolExecutionRecorder(rt.Queries))
	adapter := tool.NewAgentLoopAdapter(executor)
	titleGen := conversation.NewTitleGen(rt.ConversationManager, rt.ProviderRouter, cfg.Routing.Default.Model, logger)

	agentLoop := agent.NewAgentLoop(agent.AgentLoopDeps{
		ContextAssembler:    rt.ContextAssembler,
		ConversationManager: rt.ConversationManager,
		ProviderRouter:      rt.ProviderRouter,
		ToolExecutor:        adapter,
		ToolDefinitions:     registry.ToolDefinitions(),
		PromptBuilder:       agent.NewPromptBuilder(logger),
		TitleGenerator:      titleGen,
		Config: agent.AgentLoopConfig{
			MaxIterations:              cfg.Agent.MaxIterationsPerTurn,
			LoopDetectionThreshold:     cfg.Agent.LoopDetectionThreshold,
			ExtendedThinking:           cfg.Agent.ExtendedThinking,
			ProviderName:               cfg.Routing.Default.Provider,
			ModelName:                  cfg.Routing.Default.Model,
			EmitContextDebug:           cfg.Context.EmitContextDebug,
			ContextConfig:              cfg.Context,
			ToolResultStoreRoot:        cfg.Agent.ToolResultStoreRoot,
			CacheSystemPrompt:          cfg.Agent.CacheSystemPrompt,
			CacheAssembledContext:      cfg.Agent.CacheAssembledContext,
			CacheConversationHistory:   cfg.Agent.CacheConversationHistory,
			CompressHistoricalResults:  cfg.Agent.CompressHistoricalResults,
			StripHistoricalLineNumbers: cfg.Agent.StripHistoricalLineNumbers,
			ElideDuplicateReads:        cfg.Agent.ElideDuplicateReads,
			HistorySummarizeAfterTurns: cfg.Agent.HistorySummarizeAfterTurns,
		},
		Logger: logger,
	})
	defer agentLoop.Close()

	serverCfg := server.Config{Host: cfg.Server.Host, Port: cfg.Server.Port, DevMode: cfg.Server.DevMode}
	if !cfg.Server.DevMode {
		frontendFS, err := webfs.FS()
		if err != nil {
			logger.Warn("embedded frontend not available", "error", err)
		} else {
			serverCfg.FrontendFS = frontendFS
		}
	}

	srv := server.New(serverCfg, logger)
	runtimeDefaults := server.NewRuntimeDefaults(cfg)
	server.NewConversationHandler(srv, rt.ConversationManager, projectID, logger)
	server.NewWebSocketHandler(srv, agentLoop, rt.ConversationManager, cfg, runtimeDefaults, logger)
	server.NewProjectHandler(srv, cfg, logger)
	server.NewConfigHandler(srv, cfg, rt.ProviderRouter, runtimeDefaults, logger)
	server.NewMetricsHandler(srv, rt.Queries, logger)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.Server.OpenBrowser && !cfg.Server.DevMode {
		go yardLaunchBrowser(fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port), logger)
	}
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	logger.Info("shutting down")
	agentLoop.Cancel()
	logger.Info("shutdown complete")
	return nil
}

func yardLaunchBrowser(url string, logger *slog.Logger) {
	time.Sleep(500 * time.Millisecond)
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		logger.Debug("failed to open browser", "error", err)
	}
}
```

---

## Task 10: Create `cmd/yard/run.go` and `cmd/yard/run_helpers.go` -- yard run

**Files:**
- Create: `cmd/yard/run.go`
- Create: `cmd/yard/run_helpers.go`

**Background:** `yard run` mirrors `tidmouth run` exactly. The run command is the most complex command after serve. It needs the receipt helpers and progress sink. We create a separate `run_helpers.go` file for those since they're substantial.

- [ ] **Step 10.1: Create the run helpers file**

Create `cmd/yard/run_helpers.go` with the following content:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ponchione/sodoryard/internal/agent"
	"github.com/ponchione/sodoryard/internal/brain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/receipt"
	toolpkg "github.com/ponchione/sodoryard/internal/tool"
)

type yardReceiptFrontmatter = receipt.Receipt

type yardReceiptMetrics struct {
	TurnsUsed       int
	TokensUsed      int
	DurationSeconds int
}

func yardResolveReceiptPath(role string, chainID string, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return fmt.Sprintf("receipts/%s/%s.md", strings.TrimSpace(role), strings.TrimSpace(chainID))
}

func yardValidateReceiptContent(content string) (*yardReceiptFrontmatter, error) {
	parsed, err := receipt.Parse([]byte(content))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func yardEnsureReceipt(ctx context.Context, backend brain.Backend, brainCfg appconfig.BrainConfig, role string, chainID string, receiptPath string, verdict string, finalText string, turnResult *agent.TurnResult) (string, *yardReceiptFrontmatter, error) {
	normalizedPath, err := toolpkg.ValidateBrainWritePath(brainCfg, receiptPath)
	if err != nil {
		return "", nil, fmt.Errorf("receipt path policy: %w", err)
	}
	if backend == nil {
		return "", nil, fmt.Errorf("brain backend unavailable")
	}
	content, err := backend.ReadDocument(ctx, normalizedPath)
	if err == nil {
		r, validateErr := yardValidateReceiptContent(content)
		if validateErr != nil {
			return "", nil, fmt.Errorf("invalid receipt at %s: %w", normalizedPath, validateErr)
		}
		return normalizedPath, r, nil
	}
	if !strings.Contains(err.Error(), "Document not found") {
		return "", nil, fmt.Errorf("read receipt %s: %w", normalizedPath, err)
	}
	fallback, r := yardFormatFallbackReceipt(role, chainID, verdict, finalText, turnResult)
	if err := backend.WriteDocument(ctx, normalizedPath, fallback); err != nil {
		return "", nil, fmt.Errorf("write fallback receipt %s: %w", normalizedPath, err)
	}
	return normalizedPath, r, nil
}

func yardFormatFallbackReceipt(role string, chainID string, verdict string, finalText string, turnResult *agent.TurnResult) (string, *yardReceiptFrontmatter) {
	metrics := yardReceiptMetrics{}
	if turnResult != nil {
		metrics.TurnsUsed = turnResult.IterationCount
		metrics.TokensUsed = turnResult.TotalUsage.InputTokens + turnResult.TotalUsage.OutputTokens
		metrics.DurationSeconds = int(turnResult.Duration.Round(time.Second) / time.Second)
	}
	now := time.Now().UTC()
	r := &yardReceiptFrontmatter{
		Agent:           role,
		ChainID:         chainID,
		Step:            1,
		Verdict:         receipt.Verdict(verdict),
		Timestamp:       now,
		TurnsUsed:       metrics.TurnsUsed,
		TokensUsed:      metrics.TokensUsed,
		DurationSeconds: metrics.DurationSeconds,
	}
	body := strings.TrimSpace(finalText)
	if body == "" {
		body = "No final text was returned."
	}
	content := fmt.Sprintf(`---
agent: %s
chain_id: %s
step: 1
verdict: %s
timestamp: %s
turns_used: %d
tokens_used: %d
duration_seconds: %d
---

## Summary
%s

## Changes
- No agent-authored receipt was found; this fallback receipt was written by the harness.

## Concerns
- Review the final text and session logs if more detail is needed.

## Next Steps
- Inspect the task outcome and decide whether follow-up work is needed.
`, role, chainID, verdict, now.Format(time.RFC3339), metrics.TurnsUsed, metrics.TokensUsed, metrics.DurationSeconds, body)
	return content, r
}

type yardRunProgressSink struct {
	mu  sync.Mutex
	out io.Writer
}

func newYardRunProgressSink(out io.Writer) *yardRunProgressSink {
	return &yardRunProgressSink{out: out}
}

func (s *yardRunProgressSink) Emit(event agent.Event) {
	if s == nil || s.out == nil || event == nil {
		return
	}
	line := s.formatEvent(event)
	if line == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintln(s.out, line)
}

func (s *yardRunProgressSink) Close() {}

func (s *yardRunProgressSink) formatEvent(event agent.Event) string {
	switch e := event.(type) {
	case agent.StatusEvent:
		return fmt.Sprintf("status: %s", e.State)
	case agent.ContextDebugEvent:
		if e.Report == nil {
			return "context: assembled"
		}
		return fmt.Sprintf("context: assembled rag=%d brain=%d explicit_files=%d", len(e.Report.RAGResults), len(e.Report.BrainResults), len(e.Report.ExplicitFileResults))
	case agent.ToolCallStartEvent:
		args := ""
		if len(e.Arguments) > 0 {
			var compact map[string]any
			if json.Unmarshal(e.Arguments, &compact) == nil {
				if marshaled, err := json.Marshal(compact); err == nil {
					args = " " + string(marshaled)
				}
			}
		}
		return fmt.Sprintf("tool: start %s%s", e.ToolName, args)
	case agent.ToolCallEndEvent:
		return fmt.Sprintf("tool: end %s success=%t duration=%s", e.ToolCallID, e.Success, e.Duration)
	case agent.TurnCompleteEvent:
		return fmt.Sprintf("complete: iterations=%d input_tokens=%d output_tokens=%d duration=%s", e.IterationCount, e.TotalInputTokens, e.TotalOutputTokens, e.Duration)
	case agent.ErrorEvent:
		return fmt.Sprintf("error: %s", e.Message)
	default:
		return ""
	}
}

func yardExceededMaxTokens(turnResult *agent.TurnResult, maxTokens int) bool {
	if turnResult == nil || maxTokens <= 0 {
		return false
	}
	used := turnResult.TotalUsage.InputTokens + turnResult.TotalUsage.OutputTokens
	return used >= maxTokens
}

func yardFinalText(turnResult *agent.TurnResult) string {
	if turnResult == nil {
		return ""
	}
	return strings.TrimSpace(turnResult.FinalText)
}
```

- [ ] **Step 10.2: Create the run command file**

Create `cmd/yard/run.go` with the following content:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/agent"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/id"
	"github.com/ponchione/sodoryard/internal/role"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/tool"
)

const (
	yardRunExitOK             = 0
	yardRunExitInfrastructure = 1
	yardRunExitSafetyLimit    = 2
	yardRunExitEscalation     = 3
)

type yardRunExitError struct {
	code int
	err  error
}

func (e yardRunExitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e yardRunExitError) Unwrap() error { return e.err }
func (e yardRunExitError) ExitCode() int { return e.code }

type yardRunFlags struct {
	Role        string
	Task        string
	TaskFile    string
	ChainID     string
	Brain       string
	MaxTurns    int
	MaxTokens   int
	Timeout     time.Duration
	ReceiptPath string
	Quiet       bool
	ProjectRoot string
}

type yardRunResult struct {
	ReceiptPath string
	ExitCode    int
}

func newYardRunCmd(configPath *string) *cobra.Command {
	flags := yardRunFlags{Timeout: 30 * time.Minute}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one autonomous headless agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := yardRunHeadless(cmd, *configPath, flags)
			if result != nil && (result.ExitCode == yardRunExitOK || result.ExitCode == yardRunExitSafetyLimit) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.ReceiptPath)
			}
			if err != nil {
				return err
			}
			if result != nil && result.ExitCode != yardRunExitOK {
				return yardRunExitError{code: result.ExitCode, err: fmt.Errorf("headless run exited with code %d", result.ExitCode)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.Role, "role", "", "Agent role from config")
	cmd.Flags().StringVar(&flags.Task, "task", "", "Task text for the headless run")
	cmd.Flags().StringVar(&flags.TaskFile, "task-file", "", "Read task text from file")
	cmd.Flags().StringVar(&flags.ChainID, "chain-id", "", "Chain execution identifier")
	cmd.Flags().StringVar(&flags.Brain, "brain", "", "Override brain vault path")
	cmd.Flags().IntVar(&flags.MaxTurns, "max-turns", 0, "Override max turns for this run")
	cmd.Flags().IntVar(&flags.MaxTokens, "max-tokens", 0, "Override max total tokens for this run")
	cmd.Flags().DurationVar(&flags.Timeout, "timeout", 30*time.Minute, "Wall-clock timeout for the entire session")
	cmd.Flags().StringVar(&flags.ReceiptPath, "receipt-path", "", "Override brain-relative receipt path")
	cmd.Flags().BoolVar(&flags.Quiet, "quiet", false, "Suppress progress output")
	cmd.Flags().StringVar(&flags.ProjectRoot, "project-root", "", "Override project root")
	return cmd
}

func yardRunHeadless(cmd *cobra.Command, configPath string, flags yardRunFlags) (*yardRunResult, error) {
	if strings.TrimSpace(flags.Role) == "" {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("--role is required")}
	}
	if (strings.TrimSpace(flags.Task) == "") == (strings.TrimSpace(flags.TaskFile) == "") {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("exactly one of --task or --task-file is required")}
	}
	if flags.MaxTurns < 0 {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("--max-turns must be > 0 when supplied")}
	}
	if flags.MaxTokens < 0 {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("--max-tokens must be > 0 when supplied")}
	}
	if flags.Timeout <= 0 {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("--timeout must be > 0")}
	}

	taskText, err := yardReadTask(flags.Task, flags.TaskFile)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("load config: %w", err)}
	}
	if strings.TrimSpace(flags.ProjectRoot) != "" {
		cfg.ProjectRoot = strings.TrimSpace(flags.ProjectRoot)
	}
	if strings.TrimSpace(flags.Brain) != "" {
		cfg.Brain.VaultPath = strings.TrimSpace(flags.Brain)
	}
	if err := cfg.Validate(); err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}

	roleCfg, ok := cfg.AgentRoles[flags.Role]
	if !ok {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("agent role %q not found in config", flags.Role)}
	}
	systemPrompt, err := rtpkg.LoadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	chainID := strings.TrimSpace(flags.ChainID)
	if chainID == "" {
		chainID = id.New()
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), flags.Timeout)
	defer cancel()

	rt, err := rtpkg.BuildEngineRuntime(ctx, cfg)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	defer rt.Cleanup()

	registry, scopedBrainCfg, err := role.BuildRegistry(cfg, roleCfg, role.BuilderDeps{
		BrainBackend:     rt.BrainBackend,
		BrainSearcher:    rt.BrainSearcher,
		SemanticSearcher: rt.SemanticSearcher,
		ProviderRuntime:  rt.ProviderRouter,
		Queries:          rt.Queries,
		ProjectID:        cfg.ProjectRoot,
	})
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}

	executor := tool.NewExecutor(registry, tool.ExecutorConfig{MaxOutputTokens: cfg.Agent.ToolOutputMaxTokens, ProjectRoot: cfg.ProjectRoot}, rt.Logger)
	executor.SetRecorder(tool.NewToolExecutionRecorder(rt.Queries))
	adapter := tool.NewAgentLoopAdapter(executor)
	var sink agent.EventSink
	if !flags.Quiet {
		sink = newYardRunProgressSink(cmd.ErrOrStderr())
	}
	loopMaxTurns := roleCfg.MaxTurns
	if flags.MaxTurns > 0 {
		loopMaxTurns = flags.MaxTurns
	}
	maxTokens := roleCfg.MaxTokens
	if flags.MaxTokens > 0 {
		maxTokens = flags.MaxTokens
	}
	if loopMaxTurns == 0 {
		loopMaxTurns = cfg.Agent.MaxIterationsPerTurn
	}

	titleGen := conversation.NewTitleGen(rt.ConversationManager, rt.ProviderRouter, cfg.Routing.Default.Model, rt.Logger)
	agentLoop := agent.NewAgentLoop(agent.AgentLoopDeps{
		ContextAssembler:    rt.ContextAssembler,
		ConversationManager: rt.ConversationManager,
		ProviderRouter:      rt.ProviderRouter,
		ToolExecutor:        adapter,
		ToolDefinitions:     registry.ToolDefinitions(),
		PromptBuilder:       agent.NewPromptBuilder(rt.Logger),
		TitleGenerator:      titleGen,
		EventSink:           sink,
		Config: agent.AgentLoopConfig{
			MaxIterations:              loopMaxTurns,
			LoopDetectionThreshold:     cfg.Agent.LoopDetectionThreshold,
			ExtendedThinking:           cfg.Agent.ExtendedThinking,
			BasePrompt:                 systemPrompt,
			ProviderName:               cfg.Routing.Default.Provider,
			ModelName:                  cfg.Routing.Default.Model,
			EmitContextDebug:           cfg.Context.EmitContextDebug,
			ContextConfig:              cfg.Context,
			ToolResultStoreRoot:        cfg.Agent.ToolResultStoreRoot,
			CacheSystemPrompt:          cfg.Agent.CacheSystemPrompt,
			CacheAssembledContext:      cfg.Agent.CacheAssembledContext,
			CacheConversationHistory:   cfg.Agent.CacheConversationHistory,
			CompressHistoricalResults:  cfg.Agent.CompressHistoricalResults,
			StripHistoricalLineNumbers: cfg.Agent.StripHistoricalLineNumbers,
			ElideDuplicateReads:        cfg.Agent.ElideDuplicateReads,
			HistorySummarizeAfterTurns: cfg.Agent.HistorySummarizeAfterTurns,
		},
		Logger: rt.Logger,
	})
	defer agentLoop.Close()

	convOpts := []conversation.CreateOption{}
	if cfg.Routing.Default.Provider != "" {
		convOpts = append(convOpts, conversation.WithProvider(cfg.Routing.Default.Provider))
	}
	if cfg.Routing.Default.Model != "" {
		convOpts = append(convOpts, conversation.WithModel(cfg.Routing.Default.Model))
	}
	conv, err := rt.ConversationManager.Create(ctx, cfg.ProjectRoot, convOpts...)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: fmt.Errorf("create conversation: %w", err)}
	}
	modelContextLimit, err := rtpkg.ResolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	turnResult, turnErr := agentLoop.RunTurn(ctx, agent.RunTurnRequest{
		ConversationID:    conv.ID,
		TurnNumber:        1,
		Message:           taskText,
		ModelContextLimit: modelContextLimit,
	})

	receiptVerdict := "completed_no_receipt"
	exitCode := yardRunExitOK
	if turnErr != nil {
		if errors.Is(turnErr, agent.ErrTurnCancelled) && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			receiptVerdict = "safety_limit"
			exitCode = yardRunExitSafetyLimit
		} else {
			return nil, yardRunExitError{code: yardRunExitInfrastructure, err: turnErr}
		}
	} else if (loopMaxTurns > 0 && turnResult.IterationCount >= loopMaxTurns) || yardExceededMaxTokens(turnResult, maxTokens) {
		receiptVerdict = "safety_limit"
		exitCode = yardRunExitSafetyLimit
	}

	receiptPath, r, err := yardEnsureReceipt(ctx, rt.BrainBackend, scopedBrainCfg, flags.Role, chainID, yardResolveReceiptPath(flags.Role, chainID, flags.ReceiptPath), receiptVerdict, yardFinalText(turnResult), turnResult)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	if r != nil {
		switch r.Verdict {
		case "escalate":
			exitCode = yardRunExitEscalation
		case "safety_limit":
			exitCode = yardRunExitSafetyLimit
		}
	}
	return &yardRunResult{ReceiptPath: receiptPath, ExitCode: exitCode}, nil
}

func yardReadTask(task string, taskFile string) (string, error) {
	if strings.TrimSpace(task) != "" && strings.TrimSpace(taskFile) != "" {
		return "", fmt.Errorf("--task and --task-file are mutually exclusive")
	}
	if strings.TrimSpace(task) != "" {
		return strings.TrimSpace(task), nil
	}
	if strings.TrimSpace(taskFile) == "" {
		return "", fmt.Errorf("task text is required")
	}
	data, err := os.ReadFile(strings.TrimSpace(taskFile))
	if err != nil {
		return "", fmt.Errorf("read task file: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("task file is empty")
	}
	return text, nil
}
```

---

## Task 11: Create `cmd/yard/index.go` -- yard index

**Files:**
- Create: `cmd/yard/index.go`

**Background:** `yard index` mirrors `tidmouth index` (code indexing only). The brain index subcommand is NOT nested here -- it lives under `yard brain index` (Task 15). So `yard index` only has the code index behavior.

- [ ] **Step 11.1: Create the index command file**

Create `cmd/yard/index.go` with the following content:

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	appindex "github.com/ponchione/sodoryard/internal/index"
	"github.com/spf13/cobra"
)

func newYardIndexCmd(configPath *string) *cobra.Command {
	var (
		full    bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index the codebase for semantic retrieval",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			result, err := appindex.Run(cmd.Context(), appindex.Options{
				ProjectRoot:  cfg.ProjectRoot,
				Full:         full,
				IncludeDirty: true,
				Config:       cfg,
			})
			if err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			yardPrintIndexSummary(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Force a full rebuild of the semantic index")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output")
	return cmd
}

func yardPrintIndexSummary(out io.Writer, result *appindex.Result) {
	if result == nil {
		fmt.Fprintln(out, "index completed")
		return
	}
	fmt.Fprintf(out, "Mode: %s\n", result.Mode)
	fmt.Fprintf(out, "Previous revision: %s\n", yardDisplayValue(result.PreviousRevision))
	fmt.Fprintf(out, "Current revision: %s\n", yardDisplayValue(result.CurrentRevision))
	fmt.Fprintf(out, "Changed files: %d\n", result.FilesChanged)
	fmt.Fprintf(out, "Deleted files: %d\n", result.FilesDeleted)
	fmt.Fprintf(out, "Skipped files: %d\n", result.FilesSkipped)
	fmt.Fprintf(out, "Chunks written: %d\n", result.ChunksWritten)
	fmt.Fprintf(out, "Worktree dirty: %t\n", result.WorktreeDirty)
	fmt.Fprintf(out, "Duration: %s\n", result.Duration.Round(10_000_000))
	if len(result.IndexedFiles) > 0 {
		fmt.Fprintf(out, "Indexed files: %s\n", strings.Join(result.IndexedFiles, ", "))
	}
}

func yardDisplayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<none>"
	}
	return value
}
```

---

## Task 12: Create `cmd/yard/auth.go` -- yard auth + doctor

**Files:**
- Create: `cmd/yard/auth.go`

**Background:** `yard auth` mirrors `tidmouth auth` (group with `status` subcommand). `yard doctor` mirrors `tidmouth doctor`. Both share the same `collectProviderAuthReports` logic.

- [ ] **Step 12.1: Create the auth command file**

Create `cmd/yard/auth.go` with the following content:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/localservices"
	"github.com/ponchione/sodoryard/internal/provider"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/spf13/cobra"
)

type yardAuthProviderReport struct {
	Name       string               `json:"name"`
	Type       string               `json:"type"`
	Healthy    bool                 `json:"healthy"`
	BuildError string               `json:"build_error,omitempty"`
	PingError  string               `json:"ping_error,omitempty"`
	Auth       *provider.AuthStatus `json:"auth,omitempty"`
}

func newYardAuthCmd(configPath *string) *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect provider authentication state",
	}
	authCmd.AddCommand(newYardAuthStatusCmd(configPath))
	return authCmd
}

func newYardDoctorCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run lightweight auth diagnostics for configured providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return yardRunProviderDiagnostics(cmd, *configPath, false, true)
		},
	}
	return cmd
}

func newYardAuthStatusCmd(configPath *string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show auth mode, source, and expiry for each provider without probing connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return yardRunProviderDiagnostics(cmd, *configPath, jsonOutput, false)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit auth status as JSON")
	return cmd
}

func yardRunProviderDiagnostics(cmd *cobra.Command, configPath string, jsonOutput bool, includePing bool) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	reports := yardCollectProviderAuthReports(cmd.Context(), cfg, includePing)
	var llmStatus *localservices.StackStatus
	if includePing {
		mgr := localservices.NewManager(nil)
		status, err := mgr.Status(cmd.Context(), cfg)
		if err == nil {
			llmStatus = &status
		}
	}
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		payload := map[string]any{"providers": reports}
		if llmStatus != nil {
			payload["local_services"] = llmStatus
		}
		return enc.Encode(payload)
	}
	yardPrintProviderAuthReports(cmd.OutOrStdout(), reports)
	if llmStatus != nil {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "local_services:")
		_ = yardPrintLLMStatus(cmd.OutOrStdout(), *llmStatus, false)
	}
	return nil
}

func yardCollectProviderAuthReports(ctx context.Context, cfg *appconfig.Config, includePing bool) []yardAuthProviderReport {
	providerNames := cfg.ProviderNamesForSurfaces()
	names := make([]string, 0, len(providerNames))
	for _, name := range providerNames {
		names = append(names, name)
	}
	sort.Strings(names)

	reports := make([]yardAuthProviderReport, 0, len(names))
	for _, name := range names {
		provCfg := cfg.Providers[name]
		report := yardAuthProviderReport{Name: name, Type: provCfg.Type, Healthy: true}
		p, err := rtpkg.BuildProvider(name, provCfg)
		if err != nil {
			report.Healthy = false
			report.BuildError = err.Error()
			reports = append(reports, report)
			continue
		}
		if reporter, ok := p.(provider.AuthStatusReporter); ok {
			status, err := reporter.AuthStatus(ctx)
			if err != nil {
				if report.Auth == nil {
					report.Auth = &provider.AuthStatus{Provider: name, Detail: err.Error()}
				}
				var pe *provider.ProviderError
				if rtpkg.ErrorAsProviderError(err, &pe) {
					report.Auth.Remediation = pe.Remediation
				}
			} else {
				report.Auth = status
			}
		}
		if includePing {
			if pinger, ok := p.(provider.Pinger); ok {
				timeout := 2 * time.Second
				if name == "anthropic" {
					timeout = 5 * time.Second
				}
				pingCtx, cancel := context.WithTimeout(ctx, timeout)
				pingErr := pinger.Ping(pingCtx)
				cancel()
				if pingErr != nil {
					report.Healthy = false
					report.PingError = pingErr.Error()
					if report.Auth == nil {
						report.Auth = &provider.AuthStatus{Provider: name, Detail: pingErr.Error()}
					}
					var pe *provider.ProviderError
					if rtpkg.ErrorAsProviderError(pingErr, &pe) {
						if report.Auth.Remediation == "" {
							report.Auth.Remediation = pe.Remediation
						}
					}
				}
			}
		}
		reports = append(reports, report)
	}
	return reports
}

func yardErrorAsProviderError(err error, out **provider.ProviderError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, out)
}

func yardPrintProviderAuthReports(out io.Writer, reports []yardAuthProviderReport) {
	for _, report := range reports {
		status := "healthy"
		if !report.Healthy {
			status = "unhealthy"
		}
		_, _ = fmt.Fprintf(out, "%s (%s): %s\n", report.Name, report.Type, status)
		if report.BuildError != "" {
			_, _ = fmt.Fprintf(out, "  build_error: %s\n", report.BuildError)
			continue
		}
		if report.PingError != "" {
			_, _ = fmt.Fprintf(out, "  ping_error: %s\n", report.PingError)
		}
		if report.Auth == nil {
			_, _ = fmt.Fprintln(out, "  auth: unavailable")
			continue
		}
		_, _ = fmt.Fprintf(out, "  auth_mode: %s\n", yardBlankIfEmpty(report.Auth.Mode, "unknown"))
		if report.Auth.Source != "" {
			_, _ = fmt.Fprintf(out, "  source: %s\n", report.Auth.Source)
		}
		if report.Auth.StorePath != "" {
			_, _ = fmt.Fprintf(out, "  store_path: %s\n", report.Auth.StorePath)
		}
		if report.Auth.SourcePath != "" {
			_, _ = fmt.Fprintf(out, "  source_path: %s\n", report.Auth.SourcePath)
		}
		_, _ = fmt.Fprintf(out, "  has_access_token: %t\n", report.Auth.HasAccessToken)
		_, _ = fmt.Fprintf(out, "  has_refresh_token: %t\n", report.Auth.HasRefreshToken)
		if !report.Auth.LastRefresh.IsZero() {
			_, _ = fmt.Fprintf(out, "  last_refresh: %s\n", report.Auth.LastRefresh.Format(time.RFC3339))
		}
		if !report.Auth.ExpiresAt.IsZero() {
			_, _ = fmt.Fprintf(out, "  expires_at: %s\n", report.Auth.ExpiresAt.Format(time.RFC3339))
		} else {
			_, _ = fmt.Fprintln(out, "  expires_at: unknown")
		}
		if report.Auth.Detail != "" {
			_, _ = fmt.Fprintf(out, "  detail: %s\n", report.Auth.Detail)
		}
		if report.Auth.Remediation != "" {
			_, _ = fmt.Fprintf(out, "  remediation: %s\n", report.Auth.Remediation)
		}
	}
}

func yardBlankIfEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
```

---

## Task 13: Create `cmd/yard/config_cmd.go` and `cmd/yard/llm.go`

**Files:**
- Create: `cmd/yard/config_cmd.go`
- Create: `cmd/yard/llm.go`

**Background:** `yard config` mirrors `tidmouth config`. `yard llm` mirrors `tidmouth llm` with its four subcommands.

- [ ] **Step 13.1: Create the config command file**

Create `cmd/yard/config_cmd.go` with the following content:

```go
package main

import (
	"fmt"
	"io"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/spf13/cobra"
)

func newYardConfigCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return yardRunConfig(cmd.OutOrStdout(), *configPath)
		},
	}
	return cmd
}

func yardRunConfig(out io.Writer, configPath string) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	brainVaultPath := "<disabled>"
	if cfg.Brain.Enabled {
		brainVaultPath = cfg.BrainVaultPath()
	}

	_, _ = fmt.Fprintln(out, "config: valid")
	_, _ = fmt.Fprintf(out, "config_path: %s\n", configPath)
	_, _ = fmt.Fprintf(out, "project_root: %s\n", cfg.ProjectRoot)
	_, _ = fmt.Fprintf(out, "server_address: %s\n", cfg.ServerAddress())
	_, _ = fmt.Fprintf(out, "default_provider: %s\n", cfg.Routing.Default.Provider)
	_, _ = fmt.Fprintf(out, "default_model: %s\n", cfg.Routing.Default.Model)
	_, _ = fmt.Fprintf(out, "fallback_provider: %s\n", yardDisplayValue(cfg.Routing.Fallback.Provider))
	_, _ = fmt.Fprintf(out, "fallback_model: %s\n", yardDisplayValue(cfg.Routing.Fallback.Model))
	_, _ = fmt.Fprintf(out, "database_path: %s\n", cfg.DatabasePath())
	_, _ = fmt.Fprintf(out, "code_index_path: %s\n", cfg.CodeLanceDBPath())
	_, _ = fmt.Fprintf(out, "brain_vault_path: %s\n", brainVaultPath)
	_, _ = fmt.Fprintf(out, "embedding_base_url: %s\n", cfg.Embedding.BaseURL)
	_, _ = fmt.Fprintf(out, "brain_enabled: %t\n", cfg.Brain.Enabled)
	_, _ = fmt.Fprintf(out, "local_services_enabled: %t\n", cfg.LocalServices.Enabled)
	_, _ = fmt.Fprintf(out, "local_services_mode: %s\n", cfg.LocalServices.Mode)
	_, _ = fmt.Fprintf(out, "local_services_compose_file: %s\n", cfg.LocalServices.ComposeFile)
	_, _ = fmt.Fprintf(out, "local_services_project_dir: %s\n", cfg.LocalServices.ProjectDir)
	return nil
}
```

- [ ] **Step 13.2: Create the LLM command file**

Create `cmd/yard/llm.go` with the following content:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/localservices"
	"github.com/spf13/cobra"
)

func newYardLLMCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{Use: "llm", Short: "Manage repo-owned local LLM services"}
	cmd.AddCommand(newYardLLMStatusCmd(configPath), newYardLLMUpCmd(configPath), newYardLLMDownCmd(configPath), newYardLLMLogsCmd(configPath))
	return cmd
}

func newYardLLMStatusCmd(configPath *string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local LLM stack status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			mgr := localservices.NewManager(nil)
			status, err := mgr.Status(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			return yardPrintLLMStatus(cmd.OutOrStdout(), status, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit stack status as JSON")
	return cmd
}

func newYardLLMUpCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Ensure required local LLM services are up",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			mgr := localservices.NewManager(nil)
			status, err := mgr.EnsureUp(cmd.Context(), cfg)
			if err != nil {
				_ = yardPrintLLMStatus(cmd.ErrOrStderr(), status, false)
				return err
			}
			return yardPrintLLMStatus(cmd.OutOrStdout(), status, false)
		},
	}
}

func newYardLLMDownCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop managed local LLM services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if !cfg.LocalServices.Enabled {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "local services are disabled in config")
				return nil
			}
			mgr := localservices.NewManager(nil)
			if err := mgr.Down(cmd.Context(), cfg); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "local LLM stack stopped")
			return nil
		},
	}
}

func newYardLLMLogsCmd(configPath *string) *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show recent logs from managed local LLM services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if !cfg.LocalServices.Enabled {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "local services are disabled in config")
				return nil
			}
			mgr := localservices.NewManager(nil)
			logs, err := mgr.Logs(cmd.Context(), cfg, tail)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), logs)
			return nil
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of recent log lines")
	return cmd
}

func yardNewLLMManager() interface {
	Status(context.Context, *appconfig.Config) (localservices.StackStatus, error)
	EnsureUp(context.Context, *appconfig.Config) (localservices.StackStatus, error)
	Down(context.Context, *appconfig.Config) error
	Logs(context.Context, *appconfig.Config, int) (string, error)
} {
	return localservices.NewManager(nil)
}

func yardPrintLLMStatus(out io.Writer, status localservices.StackStatus, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}
	_, _ = fmt.Fprintf(out, "mode: %s\n", yardBlankIfEmpty(status.Mode, "manual"))
	_, _ = fmt.Fprintf(out, "compose_file: %s\n", yardBlankIfEmpty(status.ComposeFile, "<unset>"))
	_, _ = fmt.Fprintf(out, "project_dir: %s\n", yardBlankIfEmpty(status.ProjectDir, "<unset>"))
	_, _ = fmt.Fprintf(out, "docker_available: %t\n", status.DockerAvailable)
	_, _ = fmt.Fprintf(out, "docker_daemon_available: %t\n", status.DaemonAvailable)
	_, _ = fmt.Fprintf(out, "compose_available: %t\n", status.ComposeAvailable)
	_, _ = fmt.Fprintf(out, "compose_file_exists: %t\n", status.ComposeFileExists)
	if len(status.NetworkStatus) > 0 {
		names := make([]string, 0, len(status.NetworkStatus))
		for name := range status.NetworkStatus {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			_, _ = fmt.Fprintf(out, "network.%s: %t\n", name, status.NetworkStatus[name])
		}
	}
	for _, svc := range status.Services {
		_, _ = fmt.Fprintf(out, "service.%s.healthy: %t\n", svc.Name, svc.Healthy)
		_, _ = fmt.Fprintf(out, "service.%s.reachable: %t\n", svc.Name, svc.Reachable)
		_, _ = fmt.Fprintf(out, "service.%s.models_ready: %t\n", svc.Name, svc.ModelsReady)
		if strings.TrimSpace(svc.Detail) != "" {
			_, _ = fmt.Fprintf(out, "service.%s.detail: %s\n", svc.Name, svc.Detail)
		}
	}
	for _, problem := range status.Problems {
		_, _ = fmt.Fprintf(out, "problem: %s\n", problem)
	}
	for _, remediation := range status.Remediation {
		_, _ = fmt.Fprintf(out, "remediation: %s\n", remediation)
	}
	return nil
}
```

---

## Task 14: Create `cmd/yard/brain.go` -- yard brain group

**Files:**
- Create: `cmd/yard/brain.go`

**Background:** `yard brain` is a new command group with two subcommands: `yard brain index` (mirrors `tidmouth index brain`) and `yard brain serve` (mirrors `tidmouth brain-serve`). The brain index subcommand reuses the same `brainindexer` pipeline.

- [ ] **Step 14.1: Create the brain command file**

Create `cmd/yard/brain.go` with the following content:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	brainindexer "github.com/ponchione/sodoryard/internal/brain/indexer"
	brainindexstate "github.com/ponchione/sodoryard/internal/brain/indexstate"
	"github.com/ponchione/sodoryard/internal/brain/mcpserver"
	"github.com/ponchione/sodoryard/internal/brain/vault"
	"github.com/ponchione/sodoryard/internal/codeintel/embedder"
	"github.com/ponchione/sodoryard/internal/codestore"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	appdb "github.com/ponchione/sodoryard/internal/db"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/spf13/cobra"
)

func newYardBrainCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brain",
		Short: "Brain operations (index, serve)",
	}
	cmd.AddCommand(newYardBrainIndexCmd(configPath), newYardBrainServeCmd())
	return cmd
}

func newYardBrainIndexCmd(configPath *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Rebuild derived brain metadata from the vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			result, err := yardRunBrainIndex(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			yardPrintBrainIndexSummary(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output")
	return cmd
}

func yardRunBrainIndex(ctx context.Context, cfg *appconfig.Config) (brainindexer.Result, error) {
	if cfg == nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: config is required")
	}
	if !cfg.Brain.Enabled {
		return brainindexer.Result{}, fmt.Errorf("brain index: brain.enabled must be true")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backend, cleanup, err := rtpkg.BuildBrainBackend(ctx, cfg.Brain, logger)
	if err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: build brain backend: %w", err)
	}
	defer cleanup()
	if backend == nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: brain backend unavailable")
	}

	database, err := appdb.OpenDB(ctx, cfg.DatabasePath())
	if err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: open database: %w", err)
	}
	defer database.Close()
	if _, err := appdb.InitIfNeeded(ctx, database); err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: init database schema: %w", err)
	}
	if err := rtpkg.EnsureProjectRecord(ctx, database, cfg); err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: ensure project record: %w", err)
	}

	queries := appdb.New(database)
	existingDocs, err := queries.ListBrainDocumentsByProject(ctx, cfg.ProjectRoot)
	if err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: list existing brain documents: %w", err)
	}
	previousPaths := make([]string, 0, len(existingDocs))
	for _, doc := range existingDocs {
		previousPaths = append(previousPaths, doc.Path)
	}

	result, err := brainindexer.New(database, backend).RebuildProject(ctx, cfg.ProjectRoot)
	if err != nil {
		return brainindexer.Result{}, err
	}

	store, err := codestore.Open(ctx, cfg.BrainLanceDBPath())
	if err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: open brain vectorstore: %w", err)
	}
	defer store.Close()
	semanticResult, err := brainindexer.NewSemantic(backend, store, embedder.New(cfg.Embedding)).RebuildProject(ctx, cfg.ProjectName(), previousPaths)
	if err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: semantic rebuild: %w", err)
	}
	if err := brainindexstate.MarkFresh(cfg.ProjectRoot, time.Now().UTC()); err != nil {
		return brainindexer.Result{}, fmt.Errorf("brain index: persist freshness state: %w", err)
	}
	result.SemanticChunksIndexed = semanticResult.SemanticChunksIndexed
	result.SemanticDocumentsDeleted = semanticResult.SemanticDocumentsDeleted
	return result, nil
}

func yardPrintBrainIndexSummary(out io.Writer, result brainindexer.Result) {
	fmt.Fprintln(out, "Brain reindex completed")
	fmt.Fprintf(out, "Brain documents indexed: %d\n", result.DocumentsIndexed)
	fmt.Fprintf(out, "Brain links indexed: %d\n", result.LinksIndexed)
	fmt.Fprintf(out, "Brain documents deleted: %d\n", result.DocumentsDeleted)
	fmt.Fprintf(out, "Brain semantic chunks indexed: %d\n", result.SemanticChunksIndexed)
	fmt.Fprintf(out, "Brain semantic documents deleted: %d\n", result.SemanticDocumentsDeleted)
}

func newYardBrainServeCmd() *cobra.Command {
	var vaultPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the project brain as a standalone MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vaultPath == "" {
				return fmt.Errorf("--vault is required")
			}
			vc, err := vault.New(vaultPath)
			if err != nil {
				return err
			}
			server := mcpserver.NewServer(vc)
			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault", "", "Path to the Obsidian vault directory")
	return cmd
}
```

---

## Task 15: Create `cmd/yard/chain.go` -- yard chain group

**Files:**
- Create: `cmd/yard/chain.go`

**Background:** `yard chain` is a command group with seven subcommands: `start`, `status`, `logs`, `receipt`, `cancel`, `pause`, `resume`. The `start` subcommand mirrors `sirtopham chain`. The rest mirror the corresponding `sirtopham` top-level commands. Key detail: the cobra `Use` field for the start subcommand is `"start"`, not `"chain"`.

- [ ] **Step 15.1: Create the chain command file**

Create `cmd/yard/chain.go` with the following content:

```go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/agent"
	"github.com/ponchione/sodoryard/internal/chain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/id"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
)

type yardChainFlags struct {
	Specs       string
	Task        string
	ChainID     string
	MaxSteps    int
	MaxDuration time.Duration
	TokenBudget int
	DryRun      bool
}

func newYardChainCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chain",
		Short: "Chain orchestration commands",
	}
	cmd.AddCommand(
		newYardChainStartCmd(configPath),
		newYardChainStatusCmd(configPath),
		newYardChainLogsCmd(configPath),
		newYardChainReceiptCmd(configPath),
		newYardChainCancelCmd(configPath),
		newYardChainPauseCmd(configPath),
		newYardChainResumeCmd(configPath),
	)
	return cmd
}

func newYardChainStartCmd(configPath *string) *cobra.Command {
	flags := yardChainFlags{MaxSteps: 100, MaxDuration: 4 * time.Hour, TokenBudget: 5_000_000}
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new chain execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flags.Task) == "" && strings.TrimSpace(flags.Specs) == "" {
				return fmt.Errorf("one of --task or --specs is required")
			}
			return yardRunChain(cmd.Context(), *configPath, flags, cmd)
		},
	}
	cmd.Flags().StringVar(&flags.Specs, "specs", "", "Comma-separated brain-relative paths to spec docs")
	cmd.Flags().StringVar(&flags.Task, "task", "", "Free-form task description")
	cmd.Flags().StringVar(&flags.ChainID, "chain-id", "", "Chain execution identifier")
	cmd.Flags().IntVar(&flags.MaxSteps, "max-steps", 100, "Maximum total agent invocations")
	cmd.Flags().DurationVar(&flags.MaxDuration, "max-duration", 4*time.Hour, "Wall-clock timeout for entire chain")
	cmd.Flags().IntVar(&flags.TokenBudget, "token-budget", 5_000_000, "Total token ceiling across all agents")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "Create the chain row but do not run the orchestrator")
	return cmd
}

func yardRunChain(ctx context.Context, configPath string, flags yardChainFlags, cmd *cobra.Command) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	roleCfg, ok := cfg.AgentRoles["orchestrator"]
	if !ok {
		return fmt.Errorf("agent role %q not found in config", "orchestrator")
	}
	systemPrompt, err := rtpkg.LoadRoleSystemPrompt(cfg.ProjectRoot, roleCfg.SystemPrompt)
	if err != nil {
		return err
	}
	rt, err := rtpkg.BuildOrchestratorRuntime(ctx, cfg)
	if err != nil {
		return err
	}
	defer rt.Cleanup()
	chainID := strings.TrimSpace(flags.ChainID)
	if chainID == "" {
		chainID = id.New()
	}
	if _, err := rt.ChainStore.StartChain(ctx, yardChainSpecFromFlags(chainID, flags)); err != nil {
		return err
	}
	_ = rt.ChainStore.LogEvent(ctx, chainID, "", "chain_started", map[string]any{"specs": yardParseSpecs(flags.Specs), "task": flags.Task})
	if flags.DryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", chainID)
		return nil
	}
	registry, err := rtpkg.BuildOrchestratorRegistry(rt, roleCfg, chainID)
	if err != nil {
		return err
	}
	conv, err := rt.ConversationManager.Create(ctx, cfg.ProjectRoot, conversation.WithProvider(cfg.Routing.Default.Provider), conversation.WithModel(cfg.Routing.Default.Model))
	if err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}
	limit, err := rtpkg.ResolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
	if err != nil {
		return err
	}
	loop := agent.NewAgentLoop(agent.AgentLoopDeps{ContextAssembler: rt.ContextAssembler, ConversationManager: rt.ConversationManager, ProviderRouter: rt.ProviderRouter, ToolExecutor: &rtpkg.RegistryToolExecutor{Registry: registry, ProjectRoot: cfg.ProjectRoot}, ToolDefinitions: registry.ToolDefinitions(), PromptBuilder: agent.NewPromptBuilder(rt.Logger), TitleGenerator: conversation.NewTitleGen(rt.ConversationManager, rt.ProviderRouter, cfg.Routing.Default.Model, rt.Logger), Config: agent.AgentLoopConfig{MaxIterations: roleCfg.MaxTurns, BasePrompt: systemPrompt, ProviderName: cfg.Routing.Default.Provider, ModelName: cfg.Routing.Default.Model, ContextConfig: cfg.Context}, Logger: rt.Logger})
	defer loop.Close()
	turnTask := yardBuildChainTask(flags, chainID)
	if _, err := loop.RunTurn(ctx, agent.RunTurnRequest{ConversationID: conv.ID, TurnNumber: 1, Message: turnTask, ModelContextLimit: limit}); err != nil {
		return err
	}
	stored, err := rt.ChainStore.GetChain(ctx, chainID)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", chainID)
	if stored.Status == "failed" {
		return fmt.Errorf("chain %s failed", chainID)
	}
	return nil
}

func yardBuildChainTask(flags yardChainFlags, chainID string) string {
	if strings.TrimSpace(flags.Specs) != "" {
		return fmt.Sprintf("You are managing a new chain execution. Source specs: %s. Chain ID: %s. Read the specs from the brain and begin orchestrating.", strings.Join(yardParseSpecs(flags.Specs), ", "), chainID)
	}
	return fmt.Sprintf("You are managing a new chain execution. Task: %s. Chain ID: %s. Begin orchestrating.", strings.TrimSpace(flags.Task), chainID)
}

func yardChainSpecFromFlags(chainID string, flags yardChainFlags) chain.ChainSpec {
	return chain.ChainSpec{ChainID: chainID, SourceSpecs: yardParseSpecs(flags.Specs), SourceTask: strings.TrimSpace(flags.Task), MaxSteps: flags.MaxSteps, MaxResolverLoops: 3, MaxDuration: flags.MaxDuration, TokenBudget: flags.TokenBudget}
}

func yardParseSpecs(specs string) []string {
	if strings.TrimSpace(specs) == "" {
		return nil
	}
	parts := strings.Split(specs, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func newYardChainStatusCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "status [chain-id]", Short: "Show chain status", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load(*configPath)
		if err != nil {
			return err
		}
		rt, err := rtpkg.BuildOrchestratorRuntime(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer rt.Cleanup()
		if len(args) == 0 {
			chains, err := rt.ChainStore.ListChains(cmd.Context(), 20)
			if err != nil {
				return err
			}
			for _, ch := range chains {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tsteps=%d\ttokens=%d\n", ch.ID, ch.Status, ch.TotalSteps, ch.TotalTokens)
			}
			return nil
		}
		ch, err := rt.ChainStore.GetChain(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		steps, err := rt.ChainStore.ListSteps(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "chain=%s status=%s steps=%d tokens=%d duration=%d summary=%s\n", ch.ID, ch.Status, ch.TotalSteps, ch.TotalTokens, ch.TotalDurationSecs, ch.Summary)
		for _, step := range steps {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "step=%d role=%s status=%s verdict=%s receipt=%s\n", step.SequenceNum, step.Role, step.Status, step.Verdict, step.ReceiptPath)
		}
		return nil
	}}
}

func newYardChainLogsCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "logs <chain-id>", Short: "Show chain event log", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load(*configPath)
		if err != nil {
			return err
		}
		rt, err := rtpkg.BuildOrchestratorRuntime(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer rt.Cleanup()
		events, err := rt.ChainStore.ListEvents(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		for _, event := range events {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\n", event.ID, event.CreatedAt.Format(time.RFC3339), event.EventType, event.EventData)
		}
		return nil
	}}
}

func newYardChainReceiptCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "receipt <chain-id> [step]", Short: "Show orchestrator or step receipt", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load(*configPath)
		if err != nil {
			return err
		}
		rt, err := rtpkg.BuildOrchestratorRuntime(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer rt.Cleanup()
		path := fmt.Sprintf("receipts/orchestrator/%s.md", args[0])
		if len(args) == 2 {
			steps, err := rt.ChainStore.ListSteps(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			for _, step := range steps {
				if fmt.Sprintf("%d", step.SequenceNum) == args[1] {
					path = step.ReceiptPath
					break
				}
			}
		}
		content, err := rt.BrainBackend.ReadDocument(cmd.Context(), path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), content)
		return nil
	}}
}

func newYardChainCancelCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "cancel <chain-id>", Short: "Cancel a chain", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return yardSetChainStatus(cmd, *configPath, args[0], "cancelled", chain.EventChainCancelled, "cancelled")
	}}
}

func newYardChainPauseCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "pause <chain-id>", Short: "Pause a chain", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return yardSetChainStatus(cmd, *configPath, args[0], "paused", chain.EventChainPaused, "paused")
	}}
}

func newYardChainResumeCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "resume <chain-id>", Short: "Resume a paused chain", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return yardSetChainStatus(cmd, *configPath, args[0], "running", chain.EventChainResumed, "set back to running (rerun yard chain start to continue)")
	}}
}

func yardSetChainStatus(cmd *cobra.Command, configPath string, chainID string, status string, eventType chain.EventType, message string) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return err
	}
	rt, err := rtpkg.BuildOrchestratorRuntime(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer rt.Cleanup()
	if err := rt.ChainStore.SetChainStatus(cmd.Context(), chainID, status); err != nil {
		return err
	}
	_ = rt.ChainStore.LogEvent(cmd.Context(), chainID, "", eventType, nil)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "chain %s %s\n", chainID, message)
	return nil
}
```

- [ ] **Step 15.2: Verify `cmd/yard` compiles**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" go build -tags 'sqlite_fts5' ./cmd/yard/
```

- [ ] **Step 15.3: Commit all yard command files**

```bash
cd /home/gernsback/source/sodoryard
git add cmd/yard/main.go cmd/yard/serve.go cmd/yard/run.go cmd/yard/run_helpers.go cmd/yard/index.go cmd/yard/auth.go cmd/yard/config_cmd.go cmd/yard/llm.go cmd/yard/brain.go cmd/yard/chain.go
git commit -m "feat(yard): wire all 19 operator commands under the yard binary

Add serve, run, index, auth, doctor, config, llm (status/up/down/logs),
brain (index/serve), and chain (start/status/logs/receipt/cancel/pause/resume)
subcommands to cmd/yard. Each delegates to internal/runtime for runtime
construction. Completes the unified CLI surface from spec 18."
```

---

## Task 16: Full verification and tag

**Files:**
- Modify: `NEXT_SESSION_HANDOFF.md` (if applicable)

**Background:** Run the complete verification suite to ensure everything works. Build all binaries, run all tests, and verify the `yard --help` output.

- [ ] **Step 16.1: Build all binaries**

```bash
cd /home/gernsback/source/sodoryard && make all
```

If the frontend build fails (e.g. npm not available), at minimum verify the Go compilation:

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' ./cmd/tidmouth/ && echo "tidmouth OK"
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' ./cmd/sirtopham/ && echo "sirtopham OK"
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' ./cmd/yard/ && echo "yard OK"
cd /home/gernsback/source/sodoryard && go build ./cmd/knapford/ && echo "knapford OK"
```

- [ ] **Step 16.2: Run full test suite**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" go test -tags 'sqlite_fts5' ./...
```

- [ ] **Step 16.3: Verify `yard --help` shows all command groups**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' -o /tmp/yard-test ./cmd/yard/ && /tmp/yard-test --help
```

Expected output should show all commands: `auth`, `brain`, `chain`, `config`, `doctor`, `index`, `init`, `install`, `llm`, `run`, `serve`.

- [ ] **Step 16.4: Verify `yard chain --help` shows subcommands**

```bash
/tmp/yard-test chain --help
```

Expected output should show: `cancel`, `logs`, `pause`, `receipt`, `resume`, `start`, `status`.

- [ ] **Step 16.5: Verify `yard brain --help` shows subcommands**

```bash
/tmp/yard-test brain --help
```

Expected output should show: `index`, `serve`.

- [ ] **Step 16.6: Verify `yard llm --help` shows subcommands**

```bash
/tmp/yard-test llm --help
```

Expected output should show: `down`, `logs`, `status`, `up`.

- [ ] **Step 16.7: Verify legacy binaries still compile and have their original commands**

```bash
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' -o /tmp/tidmouth-test ./cmd/tidmouth/ && /tmp/tidmouth-test --help
cd /home/gernsback/source/sodoryard && CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,$(pwd)/lib/linux_amd64" go build -tags 'sqlite_fts5' -o /tmp/sirtopham-test ./cmd/sirtopham/ && /tmp/sirtopham-test --help
```

- [ ] **Step 16.8: Tag the release**

```bash
cd /home/gernsback/source/sodoryard
git tag v0.8-unified-cli
```

- [ ] **Step 16.9: Update `NEXT_SESSION_HANDOFF.md`**

Update the handoff doc to reflect Phase 8 completion. Note that the unified `yard` CLI is now the operator-facing surface, and reference the spec and plan for context.

- [ ] **Step 16.10: Clean up temp binaries**

```bash
rm -f /tmp/yard-test /tmp/tidmouth-test /tmp/sirtopham-test
```

---

## Troubleshooting

### Compilation errors from removed functions

If `cmd/tidmouth` fails to compile after Task 5, the most likely cause is another file in `cmd/tidmouth/` still referencing a function that was removed from `runtime.go`. Check these specific references:

- `buildBrainBackend` -- referenced in `index.go` line 27 (fixed in Task 6, Step 6.5)
- `ensureProjectRecord` -- referenced in `index.go` line 124 (fixed in Task 6, Step 6.5)
- `buildProvider` -- referenced in `serve.go` and `auth.go` (fixed in Task 6, Steps 6.1 and 6.3)
- `logProviderAuthStatus` -- only referenced from old `runtime.go`, removed in Task 5
- `errorAsProviderError` -- referenced in `auth.go` and `provider_auth_logging.go` (fixed in Task 6)
- `chainCleanup` -- only referenced from old `runtime.go`, removed in Task 5

### Test variables that reference removed functions

The `cmd/tidmouth/run.go` has test-injection variables:
- `var buildRunRuntime = buildAppRuntime` -- this still works because `buildAppRuntime` exists as the thin wrapper
- `var buildRunRoleRegistry = role.BuildRegistry` -- unchanged
- `var buildBrainIndexBackend = buildBrainBackend` -- needs to change to `rtpkg.BuildBrainBackend` (Task 6, Step 6.5)

The `cmd/sirtopham/chain.go` has:
- `var buildChainRuntime = buildOrchestratorRuntime` -- this still works because the thin wrapper exists
- `var newChainAgentLoop = ...` -- unchanged

### Import cycle prevention

`internal/runtime/` imports `internal/` packages but is never imported by them. The dependency graph is:

```
cmd/yard      --> internal/runtime --> internal/{brain, config, db, ...}
cmd/tidmouth  --> internal/runtime --> internal/{brain, config, db, ...}
cmd/sirtopham --> internal/runtime --> internal/{brain, config, db, ...}
```

No cycle is possible because `internal/runtime/` does not import any `cmd/` package.

### The `fmt` import in `internal/runtime/provider.go`

The `BuildProvider` function uses `fmt.Errorf` in the default switch case. Make sure `"fmt"` is in the import block. The code in Step 2.1 includes it but Step 2.2 calls it out explicitly as a verification step.

### `os.Interrupt` vs `syscall.SIGINT` in serve.go

The original `cmd/tidmouth/serve.go` uses `os.Interrupt` (which maps to `SIGINT`). The cleaned-up version in Task 6 must preserve this. Do not replace `os.Interrupt` with `syscall.SIGINT` -- keep the original `os.Interrupt, syscall.SIGTERM` pair.

### Unused imports after function removal

When removing functions from `cmd/tidmouth/serve.go`, several imports become unused:
- `"context"` -- no longer directly referenced (was used by `aliasedProvider` methods)
- `"github.com/ponchione/sodoryard/internal/provider"` -- no longer referenced
- `"github.com/ponchione/sodoryard/internal/provider/anthropic"` -- no longer referenced
- `"github.com/ponchione/sodoryard/internal/provider/codex"` -- no longer referenced
- `"github.com/ponchione/sodoryard/internal/provider/openai"` -- no longer referenced

Go will refuse to compile with unused imports. The replacement file in Step 6.1 already has the corrected import block.
