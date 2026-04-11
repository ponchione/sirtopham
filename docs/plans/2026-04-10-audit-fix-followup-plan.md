# Audit Fix Follow-up Plan

> For Hermes: Use subagent-driven-development skill to implement this plan task-by-task.

Goal: Fix the real audit issue in tagged loose-query fallback, decide and document the intentionally broadened brain-system deltas that were not listed in the change summary, and leave the repo with explicit regression coverage proving the intended behavior.

Architecture: Keep the fix narrow. The only confirmed code bug from the audit is the empty-normalized-token case in `brain_search` tagged fallback. The rest of the follow-up is mostly contract hygiene: add targeted tests for the pathological query shape, explicitly classify the other audited deltas as intended product work versus accidental scope creep, and update handoff/docs so future audits do not flag the same broadened behavior as “missed.”

Tech Stack: Go, existing `internal/tool` and `internal/context` test suites, existing brain/indexstate/server surfaces, repo Makefile test/build flow, markdown docs under `docs/` and root handoff files.

---

## Problem statement

The audit found two classes of issues:

1. Real bug to fix
- `internal/tool/brain_search.go` changed `if matched != len(queryTokens) || matched == 0` to `if matched != len(queryTokens)` in `searchTaggedDocsByLooseQuery`.
- That is only equivalent when `len(queryTokens) > 0`.
- It is not guaranteed here because `Execute()` only rejects an empty raw query, while `normalizeBrainSearchText(query)` can still collapse punctuation-only or separator-only input to `""`.
- Current behavior risk: a punctuation-only query plus tags can match every tagged document because `matched == len(queryTokens) == 0`.

2. Scope/accounting issue to clean up
- Several landed behavior changes were real and tested, but were not named in the fix summary:
  - brain freshness state becomes clean after `sirtopham index brain`
  - `serve` now wires hybrid brain runtime search and a brain vector store
  - `brain_read` now prefers indexed backlinks
  - `/api/project` now surfaces `brain_index`
  - parser preserves explicit `.md` wikilink suffixes
- These do not currently look broken, but they should be made explicit in docs/handoff/change summary so future audits do not treat them as unexplained drift.

---

## Constraints and guardrails

- Keep edits narrow; do not reopen broader brain-system redesign.
- Prefer TDD: add/adjust failing tests first for the empty-token tagged fallback case.
- Preserve the intended hybrid runtime/search/index freshness work already landed.
- Prefer `make test` / `make build`; if running Go directly, use `-tags sqlite_fts5`.
- Do not churn unrelated docs; only update the minimum operator/audit-facing artifacts needed to explain the broadened scope.

---

## Read-this-first execution context

Before implementation, read these files in order:
- `NEXT_SESSION_HANDOFF.md`
- `TECH-DEBT.md`
- `docs/plans/2026-04-09-brain-system-rebuild-implementation-plan.md`
- `internal/tool/brain_search.go`
- `internal/tool/brain_test.go`
- `internal/brain/vault/client_test.go`
- `cmd/sirtopham/index.go`
- `cmd/sirtopham/serve.go`
- `internal/tool/brain_read.go`
- `internal/server/project.go`
- `internal/brain/parser/document.go`

Optional context reads if behavior questions come up:
- `BRAIN_SYSTEM_AUDIT_AND_REBUILD.md`
- `internal/context/retrieval_test.go`
- `internal/context/brain_search_test.go`
- `internal/server/project_test.go`
- `cmd/sirtopham/index_test.go`

---

## Desired end state

Code:
- Tagged loose-query fallback does not return all tagged documents when normalized query tokens are empty.
- The fix is covered by focused tests for punctuation-only and separator-only inputs.
- No intended hybrid brain runtime behavior regresses.

Docs/audit hygiene:
- The broadened behavior changes are explicitly called out in handoff/plan/update notes or a fresh audit follow-up note.
- Future reviewer can distinguish “bug fix required” from “intentional but previously unlisted landed behavior.”

Verification:
- Targeted `internal/tool` tests pass.
- Requested broad package tests still pass.
- `make build` still passes.

---

## Phase 1: Fix the real bug in tagged loose-query fallback

### Task 1.1: Add a focused regression test for empty normalized query tokens

Objective: Prove the current loose-query fallback is unsafe when normalization strips the query to zero tokens.

Files:
- Modify: `internal/tool/brain_test.go`

Step 1: Add a failing test that calls `BrainSearch.Execute()` with:
- a raw query containing only punctuation/separators, for example `"---, \t\n"` or `"!!!"`
- at least one tag
- a fake backend containing multiple tagged docs

Step 2: Assert the result does not return all tagged docs via the loose-query fallback.
Prefer one of these explicit expected contracts:
- safest/minimal: zero results for punctuation-only query plus tags
- acceptable alternate: tag filtering can still apply only if there is a non-empty normalized query token set; otherwise fallback is skipped

Step 3: Also add a direct unit test for `searchTaggedDocsByLooseQuery` if convenient, asserting:
- `normalizeBrainSearchText(query)` can produce `""`
- empty `queryTokens` must not yield a broad match-all result

Step 4: Run:
- `go test -tags sqlite_fts5 -count=1 ./internal/tool -run 'TestBrainSearch.*(Punctuation|LooseQuery|Tagged)'`
Expected initially: FAIL if the current broad-match behavior is still present.

Implementation notes:
- Reuse the fake backend in `internal/tool/brain_test.go`.
- Mirror the normalization/pathological punctuation coverage style already present in `internal/brain/vault/client_test.go`.

### Task 1.2: Implement the narrow code fix

Objective: Prevent empty-token loose fallback from matching all tagged docs.

Files:
- Modify: `internal/tool/brain_search.go`

Recommended implementation:
- After `queryTokens := strings.Fields(normalizeBrainSearchText(query))`, add an early return:
  - `if len(queryTokens) == 0 { return nil, nil }`
- Keep the simplified `if matched != len(queryTokens)` check after that.

Why this shape is preferred:
- It makes the hidden precondition explicit instead of relying on caller behavior that does not actually exist.
- It keeps the later match loop logic simple.
- It preserves the intended equivalence claim for all non-empty-token cases.

Alternative acceptable implementation:
- Restore the old guard with `|| matched == 0`.
- This is slightly less explicit because it leaves the empty-token precondition buried inside the later condition.

Do not:
- Broaden `Execute()` to reject every raw query that normalizes to empty unless you deliberately want a user-facing contract change and update tests/messages accordingly.
- Rewrite unrelated tag filtering or runtime-mode logic.

Step 1: Make the minimal code change.
Step 2: Re-run the targeted test command.
Expected: PASS.

### Task 1.3: Add a non-regression test for valid non-empty loose queries

Objective: Prove the fix does not break the intended fallback behavior for real queries.

Files:
- Modify: `internal/tool/brain_test.go`

Step 1: Add or strengthen a test where:
- lexical search yields no hits after tag filtering
- `searchTaggedDocsByLooseQuery` is needed
- normalized query tokens are non-empty
- the correct tagged doc is still found

Good example shape:
- query: `"runtime cache"`
- tags: `[#architecture]`
- doc body contains normalized equivalents across punctuation or newlines

Step 2: Run:
- `go test -tags sqlite_fts5 -count=1 ./internal/tool -run 'TestBrainSearch.*(Tagged|LooseQuery|Runtime)'`
Expected: PASS.

---

## Phase 2: Audit/accounting cleanup for the unlisted landed behaviors

### Task 2.1: Produce a concise “intentional deltas” inventory

Objective: Separate true bugs from intentional but previously unlisted product changes.

Files:
- Create or modify one of:
  - `NEXT_SESSION_HANDOFF.md`
  - or a new note under `docs/plans/` / `docs/` such as `docs/plans/2026-04-10-audit-followup-notes.md`

Inventory to capture explicitly:
- `cmd/sirtopham/index.go`: `sirtopham index brain` now marks brain index fresh
- `cmd/sirtopham/serve.go`: live runtime now wires hybrid brain search + brain vectorstore
- `internal/tool/brain_read.go`: backlinks prefer derived `brain_links`
- `internal/server/project.go`: `/api/project` includes `brain_index`
- `internal/brain/parser/document.go`: explicit `.md` wikilink targets are preserved

For each item, record:
- file path
- short behavior change
- whether it is intended and already covered by tests
- whether any user/operator-facing docs need to mention it

Step 1: Draft the inventory.
Step 2: Keep it short and factual.

### Task 2.2: Align change summary / handoff language with the landed scope

Objective: Prevent future audits from flagging the same deltas as “missed” purely because the summary was incomplete.

Files:
- Modify: `NEXT_SESSION_HANDOFF.md`
- Optionally modify: `docs/plans/2026-04-09-brain-system-rebuild-implementation-plan.md` only if a small status note is useful

Recommended updates:
- Add a short section like “Audit follow-up: unlisted but intentional landed deltas”.
- List the five items above.
- Explicitly say they were not part of the narrower fix-summary wording but are covered by code/tests.

Do not:
- Rewrite the whole handoff.
- Remove historical detail that is still accurate.

Step 1: Add the short follow-up section.
Step 2: Verify the wording distinguishes “intentional landed behavior” from “open bug”.

### Task 2.3: Check whether operator docs need one-line updates

Objective: Make sure user-facing/operator-facing docs are not stale about the broadened runtime.

Files to inspect:
- `README.md`
- `MANUAL_LIVE_VALIDATION.md`
- `docs/v2-b4-brain-retrieval-validation.md`
- any doc currently describing `brain_search` semantic/auto mode or `brain_index` visibility

Likely minimum edits, only if missing:
- mention that Settings and `/api/project` expose brain index state
- mention that current runtime brain retrieval/search is hybrid where applicable
- mention that `brain_read include_backlinks` prefers derived backlinks when indexed state exists

Step 1: Inspect current wording.
Step 2: If already accurate enough, make no doc change.
Step 3: If stale, add the smallest possible correction.

---

## Phase 3: Verification

### Task 3.1: Run targeted tests for the fix

Run:
- `go test -tags sqlite_fts5 -count=1 ./internal/tool -run 'TestBrainSearch.*(Punctuation|LooseQuery|Tagged|Backlinks)'`

Expected:
- all targeted `internal/tool` tests pass

### Task 3.2: Re-run the broader audit command

Run:
- `go test -tags sqlite_fts5 -count=1 ./internal/brain/... ./internal/context/... ./internal/tool/... ./internal/server/...`

Expected:
- all packages pass

### Task 3.3: Re-run build

Run:
- `make build`

Expected:
- build succeeds
- frontend chunk-size warning and npm audit warning remain acceptable unless they change materially

### Task 3.4: Sanity-check the exact audit claim

Objective: Confirm the original audit finding is now fully resolved.

Checklist:
- `searchTaggedDocsByLooseQuery` no longer broad-matches on empty normalized token sets
- there is an explicit regression test for punctuation-only/separator-only query input
- intended non-empty loose-query fallback still works
- the follow-up note/handoff documents the previously unlisted broadened behavior changes

---

## Suggested implementation order

1. Add failing punctuation-only/tagged fallback regression test
2. Implement early return for empty normalized query tokens in `internal/tool/brain_search.go`
3. Add/confirm non-empty fallback preservation test
4. Run targeted `internal/tool` tests
5. Add concise intentional-deltas inventory / handoff update
6. Inspect docs and make only minimum necessary wording fixes
7. Run broad test command
8. Run `make build`
9. Prepare final audit follow-up summary with exact files changed and commands run

---

## Risks and decisions

Decision A: What should punctuation-only raw query + tags do?
- Recommended: return no fallback matches rather than “match all tagged docs”.
- Reason: this is safest, least surprising, and matches the original equivalence assumption for meaningful queries.

Decision B: Should empty-normalized queries be rejected earlier in `Execute()`?
- Recommended for this slice: no.
- Reason: it changes user-facing validation semantics and is broader than needed for the bug fix.
- Revisit later only if product wants a stronger validation contract.

Decision C: Should the unlisted landed deltas be reverted?
- Recommended: no, unless a separate bug is found.
- Reason: current evidence says they are intentional, tested, and aligned with the in-flight brain-system rebuild.

---

## Final deliverable expectations

At the end of implementation, report:
- exact code fix made in `internal/tool/brain_search.go`
- exact new/updated tests in `internal/tool/brain_test.go`
- exact handoff/doc note updated for the intentional deltas
- output summary of:
  - targeted `go test`
  - broad `go test -tags sqlite_fts5 -count=1 ./internal/brain/... ./internal/context/... ./internal/tool/... ./internal/server/...`
  - `make build`
- concise statement that the original audit issue is fixed and the broader deltas are now explicitly documented
