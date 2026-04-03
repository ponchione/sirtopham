# Next session handoff

Date: 2026-04-03
Repo: /home/gernsback/source/sirtopham
Branch: main
State: working tree dirty; proactive code retrieval was fixed and validated on a smaller external repo; nothing pushed

## What was completed today

1. Stopped treating sirtopham-self as the primary validation target
   - Used `/home/gernsback/source/my-website` as the smoke-test repo instead
   - Created a focused smoke config at `/tmp/sirtopham-smoke.yaml`
   - Disabled brain in the smoke config and excluded docs/specs/audit/plans noise from code indexing

2. Confirmed the backend code-index path works on a normal smaller repo
   - `./bin/sirtopham index --config /tmp/sirtopham-smoke.yaml --json` succeeded cleanly
   - Indexed `my-website` into repo-local state under:
     - `/home/gernsback/source/my-website/.my-website/`
     - code LanceDB path: `.my-website/lancedb/code`
   - This proved the code index itself is functional when not polluted by large markdown/spec trees

3. Tightened the default `sirtopham.yaml` code index scope
   - Removed `**/*.md` from the default include list
   - Added excludes for:
     - `**/docs/**`
     - `**/specs/**`
     - `**/design/**`
     - `**/plans/**`
     - `**/audit/**`
     - `**/README.md`
     - `**/NEXT_SESSION_HANDOFF.md`
     - `**/TECH-DEBT.md`
   - Goal: keep project-brain / documentation material out of the code semantic index

4. Fixed remaining UTF-8 truncation bugs in codeintel producers
   - Root cause: several parser/chunker paths were still slicing raw bytes at `MaxBodyLength`, leaving invalid trailing UTF-8 for LanceDB writes
   - Centralized UTF-8-safe truncation in `internal/codeintel/types.go`
   - Updated all remaining codeintel truncation sites to use the shared helper
   - Added regression tests
   - This removed the earlier invalid UTF-8 upsert failure class

5. Investigated why proactive retrieval still showed `rag_results: null`
   - Symptom on successful live turns: answers were correct, but context reports showed:
     - `rag_results: null`
     - `budget_used: 0`
     - `agent_used_search_tool: 1`
     - UI showed `Reactive search ...` and `Avg hit rate 0%`
   - Verified direct semantic search against the built code index returned relevant hits for the same queries
   - Found the real bug in `internal/vectorstore/store.go`:
     - LanceDB `_distance` was converted to score as `1 - distance`
     - Real distances were often `> 1.0`
     - This produced negative scores
     - Context-layer thresholding (`0.35`) then filtered out every RAG hit

6. Fixed retrieval score calibration
   - Changed score conversion from `1 - distance` to a bounded monotonic transform:
     - `1 / (1 + distance)`
   - Added vectorstore regression tests for the conversion
   - Rebuilt with `make build` so the live binary actually picked up the fix

7. Re-validated proactive retrieval end-to-end on `my-website`
   - Fresh live turn after rebuild:
     - question: `How is blog frontmatter parsed and turned into routes?`
     - context report showed non-empty `rag_results`, non-zero `budget_used`, and `agent_used_search_tool = 0`
   - Another live turn:
     - question: `Where is the mobile navigation behavior implemented, especially opening and closing the sidebar on small screens?`
     - context report again showed non-empty `rag_results` and real RAG budget use
   - Final thorough pass:
     - question: `Trace how document titles are managed across pages, including any shared hook and where different pages set their titles.`
     - context report showed proactive RAG with no reactive search tool use

## Validation run today

Passed:
- `go test ./internal/codeintel/...`
- `CGO_ENABLED=1 CGO_LDFLAGS='-L/home/gernsback/source/sirtopham/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread' LD_LIBRARY_PATH='/home/gernsback/source/sirtopham/lib/linux_amd64' go test -tags sqlite_fts5 ./internal/vectorstore ./internal/context`
- `make build`

Successful runtime validations:
- `./bin/sirtopham index --config /tmp/sirtopham-smoke.yaml --json`
- `./bin/sirtopham serve --config /tmp/sirtopham-smoke.yaml`
- Multiple successful live UI turns against `/home/gernsback/source/my-website`
- Direct context-report inspection via API confirmed proactive RAG is now being persisted and budgeted

Representative successful context report facts after the score fix:
- blog-frontmatter turn:
  - `rag_results`: populated
  - `budget_used`: `1084`
  - `included_count`: `10`
  - `agent_used_search_tool`: `0`
- document-title turn:
  - `rag_results`: populated
  - `budget_used`: `2620`
  - `included_count`: `10`
  - `agent_used_search_tool`: `0`

## Current remaining issue

The next real blocker is no longer indexing or proactive retrieval.

Current blocker:
- Codex auth refresh is still brittle in non-interactive runtime paths
- A final live validation turn failed with:
  - `codex: Codex credential refresh failed (exit 1): Error: stdin is not a terminal`
- This appears to be a separate provider/auth-runtime issue from retrieval

Important nuance:
- The error did NOT invalidate the retrieval fix; subsequent live validations already proved proactive RAG works
- But it is now the most obvious remaining practical runtime reliability problem for same-day use

## Most likely next session focus

Investigate and harden Codex runtime credential refresh so non-interactive server turns do not fail when refresh is attempted.

### Strong hypotheses to test first

1. `internal/provider/codex/credentials.go` still shells out to `codex refresh` too eagerly
   - Current logic in `getAccessToken()`:
     - first tries `readAuthFile()`
     - if file read/token validity check fails, immediately calls `refreshToken(ctx)`
   - If the auth file contains a usable token shape but expiry handling is too strict or parsing is off, this can force an unnecessary CLI refresh in a non-interactive server process

2. `refreshToken()` is not guarding against non-interactive execution
   - Current implementation shells out to:
     - `codex refresh`
   - When the CLI decides it needs interactive behavior, the provider returns the raw failure
   - We probably need a stronger rule: do not attempt interactive refresh in non-interactive server context; fail with a more actionable stale-token error or only use refresh when we know it can succeed unattended

3. There may be a useful comparison path in the local Hermes Agent repo
   - Available reference repo:
     - `/home/gernsback/source/hermes-agent`
   - Useful comparison hints found today:
     - `run_agent.py` uses `resolve_codex_runtime_credentials(...)` rather than directly shelling out from the request path
     - Hermes already has Codex runtime credential handling logic worth inspecting before changing Sirtopham's provider flow
   - Start by checking how Hermes decides when to reuse auth-file state vs when to refresh, and whether it explicitly avoids interactive CLI refresh in server-like paths

## Recommended next session

1. Inspect `internal/provider/codex/credentials.go`
   - Focus on:
     - `getAccessToken()`
     - `readAuthFile()`
     - `refreshToken()`
   - Reproduce the failing path with a focused test first if practical

2. Compare against Hermes Agent local reference
   - Repo:
     - `/home/gernsback/source/hermes-agent`
   - Start around:
     - `run_agent.py` Codex runtime credential refresh path (`resolve_codex_runtime_credentials` call sites)
   - Goal: understand the runtime credential policy Hermes uses for ChatGPT Codex OAuth in non-interactive execution

3. Implement the narrowest viable hardening
   - Prefer:
     - use valid token from `~/.codex/auth.json` whenever present and not truly expired
     - avoid shelling out to `codex refresh` in non-interactive server turns unless there is strong evidence it can succeed unattended
   - If needed, add a clearer provider error that explains refresh requires interactive auth renewal outside the app

4. Re-validate with the same `my-website` smoke config
   - `./bin/sirtopham serve --config /tmp/sirtopham-smoke.yaml`
   - Ask at least one fresh question after forcing the provider path that previously tripped refresh
   - Confirm the turn does not fail with `stdin is not a terminal`

## Useful commands

- Focused build/test:
  - `make build`
  - `CGO_ENABLED=1 CGO_LDFLAGS='-L/home/gernsback/source/sirtopham/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread' LD_LIBRARY_PATH='/home/gernsback/source/sirtopham/lib/linux_amd64' go test -tags sqlite_fts5 ./internal/vectorstore ./internal/context`

- Smoke index on smaller repo:
  - `./bin/sirtopham index --config /tmp/sirtopham-smoke.yaml --json`

- Smoke serve on smaller repo:
  - `./bin/sirtopham serve --config /tmp/sirtopham-smoke.yaml`

- Inspect latest conversations:
  - `curl -fsS http://localhost:8090/api/conversations`

- Inspect context report for a turn:
  - `curl -fsS http://localhost:8090/api/metrics/conversation/<conversation_id>/context/<turn_number>`

## Current git state to expect

Tracked modifications at end of session:
- `internal/vectorstore/store.go`
- `internal/vectorstore/store_test.go`
- `sirtopham.yaml`

Untracked:
- `tmp/`
  - contains temporary debug helpers created during investigation
  - cleanup was not completed because a direct `rm` attempt was blocked by the environment safety layer

## Bottom line

The code-index / proactive-retrieval slice is now genuinely working.
The next session should NOT go back to indexing theory or RAG score debugging unless new evidence appears.
The next practical blocker is Codex non-interactive credential refresh reliability, with `/home/gernsback/source/hermes-agent` available locally as a comparison target for how Hermes handles Codex runtime credentials.
