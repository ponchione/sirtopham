# Go simplification sweep

Date: 2026-04-22
Scope: Go code only
Repo root: `/home/gernsback/source/sodoryard`

## Purpose

This document records a wide Go-only simplification sweep across the current codebase. The goal is maintainability cleanup, not correctness triage: identify where the Go code can be simplified by removing duplication, shrinking overloaded files/functions, clarifying boundaries, and reducing change amplification.

This is a breadth-first audit. Findings are ranked by cleanup value and maintenance risk, not by production severity.

## Baseline checked

Architecture/context read:
- `README.md`
- `go.mod`
- package inventory via `rtk go list ./...`

Validation run:
- `make test` ✅
- `go vet -tags sqlite_fts5 ./...` ✅

Important build-context note:
- Plain `rtk go test ./...` and `rtk go vet ./...` fail in this environment because the repo expects `sqlite_fts5`-tagged tests and LanceDB linker flags supplied by the project build/test entrypoints. That is a build-surface/tooling detail, not evidence of broad code instability.

## Repo-wide Go metrics

High-level inventory:
- 381 Go files total
- 231 non-test Go files
- 150 test Go files
- ~43,143 non-test Go LOC
- ~45,467 test Go LOC

Largest non-test files surfaced by the sweep:
- `internal/agent/loop.go` — 1314 lines
- `internal/config/config.go` — 943 lines
- `cmd/yard/chain.go` — 850 lines
- `internal/context/analyzer.go` — 850 lines
- `internal/codeintel/graph/go_analyzer.go` — 756 lines
- `internal/context/retrieval.go` — 737 lines
- `internal/codeintel/graph/python_analyzer.go` — 699 lines

Largest non-test functions surfaced by the sweep:
- `internal/agent/loop.go:295` — `RunTurn` — 574 lines
- `internal/provider/codex/complete.go:72` — `Complete` — 204 lines
- `internal/tool/file_read.go:60` — `Execute` — 170 lines
- `cmd/yard/run.go:96` — `yardRunHeadless` — 170 lines
- `cmd/tidmouth/run.go:116` — `runHeadless` — 167 lines
- `internal/tool/search_text.go:77` — `Execute` — 166 lines
- `internal/tool/file_edit.go:56` — `Execute` — 160 lines
- `internal/index/service.go:70` — `runWithDependencies` — 153 lines
- `internal/tool/file_write.go:54` — `Execute` — 152 lines

## Overall take

The codebase is mostly healthy at a package-boundary level. The dominant simplification issue is not random disorder; it is concentration of complexity in a few orchestration-heavy files and a few intentionally duplicated surfaces that now look expensive to maintain.

The biggest opportunities are:
1. break apart giant orchestration functions
2. remove duplicated command/runtime flows
3. extract shared file-tool safety/helpers
4. factor provider transport/stream scaffolding
5. split catch-all config and command files by responsibility
6. reduce low-value boilerplate where it has become noisy

---

# Priority-ordered findings

## P0.1 — Decompose `internal/agent/loop.go`, especially `RunTurn`

Files:
- `internal/agent/loop.go`

Hotspots:
- `internal/agent/loop.go:295` — `RunTurn`
- `internal/agent/loop.go:897` — `handleTurnCleanup`
- repeated final result construction around lines `540`, `654`, `696`
- repeated iteration message building around lines `502` and `793`

Why this is top priority:
- This is the single biggest orchestration hotspot in the repo.
- It combines too many phases in a single function and is the highest-risk place for change amplification.
- Any work touching iteration behavior, tool execution, cancellation, compression, persistence, or finalization is likely to hit this file.

Observed metrics:
- File size: 1314 lines
- Function count: 28
- Branch density: very high
- `RunTurn`: 574 lines, ~84 branch signals in the sweep

Responsibilities currently mixed inside `RunTurn`:
- request validation
- provider/tool dependency checks
- cancellation wiring
- initial user-message persistence
- turn-context assembly
- model/provider override resolution
- prompt construction
- preflight compression
- provider streaming/retry
- emergency compression fallback
- final text completion path
- tool-call validation
- tool execution
- tool batch fallback/error mapping
- tool-result budgeting
- iteration persistence
- next-iteration message reconstruction
- loop detection and injected nudges
- terminal result creation

Concrete simplification signs:
- repeated `return &TurnResult{...}` construction for multiple success/escape paths
- repeated assistant/tool persistence shape assembly
- mixed concerns: domain flow, infrastructure, cancellation semantics, error mapping, and UI/event emission all sit in one control tower
- tool execution success and tool execution failure paths are parallel enough to share helpers but currently remain inlined

Why it matters:
- Hard to safely modify in narrow slices
- Hard to test changes in isolation without broad regression surface
- Review overhead is high because every branch sits in one function body
- Makes subtle lifecycle bugs harder to spot

Recommended simplification direction:
- Split `RunTurn` into phase helpers with explicit data carriers, for example:
  - `prepareTurn(...)`
  - `buildIterationPrompt(...)`
  - `runProviderIteration(...)`
  - `executeValidatedToolCalls(...)`
  - `persistCompletedIteration(...)`
  - `finalizeTurn(...)`
- Extract one `finalTurnResult(...)` helper to eliminate repeated result construction
- Extract one helper for “apply tool results + emit events + build persistence rows” to unify lines `694-767`
- Keep orchestration ordering in `RunTurn`, but move per-phase mechanics out

Cleanup caution:
- This should stay behavior-preserving. Do not redesign loop semantics during the first pass.
- The safest first slice is extraction-only refactoring with snapshot-preserving tests.

Expected payoff:
- Largest maintainability win in the repo
- Reduced change amplification across agent/runtime work
- Easier targeted testing of turn phases

---

## P0.2 — Unify the duplicated headless run pipeline in `cmd/yard` and `cmd/tidmouth`

Files:
- `cmd/yard/run.go`
- `cmd/tidmouth/run.go`
- `cmd/yard/run_helpers.go`
- `cmd/tidmouth/receipt.go`

Hotspots:
- `cmd/yard/run.go:96` — `yardRunHeadless`
- `cmd/tidmouth/run.go:116` — `runHeadless`

Similarity evidence:
- Direct similarity check between `cmd/yard/run.go` and `cmd/tidmouth/run.go` came out to ~0.87
- The flow is functionally the same with mostly naming/test-seam differences

Shared flow currently duplicated:
- validate flags
- read task or task-file
- load config and apply overrides
- validate config
- load role prompt
- resolve/create chain ID
- build runtime
- build role registry
- build tool executor + adapter
- build event sink
- resolve max-turns/max-tokens
- construct title generator and agent loop
- create conversation
- resolve model context limit
- execute first turn
- map timeout/safety-limit conditions
- ensure receipt / derive exit code

Why it matters:
- Behavior changes must often be mirrored in two binaries
- Drift risk is high over time, especially around safety-limit handling, receipt behavior, and loop construction
- Tests in one command may not guarantee parity with the other command

The duplication is visible even in helper surfaces:
- `cmd/yard/run_helpers.go` wraps headless helpers under `yard*` names
- `cmd/tidmouth/receipt.go` wraps the same helpers under local names

Recommended simplification direction:
- Move the shared run pipeline into a common package, likely `internal/headless` or `internal/cmdutil`
- Preserve binary-specific behavior as thin adapters only:
  - command name/help text
  - dependency injection for tests
  - any intentionally different output surface
- The common package should own:
  - run input validation
  - runtime construction orchestration
  - conversation + turn execution
  - receipt finalization
  - exit code mapping

Good end state:
- `cmd/yard/run.go` and `cmd/tidmouth/run.go` become shallow wrappers over a shared `RunSession(...)`
- helper aliases/wrappers collapse substantially

Expected payoff:
- High-value duplication removal
- Lower drift risk between public/internal binaries
- Easier future changes to headless-run semantics

---

## P0.3 — Extract shared file-operation safety/helpers from `file_read`, `file_write`, and `file_edit`

Files:
- `internal/tool/file_read.go`
- `internal/tool/file_write.go`
- `internal/tool/file_edit.go`

Hotspots:
- `internal/tool/file_read.go:60` — `Execute`
- `internal/tool/file_write.go:54` — `Execute`
- `internal/tool/file_edit.go:56` — `Execute`

Observed metrics:
- `file_read.Execute` — 170 lines
- `file_write.Execute` — 152 lines
- `file_edit.Execute` — 160 lines

Pattern evidence from search/count sweep:
- repeated boilerplate phrases across the three files for invalid input, read errors, not-found handling, write failures, stale-read checks

Shared concerns currently repeated:
- JSON input unmarshalling and malformed-input result shaping
- path resolution relative to project root
- not-found error shaping
- read-state store setup and scope resolution
- stale-read / read-before-write protection
- file stat/read error formatting
- diff/result formatting
- atomic write or safe overwrite mechanics

Why it matters:
- The safety model is good, but it is implemented repeatedly in per-tool handlers
- Repetition increases the chance that future behavior tweaks diverge across file tools
- Makes the core policy of each tool harder to see

Recommended simplification direction:
- Introduce a shared helper layer for file operations, for example:
  - `resolveFileTarget(...)`
  - `loadExistingFile(...)`
  - `requireFreshFullRead(...)`
  - `writeAtomicallyPreserveMode(...)`
  - `newToolInputError(...)`
  - `newToolPathError(...)`
  - standardized file not-found/read/write result constructors
- Keep each tool focused on its actual policy:
  - `file_read`: select/format lines and snapshot state
  - `file_write`: replace full contents safely
  - `file_edit`: verify unique match and replace

Cleanup caution:
- Preserve the existing safety semantics exactly; this area is more important to keep correct than to make clever

Expected payoff:
- High duplication reduction
- Cleaner tool code with safety logic centralized
- Easier future additions like append/patch/file-move semantics

---

## P1.1 — Split `cmd/yard/chain.go` by responsibility

Files:
- `cmd/yard/chain.go`

Hotspots:
- `cmd/yard/chain.go:119` — `yardRunChain`
- event formatting/rendering near the bottom of the file
- watch/control/status helpers throughout the file

Observed metrics:
- File size: 850 lines
- Function count: 35
- Highest branch density among non-test Go files surfaced in the sweep

Current responsibilities mixed together:
- command wiring
- chain start/resume flow
- execution registration/cleanup
- watch lifecycle management
- interruption handling
- status setting
- event formatting/rendering
- helper utilities for existing chains / flags / receipts / watch waiting

Why it matters:
- One file holds too much of the CLI chain surface
- Changes to start/resume behavior are mixed with formatting/rendering concerns
- Harder to reason about what is command construction vs. actual runtime behavior

Recommended simplification direction:
Split by responsibility, not by arbitrary line count:
- `chain_start.go`
- `chain_watch.go`
- `chain_status.go`
- `chain_render.go`
- `chain_control.go`

Particularly worth extracting:
- `yardRunChain` execution flow into its own file or shared helper module
- event formatting helpers (`formatKnownChainEvent` etc.) into a rendering-focused file
- watch handle logic into a dedicated watch file

Expected payoff:
- Easier navigation of chain command behavior
- Lower cognitive load for future chain feature work

---

## P1.2 — Split `internal/config/config.go` into concern-oriented files

Files:
- `internal/config/config.go`

Observed metrics:
- File size: 943 lines
- Function count: 36

Current concerns mixed together:
- config types/structs
- default values
- YAML load logic
- configured-provider discovery from YAML
- environment overrides
- normalization
- local-services defaults and path resolution
- path validation
- provider/routing validation
- numeric/format validation

Hotspots read during sweep:
- `Default()` around `220-339`
- `Load()` around `341-365`
- env overrides around `398+`
- `normalize()` / `normalizeLocalServices()` around `572-641`
- validation cluster around `688+`

Why it matters:
- This is a classic catch-all config file
- The file already has enough surface area that finding the right place to add a change takes longer than it should
- Local service config has grown into a subsystem but still lives inline with unrelated concerns

Recommended simplification direction:
Split into files such as:
- `config_types.go`
- `config_defaults.go`
- `config_load.go`
- `config_env.go`
- `config_localservices.go`
- `config_validate.go`

Keep public API stable:
- `Default()`
- `Load()`
- `Validate()`
- `ApplyEnvOverrides()`

Expected payoff:
- Better navigation
- Less merge conflict pressure in config work
- Easier to extend provider/local-service config cleanly

---

## P1.3 — Extract shared provider transport / SSE / retry scaffolding

Files:
- `internal/provider/anthropic/stream.go`
- `internal/provider/codex/stream.go`
- `internal/provider/openai/stream.go`
- `internal/provider/codex/complete.go`
- `internal/provider/openai/complete.go`

Why this is not P0:
- The protocols are genuinely different, so there is a real risk of over-abstraction
- The simplification opportunity is at the transport/scaffolding layer, not at the protocol-decoding layer

What repeats today:
- HTTP request construction patterns
- context cancellation checks before/during stream processing
- send-on-channel helpers for stream events
- SSE scanner setup and line loops
- parse-error emission boilerplate
- retry loops and retryable status handling in complete paths

Concrete evidence:
- Anthropic stream parse-error branches around lines `179`, `200`, `223`, `255`, `274`
- Codex stream parse-error branches around lines `238`, `249`, `260`, `280`, `298`, `318`
- OpenAI and Codex complete paths both manage request/retry/response plumbing with similar control structure

Recommended simplification direction:
Do not unify provider-specific decoding logic.

Do extract shared helpers for:
- channel send with cancel-awareness
- SSE scanner/buffer setup
- common stream-read cancellation/error emission
- generic retry wrappers for request attempts
- standardized retryable HTTP status mapping where it fits

Potential home:
- `internal/provider/streamutil`
- or a small shared helper file under `internal/provider`

Expected payoff:
- Moderate-to-high reduction in repeated plumbing
- Cleaner provider implementations without flattening real protocol differences

---

## P1.4 — Reduce repeated result/persistence assembly inside `internal/agent/loop.go`

Files:
- `internal/agent/loop.go`

This is related to P0.1 but worth tracking separately because it can be a narrower first slice.

Specific repeated patterns surfaced:
- repeated `TurnResult` creation around `540`, `654`, `696`
- repeated `persistMessages := []conversation.IterationMessage{...}` assembly around `502` and `793`
- repeated “tool result + event emission + persistence row” logic for error and success cases in the batch execution block

Why it matters:
- Good candidate for a narrow cleanup before deeper function decomposition
- Lets you reduce body size and branch noise without changing the outer control flow much

Recommended simplification direction:
- extract a single `buildTurnResult(...)`
- extract helper(s) for assistant/tool persistence rows
- extract helper(s) for applying batch results to inflight/persisted/event state

Expected payoff:
- Smaller immediate cleanup that paves the way for the full `RunTurn` refactor

---

## P2.1 — Revisit overlap between domain brain search and tool formatting layers

Files:
- `internal/context/brain_search.go`
- `internal/tool/brain_search.go`

Why it is lower priority:
- The layering is not obviously wrong
- Some duplication may be justified because one layer is domain/runtime search and the other is tool UX/presentation

Observed shape:
- runtime search logic lives in `internal/context/brain_search.go`
- tool-level fallback/formatting/filtering lives in `internal/tool/brain_search.go`
- there is some overlap in concerns like result filtering, mode handling, and formatting responsibility boundaries

Why it might matter later:
- As brain search grows, the boundary between “retrieve results” and “present tool output” could get muddier
- This area may become a maintenance burden if more search modes or result annotations are added

Recommendation:
- Do not refactor this now unless already working in the area
- If touched later, keep runtime search/domain result shaping separate from tool-output formatting and query-log side effects

Expected payoff:
- Moderate, but only if the surface continues to grow

---

## P2.2 — Trim event boilerplate in `internal/agent/events.go`

Files:
- `internal/agent/events.go`

Observed noise:
- 12 separate `Timestamp() time.Time { return e.Time }` methods
- matching repetitive `EventType()` methods across every event type

Why it matters less:
- This is mostly boilerplate noise, not structural risk
- The explicitness is easy to read, even if repetitive

Possible simplification directions:
- embed a small base event struct with common methods
- use lightweight code generation if explicit event types are desired
- or leave as-is if minimizing indirection matters more than file size

Recommendation:
- Low urgency
- Only worth doing if already touching the event system or if codegen/base-struct patterns exist elsewhere

Expected payoff:
- Small readability cleanup, low strategic impact

---

# Non-priority observations / things not to overreact to

## Generated DB code is large but not a simplification target

Files like:
- `internal/db/*.sql.go`

These are large, but generated. They should not drive cleanup priorities unless the generation approach itself changes.

## External/vendor-like Go code surfaced in package listing

Example:
- `web/node_modules/flatted/golang/pkg/flatted`

This appears in Go package inventory/metrics but should not drive first-party simplification work.

## Large files are not automatically smells

Examples such as:
- `internal/context/analyzer.go`
- `internal/context/retrieval.go`
- `internal/codeintel/graph/go_analyzer.go`

These are worth watching, but the sweep did not find them to be stronger simplification candidates than the orchestration and duplication hotspots above.

---

# Healthy signals from the sweep

The repo is not broadly disorganized. Positive signs:
- package boundaries are mostly real and legible
- tests are broad, and project-native `make test` is green
- CLI/business/runtime layers are generally separated, even where some command files are oversized
- tool safety posture is strong, especially around file mutation
- provider abstractions are coherent despite duplicated plumbing

This matters because the cleanup should stay targeted. The goal is not to rewrite the architecture; it is to simplify the highest-friction Go surfaces.

---

# Suggested execution order

If cleanup work starts from this doc, the recommended order is:

1. `internal/agent/loop.go` decomposition
2. shared headless run pipeline for `cmd/yard` + `cmd/tidmouth`
3. shared file-operation safety/helpers for `file_read` / `file_write` / `file_edit`
4. split `cmd/yard/chain.go` by responsibility
5. split `internal/config/config.go` by concern
6. extract provider transport / stream / retry scaffolding
7. trim event boilerplate only if already in the area

---

# Short verdict

The main Go simplification opportunities are concentrated and actionable:
- one dominant orchestration hotspot (`internal/agent/loop.go`)
- one major duplicated runtime path (`cmd/yard/run.go` vs `cmd/tidmouth/run.go`)
- one strong cluster of repeated file-tool safety code
- a few oversized catch-all files that should be split by responsibility

This is a good cleanup situation: the codebase appears structurally sound enough that targeted simplification should pay off without requiring architectural churn.
