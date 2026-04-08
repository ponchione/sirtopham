# Spec Implementation Audit — 2026-04-08

Project: sirtopham
Scope: full code audit against `docs/specs`
Audited against:
- `docs/specs/01-project-vision-and-principles.md`
- `docs/specs/02-tech-stack-decisions.md`
- `docs/specs/03-provider-architecture.md`
- `docs/specs/04-code-intelligence-and-rag.md`
- `docs/specs/05-agent-loop.md`
- `docs/specs/06-context-assembly.md`
- `docs/specs/07-web-interface-and-streaming.md`
- `docs/specs/08-data-model.md`
- `docs/specs/09-project-brain.md`
- `docs/specs/10 — Tool System.md`
- `docs/specs/11-tool-result-normalization.md`
- `docs/specs/12 — Claude Code Analysis Retrofits.md`

Verification status:
- `make test` passed
- `make build` passed

## Executive summary

Short answer: the implementation is mostly aligned with the spec set, but it is not fully spec-complete.

The codebase is operationally healthy and clearly implements the core architecture described by the specs. The biggest remaining gaps are not broad missing subsystems; they are mostly thinner-than-spec runtime wiring, a few unfulfilled deeper contracts, and several places where the implementation has intentionally drifted from the original pre-implementation docs.

High-level assessment:
- operational health: strong
- architectural alignment: strong
- spec completeness: partial
- documentation freshness: mixed; several specs now lag the implementation

Approximate overall alignment: 75-85%

## What this audit means

This audit distinguishes between:
- implemented: the code matches the intent and is live in the runtime
- partial: the code exists, but is thinner than the spec or not fully wired in runtime
- missing: the spec describes behavior that is not present in a meaningful way
- spec drift: the code does something different, but the difference appears intentional or product-valid and the docs should catch up

## Operational status

The repo is buildable and testable in its current form.

Evidence:
- `Makefile:18-25`
- `make test` passed
- `make build` passed

This is important because it means the audit is not describing a speculative or half-broken codebase. The code is real and healthy even where it is not perfectly faithful to the older spec text.

## Overall verdict by spec area

| Spec area | Verdict | Notes |
| --- | --- | --- |
| vision / principles | Mostly implemented | Product shape is real and coherent |
| tech stack | Strongly implemented | Some docs still describe choices as unresolved when they are already settled in code |
| provider architecture | Strongly implemented | One of the best-aligned areas |
| code intelligence / RAG | Partially implemented | Strong substrate; runtime wiring thinner than spec |
| agent loop | Strongly implemented | Main gap is live tool batching |
| context assembly | Strongly implemented | One of the strongest areas |
| web interface / streaming | Mostly implemented | Real app, but protocol/UI drift and missing surfaces remain |
| data model | Mostly implemented | Good schema; a few notable mismatches |
| project brain | Partially implemented with intentional drift | Narrower than original broad spec |
| tool system | Mostly implemented | Safety strong; schema validation weaker than implied |
| tool-result normalization | Strongly implemented | Mature compared with spec intent |
| retrofit items | Partially implemented | Several key hardening items landed |

## Strong matches to the specs

### 1. Single-binary app, embedded frontend, and local-first runtime

The project is clearly implemented as the intended local-first single-user app with an embedded frontend.

Evidence:
- frontend build and embed flow in `Makefile:18-21`
- embedded FS in `webfs/embed.go`
- static/SPA serving in `internal/server/server.go:118-124`
- fallback serving in `internal/server/static.go:10-37`
- minimal middleware stack, no user-auth product layer in `internal/server/middleware.go`

Assessment:
- implemented

### 2. Provider abstraction, auth, routing, health, and fallback

The provider stack is one of the strongest parts of the repo. The unified provider interface, multiple provider implementations, router health/fallback logic, and auth status surfaces are real and substantial.

Evidence:
- provider interface in `internal/provider/provider.go`
- response/usage types in `internal/provider/response.go`
- stream event types in `internal/provider/stream.go`
- Anthropic provider and OAuth handling in `internal/provider/anthropic/*`
- Codex provider and credential handling in `internal/provider/codex/*`
- OpenAI-compatible provider in `internal/provider/openai/*`
- router logic in `internal/provider/router/router.go`
- router health/auth in `internal/provider/router/health.go`
- API exposure in `internal/server/configapi.go`

Assessment:
- implemented

Notable strength:
- the provider/auth surface is in some places more mature than the original draft specs imply

### 3. SQLite schema, FTS5, sqlc-backed persistence

The data layer closely reflects the intended architecture.

Evidence:
- SQLite connection/pragmas in `internal/db/sqlite.go`
- full schema in `internal/db/schema.sql`
- sqlc-generated models and queries in `internal/db/*.go`
- conversation management in `internal/conversation/*`
- context report persistence in `internal/context/report_store.go`

Assessment:
- implemented

### 4. Agent loop core lifecycle

The agent loop is real and not superficial. Turn orchestration, prompt assembly, streaming, cancellation, retry behavior, compression, and persistence are all meaningfully implemented.

Evidence:
- core loop in `internal/agent/loop.go:343-421`
- prompt assembly in `internal/agent/prompt.go`
- error handling in `internal/agent/errors.go`
- cancellation cleanup in `internal/agent/turn_cleanup.go`
- loop detection in `internal/agent/loopdetect.go`

Assessment:
- implemented

### 5. Context assembly

This is one of the closest matches to the specs. The code contains a serious implementation of analyzer-driven retrieval, budgeting, reporting, serialization, and compression.

Evidence:
- analyzer in `internal/context/analyzer.go`
- query shaping in `internal/context/query.go`
- momentum in `internal/context/momentum.go`
- retrieval orchestration in `internal/context/retrieval.go`
- budgeting in `internal/context/budget.go`
- serialization in `internal/context/serializer.go`
- stored reports in `internal/context/report_store.go`
- compression in `internal/context/compression.go`

Assessment:
- implemented

### 6. Tool-result normalization and history compression

The normalization/compression story is stronger than an early draft reader might expect.

Evidence:
- normalization in `internal/tool/normalize.go:11-38`
- executor application of normalization/truncation in `internal/tool/executor.go:128-139`
- history compression in `internal/tool/historycompress.go`
- prompt history compaction in `internal/agent/prompt.go`

Assessment:
- implemented

## Major partials and gaps

These are the highest-signal places where the implementation does not yet fully match the specs.

### 1. Runtime indexing still uses a noop describer

The codebase has a real description-generation component, but the live indexing service does not use it.

Evidence:
- live index service passes `noopDescriber{}` in `internal/index/service.go:178-183`

Why this matters:
- the code-intelligence spec expects semantic chunk descriptions to materially improve retrieval quality
- current production embeddings are thinner than the intended design

Classification:
- partial implementation

### 2. Structural graph code exists, but is not live-wired into retrieval

The repo contains structural graph analyzers and storage code, but the runtime server builds the retrieval orchestrator without a graph store.

Evidence:
- retrieval orchestrator construction in `cmd/sirtopham/serve.go:173`
- retrieval orchestrator accepts graph/conventions but defaults are nil/noop in `internal/context/retrieval.go:117-134`

Why this matters:
- the spec expects structural graph retrieval to participate in context assembly
- today the runtime mainly relies on semantic search, explicit files, and brain retrieval

Classification:
- partial implementation

### 3. Convention retrieval remains a placeholder

The abstraction exists, but the default runtime implementation is still `NoopConventionSource`.

Evidence:
- `internal/context/conventions.go:5-13`
- fallback wiring in `internal/context/retrieval.go:117-125`

Classification:
- partial implementation

### 4. Live tool execution is still one-by-one despite batch-capable executor support

The lower-level executor already supports purity-aware batching, but the agent loop adapter still dispatches a single tool call at a time.

Evidence:
- batch-capable executor in `internal/tool/executor.go:53-142`
- single-call adapter in `internal/tool/adapter.go:11-45`

Why this matters:
- the tool-system spec expects batch-oriented execution behavior
- runtime performance and contract fidelity are currently thinner than intended

Classification:
- partial implementation

### 5. Tool input validation is weaker than the schemas imply

Tool schemas are present, but runtime behavior is not yet full schema-based validation before execution.

Evidence:
- schemas exist across tool implementations
- runtime relies more on JSON parsing and tool-side checks than centralized schema enforcement

Classification:
- partial implementation

### 6. Per-conversation provider/model defaults are only partly realized

The data model supports conversation-level provider/model values, but live execution primarily uses config defaults or one-shot turn overrides.

Evidence:
- conversation persistence fields in `internal/conversation/manager.go:106-120`
- runtime override selection in `internal/agent/loop.go:345-355`
- WebSocket override flow in `internal/server/websocket.go:223-232` and `internal/server/websocket.go:304-318`

Classification:
- partial implementation

### 7. Some UI surfaces implied by the specs are still absent

The backend exposes project metadata/tree/file endpoints, but the frontend routes are still limited to conversation list, conversation, and settings.

Evidence:
- project endpoints in `internal/server/project.go:33-35`
- frontend routes in `web/src/main.tsx:15-50`

Classification:
- partial implementation

## Important spec-drift items

These are places where the code differs from the original docs, but the difference looks intentional or at least product-valid.

### 1. Brain architecture has narrowed to MCP/vault-backed keyword retrieval

The current runtime brain contract is narrower than the broader original brain vision.

What exists today:
- runtime brain backend is MCP/vault-based
- proactive brain retrieval is part of context assembly
- semantic/index-backed brain retrieval is not a production path

Evidence:
- brain backend setup in `cmd/sirtopham/serve.go:167-174`
- proactive brain retrieval in `internal/context/retrieval.go:327-379`
- reactive brain search tool in `internal/tool/brain_search.go`

Assessment:
- spec drift, not just implementation debt

Recommendation:
- either implement the broader semantic brain path, or update the brain spec to fully describe the supported MCP/vault keyword contract

### 2. WebSocket protocol differs from the written spec

The protocol works, but event names and payload shapes have drifted.

Evidence:
- `conversation_created` emission in `internal/server/websocket.go:292-301`
- flatter client message handling in `internal/server/websocket.go:202-239`

Assessment:
- spec drift

Recommendation:
- reconcile docs with the real transport contract or intentionally migrate the code back to the original contract

### 3. Project identity does not follow the original UUIDv7 project model

Conversations use generated IDs, but projects are keyed by `projectRoot` during initialization.

Evidence:
- conversation UUIDv7 creation in `internal/conversation/manager.go:90-123`
- project insertion in `cmd/sirtopham/init.go:238-247`

Assessment:
- spec drift / data-model mismatch

### 4. Tool argument naming drift

The file-edit tool uses `old_str` / `new_str` rather than older spec wording like `old_string` / `new_string`.

Evidence:
- `internal/tool/file_edit.go:21-35`

Assessment:
- contract drift

## Areas where the implementation is stronger or more mature than the old docs suggest

This is also important. Some specs now understate what the code can already do.

### 1. Tool-result normalization/compression is real and mature
- normalization exists and is wired into execution
- history compression is real
- prompt-side compaction is integrated

### 2. Cancellation handling is robust
- interrupted-turn cleanup and synthetic result handling are more mature than a naive reading of the older docs might suggest

### 3. Provider/auth/runtime surfaces are mature
- provider auth status, fallback handling, config exposure, and multi-provider plumbing are stronger than “pre-implementation” docs imply

## Data-model mismatches worth calling out

### 1. `sub_calls.message_id` linkage is not wired through
The schema supports it, but tracked-provider persistence does not currently populate it.

Evidence:
- parameter construction in `internal/provider/tracking/tracked.go:222-245`

Classification:
- partial / missing linkage

### 2. Projects are not UUIDv7-backed in practice
See project identity drift above.

### 3. Some schema/runtime details have evolved beyond the draft specs
Examples:
- extra report/tool-execution fields
- broader FTS usage
- ad hoc upgrade helpers rather than a pure “nuke-only” posture

This is mostly doc drift, not a sign of broken implementation.

## Areas most in need of follow-up

If the goal is to move from “mostly aligned” to “spec-complete,” the highest-leverage sequence is:

1. wire the real describer into runtime indexing
2. wire structural graph into live indexing/retrieval
3. make live tool dispatch use real batch execution
4. implement real convention retrieval
5. reconcile conversation-scoped provider/model runtime behavior
6. wire `sub_calls.message_id`
7. add stronger schema-based tool validation
8. reconcile WebSocket / brain / project-identity docs with the real product contract

## Recommended interpretation of current state

The right reading of this repo is not “the specs were ignored.”
The right reading is:
- the original architecture was substantially implemented
- several core subsystems are already strong and production-shaped
- the remaining gaps are concentrated in deeper runtime integration and contract reconciliation
- a meaningful portion of the mismatch is now documentation lag rather than missing engineering

## Companion register

Actionable unresolved items from this audit are tracked in:
- `TECH-DEBT.md`

This document is the narrative/evidence companion to that shorter action-oriented debt register.