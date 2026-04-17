# Session handoff — audit follow-through

**Date:** 2026-04-12
**Branch:** main
**Cwd:** /home/gernsback/source/sodoryard

> Read this cold. Everything needed to continue the current audit follow-through is here. If this doc disagrees with the repo, trust the repo and update this doc before acting.

---

## What this repo is

Migrating `ponchione/sirtopham` (single-binary coding harness) into the `ponchione/sodoryard` monorepo. The local directory is `/home/gernsback/source/sodoryard`; the git remote points at `git@github.com:ponchione/sodoryard.git`.

Target monorepo layout remains:
- **Tidmouth** — headless engine harness (`cmd/tidmouth/`)
- **SirTopham** — chain orchestrator (`cmd/sirtopham/`)
- **Yard** — unified operator-facing CLI (`cmd/yard/`)
- **Knapford** — web dashboard placeholder / later `yard serve` expansion

The historical migration roadmap is `sodor-migration-roadmap.md`.

---

## Why this handoff exists

This handoff started from an audit-follow-through session, but the repo has moved since the original note was written.

Current reality as of 2026-04-12 19:07 -04:00:
- the older handoff text below still describes the earlier audit slices accurately
- the repo no longer has a root `AUDIT.md`, so treat references to it as historical context rather than a file you can read now
- a small follow-up UI/runtime contract slice and matching docs/spec reconciliation are now also landed in the working tree

Read in this order before changing code:
1. `AGENTS.md`
2. `TECH-DEBT.md`
3. this file

Use `make test` / `make build` rather than raw Go commands unless you intentionally need a focused command with the right CGO/sqlite flags.

---

## Current working-tree status

As of 2026-04-13 20:32 EDT:
- `go test -tags sqlite_fts5 ./cmd/sirtopham ./cmd/yard -run 'Test.*(RunChain|YardRunChain).*ActiveExecution.*' -count=1` ✅
- `go test -tags sqlite_fts5 ./internal/chain ./cmd/sirtopham ./cmd/yard ./internal/spawn -count=1` ✅
- `make test` ✅
- `make build` ✅
- live `yard serve --config /tmp/my-website-runtime-8092.yaml` browser rerun on `http://localhost:8092` remains the last completed runtime validation ✅

Current modified files (use live `git status` for the full dirty tree):
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_cli_flags_test.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `internal/spawn/chain_complete.go`
- `internal/spawn/chain_complete_test.go`
- `cmd/sirtopham/chain_control_sqlite_test.go`
- `cmd/yard/chain_control_sqlite_test.go`

What the new runtime-validation follow-up proved:
- exact-setup daily-driver validation ran against the intended `my-website` runtime on `:8092`
- first-turn chat, sidebar/new conversation, reload/history, settings/model routing, search quality, code-retrieval grounding, and the maintained six-scenario brain-retrieval package all passed
- backend websocket cancellation itself is live and emits `turn_cancelled` with reason `user_interrupted`

What this slice changed:
- reproduced a concrete frontend/runtime bug: the context inspector eagerly fetched `/api/metrics/conversation/:id/context/:turn` for the newest live turn before the report existed, causing transient 404s and noisy browser/backend logs during normal first-turn use
- added targeted frontend tests for the intended behavior
- changed `use-context-report` so newest-turn report fetches are briefly deferred while following the live latest turn, and the deferred fetch is cancelled if a real-time `context_debug` report arrives first
- reran the live browser check after rebuilding; the first-turn + inspector path stayed clean with no repeated browser-console 404 noise in the rerun
- a smaller remaining observability issue still exists: expected user-triggered cancellation currently logs `agent loop: turn cancelled: context canceled` at `level=ERROR` on the backend even when the turn correctly emits `turn_cancelled/user_interrupted`

The older dirty-tree inventory below is stale history; use live `git status` over that list.

**Not pushed.** User pushes manually.

---

## What was completed this session

### 1. P0 — chain CLI flags / operator surface
Files:
- `cmd/sirtopham/chain.go`
- `cmd/yard/chain.go`
- `cmd/sirtopham/chain_cli_flags_test.go`
- `cmd/yard/chain_test.go`

What changed:
- added/plumbed `--project`, `--brain`, `--max-resolver-loops`
- added numeric validation for chain start flags
- zero resolver loops are allowed
- tests added for flag presence, plumbing, and validation

### 2. P0 — WebSocket error payload contract
Files:
- `internal/server/websocket.go`
- `internal/server/websocket_test.go`

What changed:
- standardized WS error payloads to `message` + optional `recoverable` + optional `error_code`
- tests verify absence of legacy `error` field and correct new shape

### 3. P0 — brain-disabled truly disables brain runtime paths
Files:
- `internal/runtime/engine.go`
- `internal/runtime/engine_test.go`
- `internal/tool/register.go`
- `internal/tool/brain_test.go`
- `internal/role/builder_test.go`
- `cmd/tidmouth/serve_test.go`

What changed:
- disabled brain no longer opens hybrid brain runtime
- disabled brain no longer reads convention-source docs from the vault
- disabled brain no longer registers brain tools
- focused tests added/updated

### 4. P0/P1-ish — chain control semantics materially improved
Files:
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/pause_resume.go`
- `cmd/yard/chain.go`
- `internal/spawn/spawn_agent.go`
- `internal/spawn/spawn_agent_test.go`

What changed:
- resume is real now, not just cosmetic status flipping
- `sirtopham resume <chain-id>` and `yard chain resume <chain-id>` restart orchestration for an existing paused chain using stored chain task/specs
- paused/cancelled chains stop new step scheduling before next `spawn_agent`
- transition validation added so terminal chains cannot be resumed/paused/cancelled nonsensically
- best-effort live cancel path added by logging `orchestrator_pid` and signaling active orchestrator process on cancel
- chain run now listens for interrupt signals so cancel can propagate into the running orchestrator turn and then into the current `spawn_agent` subprocess through existing context-driven subprocess cancellation

Important caveat:
- this is better, but still not a full explicit control-plane implementation with `pause_requested` / `cancel_requested` durable flags or a command queue table
- current behavior is practical and materially improved, but not the last word on spec-perfect control semantics

### 5. P0 — structural hop expansion package-aware filtering
Files:
- `internal/codeintel/searcher/searcher.go`
- `internal/codeintel/searcher/searcher_test.go`

What changed:
- hop expansion no longer accepts all same-name symbols blindly
- after `GetByName(ref.Name)`, candidates are filtered against `ref.Package` using low-risk path/package heuristics
- regression test added with duplicated symbol names across packages to prove unrelated package hop hits are excluded

### 6. P1 — receipt fallback step plumbing + `chain_complete` status fidelity
Files:
- `cmd/tidmouth/receipt.go`
- `cmd/tidmouth/receipt_test.go`
- `cmd/yard/run_helpers.go`
- `cmd/yard/run_helpers_test.go`
- `internal/spawn/chain_complete.go`
- `internal/spawn/chain_complete_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/yard/chain.go`

What changed:
- fallback receipts no longer hardcode `step: 1` for step-specific receipt paths
- step number is inferred from paths like `...-step-003.md`
- direct headless run still defaults safely to step 1 for plain `receipts/{role}/{chain-id}.md`
- `chain_complete status=partial` now persists chain status `partial` instead of collapsing to `completed`
- resume logic now treats `partial` as terminal and refuses resume/continue for that state
- focused tests added for fallback step inference and partial status persistence

### 7. Web/runtime contract cleanup + docs reconciliation
Files:
- `web/src/types/events.ts`
- `web/src/pages/conversation.tsx`
- `web/src/components/inspector/context-inspector.tsx`
- `docs/specs/05-agent-loop.md`
- `docs/specs/06-context-assembly.md`
- `docs/specs/07-web-interface-and-streaming.md`

What changed:
- frontend `AgentState` now matches the shipped backend status values
- conversation streaming UI now shows useful labels for `assembling_context`, `waiting_for_llm`, `executing_tools`, `compressing`, and `idle`
- the context inspector now renders explicit load-failure UI instead of silently appearing empty
- specs/docs now describe the shipped status-state contract and the current inspector/report/signal-flow behavior
- stale doc references to a root `AUDIT.md` were cleaned up under `docs/`

### 8. Receipt-path contract cleanup
Files:
- `internal/receipt/path.go`
- `internal/receipt/path_test.go`
- `cmd/tidmouth/receipt.go`
- `cmd/yard/run_helpers.go`
- `cmd/sirtopham/receipt.go`
- `internal/spawn/spawn_agent.go`
- `internal/spawn/chain_complete.go`
- `docs/specs/13_Headless_Run_Command.md`
- `docs/specs/14_Agent_Roles_and_Brain_Conventions.md`
- `docs/specs/15-chain-orchestrator.md`
- `agents/*.md` receipt-path references for the shipped role prompts

What changed:
- introduced one shared receipt-path helper package so the shipped path conventions are defined in one place
- direct headless runs now explicitly use `receipts/{role}/{chain-id}.md`
- orchestrator-managed step runs now explicitly use `receipts/{role}/{chain-id}-step-{NNN}.md`
- final orchestrator receipts now explicitly use `receipts/orchestrator/{chain-id}.md`
- fallback step inference now reuses the shared path parser instead of duplicating local regex/path logic in multiple commands
- specs and agent prompts now describe the shipped runtime contract instead of the older task-slug or `{chain_id}-{step}` variants

### 9. Narrow chain control-plane follow-through
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/chain_cli_flags_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `internal/spawn/spawn_agent.go`
- `internal/spawn/spawn_agent_test.go`

What changed:
- introduced shared control-state helpers for `pause_requested` and `cancel_requested`
- pausing a running chain now records `pause_requested` instead of pretending the chain is already paused mid-step
- cancelling a running chain now records `cancel_requested` before the best-effort live interrupt path
- resume now accepts `pause_requested` as well as `paused`
- `spawn_agent` now stops new scheduling when a chain is in requested-stop states, not just already-finalized paused/cancelled states
- orchestrator run cleanup now finalizes requested stop states to durable terminal states (`paused` / `cancelled`) once the current turn exits

### 10. Exact-setup daily-driver validation + context-inspector latest-turn fetch hardening
Files:
- `web/src/hooks/use-context-report.ts`
- `web/src/hooks/use-context-report.test.tsx`
- `web/src/test/setup.ts`
- `web/vite.config.ts`
- `web/package.json`
- `web/package-lock.json`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- ran the maintained daily-driver validation flow against the intended `my-website` runtime on `http://localhost:8092`
- confirmed first-turn chat, sidebar/new conversation, reload/history, settings/model routing, search quality, code-retrieval grounding, and the maintained six-scenario brain-retrieval package on the live runtime
- reproduced a concrete bug where the inspector eagerly fetched newest-turn context endpoints before the report existed, causing transient `/context/:turn` and `/context/:turn/signals` 404s in browser console and backend logs
- added targeted frontend tests proving latest-turn fetches are deferred and cancelled when a live `context_debug` report arrives first
- updated `use-context-report` to defer newest-turn fetches briefly while following the live latest turn, which eliminated the repeated first-turn 404 noise in the rerun after rebuild

### 11. Expected-cancellation log-severity cleanup
Files:
- `internal/server/websocket.go`
- `internal/server/websocket_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced a second concrete observability bug: expected user-triggered cancellation emitted `turn_cancelled/user_interrupted` correctly but still logged `msg="run turn"` at `level=ERROR`
- added a regression test proving websocket-run turns that return `agent.ErrTurnCancelled` do not emit the generic run-turn error log
- updated the websocket handler to classify `agent.ErrTurnCancelled` as expected control flow and log it as `run turn cancelled` at info level instead of error
- reran the targeted websocket test, full `make test`, and `make build`
- reran the direct websocket cancellation probe and confirmed the live runtime still emits `turn_cancelled` with reason `user_interrupted`, persists the interrupted tool tombstone, and still returns sanitized `/api/conversations/search?q=interrupted` snippets

### 12. Requested-stop finalization now emits durable terminal chain events
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_control_sqlite_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_control_sqlite_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced a remaining control-plane audit gap: `pause_requested` / `cancel_requested` were finalized to `paused` / `cancelled` silently, so the durable chain event log did not show the terminal control transition after the current turn exited
- added failing tests first for a shared control-event mapping helper plus sqlite-backed command-package regressions proving finalization should append `chain_paused` / `chain_cancelled` events
- added `FinalizeControlEventType(...)` in `internal/chain/control.go`
- updated both CLI finalization paths to log a terminal chain event with `status` and `finalized_from` payload once a requested stop is durably finalized
- reran targeted `go test -tags sqlite_fts5 ./internal/chain ./cmd/sirtopham ./cmd/yard -count=1`, then `make test`, then `make build`

### 13. Resume semantics no longer restart active or pause-pending chains
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_cli_flags_test.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced a deeper control-plane bug: `pause_requested` still counted as resumable, and an already `running` chain also passed through existing-chain execution prep, which meant the CLI could try to start a second orchestrator while the first execution was still active or still winding down toward `paused`
- added failing tests first proving `pause_requested -> running` should be rejected at the shared control-state layer and that both CLI surfaces reject `pause_requested` resume plus duplicate `running` resume attempts
- tightened `NextControlStatus(...)` so `pause_requested` is no longer treated as resumable-to-running
- added shared `ResumeExecutionReady(...)` / `ErrChainAlreadyRunning` control helpers in `internal/chain/control.go`
- updated both CLI existing-chain execution prep paths to only resume from durable `paused`, reject `pause_requested` until it finishes pausing, and surface `already running` for duplicate active-chain resume attempts instead of trying to continue into a second orchestrator run
- reran targeted `go test -tags sqlite_fts5 ./internal/chain ./cmd/sirtopham ./cmd/yard -count=1`, then `make test`, then `make build`

### 14. Cancel signaling now ignores stale orchestrator pids and targets the latest active execution
Files:
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced the next control-plane hardening gap: best-effort cancel signaling could bubble an error when the recorded orchestrator pid was stale/already exited, even though the desired behavior is graceful degradation, and this path needed explicit proof that the latest logged orchestrator pid is the one signaled
- added failing tests first for both CLI surfaces proving (1) stale/already-exited orchestrator pids are ignored cleanly and (2) signaling uses the most recent logged `orchestrator_pid`
- introduced tiny interrupt seams (`interruptChainPID`, `interruptYardChainPID`) plus narrow stale-pid sentinel errors so the logic is testable without broader refactors
- hardened both signal paths to treat `os.ErrProcessDone` / `syscall.ESRCH` as expected stale-pid outcomes and return nil instead of failing the cancel request
- reran targeted `go test -tags sqlite_fts5 ./cmd/sirtopham ./cmd/yard -count=1`, then `make test`, then `make build`

### 15. Cancel targeting now trusts active execution registration events, not any later pid-shaped payload
Files:
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced an execution-identity gap in the current cancel path: simple "latest pid wins" scanning could target a later unrelated event payload that happened to contain `orchestrator_pid`, instead of the latest active execution registration event
- added failing tests first for both CLI surfaces proving cancel should ignore later non-registration events with pid-shaped payloads and still target the latest active registered execution
- tightened active execution registration logging to include `active_execution: true`
- replaced raw latest-pid scanning with narrow helpers that only consider `chain_started` / `chain_resumed` events carrying a valid `orchestrator_pid`; they also honor the new `active_execution` marker while remaining compatible with older registration events that lack the field
- reran targeted `go test -tags sqlite_fts5 ./cmd/sirtopham ./cmd/yard -count=1`, then `make test`, then `make build`

### 16. Active execution identity now carries explicit execution ids through registration and terminal events
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/cancel.go`
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/sirtopham/chain_control_sqlite_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `cmd/yard/chain_control_sqlite_test.go`
- `internal/spawn/chain_complete.go`
- `internal/spawn/chain_complete_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced the next identity gap: active-run targeting and finalization still depended mostly on event ordering/type inference, with no explicit execution id threaded from active registration into terminal events
- added failing tests first for shared active-execution resolution, terminal finalization payloads, and `chain_complete` completion events carrying the current execution id
- introduced shared `chain.LatestActiveExecution(...)` parsing logic that tracks registration events by `execution_id` and ignores execution ids already terminalized by later `chain_paused` / `chain_cancelled` / `chain_completed` events
- active orchestrator registration events now log a fresh `execution_id` alongside `orchestrator_pid` and `active_execution: true`
- requested-stop finalization and `chain_complete` now propagate the active execution id into terminal chain events when available
- both cancel paths now rely on the shared active-execution helper instead of local pid-scan heuristics
- reran targeted `go test -tags sqlite_fts5 ./internal/chain ./cmd/sirtopham ./cmd/yard ./internal/spawn -count=1`, then `make test`, then `make build`

### 17. Active execution lifecycle now closes on errored orchestrator exits too
Files:
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_control_sqlite_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_control_sqlite_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced the remaining lifecycle gap: once an orchestrator execution registered an active `execution_id`, a later non-cancel/non-`chain_complete` command error could return without any terminal event, leaving cancel targeting to believe that execution was still active even though the run had already ended
- added failing sqlite-backed tests first for both CLI surfaces proving an errored run-close helper must mark the chain failed, append a terminal `chain_completed` event carrying that `execution_id`, and leave `LatestActiveExecution(...)` empty afterward
- added narrow deferred cleanup in both `runChain` paths so any command error after active execution registration now best-effort terminalizes the still-active execution as failed
- added focused `closeErroredChainExecution(...)` / `closeErroredYardChainExecution(...)` helpers that write the failed terminal status plus `execution_id` only when an active execution is still open
- reran the focused failing-test command, then `go test -tags sqlite_fts5 ./internal/chain ./cmd/sirtopham ./cmd/yard ./internal/spawn -count=1`, then `make test`, then `make build`

### 18. Terminal execution payload logic is now shared across all run-ending paths
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/yard/chain.go`
- `internal/spawn/chain_complete.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced the next low-churn control-plane drift risk: requested-stop finalization, `chain_complete`, and errored-run closure all emitted terminal events separately, each hand-assembling the payload logic for `status` and `execution_id`
- added failing tests first for a new shared payload helper in `internal/chain/control_test.go`; initial focused `go test` failed with `undefined: BuildTerminalEventPayload`
- added shared `BuildTerminalEventPayload(...)` in `internal/chain/control.go` so terminal event payloads always carry authoritative `status`, preserve caller-specific extras, and attach the current active `execution_id` when one exists
- rewired `finalizeRequestedChainStatus(...)`, `finalizeYardRequestedChainStatus(...)`, `closeErroredChainExecution(...)`, `closeErroredYardChainExecution(...)`, and `spawn.ChainCompleteTool.Execute(...)` to use the shared helper instead of duplicating payload assembly
- reran the focused helper tests, focused terminal-path regressions, broader targeted `internal/chain`/`cmd`/`spawn` tests, then `make test` and `make build`

### 19. Terminal closure sequencing is now shared instead of duplicated per caller
Files:
- `internal/chain/control.go`
- `internal/chain/control_test.go`
- `cmd/sirtopham/chain.go`
- `cmd/yard/chain.go`
- `internal/spawn/chain_complete.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- reproduced the next remaining control-plane duplication: even after sharing terminal payload assembly, requested-stop finalization, errored-run closure, and `chain_complete` still each hand-managed the higher-level sequence of loading events, writing terminal status, and then logging the terminal event
- added failing tests first for a shared higher-level helper in `internal/chain/control_test.go`; focused `go test` initially failed with `undefined: ApplyTerminalChainClosure` / `undefined: TerminalChainClosure`
- added shared `TerminalChainClosure` plus `ApplyTerminalChainClosure(...)` in `internal/chain/control.go`; it now centralizes `ListEvents -> SetChainStatus/CompleteChain -> BuildTerminalEventPayload -> LogEvent`
- rewired `finalizeRequestedChainStatus(...)`, `finalizeYardRequestedChainStatus(...)`, `closeErroredChainExecution(...)`, `closeErroredYardChainExecution(...)`, and `spawn.ChainCompleteTool.Execute(...)` through the shared higher-level helper
- kept the errored-run callers’ explicit active-execution guard, so that path still no-ops when there is no active execution left to close
- reran the focused helper tests, focused terminal-path regressions, broader targeted `internal/chain`/`cmd`/`spawn` tests, then `make test` and `make build`

### 21. Phase 1.1 blocked in-flight command-flow harness proof
Files:
- `cmd/sirtopham/chain.go`
- `cmd/sirtopham/chain_test.go`
- `cmd/yard/chain.go`
- `cmd/yard/chain_test.go`
- `NEXT_SESSION_HANDOFF.md`

What changed:
- added blocked in-flight command-flow tests that drive the real `runChain(...)` / `yardRunChain(...)` paths with a deterministic fake turn runner instead of only exercising helper entrypoints
- each regression now waits for a real active `execution_id` registration, flips the chain into `cancel_requested` or `pause_requested` while the turn is blocked, then cancels the in-flight context and proves the final command path emits the correct user-facing message, writes the matching terminal chain event, and leaves `LatestActiveExecution(...)` empty
- introduced tiny test seams for runtime/registry/turn-runner construction so the command tests can reach the real command flow without starting external processes
- fixed the concrete bug those regressions exposed: interruption cleanup was still using the cancelled command context, so post-cancel finalization could fail with `get chain: context canceled`; interruption cleanup now switches to `context.WithoutCancel(ctx)` before finalizing requested-stop state and closing any still-active execution
- reran the focused new harness tests, broader `internal/chain` + command/spawn suites, then `make test` and `make build`

---

## Verification status
- multiple focused package test runs for touched areas
- `npx vitest run src/hooks/use-context-report.test.tsx` ✅
- `npx tsc --noEmit` ✅
- `make test` ✅
- `make build` ✅
- live browser rerun against `yard serve --config /tmp/my-website-runtime-8092.yaml` on `http://localhost:8092` ✅

---

## Audit state after this session

The original ranked list lived in a root `AUDIT.md` during the earlier audit pass; that file is not present in the current repo snapshot, so treat the summary below as the historical audit record.

### Substantially addressed in code
- P0 #2 chain CLI missing flags
- P0 #3 WebSocket error payload shape
- P0 #4 brain-disabled behavior
- P0 #5 structural hop expansion looseness
- P1 #6 receipt contract cleanup
- P1 #7 `chain_complete` partial-status collapse

### Improved but not fully closed
- P0 #1 chain pause/resume/cancel semantics
  - resume is real now for durably paused chains
  - pause/cancel requested states now exist (`pause_requested`, `cancel_requested`) so running chains no longer pretend they are already terminal mid-step
  - `spawn_agent` now respects requested-stop states before scheduling another engine
  - when a requested stop finalizes after the current turn exits, the durable event log now records the terminal `chain_paused` / `chain_cancelled` transition instead of changing status silently
  - resume no longer treats `pause_requested` as already-paused, and existing-chain execution prep no longer tries to launch a duplicate orchestrator for a chain that is still `running`
  - cancel still relies on best-effort pid/event-based live signaling rather than a first-class durable command queue or stronger orchestration coordination

### Still clearly open / best next work
Likely next highest-value items now:
1. if you still want deeper control semantics after the daily-driver and observability cleanup, move from pid/event-based best-effort signaling toward a first-class durable command/control surface
2. add stronger end-to-end tests around long-running orchestrator turns so request timing vs. finalization remains proven, not inferred from helper/unit coverage
3. if you keep the current pid-signaling approach for a while, the next narrow follow-through after interruption hardening is a more realistic long-running runChain/yardRunChain harness that drives a blocked in-flight loop and proves pause/cancel request timing against the actual command flow rather than helper entrypoints alone

---

## Recommended next slice

Phase 1.1 from `docs/plans/2026-04-13-sodoryard-stability-closeout-plan.md` is now done and the new blocked in-flight command-flow proof passed cleanly. Do not keep adding more chain-control helpers unless a fresh real-use bug appears.

Next best move:

### Phase 2.1 — repeated real-use runtime soak on the intended setup
- reuse the intended `my-website` runtime documented in the handoff unless reality has changed
- re-run the maintained validation flow plus at least one longer mixed session covering first turn, reload/history, settings/model routing, cancellation, search, and retrieval/context-inspector evidence
- if that run is clean, stop control-plane churn and move on
- if it reproduces one concrete annoyance, take exactly one narrow regression-first bugfix slice next

---

## Specific reading order for the next agent

1. `TECH-DEBT.md`
2. this file
3. if taking the runtime-validation slice: `MANUAL_LIVE_VALIDATION.md` and `docs/v2-b4-brain-retrieval-validation.md`
4. if taking the deeper control-plane slice: `internal/chain/control.go`, `cmd/sirtopham/chain.go`, `cmd/sirtopham/cancel.go`, `cmd/yard/chain.go`, and `internal/spawn/spawn_agent.go`

---

## Commands to use

Preferred:
```bash
make test
make build
```

Useful focused commands:
```bash
CGO_ENABLED=1 \
CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" \
LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" \
go test -tags sqlite_fts5 ./cmd/sirtopham ./cmd/yard ./internal/server ./internal/spawn ./internal/codeintel/searcher
```

---

## Constraints / reminders

- Do not push.
- Keep edits narrow.
- Prefer `make test` / `make build`.
- If a doc disagrees with the repo, trust the repo and patch the doc.
- If you touch the audit-follow-through areas again, update this handoff before stopping.

---

## Bottom line

The repo is in a good continuation state:
- frontend typecheck/build, `make test`, and `make build` are green
- the narrow UI/runtime contract cleanup and matching docs reconciliation are landed in the working tree
- the receipt-path contract is now aligned across shared helpers, runtime code, specs, and shipped role prompts
- running-chain stop requests now use explicit requested states before finalizing to `paused` / `cancelled`
- next best move is exact-setup daily-driver validation unless you explicitly want to keep deepening the control plane
