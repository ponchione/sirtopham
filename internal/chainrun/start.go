package chainrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ponchione/sodoryard/internal/agent"
	"github.com/ponchione/sodoryard/internal/chain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/ponchione/sodoryard/internal/conversation"
	"github.com/ponchione/sodoryard/internal/id"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/tool"
)

type TurnRunner interface {
	RunTurn(ctx context.Context, req agent.RunTurnRequest) (*agent.TurnResult, error)
	Close()
}

type WatchHandle interface {
	Wait(timeout time.Duration) error
}

type Options struct {
	ChainID          string
	SourceSpecs      []string
	SourceTask       string
	MaxSteps         int
	MaxResolverLoops int
	MaxDuration      time.Duration
	TokenBudget      int
	DryRun           bool

	OnChainID         func(string)
	OnMessage         func(string)
	StartWatch        func(context.Context, *chain.Store, string) WatchHandle
	WatchFlushTimeout time.Duration
}

type Result struct {
	ChainID string
	Status  string
}

type Deps struct {
	BuildRuntime  func(context.Context, *appconfig.Config) (*rtpkg.OrchestratorRuntime, error)
	BuildRegistry func(*rtpkg.OrchestratorRuntime, appconfig.AgentRoleConfig, string) (*tool.Registry, error)
	NewTurnRunner func(agent.AgentLoopDeps) TurnRunner
	NewChainID    func() string
	ProcessID     func() int
}

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e ExitError) Unwrap() error { return e.Err }
func (e ExitError) ExitCode() int { return e.Code }

func Start(ctx context.Context, cfg *appconfig.Config, opts Options, deps Deps) (result *Result, err error) {
	deps = withDefaultDeps(deps)
	if cfg == nil {
		return nil, fmt.Errorf("chain start: config is required")
	}
	roleCfg, ok := cfg.AgentRoles["orchestrator"]
	if !ok {
		return nil, fmt.Errorf("agent role %q not found in config", "orchestrator")
	}
	systemPrompt, _, err := rtpkg.LoadRoleSystemPrompt("orchestrator", cfg.ProjectRoot, roleCfg.SystemPrompt)
	if err != nil {
		return nil, err
	}
	rt, err := deps.BuildRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer rt.Cleanup()

	chainID := strings.TrimSpace(opts.ChainID)
	if chainID == "" {
		chainID = deps.NewChainID()
	}

	existing, err := resolveExistingChain(ctx, rt.ChainStore, chainID)
	if err != nil {
		return nil, err
	}
	isNew := existing == nil
	resumed := false
	if existing != nil {
		opts, err = populateOptionsFromExisting(opts, existing)
		if err != nil {
			return nil, err
		}
		resumed = existing.Status == "paused"
		if err := prepareExistingChainForExecution(ctx, rt.ChainStore, existing); err != nil {
			return nil, err
		}
	} else {
		if _, err := rt.ChainStore.StartChain(ctx, chainSpecFromOptions(chainID, opts)); err != nil {
			return nil, err
		}
		_ = rt.ChainStore.LogEvent(ctx, chainID, "", chain.EventChainStarted, map[string]any{"specs": opts.SourceSpecs, "task": opts.SourceTask})
	}

	if opts.OnChainID != nil {
		opts.OnChainID(chainID)
	}
	if opts.DryRun {
		return &Result{ChainID: chainID, Status: "running"}, nil
	}

	executionRegistered := false
	defer func() {
		if err == nil || !executionRegistered {
			return
		}
		if closeErr := closeErroredExecution(context.WithoutCancel(ctx), rt.ChainStore, chainID, err.Error()); closeErr != nil {
			err = fmt.Errorf("%w (while closing active execution: %v)", err, closeErr)
		}
	}()
	if err := registerActiveExecution(ctx, rt.ChainStore, chainID, isNew, resumed, deps.ProcessID()); err != nil {
		return nil, err
	}
	executionRegistered = true

	registry, err := deps.BuildRegistry(rt, roleCfg, chainID)
	if err != nil {
		return nil, err
	}
	conv, err := rt.ConversationManager.Create(ctx, cfg.ProjectRoot, conversation.WithProvider(cfg.Routing.Default.Provider), conversation.WithModel(cfg.Routing.Default.Model))
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	limit, err := rtpkg.ResolveModelContextLimit(cfg, cfg.Routing.Default.Provider)
	if err != nil {
		return nil, err
	}
	loop := deps.NewTurnRunner(agent.AgentLoopDeps{ContextAssembler: rt.ContextAssembler, ConversationManager: rt.ConversationManager, ProviderRouter: rt.ProviderRouter, ToolExecutor: &rtpkg.RegistryToolExecutor{Registry: registry, ProjectRoot: cfg.ProjectRoot}, ToolDefinitions: registry.ToolDefinitions(), PromptBuilder: agent.NewPromptBuilder(rt.Logger), TitleGenerator: conversation.NewTitleGen(rt.ConversationManager, rt.ProviderRouter, cfg.Routing.Default.Model, rt.Logger), Config: rtpkg.BuildAgentLoopConfig(cfg, roleCfg.MaxTurns, systemPrompt), Logger: rt.Logger})
	defer loop.Close()

	var watch WatchHandle
	if opts.StartWatch != nil {
		watch = opts.StartWatch(ctx, rt.ChainStore, chainID)
	}

	steps, err := rt.ChainStore.ListSteps(ctx, chainID)
	if err != nil {
		return nil, err
	}
	turnTask := buildTask(opts, chainID, existingReceiptPaths(steps))
	runCtx := ctx
	cancelRun := func() {}
	if timeout := roleCfg.Timeout.Duration(); timeout > 0 {
		runCtx, cancelRun = context.WithTimeout(ctx, timeout)
	}
	defer cancelRun()
	if _, err := loop.RunTurn(runCtx, agent.RunTurnRequest{ConversationID: conv.ID, TurnNumber: 1, Message: turnTask, ModelContextLimit: limit}); err != nil {
		if handled, handleErr := handleInterruption(runCtx, rt.ChainStore, chainID, err, opts.OnMessage); handled || handleErr != nil {
			if handleErr != nil {
				return nil, handleErr
			}
			status := terminalStatus(ctx, rt.ChainStore, chainID)
			if err := waitWatch(watch, opts.WatchFlushTimeout); err != nil {
				return nil, err
			}
			if code := exitCode(status, mustListEvents(ctx, rt.ChainStore, chainID)); code != 0 {
				return nil, ExitError{Code: code, Err: fmt.Errorf("chain %s ended with status %s", chainID, status)}
			}
			return &Result{ChainID: chainID, Status: status}, nil
		}
		return nil, err
	}
	if err := finalizeRequestedChainStatus(ctx, rt.ChainStore, chainID); err != nil {
		return nil, err
	}
	stored, err := rt.ChainStore.GetChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	if code := exitCode(stored.Status, mustListEvents(ctx, rt.ChainStore, chainID)); code != 0 {
		return nil, ExitError{Code: code, Err: fmt.Errorf("chain %s ended with status %s", chainID, stored.Status)}
	}
	if err := waitWatch(watch, opts.WatchFlushTimeout); err != nil {
		return nil, err
	}
	return &Result{ChainID: chainID, Status: stored.Status}, nil
}

func withDefaultDeps(deps Deps) Deps {
	if deps.BuildRuntime == nil {
		deps.BuildRuntime = rtpkg.BuildOrchestratorRuntime
	}
	if deps.BuildRegistry == nil {
		deps.BuildRegistry = rtpkg.BuildOrchestratorRegistry
	}
	if deps.NewTurnRunner == nil {
		deps.NewTurnRunner = func(deps agent.AgentLoopDeps) TurnRunner { return agent.NewAgentLoop(deps) }
	}
	if deps.NewChainID == nil {
		deps.NewChainID = id.New
	}
	if deps.ProcessID == nil {
		deps.ProcessID = os.Getpid
	}
	return deps
}

func resolveExistingChain(ctx context.Context, store *chain.Store, chainID string) (*chain.Chain, error) {
	if strings.TrimSpace(chainID) == "" {
		return nil, nil
	}
	existing, err := store.GetChain(ctx, chainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "sql: no rows") {
			return nil, nil
		}
		return nil, err
	}
	return existing, nil
}

func populateOptionsFromExisting(opts Options, existing *chain.Chain) (Options, error) {
	if existing == nil {
		return opts, nil
	}
	if len(opts.SourceSpecs) == 0 && len(existing.SourceSpecs) > 0 {
		opts.SourceSpecs = append([]string(nil), existing.SourceSpecs...)
	}
	if strings.TrimSpace(opts.SourceTask) == "" {
		opts.SourceTask = existing.SourceTask
	}
	if strings.TrimSpace(opts.SourceTask) == "" && len(opts.SourceSpecs) == 0 {
		return opts, fmt.Errorf("chain %s has no stored task/specs to resume from", existing.ID)
	}
	return opts, nil
}

func prepareExistingChainForExecution(ctx context.Context, store *chain.Store, existing *chain.Chain) error {
	if existing == nil {
		return nil
	}
	resumeReady, err := chain.ResumeExecutionReady(existing.Status)
	if err != nil {
		return fmt.Errorf("chain %s %w", existing.ID, err)
	}
	if !resumeReady {
		return nil
	}
	if err := store.SetChainStatus(ctx, existing.ID, "running"); err != nil {
		return err
	}
	_ = store.LogEvent(ctx, existing.ID, "", chain.EventChainResumed, map[string]any{"resumed_by": "cli"})
	return nil
}

func registerActiveExecution(ctx context.Context, store *chain.Store, chainID string, isNew bool, resumed bool, pid int) error {
	eventType := chain.EventChainStarted
	payload := map[string]any{"orchestrator_pid": pid, "active_execution": true, "execution_id": id.New()}
	if resumed {
		eventType = chain.EventChainResumed
		payload["resumed_by"] = "cli"
	} else if !isNew {
		eventType = chain.EventChainResumed
		payload["continued_by"] = "cli"
	}
	return store.LogEvent(ctx, chainID, "", eventType, payload)
}

func handleInterruption(ctx context.Context, store *chain.Store, chainID string, err error, onMessage func(string)) (bool, error) {
	if !errors.Is(err, agent.ErrTurnCancelled) {
		return false, nil
	}
	cleanupCtx := context.WithoutCancel(ctx)
	if err := finalizeRequestedChainStatus(cleanupCtx, store, chainID); err != nil {
		return true, err
	}
	ch, loadErr := store.GetChain(cleanupCtx, chainID)
	if loadErr != nil {
		return true, loadErr
	}
	switch ch.Status {
	case "cancelled":
		if err := chain.CloseTerminalizedActiveExecution(cleanupCtx, store, chainID, ch.Status, nil); err != nil {
			return true, err
		}
		emit(onMessage, "chain %s cancelled\n", chainID)
		return true, nil
	case "paused":
		if err := chain.CloseTerminalizedActiveExecution(cleanupCtx, store, chainID, ch.Status, nil); err != nil {
			return true, err
		}
		emit(onMessage, "chain %s paused\n", chainID)
		return true, nil
	case "running":
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			summary := fmt.Sprintf("chain %s hit orchestrator timeout", chainID)
			_ = store.LogEvent(cleanupCtx, chainID, "", chain.EventSafetyLimitHit, map[string]any{"role": "orchestrator", "limit": "timeout"})
			if err := chain.ApplyTerminalChainClosure(cleanupCtx, store, chainID, chain.TerminalChainClosure{Status: "failed", EventType: chain.EventChainCompleted, Summary: &summary, Extra: map[string]any{"summary": summary}}); err != nil {
				return true, err
			}
			emit(onMessage, "%s\n", summary)
			return true, nil
		}
		if ctx.Err() != nil {
			if err := chain.ApplyTerminalChainClosure(cleanupCtx, store, chainID, chain.TerminalChainClosure{Status: "cancelled", EventType: chain.EventChainCancelled, Extra: map[string]any{"finalized_from": "interrupted"}}); err != nil {
				return true, err
			}
			emit(onMessage, "chain %s cancelled\n", chainID)
			return true, nil
		}
		return false, nil
	default:
		return false, nil
	}
}

func finalizeRequestedChainStatus(ctx context.Context, store *chain.Store, chainID string) error {
	ch, err := store.GetChain(ctx, chainID)
	if err != nil {
		return err
	}
	if finalStatus, ok := chain.FinalizeControlStatus(ch.Status); ok {
		eventType, eventOK := chain.FinalizeControlEventType(ch.Status)
		if eventOK {
			return chain.ApplyTerminalChainClosure(ctx, store, chainID, chain.TerminalChainClosure{
				Status:    finalStatus,
				EventType: eventType,
				Extra:     map[string]any{"finalized_from": ch.Status},
			})
		}
		if err := store.SetChainStatus(ctx, chainID, finalStatus); err != nil {
			return err
		}
	}
	return nil
}

func closeErroredExecution(ctx context.Context, store *chain.Store, chainID string, summary string) error {
	events := mustListEvents(ctx, store, chainID)
	if _, ok := chain.LatestActiveExecution(events); !ok {
		return nil
	}
	return chain.ApplyTerminalChainClosure(ctx, store, chainID, chain.TerminalChainClosure{
		Status:    "failed",
		EventType: chain.EventChainCompleted,
		Summary:   &summary,
		Extra:     map[string]any{"summary": summary},
	})
}

func chainSpecFromOptions(chainID string, opts Options) chain.ChainSpec {
	return chain.ChainSpec{ChainID: chainID, SourceSpecs: append([]string(nil), opts.SourceSpecs...), SourceTask: strings.TrimSpace(opts.SourceTask), MaxSteps: opts.MaxSteps, MaxResolverLoops: opts.MaxResolverLoops, MaxDuration: opts.MaxDuration, TokenBudget: opts.TokenBudget}
}

func buildTask(opts Options, chainID string, receiptPaths []string) string {
	history := "No existing receipt paths were found for this chain yet."
	if len(receiptPaths) > 0 {
		history = fmt.Sprintf("Relevant existing receipt paths to read first: %s.", strings.Join(receiptPaths, ", "))
	}
	if len(opts.SourceSpecs) > 0 {
		return fmt.Sprintf("You are managing a chain execution. Source specs: %s. Chain ID: %s. Read the specs from the brain. %s Continue orchestrating from the current point.", strings.Join(opts.SourceSpecs, ", "), chainID, history)
	}
	return fmt.Sprintf("You are managing a chain execution. Task: %s. Chain ID: %s. %s Continue orchestrating from the current point.", strings.TrimSpace(opts.SourceTask), chainID, history)
}

func existingReceiptPaths(steps []chain.Step) []string {
	paths := make([]string, 0, len(steps))
	seen := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		path := strings.TrimSpace(step.ReceiptPath)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func exitCode(status string, events []chain.Event) int {
	switch status {
	case "completed", "paused":
		return 0
	case "partial":
		return 2
	case "cancelled":
		return 4
	case "failed":
		if eventsInclude(events, chain.EventSafetyLimitHit) {
			return 3
		}
		return 1
	default:
		return 0
	}
}

func eventsInclude(events []chain.Event, eventType chain.EventType) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func mustListEvents(ctx context.Context, store *chain.Store, chainID string) []chain.Event {
	events, err := store.ListEvents(ctx, chainID)
	if err != nil {
		return nil
	}
	return events
}

func terminalStatus(ctx context.Context, store *chain.Store, chainID string) string {
	ch, err := store.GetChain(context.WithoutCancel(ctx), chainID)
	if err != nil {
		return ""
	}
	return ch.Status
}

func waitWatch(watch WatchHandle, timeout time.Duration) error {
	if watch == nil {
		return nil
	}
	return watch.Wait(timeout)
}

func emit(onMessage func(string), format string, args ...any) {
	if onMessage != nil {
		onMessage(fmt.Sprintf(format, args...))
	}
}
