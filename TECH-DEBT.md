# TECH-DEBT

Last audit: 2026-04-08 (full spec-vs-implementation code audit)
Last implementation sweep: 2026-04-08 (batch tool dispatch + persisted conversation runtime defaults + runtime describer wiring)
Current phase: operationally healthy, not fully spec-complete

This register replaces the old resolved-item log. It now tracks the highest-signal gaps and spec drifts found by auditing the current code against docs/specs.

Overall verdict:
- build/test health is good (`make build`, `make test` pass)
- the codebase is broadly aligned with the spec set
- this sweep closed live batch tool dispatch (T4), conversation-scoped provider/model runtime defaults (T6), and runtime describer wiring (T1)
- the biggest remaining work is now concentrated in graph/convention runtime wiring, schema/linkage hardening, and contract reconciliation docs

## Active tech debt

### T1. Runtime indexing still uses a noop describer
Status: closed 2026-04-08
Priority: high
Area: code intelligence / RAG

What landed:
- runtime indexing now requests a describer from the index service dependency graph instead of hard-coding `noopDescriber{}`
- the default runtime path constructs a real qwen-coder-backed describer and discovers the live model from `/v1/models`
- if qwen-coder is unavailable or the describer call fails, indexing still continues via the describer's existing graceful fallback path, leaving signature-only embeddings and warn-level evidence rather than aborting the whole index run

Evidence:
- `internal/index/service.go`
- `internal/index/runtime_describer.go`
- `internal/index/service_test.go`
- `internal/index/runtime_describer_test.go`

Notes:
- this closes the wiring gap, but retrieval-quality proof after a fresh real reindex is still worth doing when taking the next code-intelligence validation pass

### T2. Structural graph exists but is not live-wired into runtime retrieval
Status: active
Priority: high
Area: code intelligence / context assembly

What exists today:
- graph analyzers, types, and storage code exist
- the live server retrieval orchestrator is constructed with no graph store

Evidence:
- `cmd/sirtopham/serve.go:173`
- `internal/context/retrieval.go:117-134`

Why this matters:
- the spec expects structural graph results to be part of context assembly
- current runtime behavior is code-RAG plus explicit files/brain, without live graph-backed retrieval

Done means:
- index/build flow populates graph data in a supported runtime path
- serve/runtime injects a real graph store into retrieval orchestration
- context reports and inspector surfaces show real graph retrieval when relevant

### T3. Convention retrieval remains a placeholder
Status: active
Priority: medium
Area: context assembly

What exists today:
- convention loading is abstracted behind an interface
- the default runtime implementation is `NoopConventionSource`

Evidence:
- `internal/context/conventions.go:5-13`
- `internal/context/retrieval.go:117-125`

Why this matters:
- the specs include conventions as a first-class context source
- current runtime cannot actually retrieve or inject project conventions unless this is replaced

Done means:
- a real convention source is implemented and wired into the runtime
- convention retrieval appears in assembled context and context reports when applicable

### T4. Live agent loop still dispatches tool calls one-by-one
Status: closed 2026-04-08
Priority: high
Area: agent loop / tool system

What landed:
- the agent loop now batches same-iteration valid tool calls when the executor supports batch dispatch
- the adapter now exposes batch execution to the loop while preserving provider/tool result conversion
- persistence and emitted tool result ordering remain per-call and input-ordered

Evidence:
- `internal/agent/loop.go`
- `internal/tool/adapter.go`
- `internal/agent/loop_test.go`
- `internal/tool/adapter_test.go`

Notes:
- malformed tool calls are still filtered and surfaced individually before batch execution
- mixed pure/mutating execution strategy still lives in the lower-level executor

### T5. Tool input validation is weaker than the tool schemas imply
Status: active
Priority: medium
Area: tool system

What exists today:
- tools expose schemas
- runtime validation is not full JSON Schema enforcement before execution

Why this matters:
- the written tool contract implies stronger schema-based validation than the current implementation guarantees
- this increases drift between provider-facing tool definitions and actual execution-time checks

Done means:
- tool inputs are validated against their declared schemas before execution
- user/agent-facing errors clearly identify missing or invalid fields
- validation behavior is covered by tests

### T6. Per-conversation provider/model defaults are only partially realized
Status: closed 2026-04-08
Priority: medium
Area: providers / conversations

What landed:
- WebSocket turn resolution now falls back to stored conversation provider/model defaults before runtime defaults
- new WebSocket-created conversations persist the resolved provider/model defaults at creation time
- existing conversations persist updated provider/model selections when a turn supplies a new override

Evidence:
- `internal/server/websocket.go`
- `internal/conversation/manager.go`
- `internal/db/query/conversation.sql`
- `internal/server/websocket_test.go`

Notes:
- REST create/get already exposed the fields; the missing part was live runtime participation and WebSocket persistence behavior

### T7. WebSocket protocol has drifted from the written spec
Status: active
Priority: medium
Area: web interface / streaming

What exists today:
- the protocol works, but event names and payload shapes differ from the original spec docs
- example: the server emits `conversation_created` rather than the spec’s earlier event naming

Evidence:
- `internal/server/websocket.go:292-301`
- `internal/server/websocket.go:202-239`

Why this matters:
- this is a maintenance/documentation mismatch
- future frontend/backend work has to infer the real contract from code rather than the spec docs

Done means:
- either the protocol is reconciled back to the spec, or the spec docs are updated to the real contract
- event names, payloads, and status states are documented consistently in one place

### T8. The UI still lacks some spec-level surfaces
Status: active
Priority: medium
Area: web interface

What exists today:
- the backend exposes project endpoints for tree/file access
- the frontend routes currently cover conversation list, conversation, and settings only

Evidence:
- `internal/server/project.go:33-35`
- `web/src/main.tsx:15-50`

Why this matters:
- the implementation is missing some of the product surfaces implied by the specs, especially file/browser-style navigation

Done means:
- either the missing UI surfaces are implemented, or the specs are narrowed to the actual supported product scope

### T9. Project IDs do not match the original UUIDv7 data-model intent
Status: active
Priority: low
Area: data model

What exists today:
- conversation IDs are UUIDv7-backed
- project records use `projectRoot` as the project ID during initialization

Evidence:
- `internal/conversation/manager.go:90-123`
- `cmd/sirtopham/init.go:238-247`

Why this matters:
- this is direct drift from the original data-model spec
- it may be fine product-wise, but the docs and implementation do not currently agree

Done means:
- either projects move to UUIDv7 IDs, or the spec/docs are updated to make path-keyed single-project identity explicit

### T10. `sub_calls.message_id` linkage is not wired through
Status: active
Priority: medium
Area: data model / observability

What exists today:
- the schema supports message-linked sub-calls
- tracked provider persistence currently does not populate `message_id`

Evidence:
- `internal/provider/tracking/tracked.go:222-245`

Why this matters:
- the data model suggests stronger per-message linkage than the current runtime records
- this weakens forensic/debug/metrics joins relative to the intended design

Done means:
- sub-call persistence is linked to the corresponding assistant message row when available
- tests cover the linkage behavior and fallback cases

### T11. Brain implementation is intentionally narrower than the original broad spec
Status: active, but likely docs/product reconciliation rather than pure implementation debt
Priority: medium
Area: project brain

What exists today:
- runtime brain behavior is MCP/vault-backed keyword retrieval
- proactive brain retrieval is live in context assembly
- semantic/index-backed brain retrieval is not implemented as a production path

Evidence:
- `cmd/sirtopham/serve.go:167-174`
- `internal/context/retrieval.go:327-379`

Why this matters:
- this is one of the biggest spec-to-product shifts in the repo
- some of the old brain spec should likely be treated as stale architecture, not pending implementation

Done means:
- either semantic brain indexing/retrieval is actually implemented, or the spec/docs are fully rewritten around the supported MCP/vault keyword contract

## Spec drift / documentation reconciliation items

These are important, but they may be better treated as doc cleanup than as engineering debt.

### D1. Specs still read as pre-implementation in places where code is now settled
Examples:
- SQLite driver choice is no longer pending
- LanceDB is no longer hypothetical in the current codebase
- some protocol and runtime decisions have already stabilized in implementation

Done means:
- docs/specs are updated so current architecture docs describe the real shipped/runtime contract

### D2. Tool contract naming drift (`old_str`/`new_str` vs older spec wording)
Evidence:
- `internal/tool/file_edit.go:21-35`

Done means:
- either the tool contract is renamed to match the spec, or the docs are updated to the real parameter names

### D3. Current runtime is stronger than some stale docs in a few areas
Examples:
- tool-result normalization/compression is real
- cancellation cleanup is more robust than the original draft implied
- provider/auth/runtime surfaces are more mature than the old pre-implementation docs suggest

Done means:
- docs distinguish closed work from actual open debt so this file stays focused on live gaps

## Recommended order of attack

1. T1 runtime describer wiring
2. T2 structural graph runtime wiring
3. T3 convention retrieval implementation
4. T10 sub-call to message linkage
5. T5 stronger schema validation
6. T7/T8/T9/T11 doc and product-contract reconciliation

## Notes

This file should stay focused on current, unresolved gaps.
Resolved historical cleanup slices should live in git history or dedicated audit notes, not remain in TECH-DEBT once closed.