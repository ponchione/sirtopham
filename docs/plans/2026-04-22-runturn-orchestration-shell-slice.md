# RunTurn orchestration-shell extraction plan

> For Hermes: Use subagent-driven-development skill to implement this plan task-by-task.

Goal: Take the next narrow slice of P0.1 from `GO_SIMPLIFICATION_SWEEP.md` by removing the remaining orchestration-shell code smell from `internal/agent/loop.go` without changing agent-loop behavior.

Architecture: `internal/agent` already has a first-pass `RunTurn` decomposition (`runturn_iteration.go`, `runturn_tools.go`, `runturn_persist.go`, `runturn_finalize.go`, `turn_cleanup.go`), but `RunTurn` still owns turn bootstrap, cancellation wiring, per-iteration control flow, and final loop exit handling inline. This slice should keep `RunTurn` as the high-level coordinator while moving the start-of-turn and single-iteration mechanics behind explicit helpers/data carriers. Do not redesign loop semantics, tool policy, prompt construction, or cancellation behavior.

Tech Stack: Go 1.25, stdlib context/errors/time, existing `internal/agent` package helpers/tests, project validation via `make test` and `make build`.

Current grounded state
- `GO_SIMPLIFICATION_SWEEP.md` identifies P0.1 (`internal/agent/loop.go` / `RunTurn`) as the top maintainability hotspot.
- Current tree already landed partial extraction helpers:
  - `internal/agent/runturn_iteration.go` — 146 lines
  - `internal/agent/runturn_tools.go` — 203 lines
  - `internal/agent/runturn_persist.go` — 95 lines
  - `internal/agent/runturn_finalize.go` — 64 lines
  - `internal/agent/turn_cleanup.go` — 208 lines
- `internal/agent/loop.go` is still 874 lines and `RunTurn` still inlines:
  - dependency/request validation
  - cancel-context setup/teardown
  - user-message persistence
  - turn-context preparation
  - top-level iteration loop body and max-iteration escape
- Existing behavior coverage already lives mainly in:
  - `internal/agent/loop_test.go`
  - `internal/agent/loop_event_test.go`
  - `internal/agent/loop_sqlite_integration_test.go`
  - `internal/agent/turn_cleanup_test.go`

Definition of the single next slice
- Complete the next extraction-only pass for P0.1 by isolating the remaining `RunTurn` orchestration shell.
- End state for this slice:
  - `RunTurn` reads as: validate -> start turn -> iterate through helper -> finalize/return.
  - Loop-body details live outside `loop.go`.
  - Turn bootstrap/error-wrapping logic lives outside `loop.go`.
  - Behavior, event order, persistence semantics, and cancellation semantics remain unchanged.
- Explicitly out of scope for this slice:
  - changing prompt-building APIs
  - changing compression heuristics
  - changing tool batching or tool-result budgeting semantics
  - changing cancellation tombstone structure
  - moving unrelated utility methods (`withDefaultConfig`, `isCancelled`, etc.) just for file-count aesthetics

---

### Task 1: Freeze the slice boundary in tests before moving code

Objective: Add narrow tests that lock down the exact orchestration seams this slice is allowed to move, so the refactor stays behavior-preserving.

Files:
- Modify: `internal/agent/loop_test.go`
- Modify: `internal/agent/loop_event_test.go`
- Optional create: `internal/agent/runturn_start_test.go`
- Optional create: `internal/agent/runturn_orchestration_test.go`

Step 1: Add/expand bootstrap-path tests

Cover these cases explicitly:
1. cancel during `PersistUserMessage` still routes through `handleCancellation(...)`
2. plain persistence failure still emits `persist_user_message_failed`
3. cancel during `PrepareTurnContext(...)` still routes through `handleCancellation(...)`
4. iteration-setup cancellation still routes through `handleIterationSetupCancellation(...)`

Suggested test names:
```go
func TestRunTurnCancelsDuringPersistUserMessage(t *testing.T)
func TestRunTurnReturnsPersistenceErrorBeforeIterations(t *testing.T)
func TestRunTurnCancelsDuringPrepareTurnContext(t *testing.T)
func TestRunTurnCancelsBeforeIterationExecution(t *testing.T)
```

Step 2: Add a single orchestration-shape regression test

Goal: prove the high-level phase order remains stable even after extracting helpers.

Example assertion shape:
```go
func TestRunTurnMaintainsBootstrapThenIterationOrder(t *testing.T) {
    // arrange stubs that record calls in order:
    // PersistUserMessage -> PrepareTurnContext/history rebuild -> provider stream -> PersistIteration
    // assert exact ordered milestones, not implementation-private helper names
}
```

Step 3: Run focused tests first

Run:
```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/agent -run 'RunTurn|PrepareTurnContext|TurnCleanup' -count=1
```

Expected: PASS on current tree before refactor.

Step 4: Commit checkpoint

```bash
git add internal/agent/*test.go
git commit -m "test: lock runturn orchestration behavior"
```

---

### Task 2: Extract turn-bootstrap mechanics out of `RunTurn`

Objective: Move validation/startup/setup mechanics out of `RunTurn` into a dedicated helper/data carrier so the method no longer owns inline bootstrap details.

Files:
- Modify: `internal/agent/loop.go`
- Create: `internal/agent/runturn_start.go`
- Modify: `internal/agent/runturn_types.go`
- Test: `internal/agent/loop_test.go`

Target design

Introduce a dedicated start helper that owns all start-of-turn mechanics after top-level nil-context normalization.

Suggested carrier/types:
```go
type preparedTurn struct {
    ctx      context.Context
    req      RunTurnRequest
    exec     *turnExecution
    started  time.Time
    cleanup  func()
}
```

Suggested helper signatures:
```go
func (l *AgentLoop) prepareRunTurn(ctx context.Context, req RunTurnRequest) (*preparedTurn, error)
func (l *AgentLoop) persistInitialUserMessage(ctx context.Context, req RunTurnRequest) error
func (l *AgentLoop) prepareTurnExecution(ctx context.Context, req RunTurnRequest, turnStart time.Time) (*turnExecution, error)
```

Required behavioral rules
- `prepareRunTurn(...)` must preserve the current order:
  1. loop validation
  2. provider/tool dependency checks
  3. request validation
  4. derive cancellable context and register `Cancel()` hook
  5. persist user message
  6. prepare turn context
  7. build `turnExecution`
- Keep the same error wrapping/messages now emitted from `RunTurn`.
- Keep cleanup ownership obvious: the returned `cleanup()` should call both deferred cancel cleanup hooks currently in `RunTurn`.
- Do not hide cancellation semantics inside `newTurnExecution(...)`; keep it strictly a data-construction helper.

Implementation note
- Keep `RunTurn` responsible for `if ctx == nil { ctx = context.Background() }` and for `defer prepared.cleanup()` after bootstrap succeeds.
- If bootstrap fails after `setCancel(cancel)`, ensure cleanup still clears stored cancel state before return.

Step 1: Write the failing bootstrap helper tests

Example shape:
```go
func TestPrepareRunTurnBuildsExecutionState(t *testing.T) {
    // assert effective provider/model, currentTurnMessages, turnCtx wiring
}
```

Step 2: Add `runturn_start.go` with the bootstrap helpers.

Step 3: Shrink `RunTurn` to call the new bootstrap helper.

Desired shape after this task:
```go
func (l *AgentLoop) RunTurn(ctx context.Context, req RunTurnRequest) (*TurnResult, error) {
    if ctx == nil {
        ctx = context.Background()
    }

    prepared, err := l.prepareRunTurn(ctx, req)
    if err != nil {
        return nil, err
    }
    defer prepared.cleanup()

    return l.runTurnIterations(prepared.ctx, prepared.exec)
}
```

Step 4: Run focused tests

Run:
```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/agent -run 'RunTurn|PrepareTurnContext' -count=1
```

Expected: PASS.

Step 5: Commit

```bash
git add internal/agent/loop.go internal/agent/runturn_start.go internal/agent/runturn_types.go internal/agent/*test.go
git commit -m "refactor: extract runturn bootstrap flow"
```

---

### Task 3: Extract one-iteration orchestration from `RunTurn`

Objective: Remove the remaining inline iteration body from `RunTurn` by wrapping a single loop iteration in one helper that returns either continue-or-finish state.

Files:
- Modify: `internal/agent/loop.go`
- Create: `internal/agent/runturn_orchestration.go`
- Modify: `internal/agent/runturn_types.go`
- Test: `internal/agent/loop_test.go`
- Test: `internal/agent/loop_event_test.go`

Target design

Introduce a single helper that owns the current inline sequence:
- iteration start logging
- pre-iteration cancellation check
- `prepareIteration(...)`
- `runProviderIteration(...)`
- assistant serialization
- text-only completion path
- tool execution path
- persistence/update/nudge path

Suggested return carrier:
```go
type iterationOutcome struct {
    done   bool
    result *TurnResult
}
```

Suggested helper:
```go
func (l *AgentLoop) runSingleIteration(ctx context.Context, turnExec *turnExecution, iteration int) (*iterationOutcome, error)
```

Helper contract
- If the turn is complete, return `&iterationOutcome{done: true, result: ...}`.
- If the iteration completed and the loop should continue, return `&iterationOutcome{done: false}`.
- If the turn should abort, return the current error semantics unchanged.
- `turnExec.completedIterations` must still advance only after successful tool-iteration persistence.

Implementation note
- Do not over-split in this task. One orchestration helper is enough.
- Reuse existing lower-level helpers exactly as they are.
- Avoid introducing generic callback abstractions; this slice is extraction-only.

Desired `RunTurn` shape after this task:
```go
for iteration := 1; l.cfg.MaxIterations == 0 || iteration <= l.cfg.MaxIterations; iteration++ {
    outcome, err := l.runSingleIteration(prepared.ctx, prepared.exec, iteration)
    if err != nil {
        return nil, err
    }
    if outcome.done {
        return outcome.result, nil
    }
}
return nil, fmt.Errorf("agent loop: exceeded max iterations (%d)", l.cfg.MaxIterations)
```

Step 1: Add failing tests for the extracted helper seam

Examples:
```go
func TestRunSingleIterationCompletesTextOnlyTurn(t *testing.T)
func TestRunSingleIterationPersistsToolIterationBeforeContinuing(t *testing.T)
func TestRunSingleIterationReturnsEarlyChainCompleteResult(t *testing.T)
```

Step 2: Implement `runSingleIteration(...)` in `runturn_orchestration.go`.

Step 3: Replace the inline body in `RunTurn` with the helper call.

Step 4: Run focused tests

Run:
```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/agent -run 'RunTurn|RunSingleIteration|EventOrdering' -count=1
```

Expected: PASS.

Step 5: Commit

```bash
git add internal/agent/loop.go internal/agent/runturn_orchestration.go internal/agent/runturn_types.go internal/agent/*test.go
git commit -m "refactor: extract runturn iteration orchestration"
```

---

### Task 4: Move remaining cleanup wrappers out of `loop.go`

Objective: Finish this slice by moving `handleCancellation(...)`, `handleIterationSetupCancellation(...)`, `handleTurnCancellation(...)`, `handleTurnStreamFailure(...)`, and `handleTurnCleanup(...)` out of `loop.go`, since the real cleanup planning logic already lives in `turn_cleanup.go`.

Files:
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/turn_cleanup.go`
- Test: `internal/agent/turn_cleanup_test.go`
- Test: `internal/agent/loop_event_test.go`

Step 1: Move the wrappers into `turn_cleanup.go`

Keep signatures stable:
```go
func (l *AgentLoop) handleCancellation(...)
func (l *AgentLoop) handleIterationSetupCancellation(...)
func (l *AgentLoop) handleTurnCancellation(...)
func (l *AgentLoop) handleTurnStreamFailure(...)
func (l *AgentLoop) handleTurnCleanup(...)
```

Step 2: Ensure no behavior changes
- same cleanup-plan construction
- same logging keys
- same event emission order
- same `ErrTurnCancelled` wrapping behavior

Step 3: Run cleanup-focused tests

Run:
```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/agent -run 'TurnCleanup|Cancellation|EventOrderingCancellation' -count=1
```

Expected: PASS.

Step 4: Commit

```bash
git add internal/agent/loop.go internal/agent/turn_cleanup.go internal/agent/*test.go
git commit -m "refactor: co-locate runturn cleanup wrappers"
```

---

### Task 5: Do the narrow post-refactor cleanup pass

Objective: Make the new helper boundaries readable and verify the slice actually paid down the code smell.

Files:
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/runturn_start.go`
- Modify: `internal/agent/runturn_orchestration.go`
- Modify: `internal/agent/runturn_types.go`

Step 1: Keep comments aligned with the new structure
- Update `RunTurn` comment so it describes orchestration role, not inline mechanics.
- Add concise comments to any new carrier structs only where needed.
- Remove comments in `loop.go` that describe code no longer located there.

Step 2: Re-check file boundaries
- `loop.go` should retain package-level types and generic loop utilities.
- `runturn_start.go` should hold only start/bootstrap mechanics.
- `runturn_orchestration.go` should hold only top-level iteration coordination.
- `turn_cleanup.go` should own cleanup behavior end-to-end.

Step 3: Measure the expected simplification outcome

Success criteria:
- `RunTurn` body is short enough to fit on one screen (~40-70 lines target; exact count not critical)
- no duplicated turn-bootstrap logic remains in `RunTurn`
- no duplicated iteration-body orchestration remains in `RunTurn`
- `loop.go` loses at least ~100 lines without semantic redesign
- all existing tests remain green

Step 4: Run full validation required by repo guidance

Run:
```bash
make test
make build
```

Expected: both PASS.

Step 5: Commit final slice

```bash
git add internal/agent/*.go
git commit -m "refactor: simplify runturn orchestration shell"
```

---

## Notes for the implementer

Preserve these invariants exactly
- User message persists before context assembly.
- Turn context is frozen once per turn.
- `WaitingForLLM` still emits once from `PrepareTurnContext(...)` and again before each streamed provider iteration.
- `turnExec.completedIterations` only advances after successful persistence of a tool iteration.
- Text-only completion still updates post-turn quality, maybe generates a title, emits `TurnCompleteEvent`, then emits `Idle`.
- Cancellation and stream-failure paths must still persist interrupted assistant/tool state exactly as they do now.
- `toolpkg.ErrChainComplete` remains an early-success escape, not an error.

Avoid these traps
- Do not inline `PrepareTurnContext(...)` into new helpers; keep it as the public turn-context seam unless you are only wrapping it.
- Do not introduce interface churn for tests unless existing stubs truly cannot express the new helper boundary.
- Do not mix compression-helper relocation into this slice; `tryEmergencyCompression(...)` and `buildPromptConfig(...)` can remain where they are.
- Do not convert this refactor into a package reorganization project.

Nice-to-have but optional within this slice
- Add one tiny unexported helper for the max-iteration loop condition if it improves readability.
- Add one tiny helper to centralize the repeated `isCancelled(ctx)` + cancellation-wrapper pattern only if it reduces duplication without obscuring control flow.

## Verification checklist
- [ ] Focused bootstrap/cancellation tests pass before refactor
- [ ] `RunTurn` delegates start/bootstrap to a helper
- [ ] `RunTurn` delegates single-iteration orchestration to a helper
- [ ] cleanup wrappers live with cleanup implementation
- [ ] event ordering tests stay green
- [ ] interrupted-iteration persistence tests stay green
- [ ] `make test` passes
- [ ] `make build` passes

## Why this is the next slice
- P0.2 (`cmd/yard` / `cmd/tidmouth` headless duplication) has already been partially/mostly addressed on the current tree via `internal/headless`, so it is no longer the highest-value immediate simplification target.
- P0.1 is also partially underway, but its remaining orchestration shell still sits in `loop.go` and continues to concentrate change risk.
- This slice is narrow, behavior-preserving, and composes cleanly with any later P0.1 follow-up around compression/prompt helpers.

Execution handoff
Plan complete and saved. Ready to execute this slice narrowly once you want implementation to start.