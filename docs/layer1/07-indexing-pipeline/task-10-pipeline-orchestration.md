# Task 10: Pipeline Orchestration, Progress, and Shutdown

**Epic:** 07 — Indexing Pipeline
**Status:** ⬚ Not started
**Dependencies:** Task 01, Task 02, Task 03, Task 04, Task 05, Task 06, Task 07, Task 08, Task 09

---

## Description

Implement the `Run` method that orchestrates the three-pass pipeline end-to-end. This is the top-level entry point called by `sirtopham init` and `sirtopham index`. The current shipped runtime does not trigger indexing automatically on conversation start or server startup; operators invoke indexing explicitly when they need a fresh code index. It coordinates schema version checking, the three passes, index state persistence, progress reporting, structured logging with timing, and clean shutdown on context cancellation.

## Function Signature

```go
// Run executes the full indexing pipeline: schema check → Pass 1 → Pass 2 → Pass 3 → persist state.
func (idx *Indexer) Run(ctx context.Context) error
```

## Acceptance Criteria

### Orchestration Flow

- [ ] `Run` executes steps in this exact order:
  1. **Schema version check** (Task 09): call `checkSchemaVersion`. If full re-index needed, call `resetForFullReindex` and set internal force flag
  2. **Pass 1 — Walk + Parse** (Task 05): call `pass1WalkAndParse`. Returns chunks and deleted file paths
  3. **Early exit:** if no changed files and no deleted files, log `"no changes detected, index is up to date"` and return nil
  4. **Pass 2 — Reverse Call Graph** (Task 06): call `pass2ReverseCallGraph` on the chunks from Pass 1
  5. **Pass 3 — Describe + Embed + Store** (Task 07): call `pass3DescribeEmbedStore` with the chunks
  6. **Persist index state** (Task 08): call `persistIndexState` with file states and deleted paths
  7. **Update project commit** (Task 08): call `updateProjectCommit` with current HEAD SHA

### Progress Reporting

- [ ] Defines a `ProgressEvent` struct:
  ```go
  type ProgressEvent struct {
      Stage       string // "walk", "parse", "call_graph", "describe", "embed", "store", "persist"
      FilesTotal  int
      FilesDone   int
      ChunksTotal int
      ChunksDone  int
      Message     string
  }
  ```
- [ ] `OnProgress` callback (defined on `IndexerConfig` in Task 01) is invoked at each stage transition and at regular intervals within Pass 3 (per-file for description, per-batch for embedding)
- [ ] If `OnProgress` is nil, progress events are silently skipped

### Structured Logging

- [ ] Each pass logs start/end with timing:
  - `"starting pass 1: walk and parse"`
  - `"pass 1 complete" duration_ms=<N> files_changed=<N> chunks=<N>`
  - `"starting pass 2: reverse call graph"`
  - `"pass 2 complete" duration_ms=<N> edges=<N>`
  - `"starting pass 3: describe, embed, store"`
  - `"pass 3 complete" duration_ms=<N> described=<N> embedded=<N> stored=<N>`
- [ ] Top-level summary logged at the end:
  `"indexing complete" total_duration_ms=<N> files_indexed=<N> chunks_indexed=<N> files_deleted=<N> mode=<"full"|"incremental">`

### Error Handling

- [ ] Individual file failures in Pass 1 and Pass 3 do not stop the pipeline (logged and skipped — handled in Task 05 and Task 07)
- [ ] Pass 2 cannot fail (in-memory computation)
- [ ] Embedding failure (entire batch) in Pass 3 returns an error that propagates up from `Run`
- [ ] Store upsert failure propagates up from `Run`
- [ ] Index state persistence failure is logged as an error but does not cause `Run` to return an error — the chunks are already stored in LanceDB. A warning is emitted: `"failed to persist index state, next run may re-index unchanged files"`

### Context Cancellation

- [ ] On `ctx.Done()`, the pipeline stops as soon as possible:
  - Mid-walk: returns immediately
  - Mid-parse: finishes current file, then returns
  - Mid-describe: finishes current file, then returns
  - Mid-embed: cancels the HTTP request
  - Mid-store: finishes current upsert, then returns
- [ ] Returns `ctx.Err()` when cancelled
- [ ] Does NOT persist index state on cancellation (partial state would be misleading)

### Indexing Triggers

- [ ] `Run` is callable by:
  - `sirtopham init` — first run, always a full index
  - `sirtopham index` — manual re-index, respects incremental unless `--force`
  - Manual incremental runs via `sirtopham index`
- [ ] The caller sets `IndexerConfig.Force = true` for full re-index scenarios
