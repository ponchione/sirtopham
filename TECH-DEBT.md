# TECH-DEBT

Open issues that should be fixed in a later focused session or need closer investigation.

## Retrieval orchestrator concurrency audit
- Status: open
- Area: `internal/context/retrieval.go`
- Why it is here:
  - Slice 4 launches multiple goroutines that assign into shared outer-scope variables (`ragHits`, `graphHits`, `fileResults`, `conventionText`, `gitContext`) without synchronization.
  - Tests pass normally, but this should be verified under `go test -race ./internal/context/...` and likely refactored to collect results through channels or a mutex-protected struct.
- Suggested next action:
  - Run the Slice 4 tests with `-race`.
  - If confirmed, refactor per-path result collection to avoid unsynchronized shared writes.

## Compression boundary orphan audit
- Status: open
- Area: `internal/context/compression.go`
- Why it is here:
  - Slice 6 sanitizes surviving assistant `tool_use` blocks when their matching `role=tool` result messages were compressed.
  - The inverse boundary case still needs a focused audit: if head/tail preservation leaves a `role=tool` result active while the originating assistant `tool_use` message gets compressed, the reconstructed history may still contain an orphaned tool result.
  - The current implementation does not widen the compressed range or perform a second pass to drop/compress those surviving tool results.
- Suggested next action:
  - Add a targeted test where the preserved tail begins with a tool result whose paired assistant tool-use message falls into the compressed middle.
  - Decide whether the fix should compress those orphaned tool-result rows too, or adjust boundary selection to keep tool-use/result pairs intact.

## User-message iteration namespace overlap
- Status: open (documented)
- Area: `internal/db/query/conversation.sql` — `InsertUserMessage`, `DeleteIterationMessages`
- Why it is here:
  - `InsertUserMessage` hardcodes `iteration=1`. `PersistIteration` also writes iteration=1 for the first assistant iteration in the same turn.
  - `CancelIteration(conversationID, turn, 1)` therefore deletes both the user message and the first iteration's assistant/tool messages because they share `iteration=1`.
  - In practice the agent loop should never cancel iteration 1 of the first iteration without re-persisting the user message, but the schema coupling is fragile.
- Suggested next action:
  - Consider using `iteration=0` for user messages (schema change) or adding a `role != 'user'` guard to `DeleteIterationMessages`.
  - Tests in `internal/conversation/history_test.go` document the current behavior explicitly.
