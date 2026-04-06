# TECH-DEBT

Open issues that should be fixed in a later focused session or need closer investigation.

**Last sweep:** 2026-04-03


## Layer 3 — Context Assembly

### Budget priority chain omits brain docs (v0.2 scope)

**Severity:** Info | **Source:** Layer 3 audit (2026-04-01)

The audit checklist listed the budget priority order as:
  explicit files > **brain docs** > top RAG > graph > conventions > git > lower RAG

The epic spec (`docs/layer3/05-budget-manager-serialization/epic-05-budget-manager-serialization.md`)
explicitly defers brain docs to v0.2 and lists 6 priority tiers without brain.
The implementation matches the v0.1 spec exactly.

**Fix direction:** When v0.2 proactive brain retrieval lands, add a `BrainHit`
priority tier in `budget.go`'s `Fit()` method between explicit files and top RAG
hits. The `BrainHit` type already exists in `types.go` and `RetrievalResults`
already has a `BrainHits` field — only the budget allocation logic needs updating.


## Layer 4 — Tool System

### Executor.Execute signature differs from spec
**Severity:** Info | **Source:** Layer 4 audit (2026-04-01)

The spec defines `Execute(ctx, projectRoot, conversationID, turnNumber, iteration,
calls) []ToolResult`. The implementation splits this into `Execute(ctx, calls)` and
`ExecuteWithMeta(ctx, calls, meta)`, with `projectRoot` on `ExecutorConfig`.

All data reaches the same destination. The refactored design is arguably cleaner
(separating per-executor config from per-call metadata). **No change needed — spec
should be updated to reflect the cleaner design.**

---

### Tool interface method named ToolPurity() instead of Purity()
**Severity:** Info | **Source:** Layer 4 audit (2026-04-01)

The spec defines the method as `Purity() Purity`. The implementation uses
`ToolPurity() Purity` to avoid the type/method name collision in Go.

**No change needed — intentional Go idiom. Spec should use `ToolPurity()`.**


## Layer 5 — Agent Loop

### Provider fallback not implemented
**Severity:** Low | **Source:** Layer 5 audit (2026-04-01)

The spec mentions "optionally fall back to configured fallback provider" when retries
are exhausted. The router already implements fallback in `handleCompleteError` and
`handleStreamError` for retriable errors. The agent loop's `streamWithRetry` does not
trigger a separate fallback — it relies on the router's built-in fallback mechanism.

**Status:** The router-level fallback covers most cases. Agent-level fallback (e.g.,
rebuilding the prompt with a different model) would require `FallbackModel` on
`AgentLoopConfig`. Deferred — low practical impact since the router handles it.

---

### Iteration analytics persistence is still non-atomic relative to messages
**Severity:** Low | **Source:** Layer 5 audit (2026-04-01), revisited 2026-04-01

The current contract is now explicit: `PersistIteration` is atomic for `messages`
rows only. `tool_executions` and `sub_calls` are persisted on separate best-effort
paths (`tool.Executor` and `provider/tracking.TrackedProvider`) and may be missing if
an analytics write fails after message persistence succeeds.

This is currently tolerated because:
- the user-visible source of truth is the `messages` table
- cancellation cleanup now prefers durable tombstones for materialized assistant/tool state, skips untouched iterations, and only falls back to raw iteration cancellation when there is no better transcript-preserving record to persist
- missing analytics rows are recoverable and far lower severity than losing the
  canonical conversation history

**Future fix direction:** If stronger guarantees become necessary, extend the
iteration persistence contract so the agent loop can hand `PersistIteration`
optional tool-execution and sub-call payloads and commit all three record classes in a
single transaction.

---

### Interrupted assistant/tool tombstones still reuse existing message schemas
**Severity:** Low | **Source:** Claude-handoff cancellation cleanup follow-up (2026-04-01)

Cancellation cleanup now persists two distinct durable markers inside existing message content:
- assistant tombstones: `[interrupted_assistant]` or `[failed_assistant]`
- tool tombstones: `[interrupted_tool_result]`

This is good enough to preserve transcript truthfulness today, but it still has follow-up debt:
- no first-class DB/message type distinguishes tombstones from ordinary assistant/tool payloads
- the main web transcript and conversation search snippets now render tombstones human-readably, and title generation now refuses tombstone-like outputs, but any future transcript export/share/derivation surfaces may still need explicit rules for these markers

**Future fix direction:** If interrupted-state UX or analytics become important, introduce a
first-class durable representation (schema field, content-block type, or explicit metadata)
for interrupted assistant/tool records and teach remaining transcript consumers to render,
filter, or down-rank them intentionally.

---

### Remaining Claude Code retrofit items are still intentionally deferred
**Severity:** Info | **Source:** NEXT_SESSION_HANDOFF / Claude retrofit reconciliation (2026-04-01)

The highest-value Claude-handoff slices are no longer the immediate blocker for early runtime testing, but several architecture items remain intentionally incomplete:
- prompt-cache latching is still absent as an explicit stable-vs-dynamic prompt-byte subsystem
- token-budget accounting still lacks a `BudgetTracker`-style reserve/estimate/reconcile flow
- tool-output handling still lives in loop-adjacent helpers rather than a dedicated `ToolOutputManager` package boundary
- shell/build/test tail-preserving formatting is only partially embodied, not a first-class formatter subsystem
- `file_write` still does not participate in the read-state/stale-write safety model used for `file_edit`
- cancellation cleanup still uses existing message/content schemas rather than first-class interrupted record types

**Future fix direction:** Resume these only after the concrete bring-up blockers are solved. If/when Claude-retrofit work resumes, the best remaining order is: prompt-cache latching, better token-budget accounting, tool-output subsystem cleanup, then any broader mutation-safety follow-through for `file_write`.

---

### Executor.Execute signature differs from spec (agent loop interface)
**Severity:** Info | **Source:** Layer 5 audit (2026-04-01)

The agent loop's `ToolExecutor` interface uses `Execute(ctx, call) (*ToolResult, error)`
(single call). The batch dispatch logic lives in the agent loop itself. **No change
needed — documented for spec reconciliation.**


## Layer 6 — Web Interface & Streaming

### `search_semantic` should stay deferred until programmatic retrieval is proven end to end
**Severity:** Info | **Source:** RAG indexing/retrieval planning review (2026-04-02)

The intended architecture is that indexing and retrieval/context assembly are backend/programmatic responsibilities, not agent-orchestrated maintenance behavior. `search_semantic` already exists as a tool surface, but the next slice should focus on making the real indexing pipeline, semantic store wiring, and automatic context assembly work first.

**Future fix direction:** Do not spend the next slice wiring or polishing `search_semantic` as part of the critical path. First prove: real `sirtopham index`, semantic store/searcher construction in `serve`, and context assembly consuming indexed retrieval programmatically. After that, revisit whether `search_semantic` should remain as a read-only diagnostic/power-user tool or be removed/deprioritized entirely.

---

### Conversation list page is a landing page, not a dedicated list view
**Severity:** Info | **Source:** Layer 6 audit (2026-04-01)

The spec mentions a conversation list page at `/`. The implementation uses root as a
landing page with quick-start input; the actual list lives in the sidebar. **No change
needed — reasonable UX choice. Documented for spec reconciliation.**


## Cross-Cutting Codebase Audit

**Sweep date:** 2026-04-01 | **Scope:** All 80+ production .go files (244 total incl. tests)
**Method:** Three parallel audit streams covering agent+context, tool, and all remaining packages.

---

### P1 — Fix This Sprint

#### 13. goparser vs go_analyzer — massive duplication (~1200 LOC)
**Severity:** High | **Files:** `internal/codeintel/goparser/goparser.go` + `internal/codeintel/graph/go_analyzer.go`

Both load packages, walk AST, extract symbols/calls, check implements. ~470 LOC +
~750 LOC doing overlapping work. Consolidate into a single package.

---

### P2 — Fix Soon

#### 14. N+1 delete pattern in vectorstore
**File:** `internal/vectorstore/store.go:101-124`

`Upsert()` deletes chunks one-by-one in a loop before batch insert. Should batch
deletes into a single filter expression.

---

#### 15. O(N*M) reverse call graph
**File:** `internal/codeintel/indexer/indexer.go:218-258`

Inner loop iterates ALL directories for each chunk with calls. Quadratic on large
codebases.

---

#### 19. O(n²) in markIncluded/markExcluded
**File:** `internal/context/budget.go:294-311`

Linear scan slices for dedup. Use a map-backed set for large chunk sets.

---

#### 22. Stub "index" and "config" commands
**File:** `cmd/sirtopham/main.go:53-59`

Print "not yet implemented" and return nil. Dead weight in binary. Remove or wire up.

---

#### 25. Unused exported types in agent/types.go
**File:** `internal/agent/types.go:28-64`

`Session`, `Turn`, `Iteration`, `ToolCallRecord` — exported types not constructed or
referenced in production code. `TurnInProgress` constant also unused.

---

#### 27. Empty package: internal/index/
**File:** `internal/index/doc.go`

No production code exists. Remove or add a TODO explaining intent.

---

#### 29. Unused provider types
**File:** `internal/provider/types.go:21-32`

`ToolCall` and `ToolResult` types are defined but never referenced. `NewProviderError`
(lines 88-103) also never called. Remove.

---

#### 30. BrainHit type always empty
**File:** `internal/context/types.go:52-65`

Every `BrainHits` field is always empty. Brain retrieval is "deferred until v0.2."
Dead weight in serialization paths. (See also existing item above about brain docs.)

---

#### 31. Triple-implemented retry logic
**Files:** `anthropic/retry.go`, `openai/complete.go`, `codex/complete.go`

Each has slightly different backoff/retry behavior. Extract a shared
`internal/provider/retry` package.

---

### P2 — Missing Error Handling

#### 46. Two SQLite drivers in binary
**Files:** `internal/codeintel/graph/store.go` (modernc.org/sqlite) + main DB (mattn/go-sqlite3)

Having both CGO and pure-Go SQLite drivers in the same binary doubles bloat. Pick one.

---

### P3 — Idiomatic Go / Cleanup

#### 48. Nil context accepted and defaulted to Background()
**Files:** `agent/loop.go:277`, `context/assembler.go:61`, `compression.go:97`, `report_store.go:39`

Go convention: never pass nil context. Remove the guards.

---

#### 49. Mixed clock sources in assembler
**File:** `internal/context/assembler.go:68-69`

Uses `a.now()` for total latency but `time.Now()` for sub-timings. Tests can't
control sub-measurements. Use `a.now()` consistently.

---

#### 50. Redundant ServerPort/ServerHost config fields
**File:** `internal/config/config.go:28-29`

Duplicated with `Server.Port` / `Server.Host`. `normalize()` syncs bidirectionally.
Maintenance hazard. Remove the top-level fields.

---

#### 51. math/rand instead of math/rand/v2
**Files:** `internal/provider/anthropic/retry.go:8`, `openai/complete.go:10`

Deprecated global source. Use `math/rand/v2`.

---

#### 52. Custom errorAs reimplements errors.As
**File:** `internal/brain/client.go:260-291`

Use `errors.As()` from stdlib.

---

#### 53. Direct type assertion instead of errors.As
**File:** `internal/provider/codex/credentials.go:169`

Uses `err.(*exec.ExitError)` instead of `errors.As()`.

---

### P3 — Security Hardening

#### 55. InsecureSkipVerify always on for WebSocket
**File:** `internal/server/websocket.go:96`

Not gated by any dev-mode flag. Accepts connections from any origin.

---

#### 56. Git ref injection
**File:** `internal/tool/git_diff.go:82-87`

`ref1`/`ref2` passed directly to git without sanitization. Refs starting with `-`
could inject flags. Reject refs starting with `-` or use `--` separator.

---

#### 57. Shell denylist bypass via whitespace/quoting
**File:** `internal/tool/shell.go:90-98`

`strings.Contains` matching is trivially bypassable. Defense-in-depth layer but
worth hardening.

---

#### 58. Incomplete LanceDB filter escaping
**File:** `internal/vectorstore/store.go:107`

Only escapes single quotes. Other injection vectors may exist in LanceDB filter
syntax.

---


