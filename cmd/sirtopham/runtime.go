package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ponchione/sirtopham/internal/brain"
	"github.com/ponchione/sirtopham/internal/brain/mcpclient"
	"github.com/ponchione/sirtopham/internal/codeintel/embedder"
	codegraph "github.com/ponchione/sirtopham/internal/codeintel/graph"
	codesearcher "github.com/ponchione/sirtopham/internal/codeintel/searcher"
	"github.com/ponchione/sirtopham/internal/codestore"
	appconfig "github.com/ponchione/sirtopham/internal/config"
	contextpkg "github.com/ponchione/sirtopham/internal/context"
	"github.com/ponchione/sirtopham/internal/conversation"
	appdb "github.com/ponchione/sirtopham/internal/db"
	"github.com/ponchione/sirtopham/internal/logging"
	"github.com/ponchione/sirtopham/internal/provider/router"
	"github.com/ponchione/sirtopham/internal/provider/tracking"
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
	closeOnError := func(err error) (*appRuntime, error) {
		cleanup()
		return nil, err
	}

	if err := appdb.EnsureMessageSearchIndexesIncludeTools(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("upgrade message search indexes: %w", err))
	}
	if err := appdb.EnsureContextReportsIncludeTokenBudget(ctx, database); err != nil {
		return closeOnError(fmt.Errorf("upgrade context report token budget storage: %w", err))
	}
	queries := appdb.New(database)
	if err := ensureProjectRecord(ctx, database, cfg); err != nil {
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
		p, err := buildProvider(name, provCfg)
		if err != nil {
			return closeOnError(fmt.Errorf("build provider %q: %w", name, err))
		}
		if err := provRouter.RegisterProvider(p); err != nil {
			return closeOnError(fmt.Errorf("register provider %q: %w", name, err))
		}
		logProviderAuthStatus(ctx, logger, name, provCfg, p)
	}
	if err := provRouter.Validate(ctx); err != nil {
		return closeOnError(fmt.Errorf("validate providers: %w", err))
	}

	codeStore, err := codestore.Open(ctx, cfg.CodeLanceDBPath())
	if err != nil {
		return closeOnError(fmt.Errorf("open code vectorstore: %w", err))
	}
	prevCleanup := cleanup
	cleanup = func() {
		_ = codeStore.Close()
		prevCleanup()
	}
	semanticEmbedder := embedder.New(cfg.Embedding)
	semanticSearcher := codesearcher.New(codeStore, semanticEmbedder)

	brainStore, err := codestore.Open(ctx, cfg.BrainLanceDBPath())
	if err != nil {
		return closeOnError(fmt.Errorf("open brain vectorstore: %w", err))
	}
	prevCleanup = cleanup
	cleanup = func() {
		_ = brainStore.Close()
		prevCleanup()
	}

	brainBackend, closeBrainBackend, err := buildBrainBackend(ctx, cfg.Brain, logger)
	if err != nil {
		return closeOnError(fmt.Errorf("build brain backend: %w", err))
	}
	prevCleanup = cleanup
	cleanup = func() {
		closeBrainBackend()
		prevCleanup()
	}

	graphStore, closeGraphStore, err := buildGraphStore(cfg)
	if err != nil {
		return closeOnError(fmt.Errorf("build graph store: %w", err))
	}
	prevCleanup = cleanup
	cleanup = func() {
		closeGraphStore()
		prevCleanup()
	}

	conventionSource := buildConventionSource(cfg)
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

	return &appRuntime{
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

func buildBrainBackend(ctx context.Context, cfg appconfig.BrainConfig, logger *slog.Logger) (brain.Backend, func(), error) {
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

func buildGraphStore(cfg *appconfig.Config) (*codegraph.Store, func(), error) {
	if err := os.MkdirAll(filepath.Dir(cfg.GraphDBPath()), 0o755); err != nil {
		return nil, func() {}, err
	}
	store, err := codegraph.NewStore(cfg.GraphDBPath())
	if err != nil {
		return nil, func() {}, err
	}
	return store, func() { _ = store.Close() }, nil
}

func buildConventionSource(cfg *appconfig.Config) contextpkg.ConventionSource {
	return contextpkg.NewBrainConventionSource(cfg.BrainVaultPath())
}

func ensureProjectRecord(ctx context.Context, database *sql.DB, cfg *appconfig.Config) error {
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
