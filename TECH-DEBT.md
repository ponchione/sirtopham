# TECH-DEBT

Open issues that should be fixed in a later focused session or need closer investigation.


## Layer 2 — Provider Router

### Router Validate() uses generic Models() for all provider types
**Severity:** Medium | **Source:** Layer 2 audit (2026-03-31)

The spec (`docs/layer2/07-provider-router/`) calls for provider-specific startup
validation:
- **Anthropic:** auth check with 5 s timeout
- **Codex:** `exec.LookPath` (already implemented)
- **Local / OpenAI-compatible:** HTTP HEAD to `baseURL` with 2 s timeout

The current implementation uses a generic `Models()` call with a 5 s timeout for
all non-codex providers. This works but:
1. Does not distinguish a slow-but-reachable local server (HEAD would succeed in
   < 2 s) from one whose `Models()` endpoint is unimplemented.
2. Gives the same 5 s timeout to lightweight local checks and heavyweight
   Anthropic auth checks.

**Fix direction:** Add a `Ping(ctx) error` method to the `Provider` interface (or
a separate `Validator` interface checked via type assertion) so each provider can
supply a lightweight reachability check. The router would call `Ping` during
`Validate()` and fall back to `Models()` when the provider does not implement it.

---

### Codex integration tests gated behind build tag
**Severity:** Low | **Source:** Layer 2 audit (2026-03-31)

`internal/provider/codex/integration_test.go` uses `//go:build integration` so
the tests never run in the default `make test` / `go test ./...` invocation.
This is intentional (CI hosts lack the `codex` binary), but it means the codex
streaming and retry paths only get coverage when the tag is explicitly passed.

**Fix direction:** Add an `httptest`-based integration test file (no build tag)
that uses the existing `newTestProvider` helper to bypass the `LookPath` check,
similar to how `anthropic/integration_test.go` and `openai/integration_test.go`
work. Move the CLI-dependent tests to a separate file that keeps the tag.


## Layer 3 — Context Assembly

**Audited:** 2026-04-01 | **Result:** Clean — no tech debt items.

All 7 epics (42 checklist items) pass. Three test/doc gaps found during audit
were fixed in the same session:
1. GoDoc comments added to token approximation functions
2. Assembler tests added for error propagation and nil optional components
3. Cascading compression test added (two rounds)

Race detector clean. 43 tests pass across 9 test files.


