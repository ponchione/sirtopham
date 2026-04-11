# Agent Roles and Brain Conventions Implementation Plan

> For Hermes: use subagent-driven-development for execution. Keep edits narrow, avoid conductor implementation, and verify with `make test` and `make build`.

Goal: finish the SirTopham-side implementation for spec 14 by adding read-only auditor file access, checking in and wiring role prompts plus canonical role configuration, and documenting the `.brain/` conventions and SirTopham/conductor boundary.

Architecture: build on the already in-flight spec-13 headless `run` work instead of inventing a new runtime path. Keep enforcement narrow and concrete: role-scoped tool registration, role-scoped brain write policy, checked-in prompt assets, checked-in config/example wiring, and operator-facing documentation. Do not implement conductor orchestration behavior in this repo.

Tech stack: Go CLI/runtime, existing Cobra command tree, existing `internal/tool` registry and brain path enforcement, repo-root markdown prompt assets, checked-in YAML config, markdown docs.

## Grounded current state

Already present locally:
- `cmd/sirtopham/run.go`, `cmd/sirtopham/receipt.go`, `cmd/sirtopham/runtime.go`, `cmd/sirtopham/run_progress.go`
- `internal/role/builder.go`
- `internal/tool/brain_paths.go`
- `agent_roles` config parsing and validation in `internal/config/config.go`
- brain write allow/deny enforcement in brain tools

Known remaining SirTopham-side gaps versus spec 14:
- `file:read` read-only auditor tool group is not implemented yet
- `internal/tool/register.go` only has full `RegisterFileTools`
- no checked-in `agents/` prompt assets exist yet
- no checked-in canonical `agent_roles` config is present in `sirtopham.yaml`
- no checked-in operator doc for `.brain/` directory conventions and SirTopham/conductor boundaries exists yet

Important scope rule:
- do not implement `spawn_agent`, `chain_complete`, chain loops, or reindex subprocess orchestration in this repo

---

## Task 1: Add read-only file role support

Objective: implement the spec-14 `file:read` tool group so auditor-style roles can inspect files without receiving write/edit tools.

Files:
- Modify: `internal/tool/register.go`
- Modify: `internal/config/config.go`
- Modify: `internal/role/builder.go`
- Modify/add tests: `internal/config/config_test.go`
- Modify/add tests: `internal/role/builder_test.go`
- Modify/add tests: `internal/tool/fileutil_test.go` and/or `internal/tool/registry_test.go`

Implementation details:
1. Split file registration in `internal/tool/register.go` into:
   - `RegisterFileReadTools(r *Registry)` -> registers only `file_read`
   - `RegisterFileWriteTools(r *Registry)` -> registers `file_write` and `file_edit` using the same read-state store pattern
   - keep `RegisterFileTools(r *Registry)` as the full-access composition helper
2. Extend `allowedAgentRoleToolGroups` in `internal/config/config.go` to include `file:read`
3. Update agent role validation error text to mention `file:read`
4. Update `internal/role/builder.go` mapping:
   - `file` -> `RegisterFileTools`
   - `file:read` -> `RegisterFileReadTools`
5. Preserve current behavior for full-access roles and current custom-tool rejection behavior

Tests to add/update:
- config load/validation accepts `file:read`
- invalid tool error text reflects the new accepted list
- role builder with `file:read` registers only `file_read`
- role builder with `file` still registers `file_read`, `file_write`, `file_edit`
- direct register tests confirm split registration helpers behave as intended

Validation commands:
- `make test`

Suggested commit message:
- `feat: add read-only file role support`

---

## Task 2: Check in role prompt assets

Objective: add the checked-in role prompt files required by specs 13 and 14 and make them usable by `sirtopham run`.

Files:
- Create: `agents/orchestrator.md`
- Create: `agents/epic-decomposer.md`
- Create: `agents/task-decomposer.md`
- Create: `agents/planner.md`
- Create: `agents/coder.md`
- Create: `agents/correctness-auditor.md`
- Create: `agents/quality-auditor.md`
- Create: `agents/performance-auditor.md`
- Create: `agents/security-auditor.md`
- Create: `agents/integration-auditor.md`
- Create: `agents/test-writer.md`
- Create: `agents/resolver.md`
- Create: `agents/docs-arbiter.md`
- Optionally modify tests: `cmd/sirtopham/run_test.go` if a stronger checked-in prompt-path test is useful

Prompt content requirements:
- align closely with spec-14 role definitions and system-prompt guidance
- reflect actual SirTopham runtime behavior:
  - all brain reads are allowed
  - brain writes are path-scoped
  - `custom_tools` are external/conductor-provided, not implemented by SirTopham
  - receipts must follow the spec-13 receipt contract
- keep prompts short, explicit, and operational rather than essay-style

Validation commands:
- `make test`
- optionally run focused Go tests for command helpers if prompt-path coverage is expanded

Suggested commit message:
- `feat: add checked-in agent role prompts`

---

## Task 3: Check in canonical role config wiring

Objective: wire the checked-in roles into repo config so the feature is actually usable from SirTopham.

Files:
- Modify: `sirtopham.yaml`
- Modify/add tests: `internal/config/config_test.go`
- Optionally modify: `cmd/sirtopham/run_test.go` if command-level coverage is added for role lookup / failure modes

Implementation details:
- add an `agent_roles:` section to `sirtopham.yaml` covering the full spec-14 role set:
  - `orchestrator`
  - `epic-decomposer`
  - `task-decomposer`
  - `planner`
  - `coder`
  - `correctness-auditor`
  - `quality-auditor`
  - `performance-auditor`
  - `security-auditor`
  - `integration-auditor`
  - `test-writer`
  - `resolver`
  - `docs-arbiter`
- use `file:read` for the auditor roles
- ensure each role’s `brain_write_paths` and `brain_deny_paths` match spec 14 as closely as possible while remaining SirTopham-only
- keep orchestrator present in config, but preserve the current runtime behavior where selecting a role with `custom_tools` fails clearly unless an external conductor supplies them

Required role/tool mapping:
- orchestrator: `brain` + `custom_tools`
- epic/task decomposers: `brain`
- planner: `brain`, `search`
- coder: `brain`, `file`, `git`, `shell`, `search`
- auditors: `brain`, `file:read`, `git`
- test-writer: `brain`, `file`, `git`, `shell`
- resolver: `brain`, `file`, `git`, `shell`
- docs-arbiter: `brain`

Tests to add/update:
- config load for a representative `file:read` role
- config load for representative orchestrator role with `custom_tools`
- validation that path lists and prompt paths parse as expected

Validation commands:
- `make test`

Suggested commit message:
- `feat: add canonical agent role config`

---

## Task 4: Document `.brain/` conventions and the conductor boundary

Objective: document everything needed for human operators and future conductor integration without implementing conductor logic here.

Files:
- Create: `docs/agent-roles-and-brain-conventions.md` (or nearby equivalent under `docs/`)
- Create: `docs/agent-role-conductor-boundary.md` (small companion boundary doc)
- Optionally modify: `README.md` to link these docs and the new role/run feature

Required doc content:
1. `.brain/` directory structure from spec 14
2. ownership and allowed writers per directory
3. naming conventions:
   - `epics/{slug}/epic.md`
   - `tasks/{epic-slug}/{NN-task-slug}.md`
   - `plans/{epic-slug}/{NN-task-slug}.md`
   - `receipts/{role}/{chain-id}--{task-slug}.md`
4. what SirTopham enforces in code today:
   - role tool scoping
   - brain write path scoping
   - checked-in prompts/config
   - headless `run`
5. what conductor owns and is out of scope for this repo:
   - `spawn_agent`
   - `chain_complete`
   - sequencing / loops / reindex trigger orchestration / parallelism
6. operator note that plain `sirtopham run --role orchestrator` is expected to fail without conductor-provided custom tools

Recommended README follow-through:
- add a concise note/link under the implementation/resume or CLI sections so this feature is discoverable

Validation:
- docs paths exist and are linked from at least one discoverable place
- documentation matches actual runtime behavior and config names

Suggested commit message:
- `docs: add agent roles and brain convention guides`

---

## Task 5: Full verification and cleanup

Objective: verify the complete implementation, check for obvious drift, and summarize the outcome.

Files:
- Review only; edit as needed if verification reveals narrow issues

Steps:
1. Run `make test`
2. Run `make build`
3. Review `git diff --stat`
4. Review the changed files for spec drift and accidental scope creep
5. If tests/build reveal narrow issues, fix them without unrelated refactors

Success criteria:
- `file:read` roles work as intended
- checked-in prompts resolve from project root
- checked-in role config exists and matches spec-14 SirTopham responsibilities
- `.brain/` conventions and conductor boundary are documented clearly
- `make test` passes
- `make build` passes

Suggested commit message:
- `feat: complete agent roles and brain conventions support`

---

## Recommended execution order

1. Task 1: `file:read` runtime support
2. Task 2: checked-in role prompts
3. Task 3: canonical role config wiring
4. Task 4: docs for conventions and conductor boundary
5. Task 5: verification

## Risks and constraints

- Keep edits narrow; do not refactor unrelated code
- Do not implement conductor behavior in this repo
- Avoid churn in `.brain/` project state beyond documentation and path-policy/config alignment
- Prefer `make test` and `make build` for validation

## Execution note

This plan is ready for subagent execution in two mostly independent workstreams:
- Workstream A: Task 1 (`file:read` runtime/tooling/tests)
- Workstream B: Tasks 2-4 (prompt assets, config wiring, docs)

After both complete, run Task 5 verification in the controller session or a final review agent.