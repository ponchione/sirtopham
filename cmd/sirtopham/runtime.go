package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

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
	anthropicprovider "github.com/ponchione/sodoryard/internal/provider/anthropic"
	"github.com/ponchione/sodoryard/internal/provider/codex"
	openai "github.com/ponchione/sodoryard/internal/provider/openai"
	"github.com/ponchione/sodoryard/internal/provider/router"
	"github.com/ponchione/sodoryard/internal/provider/tracking"
	"github.com/ponchione/sodoryard/internal/role"
	spawnpkg "github.com/ponchione/sodoryard/internal/spawn"
	"github.com/ponchione/sodoryard/internal/tool"
)

type orchestratorRuntime struct {
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

type noopContextAssembler struct{}

func (noopContextAssembler) Assemble(ctx context.Context, message string, history []appdb.Message, scope contextpkg.AssemblyScope, modelContextLimit int, historyTokenCount int) (*contextpkg.FullContextPackage, bool, error) {
	return &contextpkg.FullContextPackage{Content: "", Frozen: true, Report: &contextpkg.ContextAssemblyReport{TurnNumber: scope.TurnNumber}}, false, nil
}
func (noopContextAssembler) UpdateQuality(context.Context, string, int, bool, []string) error {
	return nil
}

type registryToolExecutor struct {
	registry    *tool.Registry
	projectRoot string
}

func (e *registryToolExecutor) Execute(ctx context.Context, call provider.ToolCall) (*provider.ToolResult, error) {
	t, ok := e.registry.Get(call.Name)
	if !ok {
		return &provider.ToolResult{ToolUseID: call.ID, Content: fmt.Sprintf("Unknown tool: %s", call.Name), IsError: true}, nil
	}
	result, err := t.Execute(ctx, e.projectRoot, call.Input)
	if err != nil {
		return nil, err
	}
	result.CallID = call.ID
	return &provider.ToolResult{ToolUseID: call.ID, Content: result.Content, IsError: !result.Success}, nil
}

func buildOrchestratorRuntime(ctx context.Context, cfg *appconfig.Config) (*orchestratorRuntime, error) {
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
	if err := ensureProjectRecord(ctx, database, cfg); err != nil {
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
		p, err := buildProvider(name, provCfg)
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
	brainBackend, err := buildBrainBackend(ctx, cfg.Brain)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("build brain backend: %w", err)
	}
	convManager := conversation.NewManager(database, nil, logger)
	return &orchestratorRuntime{Config: cfg, Logger: logger, Database: database, Queries: queries, ProviderRouter: provRouter, BrainBackend: brainBackend, ConversationManager: convManager, ContextAssembler: noopContextAssembler{}, ChainStore: chain.NewStore(database), Cleanup: func() {
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

func buildBrainBackend(ctx context.Context, cfg appconfig.BrainConfig) (brain.Backend, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return mcpclient.Connect(ctx, cfg.VaultPath)
}

func buildProvider(name string, cfg appconfig.ProviderConfig) (provider.Provider, error) {
	apiKey := resolveProviderAPIKey(cfg)
	switch cfg.Type {
	case "anthropic":
		var opts []anthropicprovider.CredentialOption
		if apiKey != "" {
			opts = append(opts, anthropicprovider.WithAPIKey(apiKey))
		}
		creds, err := anthropicprovider.NewCredentialManager(opts...)
		if err != nil {
			return nil, err
		}
		return anthropicprovider.NewAnthropicProvider(creds), nil
	case "openai-compatible":
		return openai.NewOpenAIProvider(openai.OpenAIConfig{Name: name, BaseURL: cfg.BaseURL, APIKey: apiKey, Model: cfg.Model, ContextLength: cfg.ContextLength})
	case "codex":
		var opts []codex.ProviderOption
		if cfg.BaseURL != "" {
			opts = append(opts, codex.WithBaseURL(cfg.BaseURL))
		}
		return codex.NewCodexProvider(opts...)
	default:
		return nil, fmt.Errorf("unsupported provider type: %q", cfg.Type)
	}
}

func resolveProviderAPIKey(cfg appconfig.ProviderConfig) string {
	if cfg.APIKey != "" {
		return cfg.APIKey
	}
	if cfg.APIKeyEnv != "" {
		return os.Getenv(cfg.APIKeyEnv)
	}
	return ""
}

func ensureProjectRecord(ctx context.Context, database *sql.DB, cfg *appconfig.Config) error {
	now := time.Now().UTC().Format(time.RFC3339)
	name := cfg.ProjectRoot
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	_, err := database.ExecContext(ctx, `INSERT INTO projects(id, name, root_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, root_path=excluded.root_path, updated_at=excluded.updated_at`, cfg.ProjectRoot, name, cfg.ProjectRoot, now, now)
	return err
}

func buildOrchestratorRegistry(rt *orchestratorRuntime, roleCfg appconfig.AgentRoleConfig, chainID string) (*tool.Registry, error) {
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
