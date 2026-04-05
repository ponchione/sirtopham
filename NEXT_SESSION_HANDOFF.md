# Next session handoff

Date: 2026-04-05
Repo: /home/gernsback/source/sirtopham
Branch: main
State: brain runtime is still healthy; the earlier brain-note path ergonomics issue is now fixed live end-to-end; websocket runtime defaults no longer diverge from `/api/config` overrides; the Codex provider's hardcoded model catalog was updated to match the current `codex /model` list seen locally; and live websocket smokes now succeed on `gpt-5.4-mini`, including the exact brain-note-routing flow that previously detoured through repo tools. Nothing pushed.

## Current state

Latest sessions pivoted from brain bring-up to realistic runtime validation and harness-quality follow-through.

### 0. Fresh session outcome: brain-note routing, runtime-default wiring, and Codex model catalog are now reconciled

This session completed the runtime slice that was still open at the end of the previous handoff.

Completed fixes:

1. brain-note tool routing is now steered explicitly
- root cause: there was almost no contrastive guidance telling the model that `notes/...md` / `.brain/notes/...md` are vault-relative brain-note paths and should use brain tools rather than repo file tools
- fix: strengthened `brain_read` and `brain_search` tool descriptions to explicitly mention `notes/...md` / `.brain/notes/...md`, and added base-prompt routing guidance in `internal/agent/loop.go` to prefer `brain_read` / `brain_search` over `file_read` / `search_text` for vault-relative brain-note paths
- regression tests added:
  - `TestBrainToolDefinitionsSteerVaultNotePathsToBrainTools`
  - `TestWithDefaultConfigIncludesBrainNoteRoutingGuidance`

2. websocket runtime defaults now share the same live state as `/api/config`
- root cause: `internal/server/websocket.go` snapshotted `cfg.Routing.Default.*` at server startup, while `internal/server/configapi.go` kept its own separate in-memory override fields
- practical effect: changing runtime defaults through `PUT /api/config` updated the settings/API view but not the websocket turn path
- fix: introduced shared `internal/server/runtime_defaults.go`, passed one shared instance into both `NewConfigHandler(...)` and `NewWebSocketHandler(...)`, and changed websocket model/provider resolution to read the shared effective defaults instead of stale startup fields
- regression test added:
  - `TestWebSocketUsesUpdatedRuntimeDefaultsFromConfigAPI`
- live proof: setting a fake runtime override model via `PUT /api/config` caused a websocket turn to fail with that exact fake model name, proving the websocket path was now reading the shared override rather than a stale startup default

3. Codex model catalog is now aligned with the actual local `codex /model` list
- root cause: `internal/provider/codex/provider.go` advertised a stale static list (`gpt-5.1-codex-mini`, `o3`, `o4-mini`, `gpt-4.1`) that did not match the current models available directly in Codex CLI
- user-confirmed current valid models from `codex /model`:
  - `gpt-5.4`
  - `gpt-5.4-mini`
  - `gpt-5.3-codex`
  - `gpt-5.3-codex-spark`
  - `gpt-5.2-codex`
  - `gpt-5.2`
  - `gpt-5.1-codex-max`
  - `gpt-5.1-codex-mini`
- fix: updated `CodexProvider.Models()` and its regression test to match that current list

Live validation completed this session:
- rebuilt and relaunched the server
- `/api/config` now reports the shared effective default model correctly and the updated Codex provider model list
- websocket basic smoke succeeded on `gpt-5.4-mini` with prompt `Reply with exactly: smoke-ok`
- websocket brain-note routing smoke on `gpt-5.4-mini` succeeded with:
  - turn 1: `brain_write`
  - turn 2: `brain_read`, `brain_search`
  - no `file_read`, `search_text`, or shell detours

Artifacts from this session:
- conversation id: `019d5e54-ac00-7f12-af46-d13dfa4b9000`
- note: `notes/runtime/brain-routing-gpt54mini-1775404100608333852.md`

### 1. Brain runtime remains validated live

Current runtime wiring is still:
- `cmd/sirtopham/serve.go` builds brain via `internal/brain/mcpclient.Connect(...)`
- `internal/brain/mcpclient` starts an in-process MCP server backed by `internal/brain/vault`
- live brain tools operate directly on the configured vault directory (`brain.vault_path`), not through Obsidian Local REST

This remained healthy during later live sessions too:
- `brain_write`
- `brain_read`
- `brain_search`

### 2. Conversation search runtime path was improved in three focused slices

A realistic websocket session exposed a real REST/runtime search bug and then two search-quality problems.

Completed fixes:

1. unquoted hyphenated token queries no longer 500
- root cause: raw FTS5 `MATCH` on strings like `runtime-token-...` produced parser/column errors
- fix: `internal/conversation/manager.go` now retries with a literalized FTS query when raw search fails with syntax/column-style errors
- regression test added in `internal/conversation/manager_test.go`

2. one conversation no longer appears many times in REST search results
- root cause: SQL returns message-level matches and `Manager.Search()` was returning all of them
- fix: `Manager.Search()` now deduplicates by conversation ID
- regression test added in `internal/conversation/manager_test.go`

3. the surviving snippet is now more operator-useful
- root cause: after dedupe, the first-ranked row was often tool-output-heavy instead of natural-language assistant/user text
- fix: search now scores per-message snippets and prefers better assistant/user natural-language snippets over tool-output-heavy rows
- `SearchConversations` row shape now includes `m.role`
- files touched:
  - `internal/conversation/manager.go`
  - `internal/conversation/manager_test.go`
  - `internal/db/query/conversation.sql`
  - `internal/db/conversation.sql.go`

Operator-useful result:
- REST search for unique runtime tokens now returns one result per conversation
- the chosen snippet is materially better than before

### 3. Longer real-world websocket soak session completed

Ran a longer websocket-driven soak session with a purpose-built Go client.

Artifacts:
- client: `/tmp/ws_soak_runtime.go`
- output bundle: `/tmp/soak-token-1775384690184819403-soak.json`
- conversation id: `019d5d2c-7e09-7d74-8b2e-475333298000`
- title: `Comparing WebSocket and REST conversation actions`
- note: `notes/runtime/soak-token-1775384690184819403.md`

Soak coverage:
- 7 turns
- mixed `brain_write`, `brain_read`, `brain_search`, `file_read`, `search_text`
- multi-turn reasoning about websocket flow, search shaping, and runtime behavior

What looked healthy:
- websocket turn loop remained stable across 7 turns
- title generation worked
- turn numbering/persistence stayed coherent
- conversation search improvements held up live
- brain tool flow remained healthy end to end
- context assembly latency stayed reasonable in this run

### 4. Most important new issue surfaced by the soak session

The most valuable new finding was not another search problem.
It was a context-assembly / retrieval heuristic issue.

Observed symptom:
- logs showed false-positive file/path interpretation from ordinary prose, for example:
  - `context retrieval file read failed path=search/title/runtime`

That came from prompt wording like:
- `search/title/runtime helpers`

Interpretation:
- the analyzer / extractor / retrieval path was too eager to promote slash-delimited prose into explicit file candidates
- this polluted logs, could distort retrieval/context assembly, and was likely to matter in many realistic operator prompts

### 5. First context-assembly hardening slice is now complete

Completed in this session:
- `internal/context/analyzer.go` now rejects unanchored multi-segment slash-delimited prose like `search/title/runtime`
- real repo-style explicit paths still pass, for example `internal/server/websocket.go`
- analyzer observability now records rejected candidates as `file_ref_rejected` signals with a stable reason such as `unanchored_multi_segment_path`
- momentum extraction was updated to match the new path-normalization signature without changing its behavior for real paths

Focused tests added/updated:
- `TestRuleBasedAnalyzerRejectsSlashDelimitedProseButKeepsRealPaths`

### 6. Operator-facing signal visibility improved too

Completed in this session:
- the web context inspector signal badges now distinguish `file_ref_rejected`
- rejected path candidates render as `rejected: <candidate>` instead of only showing the internal reason token
- tooltip text now makes the rejection reason explicit while keeping accepted file refs readable as `file: ...`

Validation:
- `npm run build` in `web/`

### 7. Live runtime proof is now complete, with one extra analyzer cleanup

Completed in this session:
- ran a fresh live websocket/context-report probe using a realistic `search/title/runtime` prompt shape
- confirmed the resulting context report includes:
  - accepted explicit file ref for `internal/server/websocket.go`
  - rejected file candidate signal for `search/title/runtime`
- confirmed the earlier runtime-only false-positive symptom is gone in the actual turn path
- the live run also exposed a smaller analyzer quality issue: sentence-capitalized prose words like `Investigate` / `Treat` were being inferred as `symbol_ref`
- fixed that by tightening symbol detection so ordinary sentence capitalization no longer counts as a code identifier while real PascalCase/camelCase identifiers still do

Focused tests added/updated:
- `TestRuleBasedAnalyzerIgnoresSentenceCapitalizationAsSymbolReference`

Interpretation now:
- the original false-positive path class has a deterministic analyzer-level guard and live runtime proof
- the main operator-facing inspector is clearer about accepted vs rejected explicit path candidates
- the remaining question in this area is no longer correctness of the explicit-file path guard; it is whether semantic query / retrieval relevance for these prompts is good enough to stop here

### 8. Follow-up retrieval relevance check led to one small query-extraction hardening slice

Completed after the handoff above:
- inspected `/tmp/ws_context_probe_output.json`
- confirmed that the saved context report still carried RAG hits like `internal/conversation/title.go` for the prompt family centered on `search/title/runtime`
- the main cause was not explicit-file misclassification anymore; it was that the semantic query extractor still left analyzer-rejected slash-delimited prose in the generated retrieval queries
- fix: `internal/context/query.go` now treats `file_ref_rejected` analyzer candidates as exclusions for semantic-query generation, just like explicit file/symbol refs are already excluded
- effect: rejected pseudo-paths such as `search/title/runtime` are removed before semantic query normalization, so they no longer bias retrieval toward unrelated `title` / `runtime` chunks

Focused regression test added:
- `TestHeuristicQueryExtractorExcludesRejectedSlashDelimitedProse`

Interpretation now:
- the prompt family that originally surfaced the false-positive path issue now has protection at both stages:
  - analyzer stage: rejected pseudo-path does not become an explicit file read
  - semantic-query stage: the same rejected pseudo-path does not leak into RAG queries either
- because of that, another retrieval-ranking code slice no longer looks justified from the current evidence alone

### 9. Prompt-boundary / cache-config reconciliation slice is now complete

Completed in this session:
- verified that `agent.cache_system_prompt`, `agent.cache_assembled_context`, and `agent.cache_conversation_history` were previously present in config defaults but were not actually wired through `AgentLoopConfig` into `PromptBuilder`
- fixed that wiring so the Anthropic prompt-builder path now honors those three toggles as real per-block controls
- kept the provider contract explicit: only Anthropic uses explicit `cache_control` markers; non-Anthropic providers still ignore these toggles because they do not use explicit cache breakpoints in the request shape
- surfaced the three cache toggles in `GET /api/config` and on the Settings page so operators can see the effective runtime configuration without reading YAML
- reconciled prompt/cache docs to the actual implementation:
  - `docs/specs/05-agent-loop.md` now states the toggles are real Anthropic-only controls
  - `docs/specs/06-context-assembly.md` now reflects request rebuilds instead of an in-memory `_cached_system_prompt`
  - compression-layer docs that still mentioned `_cached_system_prompt` were updated to the same reality

Focused regression tests added/updated:
- `TestBuildPromptAnthropicHonorsPerBlockCacheToggles`
- `TestBuildPromptAnthropicDisablesAllCacheMarkersWhenTogglesOff`
- `TestGetConfigIncludesToolOutputLimitAndStoreRoot` now also asserts the three cache-toggle fields

### 10. Brain-note path false positives from live history are now fixed too

Fresh live runtime evidence from a new websocket soak exposed one more concrete context-quality issue outside the earlier `search/title/runtime` family.

Observed symptom:
- follow-up turns that mentioned brain note paths like `notes/runtime/soak-token-...md` triggered noisy logs such as:
  - `context retrieval file read failed path=notes/runtime/soak-token-...md`
- root cause: the analyzer treated vault-rooted brain note paths as explicit repo-file refs, but explicit-file retrieval resolves against `project_root`, not the brain vault

Completed in this session:
- `internal/context/analyzer.go` now rejects vault-rooted note paths like `notes/...md` and `.brain/notes/...md` as explicit repo-file candidates
- those paths now emit a stable `file_ref_rejected` signal with reason `vault_rooted_note_path`
- because rejected file refs are already excluded from semantic-query generation, the same note-path token no longer pollutes explicit-file retrieval or downstream query extraction

Focused regression tests added:
- `TestRuleBasedAnalyzerRejectsVaultRootedNotePathsAsExplicitFiles`

Live validation:
- rebuilt the binary and reran the longer websocket soak client
- confirmed the new soak still completed successfully
- confirmed the server log no longer emitted `context retrieval file read failed` warnings for `notes/runtime/...md` during later turns

Interpretation now:
- slash-delimited prose false positives are fixed
- rejected pseudo-paths are stripped from semantic-query generation
- Anthropic prompt-cache toggles are wired end to end
- vault-rooted brain note paths from live history no longer masquerade as repo explicit files
- so this whole recent context-quality pocket now looks even more complete; the next session should only return here if fresh live evidence exposes another distinct path/query class

### 11. Required index excludes now survive local YAML overrides

Fresh follow-up from the latest soak pointed to one adjacent search/indexing risk: even though generated init configs already exclude `.brain/**`, the checked-in local `sirtopham.yaml` override only excluded `.git` and `.sirtopham`, which meant hidden vault metadata could still re-enter code search if operators trimmed the exclude list.

Root cause:
- `config.Default()` had sane index excludes including `**/.git/**`, `**/vendor/**`, and `**/node_modules/**`
- `cmd/sirtopham/init.go` was even stricter and generated `**/.brain/**`
- but YAML list unmarshalling replaced `Index.Exclude` entirely, and `normalize()` was not re-appending required hidden-state excludes after config load
- practical effect: a local config that only listed a subset of excludes could accidentally re-index `.brain/**` and therefore `.brain/.obsidian/workspace.json` noise

Completed in this session:
- `internal/config/config.go` now defines a small required exclude set for index safety:
  - `**/.git/**`
  - `**/.sirtopham/**`
  - `**/.brain/**`
  - `**/node_modules/**`
  - `**/vendor/**`
- `normalize()` now appends any missing required patterns without duplicating existing entries
- this keeps operator-authored config overrides working while preventing hidden state / vault metadata from re-entering the code index by accident

Focused regression test added:
- `TestLoadAppendsRequiredIndexExcludesWhenCustomListOmitsThem`

Validation:
- `go test -tags sqlite_fts5 ./internal/config -run TestLoadAppendsRequiredIndexExcludesWhenCustomListOmitsThem -count=1`
- `go test -tags sqlite_fts5 ./internal/config -count=1`
- `make test`
- `make build`
- fresh live soak: `go run -tags sqlite_fts5 /tmp/ws_soak_runtime.go`
- fresh REST check: `curl -sS 'http://localhost:8090/api/conversations/search?q=soak-token-1775395201890645533'`
- fresh vault search check: `search_files("soak-token-1775395201890645533", path="./.brain", output_mode="files_only")`

Interpretation now:
- brain-note explicit-file false positives are fixed in context assembly
- hidden vault metadata is now also guarded at config/index setup time
- a fresh live soak after the config hardening produced token `soak-token-1775395201890645533`, and the token was found only in `./.brain/notes/runtime/soak-token-1775395201890645533.md`
- the corresponding REST conversation search result was clean and operator-useful
- server logs for that run showed no `.brain/.obsidian/workspace.json`-style noise and no `context retrieval file read failed` warning for the soak note path
- this meaningfully reduces the chance that `.brain/.obsidian/workspace.json` or similar vault state will pollute `search_text` / RAG just because a local config list was too narrow

### 12. Search snippets now strip FTS highlight tags from assistant JSON/text too

A fresh realistic websocket soak against the newly built binary exposed one more operator-facing search issue in the already-improved conversation-search path.

Observed symptom:
- `/api/conversations/search?q=<soak-token>` returned a good conversation result, but the `snippet` still contained raw SQLite FTS highlight markup such as `<b>...</b>` when the matched row came from assistant JSON/text content
- this was especially visible for note-path style snippets like:
  - `Path: notes/runtime/<b>soak-token-...</b>.md ...`

Root cause:
- `sanitizeSearchSnippet()` stripped `<b>` tags only from the raw outer snippet string for tombstone detection
- but the assistant-JSON extraction path (`sanitizeAssistantSnippetHeuristically` plus `ContentBlocksFromRaw`) was still operating on the highlighted JSON/text and then returning extracted text with the tags preserved

Completed in this session:
- `internal/conversation/manager.go` now normalizes search snippets through a shared highlight-strip helper before both heuristic assistant-text extraction and JSON block parsing
- extracted assistant text, truncated-text fallback, and tool-call summaries now all return plain operator-facing text without embedded FTS markup
- non-JSON fallback paths now also return the unhighlighted snippet instead of the original highlighted raw string

Focused regression tests updated:
- `TestSearchSnippetExtractsAssistantTextFromJSONBlocks`
- `TestSearchSnippetSanitizesTruncatedTextJSON`
- `TestManagerSearchSanitizesNormalAssistantToolJSONSnippets`
- `TestManagerSearchPrefersNaturalLanguageSnippetOverToolOutput`

Live validation:
- rebuilt and reran the websocket soak client
- fresh token: `soak-token-1775397162232844728`
- fresh artifact: `/tmp/soak-token-1775397162232844728-soak.json`
- confirmed `/api/conversations/search?q=soak-token-1775397162232844728` returned a clean snippet with no `<b>` tags

Interpretation now:
- the conversation-search path is not only deduplicating and choosing better snippets; it is also returning cleaner operator-facing snippet text in the REST API
- if search quality is revisited again, the next likely issue is not FTS highlight leakage in assistant JSON anymore
- the freshest adjacent runtime evidence is instead that raw file/codebase search tooling can still surface `.brain/.obsidian/workspace.json` when a just-written note remains open in the vault UI, so if another concrete search-noise slice is needed, investigate whether that tool/runtime surface should inherit stronger hidden-state exclusions too

### 13. `search_text` now blocks explicit hidden-state scopes too

The next narrow runtime slice took that adjacent search-noise evidence and checked whether the raw ripgrep-backed `search_text` tool could still be steered into hidden vault state explicitly.

Observed symptom / reproduction:
- default repo-wide `search_text` already avoided hidden files in many cases because ripgrep does not traverse hidden paths unless asked
- but a focused tool test showed that an explicit scope like `path: ".brain"` bypassed the practical protection and returned note contents from hidden vault state
- this meant operator/tool prompts could still surface `.brain` note contents or `.obsidian` metadata noise just by aiming the raw search tool at those directories

Completed in this session:
- `internal/tool/search_text.go` now treats `.git`, `.sirtopham`, `.brain`, and `.obsidian` as hidden-state search excludes
- explicit scoped searches into those paths now short-circuit to the normal `No matches found for pattern: '...'` success result instead of traversing hidden-state directories
- repo-wide ripgrep excludes were updated to include the same hidden-state directories for consistency with the new scoped-path guard

Focused regression test added/expanded:
- `TestSearchTextExcludesBrainAndWorkspaceHiddenStateByDefault`
  - covers both unscoped search and explicit `path: ".brain"`

Validation:
- `go test -tags sqlite_fts5 ./internal/tool -run TestSearchTextExcludesBrainAndWorkspaceHiddenStateByDefault -count=1`
- `go test -tags sqlite_fts5 ./internal/tool -run 'TestSearchText(Success|NoResults|FileGlob|Regex|ContextLines|MaxResultsIsGlobalAcrossFiles|ExcludesBrainAndWorkspaceHiddenStateByDefault|PathScope|PathTraversal|PathAbsolute)$' -count=1`
- `go test -tags sqlite_fts5 ./internal/tool -count=1`
- `make test`
- live websocket probe via `/tmp/ws_hidden_search_probe.go`

Live validation:
- rebuilt server already running from the freshly built binary
- websocket probe prompt: use `search_text` with `pattern=workspace.json` and `path=.brain`
- actual assistant result: `No matches found for pattern: 'workspace.json'`

Interpretation now:
- the earlier adjacent search-noise concern is materially reduced: operator prompts cannot trivially force the raw `search_text` tool into hidden vault state anymore
- if search noise resurfaces again, the next likely place is no longer explicit `search_text path=.brain`; it would be another tool/runtime surface with its own path policy

### 14. `search_text` file-glob overrides were still able to re-include hidden vault state

A fresh realistic websocket soak immediately surfaced one more concrete hole in the same tool surface.

Observed symptom / reproduction:
- during a live runtime turn, the model called `search_text` with:
  - `pattern: "soak-token-1775399818325593258"`
  - `file_glob: "*"`
  - empty `path`
- despite the earlier hidden-state guard, the tool result still leaked:
  - `./.brain/.obsidian/workspace.json`
  - `./.brain/notes/runtime/soak-token-1775399818325593258.md`
- root cause: ripgrep applies later `--glob` rules last, so appending the user file glob after the default negative globs let a broad include like `*` effectively undo the hidden-state exclusions

Completed in this session:
- `internal/tool/search_text.go` now applies any user `file_glob` before the built-in hidden-state exclude globs, so the required exclusions stay authoritative even when the model/operator asks for a broad include glob
- `internal/tool/search_text_test.go` now extends the hidden-state regression to cover runtime-style calls with:
  - `file_glob: "*"`
  - `file_glob: "*", path: "."`

Focused regression test proof:
- `TestSearchTextExcludesBrainAndWorkspaceHiddenStateByDefault` failed before the fix with live hidden hits from `.brain/note.md` and `.brain/.obsidian/workspace.json`
- the same test passes after the glob-order change

Validation:
- `go test -tags sqlite_fts5 ./internal/tool -run TestSearchTextExcludesBrainAndWorkspaceHiddenStateByDefault -count=1`
- `go test -tags sqlite_fts5 ./internal/tool -run 'TestSearchText(Success|NoResults|FileGlob|Regex|ContextLines|MaxResultsIsGlobalAcrossFiles|ExcludesBrainAndWorkspaceHiddenStateByDefault|PathScope|PathTraversal|PathAbsolute)$' -count=1`
- `make test`
- `make build`
- fresh live websocket probe via `/tmp/ws_search_glob_hidden_probe.go`

Live validation:
- after rebuilding and restarting the server from the new binary, the probe prompt asked the model to use `search_text` for the earlier soak token with `file_glob *` and no path
- actual assistant result: `No matches found in any files.`

Interpretation now:
- the hidden-state guard for `search_text` is no longer limited to explicit `.brain` path scopes
- broad runtime/model-generated include globs no longer re-open `.brain` / `.obsidian` leakage through the raw search tool
- if similar leakage resurfaces again, it is likely a different search/runtime surface rather than this specific ripgrep glob-order bug

### 15. Fresh default/runtime probe exposed a brain-note tool-selection ergonomics gap

A new live websocket probe moved away from the earlier `search/title/runtime` family and instead exercised runtime defaults, hidden-state search policy, and settings visibility.

Probe artifact:
- `/tmp/ws_runtime_probe_defaults.go`
- output: `/tmp/defaults-probe-1775400659005694387-probe.json`
- conversation id: `019d5e20-283e-7631-8ff2-66780808f000`
- title: `Runtime Default Provider and Model Exposure`
- note: `notes/runtime/defaults-probe-1775400659005694387.md`

What looked healthy:
- `/api/config` still reported coherent runtime defaults (`codex` / `gpt-5.1-codex-mini`) and the cache-toggle fields
- the earlier `search_text file_glob=*` hidden-state fix held up live: the tool returned no matches for the probe token even after a brain note had been written
- the settings/API/defaults/cache-toggle path looked coherent in this runtime slice

Most useful new finding:
- the agent still handled an explicit brain-note path awkwardly during a realistic follow-up turn
- prompt asked it to read `notes/runtime/defaults-probe-1775400659005694387.md` and search the brain
- instead of going directly to `brain_read` / `brain_search`, it spent extra iterations on:
  - `file_read` for `notes/runtime/...md` -> `File not found`
  - `search_text` for the token -> no matches
  - `shell` with `ls notes/runtime` -> `No such file or directory`
  - only then `brain_search`, which succeeded immediately

Interpretation:
- the earlier path guards are still correct: brain notes should not be treated as repo-root files or repo search results
- but operator ergonomics are still weaker than they should be when the prompt explicitly names a vault-relative note path
- the concrete runtime waste here is not retrieval noise; it is tool-selection confusion between repo file paths and brain-vault note paths

Best next direction from this evidence:
- focus the next slice on brain-note path/tool-selection ergonomics rather than more search/context tuning
- likely target: make prompts that mention `notes/...md` or `.brain/notes/...md` more reliably choose `brain_read`/`brain_search` first and avoid fallback `file_read` / raw shell probing
- a good low-churn starting point is to inspect brain tool descriptions / prompt guidance / any path-aware routing hints before changing broader architecture

## Files changed in these sessions

Code:
- `internal/context/analyzer.go`
- `internal/context/analyzer_test.go`
- `internal/context/momentum.go`
- `internal/context/query.go`
- `internal/context/query_test.go`
- `internal/context/types.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/agent/loop.go`
- `internal/agent/prompt.go`
- `internal/agent/prompt_test.go`
- `internal/conversation/manager.go`
- `internal/conversation/manager_test.go`
- `internal/db/query/conversation.sql`
- `internal/db/conversation.sql.go`
- `internal/server/configapi.go`
- `internal/server/configapi_test.go`
- `internal/tool/search_text.go`
- `internal/tool/search_text_test.go`
- `cmd/sirtopham/serve.go`
- `web/src/components/inspector/context-inspector.tsx`
- `web/src/pages/settings.tsx`
- `web/src/types/metrics.ts`
- `docs/specs/05-agent-loop.md`
- `docs/specs/06-context-assembly.md`
- `docs/layer3/07-compression-engine/epic-07-compression-engine.md`
- `docs/layer3/07-compression-engine/task-06-fallback-and-cache-invalidation.md`
- `NEXT_SESSION_HANDOFF.md`

Runtime validation helpers / artifacts:
- `/tmp/ws_context_probe.go`
- `/tmp/ws_context_probe_output.json`
- `/tmp/ws_hidden_search_probe.go`
- `/tmp/ws_search_glob_hidden_probe.go`
- `/tmp/ws_runtime_probe_defaults.go`

Runtime artifacts / local notes:
- `.brain/notes/runtime/runtime-token-1775380560910306008.md`
- `.brain/notes/runtime/runtime-token-1775381019630534665.md`
- `.brain/notes/runtime/soak-token-1775384690184819403.md`

Transient local helpers (not repo files):
- `/tmp/ws_realistic_runtime.go`
- `/tmp/ws_soak_runtime.go`
- `/tmp/soak-token-1775384690184819403-soak.json`

## Tests / validation run

Targeted regression tests:
- `go test -tags sqlite_fts5 ./internal/context -run TestRuleBasedAnalyzerRejectsSlashDelimitedProseButKeepsRealPaths -count=1`
- `go test -tags sqlite_fts5 ./internal/context -run TestRuleBasedAnalyzerIgnoresSentenceCapitalizationAsSymbolReference -count=1`
- `go test -tags sqlite_fts5 ./internal/context -run TestRuleBasedAnalyzerRejectsVaultRootedNotePathsAsExplicitFiles -count=1`
- `go test -tags sqlite_fts5 ./internal/context -run TestHeuristicQueryExtractorExcludesRejectedSlashDelimitedProse -count=1`
- `go test -tags sqlite_fts5 ./internal/config -run TestLoadAppendsRequiredIndexExcludesWhenCustomListOmitsThem -count=1`
- `go test -tags sqlite_fts5 ./internal/agent -run TestBuildPrompt -count=1`
- `go test -tags sqlite_fts5 ./internal/conversation -run TestManagerSearchHandlesUnquotedHyphenatedQueries -count=1`
- `go test -tags sqlite_fts5 ./internal/conversation -run TestManagerSearchDeduplicatesConversationResults -count=1`
- `go test -tags sqlite_fts5 ./internal/conversation -run TestManagerSearchPrefersNaturalLanguageSnippetOverToolOutput -count=1`
- `go test -tags sqlite_fts5 ./internal/server -run TestGetConfigIncludesToolOutputLimitAndStoreRoot -count=1`

Broader targeted suites:
- `go test -tags sqlite_fts5 ./internal/context -run 'TestHeuristicQueryExtractor|TestRuleBasedAnalyzer' -count=1`
- `go test -tags sqlite_fts5 ./internal/context -count=1`
- `go test -tags sqlite_fts5 ./internal/agent ./internal/server ./internal/config -count=1`
- `go test -tags sqlite_fts5 ./internal/conversation -run 'TestManagerSearch|TestSearchSnippet' -count=1`
- `go test -tags sqlite_fts5 ./internal/server -run TestSearchConversations -count=1`
- `make test`

Build validation:
- `npm run build` in `web/`
- `make build`

Live runtime validation:
- `./bin/sirtopham serve --config /home/gernsback/source/sirtopham/sirtopham.yaml`
- `go run -tags sqlite_fts5 /tmp/ws_context_probe.go`
- `go run -tags sqlite_fts5 /tmp/ws_realistic_runtime.go`
- `go run -tags sqlite_fts5 /tmp/ws_soak_runtime.go`
- `go run -tags sqlite_fts5 /tmp/ws_hidden_search_probe.go`
- `go run -tags sqlite_fts5 /tmp/ws_search_glob_hidden_probe.go`
- `go run -tags sqlite_fts5 /tmp/ws_runtime_probe_defaults.go`
- direct REST checks against `/api/conversations/search`
- direct REST check against `/api/config`

Notes:
- `go run` of the temporary websocket clients initially failed because `nhooyr.io/websocket` default read limits were too small for larger event payloads; the soak helper needed `conn.SetReadLimit(2 << 20)` to avoid `failed to read: read limited at 32769 bytes`
- plain `go run -tags sqlite_fts5 ./cmd/sirtopham serve ...` is still not the preferred bring-up path because the built binary / Makefile wrapper remains the reliable operational path here

## Important current reality

What now looks materially healthy:
- interrupted-tool tombstone search/history behavior
- brain tool bring-up on the repo-local MCP/vault path
- multi-turn websocket runtime under a moderate realistic session
- REST conversation search for realistic unique tokens
- prompt-family handling for rejected slash-delimited pseudo-paths across both analyzer and semantic-query stages
- vault-rooted brain note paths from history no longer trigger noisy explicit-file retrieval under `project_root`
- prompt/cache boundary docs and Anthropic cache-toggle wiring now match each other end to end

What now looks like the highest-value unresolved runtime issue:
- not the explicit-path false positive anymore; that path is fixed and validated live
- not prompt-boundary/cache-config ambiguity anymore; that contract is now explicit in both code and docs
- not FTS snippet highlight leakage in REST search anymore; that path is now fixed too
- not `search_text` hidden-state leakage via `.brain` path scopes or `file_glob=*` anymore; those paths are now fixed too
- not brain-note path ergonomics anymore; that path is now fixed and validated live on `gpt-5.4-mini`
- the freshest concrete runtime follow-through is provider/model capability truthfulness: the websocket/runtime-default path now honors shared live defaults, but Codex model availability is still maintained as a static list and should ideally stop drifting from the real CLI/runtime capability surface

## Prompt / context architecture reality to remember

Context assembly is not the static system prompt.
Current prompt structure is:
- Block 1: static/thin base prompt (`BasePrompt` in `internal/agent/prompt.go`)
- Block 2: assembled context from `internal/context/assembler.go`
- Block 3: conversation history

So the next context-assembly slice should primarily target:
- analyzer / extractor / retrieval heuristics
not a rewrite of the static base prompt.

But the next plan should still explicitly document prompt-boundary ownership:
- what belongs in the stable base prompt
- what belongs in dynamic assembled context
- what junk/false positives must never be promoted into Block 2

Also note:
- config has `agent.cache_system_prompt`, `agent.cache_assembled_context`, and `agent.cache_conversation_history`
- those toggles now flow through `cmd/sirtopham/serve.go` -> `AgentLoopConfig` -> `PromptConfig` -> `PromptBuilder`
- they only affect Anthropic's explicit `cache_control` markers; other providers remain provider-driven and ignore the toggles

## Recommended next session plan

Best next session should keep the context/search pocket parked and move on from the now-fixed brain-note routing/runtime-default mismatch.

Recommended plan:
1. harden Codex model-capability truthfulness
- inspect whether `CodexProvider.Models()` should remain a manually curated static list or switch to a runtime-discovered source (for example shelling out to `codex /model` with a parseable mode, if one exists, or another low-churn capability probe)
- goal: stop `/api/config` and provider metadata from drifting behind the real Codex CLI capability surface again

2. reconcile provider-health semantics after bad runtime overrides
- in this session, intentionally setting a fake runtime override model correctly proved websocket/runtime-default wiring, but it also left the Codex provider marked unhealthy in `/api/config` until a later successful path
- inspect whether a bad requested model should poison provider health globally, or whether that should remain a per-request/runtime error instead

3. if capability discovery is too much churn for that day, take the deterministic test cleanup slice
- `go test -tags sqlite_fts5 ./internal/tool -count=1` exposed `TestBrainSearchMaxResults` flakiness from fake-backend ordering (`a.md` vs `b.md`)
- this looks like test nondeterminism rather than a runtime regression from this session
- low-churn follow-up: make fake brain-search ordering deterministic and rerun the broad tool suite

Why this is the best next slice:
- the brain-note routing/runtime-default blocker is now closed
- live websocket turns already succeed on `gpt-5.4-mini`
- the next likely time-waster is stale or misleading model capability metadata rather than another retrieval/tool-choice issue

## Useful commands

- `make build`
- `make test`
- `./bin/sirtopham serve --config /home/gernsback/source/sirtopham/sirtopham.yaml`
- `go test -tags sqlite_fts5 ./internal/conversation -run 'TestManagerSearch|TestSearchSnippet' -count=1`
- `go run -tags sqlite_fts5 /tmp/ws_realistic_runtime.go`
- `go run -tags sqlite_fts5 /tmp/ws_soak_runtime.go`
- `curl -sS 'http://localhost:8090/api/conversations/search?q=<token>'`

## Operator preferences to remember

- keep responses short and focused
- do not report git status unless asked
- do not push unless explicitly asked

## Bottom line

The practical brain/runtime validation slice is no longer the blocker. Search runtime behavior is materially better, the slash-delimited pseudo-path prompt family is now hardened at both analyzer and semantic-query stages, and the prompt/cache boundary contract now matches reality in code, UI/API visibility, and docs. The next session should probably move on to the next concrete runtime blocker rather than keep polishing this area speculatively.