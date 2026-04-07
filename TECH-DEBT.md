# TECH-DEBT

Open issues that should be fixed in a later focused session or need closer investigation.

**Last sweep:** 2026-04-07


## Local LLM Stack Status (2026-04-07)

The repo now owns the local llama.cpp stack under `ops/llm/`.

What is done:
- repo-owned compose file at `ops/llm/docker-compose.yml`
- repo-local model directory at `ops/llm/models/`
- real GGUF copies are present in the repo-local models dir (not symlinks)
- default ports changed from 8080/8081 to 12434/12435 to avoid common-dev-port conflicts
- `local_services` config block exists and is wired through config/init/config CLI
- `sirtopham llm status/up/down/logs` exists and was live-validated
- index precheck now goes through `internal/localservices` and `make test` no longer depends on real localhost services

What remains worth cleaning up later:
- If multiple repos will own separate local stacks on the same machine, container names (`qwen-coder-server`, `nomic-embed-server`) are still global and can conflict. Fine for a single shared stack; not ideal for multi-stack isolation.
- `llm up` currently surfaces raw Docker conflict text when stale old containers with the same names already exist. Correct and diagnosable, but the UX could become friendlier by auto-detecting/stating that these are stale-name conflicts before compose-up.
- `llm status` always prints remediation lines, even when services are healthy. Harmless, but noisy.

This is no longer a core architecture blocker. The old workstation-local `~/LLM/stacks/docker-compose.yml` dependency is superseded by repo-owned `ops/llm/docker-compose.yml` for this repo.

---

## Harness Readiness Gaps (2026-04-06)

Assessment done against HARNESS_COMPLETION_PUNCHLIST.md on 2026-04-06. The punchlist is effectively cleared (P0 1-4, P1 5-8, P2 9/10/11 all landed, plus item-13 polish pass). The gaps below are what still separates "punchlist complete" from "trustable as a daily-driver harness." Ordered by how much they would bite in real use.

### H1. Fresh-session end-to-end runtime validation never happened
**Severity:** High | **Source:** NEXT_SESSION_HANDOFF.md (2026-04-06)

`NEXT_SESSION_HANDOFF.md` explicitly says the next action is a fresh-session browser/runtime validation pass over the harness flows (historical inspector nav, manual-vs-live follow, metrics auto-refresh, inspector persistence across SPA nav, settings default model dropdown, conversation model override). Until that pass runs clean on a real project, the "complete" claim is paper-only.

**Fix direction:** Run the validation pass against a small non-sirtopham test repo (e.g. `~/source/my-website/`), document findings, fix regressions, then re-call completeness.

---

### H2. Real indexing + programmatic retrieval is not proven end to end
**Severity:** High | **Source:** Harness-readiness assessment (2026-04-06)

`sirtopham index` is no longer a stub (it calls `internal/index.Run`), and `search_semantic` exists as a tool, but there is no recorded evidence that:
- `sirtopham index` runs to completion against a real fresh project
- The semantic store is actually constructed inside `serve` on that project
- Context assembly is consuming indexed retrieval results in the agent loop
- The inspector reflects real (non-empty) RAG results during a turn

Without that proof, a big chunk of sirtopham's reason-for-existing is dark. Every inspector polish item assumes retrieval actually fires.

**Fix direction:** Part of the H1 validation pass. Explicitly verify: (1) `sirtopham index` exits 0 on my-website, (2) `.<project>/lancedb/code/` has real data after, (3) serve logs show a searcher was wired, (4) a live turn shows non-empty RAG results in the inspector.

---

### H3. `file_write` has no stale-write safety model
**Severity:** Medium | **Source:** TECH-DEBT Layer 5 retrofit section, re-flagged 2026-04-06

`file_edit` participates in a read-state store (`memoryReadStateStore`) so edits that don't match current contents are rejected as stale. `file_write` does not. An agent can blow away a file it never read, in a state it doesn't know about. Low frequency, high blast radius — exactly the class of bug that silently eats work.

**Fix direction:** Route `file_write` through the same read-state model or, at minimum, require a recent read of the target path before a write that would overwrite existing non-empty content. Unconditional writes to new paths should stay allowed.

---

### H4. Brain retrieval is deferred to v0.2 — only manual use works today
**Severity:** Medium | **Source:** Layer 3 tech debt, re-flagged 2026-04-06

Brain tools work for explicit `brain_read`/`brain_write`/`brain_search`/`brain_update`/`brain_lint`, but automatic brain-hit inclusion in context assembly is not wired. `RetrievalResults.BrainHits` is always empty in the budget priority chain. The agent has to remember to call brain tools itself. For daily-driver harness use this means brain context isn't "free" — it's gated on the agent remembering to ask.

**Fix direction:** Already documented under Layer 3. Adding a `BrainHit` priority tier in `budget.go`'s `Fit()` and wiring the brain backend into context assembly unlocks the v0.2 experience. Low-risk once the retrieval side exists.

---

### H5. Prompt-cache latching is absent
**Severity:** Medium (cost, not correctness) | **Source:** TECH-DEBT Layer 5 retrofit list

No explicit stable-vs-dynamic prompt byte subsystem. For Anthropic especially, this means every turn pays for prompt tokens that could be cached. Full-time harness use → continuous cost leak. Not a correctness blocker, but it gets expensive fast at any real duty cycle.

**Fix direction:** Introduce a prompt-cache latching subsystem that separates stable prompt prefix (system prompt, project conventions, explicit files that haven't changed) from dynamic suffix (latest retrieval, live messages). Wire Anthropic `cache_control` on the stable segments.

---

### H6. Token-budget accounting lacks a reserve/estimate/reconcile tracker
**Severity:** Medium | **Source:** TECH-DEBT Layer 5 retrofit list

Budgeting works but is not tight. There's no dedicated `BudgetTracker`-style reserve→estimate→reconcile flow. Real harness use with long turns + large tool outputs risks silently blowing the context window or under-filling it.

**Fix direction:** Introduce a `BudgetTracker` that reserves output headroom, estimates per-tool-call cost pre-dispatch, and reconciles against actual token usage post-response. Surface discrepancies in the inspector.

---

### H7. `sirtopham index` result feedback not yet verified on a real JS/TS project
**Severity:** Low (but unknown) | **Source:** Harness-readiness assessment (2026-04-06)

The default `index.include` from `init.go` includes `*.go`, `*.py`, `*.ts`, `*.tsx`, `*.js`, `*.jsx`, `*.sql`, `*.md`, `*.yaml`, `*.yml`, `*.json`. Good coverage. But there is no test record of indexing a non-Go project and confirming chunking/embedding/storage all work end-to-end. The goparser/graph paths may or may not gracefully no-op on TS.

**Fix direction:** Part of H1/H2 validation. Confirm `sirtopham index` on my-website produces chunks for `.ts`/`.tsx`/`.md` files and doesn't error on missing Go AST.

---

### H8. Security polish only matters past single-user localhost
**Severity:** Low (for local single-user) / High (for any networked deploy) | **Source:** Cross-cutting audit (2026-04-01)

Not blockers for daily-driver harness use on localhost single-user, but would flunk any deployment review:
- `websocket.go` `InsecureSkipVerify` always on, no dev-mode gate (item 55)
- Shell denylist uses `strings.Contains` — trivially bypassable via whitespace/quoting (item 57)
- Git ref injection in `git_diff.go` (item 56)
- Incomplete LanceDB filter escaping (item 58)

**Fix direction:** Gate `InsecureSkipVerify` behind `server.dev_mode`, switch shell denylist to token-based matching with a shared quote-aware parser, validate refs against `^[A-Za-z0-9][A-Za-z0-9._/-]*$` or use `--` separator, audit LanceDB filter surface.

---

### Status note
The punchlist was about "complete vs spec." This block is about "trustable vs daily use." H1 and H2 are the only ones that block calling the harness ready-to-drive; H3-H8 are incremental improvements that should happen during real use, not before it.

---

## Validation Run Findings (2026-04-06)

Bugs found during the fresh-session runtime validation pass (H1) against `~/source/my-website/` — a Vite/React/TS/Tailwind project. Numbered B1-B7 and referenced in future work.

**Status summary as of 2026-04-06:**
- B1, B2, B3, B5 — Fixed in this session. B1/B2/B3 still need live browser re-validation (the validation run was preempted before reaching the verify step, but the Go + frontend builds pass cleanly).
- B4 — Open. Root cause fully identified during B3 investigation (see below) but fix deferred. Two-sided bug: backend returns a raw token count under a `_pct` column name, frontend multiplies that by 100 again on render. Hidden from casual use because the metrics panel is collapsed by default.
- B6, B7 — Open. Low severity, cosmetic / investigatory.

### B1. Live context_debug events do NOT update the inspector
**Severity:** High | **Source:** Validation pass 2026-04-06 | **Status:** Fixed (2026-04-06, needs live browser re-validation)

After a second turn completes live on a visible conversation, the inspector stayed stuck on the previous turn:
- `Turn 1 of 1` still displayed after turn 2 completes
- Code Chunks still shows turn-1 data
- DB has the turn-2 report; `/api/metrics/conversation/{id}/context/2` returns 200 with full data
- On full page reload, `Turn 2 of 2` displays correctly — so the problem was confined to the live-event path

**Actual root cause (confirmed via Python `websockets` wire sniffer against the running server):**
The WebSocket delivered `context_debug` correctly — turn 1 and turn 2 both arrived on the wire with `report.turn_number` set. The bug was in the frontend's `use-websocket.ts` hook: it stored the latest event as a single `useState` value (`lastEvent`). React 18's automatic batching means that when multiple WebSocket frames arrive in rapid succession (as happens at the start of every turn: `status:assembling_context`, `context_debug`, `status:waiting_for_llm` all within ~1ms), only the LAST `setLastEvent` call wins, and the consumer `useEffect([lastEvent])` fires once with the final frame — silently dropping every earlier frame in the batch. `context_debug` is the middle frame of the burst, so it was consistently lost.

**Fix applied (2026-04-06):**
- `web/src/hooks/use-websocket.ts`: replaced the single `lastEvent` state with a ref-backed append-only queue (`eventQueue: MutableRefObject<ServerEvent[]>`) plus an `eventTick` counter that bumps on each frame. Ref writes are synchronous and not subject to batching, so every frame is preserved in arrival order.
- `web/src/hooks/use-conversation.ts`: switched the dispatch `useEffect` from watching `lastEvent` to draining the queue on every `eventTick` bump, using a `processedEventsRef` cursor so we never reprocess events. The entire switch-on-`msg.type` body was preserved verbatim — only the enclosing loop changed.
- Build passes (`make build` clean, tsc passes, vite bundles).

**Still TODO:** Browser validation of the fix end-to-end against my-website was preempted before completion. Re-run the validation flow (create conversation, send two turns, confirm inspector shows `Turn 2 of 2` live without a reload) in a future session.

---

### B2. Sidebar conversations list does not auto-refresh on new conversation creation
**Severity:** Medium | **Source:** Validation pass 2026-04-06 | **Status:** Fixed (2026-04-06, needs live browser re-validation)

Creating a new conversation from the landing page:
- User sends first message, new conversation is created and navigated to
- Sidebar continued to show "No conversations yet"
- Full page reload → conversation appears with auto-generated title

**Root cause:** `useConversationList` fetched `/api/conversations?limit=50` once on mount and had no mechanism to refetch. The sidebar component never triggered a refresh when a new conversation was created in the same SPA session.

**Fix applied (2026-04-06):**
- `web/src/components/layout/sidebar.tsx`: added a `useEffect` that watches `activeId` (derived from the URL). When `activeId` changes to an id that is not currently in the `conversations` list, the sidebar calls `refresh()` once. A ref-backed `Set<string>` guards against repeat refresh attempts for truly missing ids, and the effect bails if a fetch is already in flight (`loading`). This also serves as a general fallback for any situation where the sidebar list drifts out of sync with the server state.

**Still TODO:** Browser validation preempted. Re-run: create a new conversation from the landing page and confirm it appears in the sidebar immediately without a reload.

---

### B3. Per-conversation metrics chip disappears on reload
**Severity:** Medium | **Source:** Validation pass 2026-04-06 | **Status:** Fixed (2026-04-06, needs live browser re-validation)

The per-conversation metrics counter (`X.Xk in / Y out  Zs`) rendered correctly in the conversation header right after a turn completed, but was missing on reload of the same conversation.

**Root cause (deeper than originally suspected):** The chip itself is the `TurnUsageBadge` rendered under the last assistant message in `conversation.tsx:323`. It was gated on `lastTurnUsage`, which is state ONLY populated by the `turn_complete` WebSocket event. On page reload there is no new `turn_complete` event, so `lastTurnUsage` stayed `null` and the badge was never rendered. The server had no API surface to hydrate per-turn usage on load — the aggregate `/api/metrics/conversation/{id}` endpoint only returned totals across all turns, with no last-turn breakdown.

**Fix applied (2026-04-06) — required a backend + frontend change:**
- `internal/db/query/analytics.sql`: added new `GetConversationLastTurnUsage :one` query. Uses a CTE to pick the latest turn_number from `sub_calls` (scoped to `purpose='chat'` and non-null turn_number) and returns that turn's aggregated `tokens_in`, `tokens_out`, `iteration_count`, `latency_ms`. Regenerated via `sqlc generate`.
- `internal/server/metrics.go`: added `LastTurn *lastTurnView` to `conversationMetricsResponse`. The handler calls the new query best-effort — absent rows are silently omitted, real errors are logged as warnings but don't fail the overall metrics fetch.
- `web/src/types/metrics.ts`: added `LastTurnUsage` interface and optional `last_turn?` field on `ConversationMetrics`.
- `web/src/pages/conversation.tsx`: added a `hydratedLastTurn` state + a `useEffect` on `convId` that fetches `/api/metrics/conversation/{convId}` on mount, converts `last_turn` into the `TurnUsage` shape the badge expects (including ms→ns conversion for `duration`), and stores it. Introduced a derived `displayLastTurnUsage = lastTurnUsage ?? hydratedLastTurn` — live state takes priority when present, hydrated falls back otherwise. The badge render site now reads `displayLastTurnUsage`.

**Still TODO:** Browser validation preempted. Re-run: reload an existing conversation and confirm the usage chip renders under the last assistant message without a new turn needing to fire.

---

### B4. `avg_budget_used_pct` metric is wrong on BOTH sides (backend + frontend)
**Severity:** Medium (display lies, doesn't block use) | **Source:** Validation pass 2026-04-06 | **Status:** Open

`/api/metrics/conversation/{id}` returned `avg_budget_used_pct: 2831` for a 2-turn conversation where actual budget use was ~11% (3.4k / 30k tokens).

**Confirmed root cause (found during B3 investigation, not yet fixed):**
Two separate bugs compound each other:
1. **Backend:** `internal/db/query/analytics.sql` `GetConversationContextQuality` computes `AVG(budget_used) AS avg_budget_used` — but `budget_used` in `context_reports` is a raw TOKEN COUNT (e.g. 3400), not a ratio or percentage. So the "pct" column name is a lie and the returned value is actually "average token count used across turns". For a 2-turn sample at 3400 + 2262 budget_used, the average is 2831, matching what we saw.
2. **Frontend:** `web/src/components/chat/conversation-metrics.tsx:103` renders that value as `(ctxQ.avg_budget_used_pct * 100).toFixed(0)%`. So the user would see `283100%` — except the panel is collapsed by default during normal use, which is why this never shows up prominently.

**Fix direction:** Fix the SQL to compute `AVG(CAST(budget_used AS REAL) * 100.0 / NULLIF(budget_total, 0))` so the backend returns a real 0-100 percentage, AND remove the `* 100` on the frontend since the column name already says `_pct`. Add a unit test with known 2-turn inputs covering both sides.

---

### B5. `sirtopham init` generates an invalid default Anthropic model ID
**Severity:** Medium (day-1 blocker for new Anthropic users) | **Source:** Validation pass 2026-04-06 | **Status:** Fixed (2026-04-06)

`cmd/sirtopham/init.go` `generateConfigYAML()` hardcoded `claude-sonnet-4-6`, but the actual Anthropic provider catalog uses dated IDs (`claude-sonnet-4-6-20250514`, etc.). The same bad literal also appeared in `internal/config/config.go` as the default config defaults, plus several test fixtures.

**Fix applied (2026-04-06):**
- `cmd/sirtopham/init.go`: both generated YAML literals updated to `claude-sonnet-4-6-20250514`
- `internal/config/config.go`: both default-config `Model` fields (routing.default and providers.anthropic) updated to `claude-sonnet-4-6-20250514` — this was the deeper source, since the binary's built-in defaults also used the short ID
- `cmd/sirtopham/config_test.go`: two test fixtures updated to match
- `internal/config/config_test.go`: one parse-round-trip fixture updated for consistency

**Remaining drift risk:** The default is still a hardcoded string in two places (`init.go` and `config.go`). Ideally both would pull from the anthropic provider package's model catalog at build or runtime. Out of scope for this session — logged as future improvement.

---

### B6. Empty `<optgroup>` renders in default-model dropdown for unavailable providers
**Severity:** Low (cosmetic) | **Source:** Validation pass 2026-04-06 | **Status:** Open

When `anthropic` has no auth, the settings page default-model dropdown still renders an empty `<optgroup label="anthropic">` with zero options inside. Visually confusing — user can see the label but nothing under it.

**Fix direction:** In `settings.tsx`, filter providers with zero models before emitting the optgroup, or render a disabled option like "no models available" under the group.

---

### B7. `doctor` and `/api/config` show providers not present in the project yaml
**Severity:** Low (observation, may be intentional) | **Source:** Validation pass 2026-04-06 | **Status:** Open

The project yaml for my-website only configured `codex`. Yet `sirtopham doctor` and `GET /api/config` both returned:
- `anthropic` (unavailable)
- `openrouter` (marked available but with `auth: unavailable`)
- `codex` (healthy)

This suggests there is a built-in default provider catalog that always appears regardless of what's in the yaml.

**Fix direction:** Confirm this is intentional (e.g. "show the full catalog so users can see what providers are supported and which are active") vs accidental (e.g. "config merging is leaking a global default file"). If intentional, document it; if accidental, fix the scoping.

---




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


