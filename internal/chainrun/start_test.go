package chainrun

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ponchione/sodoryard/internal/agent"
	"github.com/ponchione/sodoryard/internal/chain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	appdb "github.com/ponchione/sodoryard/internal/db"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/tool"
)

func TestExitCodeMapsSpecStatuses(t *testing.T) {
	tests := []struct {
		status string
		events []chain.Event
		want   int
	}{
		{status: "completed", want: 0},
		{status: "partial", want: 2},
		{status: "cancelled", want: 4},
		{status: "failed", want: 1},
		{status: "failed", events: []chain.Event{{EventType: chain.EventSafetyLimitHit}}, want: 3},
	}

	for _, tc := range tests {
		if got := exitCode(tc.status, tc.events); got != tc.want {
			t.Fatalf("exitCode(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestBuildTaskIncludesReceiptHistory(t *testing.T) {
	msg := buildTask(Options{
		SourceTask:       "fix auth",
		MaxSteps:         10,
		MaxResolverLoops: 3,
		MaxDuration:      time.Hour,
		TokenBudget:      100,
	}, "chain-1", []string{"receipts/coder/chain-1-step-001.md"})

	if !containsAll(msg, "fix auth", "chain-1", "receipts/coder/chain-1-step-001.md") {
		t.Fatalf("message = %q, want task, chain id, and receipt history", msg)
	}
}

func TestStartReturnsCancelExitCodeForHandledInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := appconfig.Default()
	cfg.ProjectRoot = t.TempDir()
	cfg.Routing.Default.Provider = "test"
	cfg.Routing.Default.Model = "test-model"
	cfg.Providers = map[string]appconfig.ProviderConfig{
		"test": {Type: "openai-compatible", Model: "test-model", ContextLength: 128},
	}
	cfg.AgentRoles = map[string]appconfig.AgentRoleConfig{
		"orchestrator": {SystemPrompt: "builtin:orchestrator", MaxTurns: 3},
	}

	db := newChainrunTestDB(t)
	store := chain.NewStore(db)
	deps := Deps{
		BuildRuntime: func(ctx context.Context, cfg *appconfig.Config) (*rtpkg.OrchestratorRuntime, error) {
			if err := rtpkg.EnsureProjectRecord(ctx, db, cfg); err != nil {
				return nil, err
			}
			return &rtpkg.OrchestratorRuntime{
				Config:              cfg,
				Logger:              slog.Default(),
				Database:            db,
				Queries:             appdb.New(db),
				ConversationManager: conversation.NewManager(db, nil, slog.Default()),
				ContextAssembler:    rtpkg.NoopContextAssembler{},
				ChainStore:          store,
				Cleanup:             func() {},
			}, nil
		},
		BuildRegistry: func(*rtpkg.OrchestratorRuntime, appconfig.AgentRoleConfig, string) (*tool.Registry, error) {
			return tool.NewRegistry(), nil
		},
		NewTurnRunner: func(agent.AgentLoopDeps) TurnRunner {
			return fakeTurnRunner{run: func(runCtx context.Context, req agent.RunTurnRequest) (*agent.TurnResult, error) {
				cancel()
				<-runCtx.Done()
				return nil, agent.ErrTurnCancelled
			}}
		},
		NewChainID: func() string { return "cancel-exit-code" },
		ProcessID:  func() int { return 1234 },
	}

	_, err := Start(ctx, cfg, Options{SourceTask: "cancel me", MaxSteps: 10, MaxResolverLoops: 1, MaxDuration: time.Hour, TokenBudget: 100}, deps)
	var exitErr ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Start error = %T %[1]v, want ExitError", err)
	}
	if exitErr.ExitCode() != 4 {
		t.Fatalf("ExitCode = %d, want 4", exitErr.ExitCode())
	}
	stored, loadErr := store.GetChain(context.Background(), "cancel-exit-code")
	if loadErr != nil {
		t.Fatalf("GetChain returned error: %v", loadErr)
	}
	if stored.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", stored.Status)
	}
}

func TestStartAppliesOrchestratorRoleTimeout(t *testing.T) {
	ctx := context.Background()
	cfg := appconfig.Default()
	cfg.ProjectRoot = t.TempDir()
	cfg.Routing.Default.Provider = "test"
	cfg.Routing.Default.Model = "test-model"
	cfg.Providers = map[string]appconfig.ProviderConfig{
		"test": {Type: "openai-compatible", Model: "test-model", ContextLength: 128},
	}
	cfg.AgentRoles = map[string]appconfig.AgentRoleConfig{
		"orchestrator": {SystemPrompt: "builtin:orchestrator", MaxTurns: 3, Timeout: appconfig.Duration(20 * time.Millisecond)},
	}

	db := newChainrunTestDB(t)
	store := chain.NewStore(db)
	deps := Deps{
		BuildRuntime: func(ctx context.Context, cfg *appconfig.Config) (*rtpkg.OrchestratorRuntime, error) {
			if err := rtpkg.EnsureProjectRecord(ctx, db, cfg); err != nil {
				return nil, err
			}
			return &rtpkg.OrchestratorRuntime{
				Config:              cfg,
				Logger:              slog.Default(),
				Database:            db,
				Queries:             appdb.New(db),
				ConversationManager: conversation.NewManager(db, nil, slog.Default()),
				ContextAssembler:    rtpkg.NoopContextAssembler{},
				ChainStore:          store,
				Cleanup:             func() {},
			}, nil
		},
		BuildRegistry: func(*rtpkg.OrchestratorRuntime, appconfig.AgentRoleConfig, string) (*tool.Registry, error) {
			return tool.NewRegistry(), nil
		},
		NewTurnRunner: func(agent.AgentLoopDeps) TurnRunner {
			return fakeTurnRunner{run: func(runCtx context.Context, req agent.RunTurnRequest) (*agent.TurnResult, error) {
				<-runCtx.Done()
				if !errors.Is(runCtx.Err(), context.DeadlineExceeded) {
					t.Fatalf("RunTurn context error = %v, want deadline exceeded", runCtx.Err())
				}
				return nil, agent.ErrTurnCancelled
			}}
		},
		NewChainID: func() string { return "timeout-exit-code" },
		ProcessID:  func() int { return 1234 },
	}

	_, err := Start(ctx, cfg, Options{SourceTask: "timeout", MaxSteps: 10, MaxResolverLoops: 1, MaxDuration: time.Hour, TokenBudget: 100}, deps)
	var exitErr ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Start error = %T %[1]v, want ExitError", err)
	}
	if exitErr.ExitCode() != 3 {
		t.Fatalf("ExitCode = %d, want 3 after orchestrator role timeout", exitErr.ExitCode())
	}
	stored, loadErr := store.GetChain(context.Background(), "timeout-exit-code")
	if loadErr != nil {
		t.Fatalf("GetChain returned error: %v", loadErr)
	}
	if stored.Status != "failed" {
		t.Fatalf("status = %q, want failed after timeout", stored.Status)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

type fakeTurnRunner struct {
	run func(context.Context, agent.RunTurnRequest) (*agent.TurnResult, error)
}

func (f fakeTurnRunner) RunTurn(ctx context.Context, req agent.RunTurnRequest) (*agent.TurnResult, error) {
	return f.run(ctx, req)
}

func (f fakeTurnRunner) Close() {}

func newChainrunTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	database, err := appdb.OpenDB(ctx, filepath.Join(t.TempDir(), "chainrun.db"))
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := appdb.InitIfNeeded(ctx, database); err != nil {
		t.Fatalf("InitIfNeeded returned error: %v", err)
	}
	if err := appdb.EnsureChainSchema(ctx, database); err != nil {
		t.Fatalf("EnsureChainSchema returned error: %v", err)
	}
	return database
}
