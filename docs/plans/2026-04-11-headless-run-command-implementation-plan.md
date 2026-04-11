# Headless Run Command Implementation Plan

> For Hermes: use subagent-driven-development if executing this later. Keep edits narrow, prefer shared-runtime extraction over duplicate wiring, and verify with `make test` / `make build`.

Goal: Add a new `sirtopham run` subcommand that executes one autonomous headless agent session, writes a receipt into the brain vault, prints the receipt path to stdout, and exits with spec-defined codes.

Architecture: Reuse the existing runtime pieces already used by `serve`: config loading, provider router, brain backend, retrieval/context assembly, tool executor, conversation manager, and `agent.AgentLoop.RunTurn`. Add a thin headless command wrapper, role-based tool/runtime scoping, brain write path enforcement, and receipt verification/fallback logic. Avoid building a second agent loop or duplicating large blocks of `serve.go`; instead extract shared bootstrap helpers.

Tech stack: Cobra CLI, existing `internal/agent`, existing `internal/tool` registry/executor, existing SQLite conversation storage, existing brain backend, existing `internal/pathglob` matcher.

Confirmed decisions:
- Receipt frontmatter `step` defaults to `1` for v0.
- `agent_roles[*].system_prompt` paths resolve relative to project root.
- `custom_tools` may exist in config, but `sirtopham run` fails at runtime if the selected role declares any custom tools.

---

## Scope summary

In scope:
- New `sirtopham run` command
- `agent_roles` config parsing/validation
- Role-scoped tool registry construction
- Brain write allow/deny path enforcement for `brain_write` and `brain_update`
- Headless single-turn execution using existing `AgentLoop`
- Receipt verification and fallback receipt writing
- Exit-code mapping and stdout/stderr contract
- Focused unit/integration tests

Out of scope:
- Multi-turn headless orchestration
- Parallel agent execution
- Implementing `custom_tools`
- Changes to web UI / `serve` behavior beyond shared-runtime extraction
- New provider-layer capabilities

---

## Design constraints from current codebase

- `internal/agent/loop.go` already has the right primitive: `RunTurn(...)` returns `FinalText`, `IterationCount`, aggregate token usage, and duration.
- `cmd/sirtopham/serve.go` already builds all core runtime dependencies; reuse this wiring via helper extraction rather than copy-pasting.
- `internal/tool/register.go` already has clean per-group registration functions.
- `internal/pathglob/matcher.go` already supports `**` patterns through doublestar; reuse it for brain allow/deny policy.
- `internal/tool/brain_write.go` and `internal/tool/brain_update.go` are the only places where brain path mutation enforcement needs to happen.
- `internal/config/config.go` currently has no `agent_roles` concept; validation must be added there.

---

## Implementation phases

### Phase 1: Config schema and validation

Objective: Add `agent_roles` support with strict validation and clear runtime defaults.

Files:
- Modify: `internal/config/config.go`
- Modify/add tests: `internal/config/config_test.go`

Deliverables:
- Add `AgentRoles map[string]AgentRoleConfig `yaml:"agent_roles"`` to `config.Config`
- Add `AgentRoleConfig` fields:
  - `SystemPrompt string `yaml:"system_prompt"``
  - `Tools []string `yaml:"tools"``
  - `CustomTools []string `yaml:"custom_tools"``
  - `BrainWritePaths []string `yaml:"brain_write_paths"``
  - `BrainDenyPaths []string `yaml:"brain_deny_paths"``
  - `MaxTurns int `yaml:"max_turns"``
  - `MaxTokens int `yaml:"max_tokens"``
- Add validation rules:
  - role names must be non-empty
  - `system_prompt` must be non-empty
  - tool groups must be from: `brain`, `file`, `git`, `shell`, `search`
  - `max_turns` / `max_tokens` must be > 0 when specified
- Add helper to resolve a role system prompt path relative to `ProjectRoot`

Verification:
- Config loads valid sample `agent_roles`
- Invalid tool group is rejected
- Empty `system_prompt` is rejected
- Zero/negative role limits are rejected

### Phase 2: Brain write path enforcement

Objective: Restrict brain writes per role without affecting brain reads/search.

Files:
- Modify: `internal/config/config.go`
- Add: `internal/tool/brain_paths.go`
- Modify: `internal/tool/brain_write.go`
- Modify: `internal/tool/brain_update.go`
- Modify/add tests: `internal/tool/brain_test.go` or new focused test file

Deliverables:
- Extend runtime `config.BrainConfig` with optional scoped fields:
  - `BrainWritePaths []string`
  - `BrainDenyPaths []string`
- Add helper(s):
  - normalize a vault-relative path safely
  - reject denied paths first
  - if allow list is non-empty, require at least one allow match
  - use `internal/pathglob.Match` / `MatchAny`
- Call the helper before `WriteDocument(...)` / `PatchDocument(...)`
- Preserve current behavior for unrestricted configs

Verification:
- Allowed path succeeds
- Denied path fails even if allow matches
- Empty allow list means unrestricted
- `brain_read` / `brain_search` remain unrestricted

### Phase 3: Shared runtime extraction from `serve`

Objective: Make `run` and `serve` share the same core wiring with minimal duplication.

Files:
- Modify: `cmd/sirtopham/serve.go`
- Add: `cmd/sirtopham/runtime.go`
- Add/modify tests only if extraction requires it

Deliverables:
- Extract helper(s) or a bundle builder for:
  - logger initialization
  - DB open/init + `ensureProjectRecord(...)`
  - provider router construction/registration/validation
  - code vectorstore / brain vectorstore opening
  - brain backend creation
  - graph store creation
  - retrieval/context assembler creation
  - conversation manager creation
- Keep `serve.go` behavior unchanged after refactor
- Expose a small runtime bundle struct for command-layer assembly

Suggested bundle contents:
- `Config *appconfig.Config`
- `Logger *slog.Logger`
- `Database *sql.DB`
- `Queries *appdb.Queries`
- `ProviderRouter ...`
- `BrainBackend brain.Backend`
- `SemanticSearcher ...`
- `BrainSearcher ...`
- `ConversationManager *conversation.Manager`
- `ContextAssembler ...`
- `Cleanup func()`

Verification:
- `serve` still builds/uses the extracted runtime without behavior drift
- Existing tests still pass

### Phase 4: Role-based registry builder

Objective: Construct only the tools allowed for the selected role.

Files:
- Add: `internal/role/builder.go`
- Add: `internal/role/builder_test.go`

Deliverables:
- Add a builder that takes:
  - base config
  - selected `AgentRoleConfig`
  - runtime dependencies (brain backend, runtime brain searcher, semantic searcher, provider router, queries, project ID)
- Return:
  - `*tool.Registry`
  - role-scoped `config.BrainConfig`
- Group mapping:
  - `brain` → `RegisterBrainToolsWithProviderRuntimeAndIndex`
  - `file` → `RegisterFileTools`
  - `git` → `RegisterGitTools`
  - `shell` → `RegisterShellTool`
  - `search` → `RegisterSearchTools`
- Inject role brain policy into the copied `BrainConfig`

Runtime rule:
- If selected role contains any `custom_tools`, `sirtopham run` returns an error explaining they are not implemented by SirTopham and must be provided by the external orchestrator.

Verification:
- Role with `file`+`git` only yields those tool definitions
- Role with `brain` gets brain tools wired with scoped brain config
- Role with `custom_tools` causes runtime error in run path

### Phase 5: `sirtopham run` CLI command

Objective: Add the new command and CLI contract.

Files:
- Add: `cmd/sirtopham/run.go`
- Modify: `cmd/sirtopham/main.go`
- Add tests: `cmd/sirtopham/run_test.go`

Deliverables:
- Register `newRunCmd(&configPath)` in `main.go`
- Support flags:
  - required-ish: `--role`, one of `--task` / `--task-file`
  - optional: `--chain-id`, `--brain`, `--max-turns`, `--max-tokens`, `--timeout`, `--receipt-path`, `--quiet`, `--project-root`
- Validate:
  - role is required
  - exactly one task source is provided
  - timeout parses cleanly
  - integer overrides > 0 when supplied
- Override config values for this invocation:
  - `ProjectRoot`
  - `Brain.VaultPath`
- Resolve role and load system prompt text from project-root-relative file

Helper functions to add:
- `readTask(task string, taskFile string) (string, error)`
- `resolveChainID(input string) string`
- `resolveReceiptPath(role, chainID, override string) string`
- `loadRoleSystemPrompt(projectRoot string, path string) (string, error)`

Verification:
- Missing role fails
- Both `--task` and `--task-file` fails
- Neither task source fails
- Prompt file resolution is relative to project root

### Phase 6: Headless runner wrapper over `AgentLoop`

Objective: Execute one autonomous run with minimal new orchestration code.

Files:
- Mostly in: `cmd/sirtopham/run.go`
- Optional tiny helper: `internal/agent/headless.go` only if needed to keep command code readable

Deliverables:
- Build agent loop using extracted runtime + role registry
- Create a new conversation via `conversation.Manager.Create(...)`
  - set provider/model if known from selected route
- Run one turn with:
  - `ConversationID`
  - `TurnNumber: 1`
  - `Message: task text`
- Set loop config:
  - `BasePrompt` = loaded role prompt text
  - provider/model defaults from normal routing
  - `MaxIterations` = resolved role/CLI max turns
- Apply whole-session timeout using context deadline

Important implementation note:
- Do not build a second loop engine; use `agent.NewAgentLoop(...).RunTurn(...)`
- Headless behavior is command-layer orchestration plus receipt handling, not a separate model interaction path

Verification:
- Text-only completion returns `TurnResult` and no server/UI involvement
- Conversation row is created and history persists as usual

### Phase 7: Progress sink and quiet mode

Objective: Match the stdout/stderr behavior in the spec.

Files:
- Add: `cmd/sirtopham/run_progress.go`
- Tests: `cmd/sirtopham/run_test.go`

Deliverables:
- Add an `agent.EventSink` implementation for headless mode that writes progress to stderr
- Emit lightweight lines for key events:
  - context assembly
  - iteration count
  - tool calls
  - completion metrics
- In `--quiet` mode:
  - suppress progress output entirely
  - still print final receipt path to stdout on successful/safety-limit exit

Verification:
- Non-quiet mode writes progress to stderr
- Quiet mode suppresses progress
- Final stdout line remains receipt path

### Phase 8: Receipt verification and fallback receipts

Objective: Make the receipt the durable machine/human output contract.

Files:
- Add: `cmd/sirtopham/receipt.go`
- Add tests: `cmd/sirtopham/receipt_test.go`

Deliverables:
- Compute expected receipt path:
  - default: `receipts/{role}/{chain-id}.md`
  - override from `--receipt-path`
- After turn completion or safety stop:
  - attempt to read receipt from brain backend at expected path
  - if present, validate required frontmatter fields:
    - `agent`
    - `chain_id`
    - `step`
    - `verdict`
    - `timestamp`
    - `turns_used`
    - `tokens_used`
    - `duration_seconds`
- If missing, write fallback receipt with:
  - `step: 1`
  - `verdict: completed_no_receipt` for normal completion without an agent-authored receipt
  - `verdict: safety_limit` for timeout/max-turn/max-token stop
  - body containing summary/final text and metrics
- If fallback receipt path is disallowed by role brain policy, fail the run as infrastructure/config error

Recommended receipt body template:
- `## Summary`
- `## Changes`
- `## Concerns`
- `## Next Steps`

Verification:
- Existing valid receipt passes validation
- Missing receipt triggers fallback write
- Fallback contains final text + metrics
- Disallowed receipt path fails loudly

### Phase 9: Outcome classification and exit codes

Objective: Return the contractually correct process outcome.

Files:
- Modify: `cmd/sirtopham/run.go`
- Tests: `cmd/sirtopham/run_test.go`

Deliverables:
- Map outcomes to exit codes:
  - `0` → normal completion with receipt written/validated
  - `1` → infrastructure/config/runtime failures
  - `2` → safety limit reached (`timeout`, `max turns`, `max tokens`)
  - `3` → explicit escalation
- For v0, determine escalation by receipt verdict when a receipt exists:
  - `escalate` verdict → exit `3`
- If no receipt exists and the agent only returned text, default fallback verdict is `completed_no_receipt`, not escalation

Implementation note:
- `max turns` can map directly to the loop’s iteration ceiling
- `timeout` maps to context deadline
- `max tokens` may require a small wrapper check around aggregate `TurnResult.TotalUsage` and/or future loop enhancement if hard-stop mid-turn is needed; for v0 the plan should prefer true enforcement if it can be added narrowly, otherwise document and implement post-turn classification only if current loop structure blocks narrower enforcement

Verification:
- Timeout returns exit 2 + fallback safety receipt
- Receipt with verdict `escalate` returns exit 3
- Config/provider/backend failures return exit 1

### Phase 10: End-to-end verification

Objective: Prove the command works on the real CLI path.

Files:
- Tests in `cmd/sirtopham/run_test.go`, `cmd/sirtopham/receipt_test.go`, `internal/role/builder_test.go`, config/tool tests

Verification commands:
- `make test`
- `make build`

Optional focused commands during development:
- `go test ./cmd/sirtopham ./internal/config ./internal/tool ./internal/role -tags sqlite_fts5`

Done means:
- All targeted tests pass
- Full `make test` passes
- `make build` passes
- `sirtopham run` prints a receipt path as the final stdout line on success and safety-limit exits

---

## Detailed task list

### Task 1: Add config types for agent roles

Objective: Create the new config schema.

Files:
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

Steps:
1. Add `AgentRoles` to `Config`
2. Add `AgentRoleConfig`
3. Add validation helper for supported tool groups
4. Add tests for YAML parsing and validation
5. Run focused config tests

### Task 2: Add runtime brain path policy fields

Objective: Allow scoped runtime policy to flow into brain tools.

Files:
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` or tool tests

Steps:
1. Add optional `BrainWritePaths` / `BrainDenyPaths` to `BrainConfig`
2. Keep YAML compatibility unchanged for existing configs
3. Confirm no existing codepath breaks when these are unset

### Task 3: Implement brain write path validator

Objective: Enforce allow/deny policy consistently.

Files:
- Add: `internal/tool/brain_paths.go`
- Modify: `internal/tool/brain_write.go`
- Modify: `internal/tool/brain_update.go`
- Test: `internal/tool/brain_test.go`

Steps:
1. Add normalized path validation helper
2. Call it from `brain_write`
3. Call it from `brain_update`
4. Add tests for allow/deny precedence and unrestricted mode
5. Run focused tool tests

### Task 4: Extract shared runtime bootstrap from `serve`

Objective: Reuse runtime wiring for `run`.

Files:
- Add: `cmd/sirtopham/runtime.go`
- Modify: `cmd/sirtopham/serve.go`

Steps:
1. Identify minimally reusable blocks in `serve.go`
2. Extract runtime bundle constructor/helper functions
3. Convert `serve.go` to use helpers
4. Run existing command tests / build

### Task 5: Add role registry builder

Objective: Translate role tool groups into a scoped registry.

Files:
- Add: `internal/role/builder.go`
- Add: `internal/role/builder_test.go`

Steps:
1. Add builder input/output types
2. Implement tool-group mapping
3. Inject scoped `BrainConfig`
4. Add tests for expected tool names and custom tool rejection path

### Task 6: Add CLI command skeleton

Objective: Wire `sirtopham run` into Cobra.

Files:
- Add: `cmd/sirtopham/run.go`
- Modify: `cmd/sirtopham/main.go`
- Test: `cmd/sirtopham/run_test.go`

Steps:
1. Add `newRunCmd`
2. Register it in `main.go`
3. Add flag parsing and argument validation
4. Add helper(s) for task loading, chain ID, receipt path, system prompt loading
5. Add focused parsing tests

### Task 7: Execute one headless run through the existing agent loop

Objective: Complete a single autonomous session.

Files:
- Modify: `cmd/sirtopham/run.go`
- Optional helper: `internal/agent/headless.go`

Steps:
1. Build runtime bundle from extracted helpers
2. Select role and validate unsupported `custom_tools`
3. Build scoped registry and executor
4. Build agent loop with role prompt and resolved limits
5. Create conversation row
6. Call `RunTurn(...)`
7. Capture final metrics/result
8. Add focused command tests with stubs/fakes where practical

### Task 8: Add stderr progress sink

Objective: Provide headless observability without affecting stdout contract.

Files:
- Add: `cmd/sirtopham/run_progress.go`
- Test: `cmd/sirtopham/run_test.go`

Steps:
1. Implement lightweight event sink
2. Subscribe it only when not quiet
3. Verify stderr content in tests

### Task 9: Add receipt verifier and fallback writer

Objective: Persist a canonical receipt every run.

Files:
- Add: `cmd/sirtopham/receipt.go`
- Add: `cmd/sirtopham/receipt_test.go`
- Modify: `cmd/sirtopham/run.go`

Steps:
1. Add expected-path resolver
2. Add receipt frontmatter validator
3. Add fallback receipt formatter/writer
4. Add run-path integration
5. Add tests for present/missing/invalid/disallowed receipt cases

### Task 10: Final exit code plumbing and full verification

Objective: Match the spec’s process contract.

Files:
- Modify: `cmd/sirtopham/run.go`
- Test: `cmd/sirtopham/run_test.go`

Steps:
1. Add exit-code classification
2. Ensure final stdout line is receipt path for exit 0/2 cases
3. Run `make test`
4. Run `make build`
5. Fix any regression narrowly

---

## Acceptance checklist

- [ ] `sirtopham run` exists and is registered in CLI
- [ ] Exactly one of `--task` / `--task-file` is required
- [ ] `--role` selects a config-defined role
- [ ] Role `system_prompt` loads relative to project root
- [ ] Role-scoped tool registry is enforced
- [ ] Unsupported `custom_tools` fail clearly at runtime
- [ ] Brain write allow/deny policy is enforced on writes/updates only
- [ ] Command runs one headless turn via existing `AgentLoop`
- [ ] Receipt is validated if present, or fallback-written if missing
- [ ] `step` defaults to `1`
- [ ] Last stdout line is receipt path on exit 0 and 2
- [ ] Quiet mode suppresses progress output
- [ ] Exit codes match spec intent
- [ ] `make test` passes
- [ ] `make build` passes

---

## Risks and mitigations

Risk: `run.go` duplicates too much of `serve.go`
Mitigation: Extract bootstrap helpers first, then implement `run` on top of them.

Risk: receipt fallback bypasses role brain policy
Mitigation: use the same scoped brain config and normal brain tools/backend policy path; fail loudly if receipt path is disallowed.

Risk: explicit escalation is ambiguous without a receipt
Mitigation: for v0, trust receipt verdict when present; otherwise treat missing-receipt text-only completion as `completed_no_receipt`.

Risk: hard max-token enforcement may not fit current loop cleanly
Mitigation: implement the narrowest true enforcement possible; if loop internals block that, keep classification logic explicit and document the limitation in code/tests instead of silently pretending it is enforced.

---

## Suggested commit sequence

1. `feat(config): add agent role schema and validation`
2. `feat(brain): enforce scoped brain write paths`
3. `refactor(cli): extract shared runtime bootstrap from serve`
4. `feat(role): add role-scoped tool registry builder`
5. `feat(run): add headless run command skeleton`
6. `feat(run): execute headless single-turn agent sessions`
7. `feat(run): add receipt verification and fallback writing`
8. `test(run): cover exit codes, quiet mode, and scoped tools`

---

## Execution note for later implementer

Keep the implementation narrow and bias toward reuse:
- no second agent loop
- no new dependency for glob matching
- no broad config refactor
- no web-surface changes
- no hidden receipt-writing bypass around role policy

The best outcome is a thin command that composes already-landed SirTopham subsystems into a stable headless contract.