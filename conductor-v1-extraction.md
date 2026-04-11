# Conductor V1 → V2 Extraction Guide

**Purpose:** Identify reusable patterns and code from `agent-conductor` (v1) for the new orchestrator repo. This is a reference doc for an extraction agent — pull the listed items into standalone files, adapting imports and naming for the new project.

**Source repo:** `github.com/ponchione/agent-conductor`
**Target repo:** New repo (TBD name)

---

## What to Extract

### 1. SQLite Schema Patterns (High Value)

**Source:** `internal/database/schema.sql`

The core three-table pattern is directly reusable with modifications:

- **`workflows` table** → rename to `chains`. Represents a chain execution. Keep: `id`, `current_state`, `created_at`, `updated_at`, `completed_at`, `error_message`. Keep budget fields but rename to match new model (`max_resolver_loops`, `max_chain_duration_mins`, `total_token_budget`). Drop: `original_file`, `target_repo`, `git_branch`, `context_package_path`, `verification_report_path`, `max_depth`, `max_files_changed`, `current_depth`, `files_changed`.

- **`tasks` table** → rename to `steps`. Represents a single agent invocation in a chain. Keep: `id`, `workflow_id` (→ `chain_id`), `sequence_num`, `state`, `attempts`, `max_attempts`, `created_at`, `started_at`, `completed_at`, `error_message`. Add: `agent_role`, `receipt_path`, `verdict`, `tokens_used`, `turns_used`, `duration_seconds`. Drop: `task_type`, `agent_type`, `target_repo`, `phase`, `input_artifact`, `output_artifact`, `claimed_by`, `claimed_at`, `exit_code`, `stdout_log`, `stderr_log`, `files_changed`.

- **`events` table** → keep as-is. The event log pattern is universal. Rename `workflow_id` → `chain_id`, keep `task_id` → `step_id`.

**Do NOT extract:** `pipeline_runs`, `plan_runs`, `sub_calls`, `sessions`, `artifacts` tables. These are pipeline-specific and won't map to the new model.

---

### 2. Atomic Task Claiming (High Value)

**Source:** `internal/database/tasks.go` — the `AtomicClaimTask` function
**Source:** `sql/queries.sql` — the `ClaimTask` query

The atomic claim pattern (UPDATE WHERE state='pending' + check rows affected) is useful if the conductor ever runs multiple chain workers. Extract the pattern, not the exact implementation — the new schema will have different columns.

---

### 3. Queue State Machine (Medium Value)

**Source:** `internal/queue/queue.go`

The `Queue` struct's state machine logic is a good reference:
- `ClaimNextTask` — claim + workflow state check + budget check
- `CompleteTask` / `FailTask` / `RetryTask` — state transitions
- `checkBudgetExceeded` — budget limit enforcement
- `triggerGate` — transition to human review when limits hit

Extract the state constants and the budget-check pattern. The actual Queue implementation is tightly coupled to the old schema and will need rewriting, but the flow logic is the reference.

---

### 4. Event Logging Pattern (Medium Value)

**Source:** `internal/database/events.go`
**Source:** `sql/queries.sql` — `CreateEvent`, `ListEvents`, `ListEventsSince`

The event sourcing pattern (JSON blob per event, ordered by timestamp, filterable by chain/step) is directly reusable. The `ListEventsSince` cursor pattern is useful for streaming/polling.

---

### 5. Work Order Schema (Reference Only)

**Source:** `work-order.template.yaml`
**Source:** `internal/models/workorder.go`

Don't extract the code, but use the schema structure as reference for the new chain definition YAML format. The typed acceptance criteria pattern (`verification.kind`, `verification.check`) is a good precedent for defining how receipts should be evaluated.

---

### 6. Git Branch Management (Reference Only)

**Source:** `internal/git/git.go`

The branch create/checkout/reset functions are clean. May be useful later if the conductor manages git branches for chain executions (creating a feature branch before the coder starts, for example). Don't extract now — reference later if needed.

---

### 7. Project Config Loading (Low Value)

**Source:** `internal/config/config.go`

Cobra + Viper YAML config loading with defaults. Standard pattern — faster to rewrite from scratch for the new config shape than to adapt the old one.

---

## What NOT to Extract

| Package | Reason |
|---|---|
| `internal/context/` | Replaced by SirTopham's context assembly |
| `internal/rag/` | Replaced by SirTopham's RAG/retrieval layer |
| `internal/graph/` | Replaced by SirTopham's code intelligence |
| `internal/scope/` | Replaced by SirTopham's context assembly |
| `internal/llm/` | Replaced by SirTopham's provider system |
| `internal/planner/` | Replaced by the Planner agent role |
| `internal/verify/` | Replaced by auditor agent roles |
| `internal/worker/` | Replaced by headless `sirtopham run` |
| `internal/executor/` | Replaced by SirTopham's tool executor |
| `internal/streaming/` | Replaced by SirTopham's streaming |
| `internal/templates/` | Replaced by agent system prompts |
| `internal/api/` | Web UI — not needed for conductor v2 |
| `web/` | Frontend — not needed for conductor v2 |
| `internal/lock/` | File locking — brain vault handles this |
| `internal/gate/` | Human review gates — rethink for new model |
| `internal/models/structs.go` | Pipeline-specific types (ContextPackage, VerificationReport) |

---

## Extraction Output

The extraction agent should produce standalone files in a staging directory:

```
extracted/
├── schema_reference.sql      # Adapted schema with new table/column names
├── queue_patterns.go          # State machine logic, budget checks, gate triggers
├── event_patterns.go          # Event logging and cursor-based listing
├── atomic_claim_pattern.go    # The atomic UPDATE + rows-affected claim pattern
└── notes.md                   # Any observations about patterns worth preserving
```

These files are reference material for the new repo's author, not drop-in code. Imports will need rewriting, types will change, and the schema is different. The value is in preserving the proven patterns without having to reverse-engineer them from the old codebase later.