# Next session handoff

Date: 2026-04-01
Repo: /home/gernsback/source/sirtopham
Branch: main
State: working tree dirty for cancellation-cleanup/tombstone follow-through files plus docs, ahead of origin/main by 5 commits

What is actually complete from the Claude Code / sirtopham handoff

This is not a full completion of the entire Claude Code retrofit plan.

What is substantially complete is the highest-priority tool-output slice plus a few adjacent correctness/documentation follow-ups:

1. Aggregate tool-result budgeting in the real agent loop
- Fresh tool results are budgeted in aggregate before they are appended into the next model-visible request.
- Budgeting is deterministic:
  - larger results are reduced first
  - `file_read` is deprioritized for replacement when another tool can absorb the cut
- Main files:
  - `internal/agent/loop.go`
  - `internal/agent/toolresult_budget.go`
  - `internal/agent/loop_compression_test.go`

2. Persisted oversized tool outputs with structured refs
- Oversized non-`file_read` tool outputs can be persisted to disk and replaced with a structured reference + preview.
- Current model-visible format includes:
  - `[persisted_tool_result]`
  - `path=...`
  - `tool=...`
  - `tool_use_id=...`
  - `preview=`
- Tiny-budget fallback preserves the path when possible.
- Main files:
  - `internal/agent/toolresult_budget.go`
  - `internal/agent/toolresult_store.go`
  - `internal/agent/toolresult_budget_test.go`
  - `internal/agent/loop_compression_test.go`

3. Configurable persisted artifact storage root
- Persisted tool-result artifacts are no longer tempdir-only in practice; the root is configurable.
- Config field added:
  - `agent.tool_result_store_root`
- Wired through config -> serve -> agent loop.
- Main files:
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `cmd/sirtopham/serve.go`
  - `internal/agent/loop.go`
  - `internal/agent/toolresult_store_config_test.go`

4. Observability for aggregate budgeting
- Aggregate budget helper now returns a report struct.
- Agent loop emits debug logging when fresh tool results are replaced.
- Current report includes:
  - original chars
  - final chars
  - max chars
  - replaced result count
  - persisted result count
  - inline-shrunk result count
  - chars saved
- Main files:
  - `internal/agent/toolresult_budget.go`
  - `internal/agent/toolresult_budget_test.go`
  - `internal/agent/loop.go`

5. Tool execution recording correctness fix
- The normal loop path now correctly passes execution metadata through the tool adapter so `tool_executions` rows are not skipped.
- Main files:
  - `internal/tool/execution_context.go`
  - `internal/tool/adapter.go`
  - `internal/agent/loop.go`
  - `internal/tool/adapter_persistence_test.go`
  - `internal/agent/loop_test.go`

6. API/settings visibility for useful runtime config
- `/api/config` now exposes:
  - `agent.tool_output_max_tokens`
  - `agent.tool_result_store_root`
- Settings page shows those values read-only.
- Main files:
  - `internal/server/configapi.go`
  - `internal/server/configapi_test.go`
  - `web/src/types/metrics.ts`
  - `web/src/pages/settings.tsx`

7. Persistence atomicity contract clarified
- Current contract is now explicitly documented:
  - `PersistIteration` is atomic for `messages`
  - `tool_executions` and `sub_calls` are best-effort and non-atomic relative to message persistence
  - cancellation cleanup still deletes all three together for in-flight iterations
- Main files:
  - `internal/conversation/history.go`
  - `docs/specs/08-data-model.md`
  - `TECH-DEBT.md`

8. File-edit hardening is now substantially complete
- `file_edit` now enforces a real full-read-before-edit invariant.
- Partial `file_read` results do not satisfy edit preconditions.
- Stale-write detection now happens both before edit planning and immediately before write.
- Successful edits clear the saved read snapshot so a fresh read is required before another edit.
- Recovery-oriented error payloads are now much stronger:
  - `invalid_create_via_edit`
  - `not_read_first`
  - `stale_write`
  - `zero_match`
  - `multiple_matches`
- Zero-match failures include a preview of current file content.
- Multiple-match failures include candidate lines plus candidate snippets, including multiline snippets when `old_str` spans lines.
- Match-analysis/helper logic is now separated into its own file with focused unit tests.
- Main files:
  - `internal/tool/file_read.go`
  - `internal/tool/file_read_state.go`
  - `internal/tool/file_edit.go`
  - `internal/tool/file_edit_analysis.go`
  - `internal/tool/file_edit_test.go`
  - `internal/tool/file_edit_analysis_test.go`
  - `internal/tool/register.go`

9. Cancellation cleanup now has a phase-6/7 follow-through for tombstone-aware downstream consumers
- The loop now builds a structured cleanup plan from explicit in-flight turn state instead of open-coding raw `CancelIteration` calls.
- `loop.Cancel()` is now distinguished from generic external context cancellation:
  - loop-triggered cancel emits `user_interrupted`
  - external context cancellation still emits `user_cancelled`
  - deadlines still emit `context_deadline_exceeded`
- If the assistant already produced a complete tool-use message and the turn is interrupted before tool results are durably persisted, cleanup now persists a coherent interrupted iteration instead of deleting it outright.
- The synthesized tool-result payload is a deterministic text placeholder beginning with `[interrupted_tool_result]` and includes reason/tool/tool_use_id/status fields.
- Partial assistant responses now persist as dedicated tombstone content inside assistant content blocks.
- Interrupt/cancel and stream failure now diverge in durable assistant transcript treatment:
  - interruption/cancel => `[interrupted_assistant]`
  - stream failure => `[failed_assistant]`
- `consumeStream` now returns partial accumulated content on context cancellation so the cleanup path can preserve partial assistant text when available.
- Compression input rendering now collapses assistant tombstones to compact summaries instead of leaking full tombstone payloads / partial text back into compression prompts.
- The web conversation history path now parses persisted assistant JSON content blocks instead of showing raw JSON blobs and renders assistant/tool tombstones as human-readable transcript entries.
- Conversation search snippets now collapse tombstone-bearing assistant/tool payloads to compact summaries instead of leaking raw markers or `partial_text=...` bodies into result previews.
- Title generation now rejects model outputs that look like transcript tombstones instead of persisting marker text as conversation titles.
- This is still not the full Claude-Code-style cleanup model.
- Remaining gap: interrupted assistant/tool state still reuses the existing message/content-block schema rather than a first-class DB record type, and any future export-style transcript consumers still do not have explicit tombstone filtering or rendering rules.
- Main files:
  - `internal/agent/turn_cleanup.go`
  - `internal/agent/turn_cleanup_test.go`
  - `internal/agent/loop.go`
  - `internal/agent/loop_test.go`
  - `internal/agent/stream.go`
  - `internal/agent/stream_test.go`
  - `internal/agent/retry.go`
  - `internal/context/compression.go`
  - `internal/context/compression_test.go`
  - `internal/conversation/manager.go`
  - `internal/conversation/manager_test.go`
  - `internal/conversation/title.go`
  - `web/src/lib/history.ts`
  - `TECH-DEBT.md`

What is NOT complete from the Claude Code / sirtopham handoff

The overall retrofit handoff is still incomplete. The following major areas remain mostly unimplemented:

1. Cancellation cleanup / transcript invariants
- Existing cancellation cleanup is present, tested, and now has a structured planner/executor seam that covers synthesized interrupted tool results, divergent assistant tombstones for interruption vs stream failure, and tombstone-aware compression input rendering.
- The richer Claude-Code-style cleanup model is still not implemented:
  - no first-class DB record type for assistant/tool tombstones yet
  - cleanup still relies on the existing assistant message/content-block schema rather than a richer interrupted-state taxonomy
  - no broader `InflightTurn` / `CleanupPlan` subsystem shared beyond the current agent-loop seam
- Relevant handoff stub:
  - `sirtopham-handoff/stubs/turnstate/turnstate.go`

2. Prompt-cache latching
- No explicit prompt block / cache-latch subsystem from the handoff is implemented.
- Stable-vs-dynamic prompt bytes are not modeled as their own seam yet.
- Relevant handoff stub:
  - `sirtopham-handoff/stubs/promptcache/promptcache.go`

3. Better token-budget accounting
- No `BudgetTracker`-style reserve + estimate + reconcile implementation from the handoff is wired into requests.
- Current system still does not fully embody the handoff’s token-budget plan.
- Relevant handoff stub:
  - `sirtopham-handoff/stubs/tokenbudget/tokenbudget.go`

4. A few tool-output subtleties are only partially done
- There is no dedicated `ToolOutputManager` package boundary yet; the logic currently lives directly in agent-loop helper code.
- No explicit shell/build/test tail-preserving formatter strategy is implemented as a first-class subsystem.
- No formal memoization-by-tool-call-ID subsystem exists beyond the deterministic current-pass behavior.

5. File-write freshness policy is still unresolved
- `file_write` remains the explicit overwrite/create escape hatch.
- The stronger read-state/stale-write contract has been implemented for `file_edit`, not for `file_write`.
- If future correctness work focuses on broader mutation safety, decide whether `file_write` should stay intentionally unconstrained or gain a related freshness policy.

Bottom-line assessment

The right way to describe status now is:
- the top recommendation from the Claude Code handoff is implemented enough to be useful
- the entire Claude Code / sirtopham handoff is NOT complete
- treat the current state as “phase 1 complete”, not “handoff complete”

What is worth double-checking next session

Only a few things feel worth active verification before doing more implementation:

1. Codex runtime path end-to-end
- Codex provider wiring in `serve.go` is now fixed.
- Verify a real codex-authenticated startup and turn with:
  - `codex` present on PATH
  - valid `~/.codex/auth.json`
  - codex selected as the default provider/model in config
- Confirm provider health/model listing/UI behavior is sane when codex is the main runtime path.

2. Obsidian API layer bring-up
- The intended brain vault is repo-local (for example `.brain/`), but the current v0.1 brain tools still operate through the Obsidian Local REST API layer.
- Next session should focus on the concrete bring-up path for that layer, not on replacing it with a different architecture yet:
  - decide exactly how the local repo vault should be surfaced to Obsidian
  - document or script the Local REST API setup path
  - verify read/write/search/list behavior against the repo-local vault

3. Budget/config semantics
- Double-check that runtime config exposure is not confusing between:
  - per-tool output cap (`tool_output_max_tokens`)
  - aggregate next-message fresh-tool-result budgeting (`MaxToolResultsPerMessageChars` in the agent loop)
- If this feels confusing, either document it better or expose the aggregate cap explicitly too.

4. Artifact lifecycle / cleanup policy
- Persisted tool results now accumulate under a configurable root.
- Decide whether this is acceptable as-is or whether old artifacts need cleanup/retention behavior.

Recommended next implementation slice

Unless priorities changed, the best next slice is now:
- Obsidian API layer bring-up for a repo-local brain vault

Why this should be next
- The immediate user goal is early realistic testing, not more Claude-retrofit architecture.
- Codex provider wiring is fixed enough to start real auth/runtime validation.
- The main remaining blocker for brain-backed testing is getting the Obsidian API layer working cleanly with a repo-local `.brain/` vault.

Suggested exact next-session plan

1. Read these first
- `docs/layer4/06-obsidian-client-brain-tools/epic-06-obsidian-client-brain-tools.md`
- `docs/layer4/06-obsidian-client-brain-tools/task-01-obsidian-client.md`
- `internal/brain/client.go`
- `internal/tool/brain_search.go`
- `internal/tool/brain_read.go`
- `internal/tool/brain_write.go`
- `internal/tool/brain_update.go`
- `cmd/sirtopham/serve.go`
- `sirtopham.yaml`

2. Confirm the real bring-up path
- How the repo-local `.brain/` vault should be opened/owned by Obsidian
- What exact Local REST API plugin/runtime setup is required for testing on this machine
- Whether any small config/docs ergonomics are missing for codex-default + repo-local-brain startup

3. Implement the next narrow TDD slice
Minimum worthwhile target now:
- keep codex as the preferred default runtime path
- make Obsidian API bring-up clearer/easier via docs, config surfacing, or light runtime validation
- if a concrete defect appears in the Obsidian client/tool path during live setup, fix that defect directly

4. Validate with focused tests first, then live bring-up
At minimum run the relevant Go suites, then do a real startup test with codex auth plus the repo-local brain vault wired through the Obsidian API layer.

What not to do next session unless there is strong evidence it is worth it
- Do not jump into prompt-cache-latching architecture first.
- Do not do broad token-budget architecture work first.
- Do not reopen file-edit work unless a concrete bug shows up.
- Do not describe the Claude handoff as complete.

Useful recent commits
- `af62069` chore: checkpoint provider and tool-output retrofit work
- `3903a66` feat(agent): structure persisted tool result references
- `80207f0` feat(agent): configure persisted tool result storage root
- `454ad96` feat(agent): report aggregate tool-result budget savings
- `78d7f25` docs: clarify iteration analytics persistence contract
- `5033cf4` feat(settings): expose tool result storage config
- `62d9285` feat(tool): require full reads before file edits
- `f5a7a76` feat(tool): clarify file edit recovery errors
- `6f7c610` feat(tool): enrich file edit disambiguation errors
- `02b0718` feat(tool): add file edit candidate snippets
- `f2e8ecc` feat(tool): improve multiline file edit diagnostics
- `e7de3e3` refactor(tool): separate file edit match analysis

Suggested first commands next session
- `git status --short --branch`
- `go test ./internal/server ./internal/agent/... ./internal/config ./internal/conversation ./cmd/sirtopham && go test -tags sqlite_fts5 ./internal/tool/... ./internal/context`
- then inspect tombstone downstream-consumer paths (`web`, transcript rendering, search/export/title-adjacent utilities) and begin the cancellation/transcript-invariants phase-7 slice
