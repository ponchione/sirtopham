# Layer 3 Audit: Context Assembly

## Scope

Layer 3 is sirtopham's core differentiator: per-turn RAG-driven context assembly.
Before every LLM call, it analyzes the user message, retrieves relevant code from
multiple sources in parallel, fits results into a token budget, serializes as markdown,
and handles conversation history compression when context exceeds limits.

## Spec References

- `docs/specs/06-context-assembly.md` — Full architecture
- `docs/layer3/layer-3-overview.md` — Epic index (7 epics)
- `docs/layer3/01-context-assembly-types/` through `07-compression-engine/` — Task-level specs
- `TECH-DEBT.md` — Three resolved items (retrieval concurrency, compression orphans, iteration namespace)

## Packages to Audit

| Package | Src | Test | Purpose |
|---------|-----|------|---------|
| `internal/context` | 14 | 9 | All context assembly logic |

Key source files:
- `types.go`, `interfaces.go`, `scope.go` — Types and contracts (Epic 01)
- `analyzer.go` — Rule-based turn analyzer (Epic 02)
- `query.go`, `momentum.go` — Query extraction and momentum (Epic 03)
- `retrieval.go` — Parallel retrieval orchestrator (Epic 04)
- `budget.go`, `serializer.go` — Budget fitting and markdown serialization (Epic 05)
- `assembler.go` — Full assembly pipeline (Epic 06)
- `compression.go` — History compression engine (Epic 07)
- `report_store.go` — Context report persistence

## Test Commands

```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" \
  LD_LIBRARY_PATH="$(pwd)/lib/linux_amd64" \
  go test -tags 'sqlite_fts5' ./internal/context/...

# Race detector (verified clean):
CGO_ENABLED=1 CGO_LDFLAGS="..." go test -tags 'sqlite_fts5' -race ./internal/context/...
```

## Audit Checklist

### Epic 01: Context Assembly Types
- [x] `types.go` defines: `ContextNeeds`, `RetrievalResults`, `RAGHit`, `GraphHit`, `FileResult`, `BrainHit`
- [x] `FullContextPackage` struct with all fields needed by Layer 5
- [x] `ContextAssemblyReport` with timing, quality metrics, retrieval stats
- [x] `interfaces.go` defines: `TurnAnalyzer`, `QueryExtractor`, `MomentumTracker`, `ConventionSource`
      Also defines: `Retriever`, `BudgetManager`, `Serializer` (for Epics 04-05)
- [x] `scope.go` defines context scope boundaries (`SeenFileLookup`, `AssemblyScope`)

### Epic 02: Turn Analyzer
- [x] `analyzer.go` — `RuleBasedAnalyzer` implements `TurnAnalyzer` interface
- [~] Signal extraction: explicit files, symbols, modification intent, creation intent, git context, continuation
      NOTE: Checklist originally listed "question intent" and "debugging hints" — these are
      NOT in the epic spec. The spec defines 6 signal types and all 6 are fully implemented.
      The checklist items were aspirational. Marking partial only for checklist accuracy.
- [x] Signals derive from user message content + recent conversation context
- [x] Test covers: file mentions, symbol references, modification intent, creation keywords,
      git context, continuation, empty messages, PascalCase stopword filtering (8 tests)

### Epic 03: Query Extraction & Momentum
- [x] `query.go` — `HeuristicQueryExtractor` produces search queries from signals
      Three-source strategy: cleaned message, technical keywords, momentum-enhanced
- [x] `momentum.go` — `HistoryMomentumTracker` carries forward context from prior turns
      Extracts from file_read/file_write/file_edit/search tool calls in recent history
- [x] Momentum narrows queries based on files/symbols already discussed
      Strong-signal turns clear stale momentum; weak-signal turns inherit it
- [x] Test covers: query generation (6 tests), momentum accumulation (3 tests),
      momentum decay/clearing (2 tests), integration tests (2 tests)

### Epic 04: Retrieval Orchestrator
- [x] `retrieval.go` — `RetrievalOrchestrator` runs 5 paths in parallel:
  1. Semantic search (RAG via Layer 1 Searcher) — topK=10, hop expansion enabled
  2. Explicit file reads — with path traversal prevention
  3. Structural graph lookups — configurable depth/budget (defaults 1/10)
  4. Convention loading — via ConventionSource interface
  5. Git context (recent commits) — via exec.CommandContext
- [x] Each goroutine writes to a distinct variable; `sync.WaitGroup` synchronizes
- [x] Race detector verified clean (TECH-DEBT.md item 1 — resolved)
- [x] Post-processing: `filterAndDedupRAGHits`, `mergeGraphHitsIntoRAG`
      Dedup by ChunkID keeping highest score; graph hits merged into RAG with "graph" source
- [x] Relevance threshold filtering (configurable, default 0.35)
- [x] Timeout per retrieval path (default 5s) via per-path context.WithTimeout
- [x] Test covers: all paths (5-path exercise), dedup + threshold filtering, timeout
      (200ms delay vs 20ms timeout), nil components, error resilience (5 tests)

### Epic 05: Budget Manager & Serialization
- [x] `budget.go` — priority-based token budget allocation via `PriorityBudgetManager`
- [~] Priority order: explicit files > top RAG > graph > conventions > git > lower RAG
      NOTE: Checklist listed "brain docs" in priority chain — correctly deferred to v0.2
      per epic spec. Code matches v0.1 spec exactly.
- [x] Token counting for budget enforcement — `approximateTokenCount` using (len+3)/4
      Budget formula: model_limit - 3000(system) - 3000(tools) - 16000(response) - history
- [x] Compression-needed flag when budget exceeded — `shouldCompressHistory` checks
      historyTokenCount > modelContextLimit * CompressionThreshold
- [x] `serializer.go` — `MarkdownSerializer` produces deterministic markdown output
      Sections: Relevant Code (grouped by file), Structural Context, Conventions, Recent Changes
      Features: description before code, language-tagged fences, previously-viewed annotations
- [x] Test covers: budget fitting (3 tests), overflow/exclusion tracking, serialization format
      including grouping, annotations, determinism, empty input (2 tests)

### Epic 06: Context Assembly Pipeline
- [x] `assembler.go` — `ContextAssembler` wires all components together
- [x] `Assemble(ctx, message, history, scope, modelContextLimit, historyTokenCount)`
      returns `(*FullContextPackage, bool, error)` — bool is compressionNeeded
- [x] Emits `ContextAssemblyReport` with timing for each phase
      (AnalysisLatencyMs, RetrievalLatencyMs, TotalLatencyMs)
- [x] Nil-safe: handles missing searcher, graph, conventions gracefully
      Guards: nil ctx, nil momentum, nil extractor, nil needs/results/budget
- [x] Test covers: full pipeline with report persistence, quality update with hit rate,
      error propagation from retriever, nil optional components (momentum/extractor)

### Epic 07: Compression Engine
- [x] `compression.go` — `CompressionEngine` compresses persisted conversation history
- [x] Head/tail preservation: keeps first N and last M messages, compresses middle
      Configurable via CompressionHeadPreserve (default 3) and CompressionTailPreserve (default 4)
- [x] Summary generation via LLM provider (falls back gracefully if provider unavailable)
      `prepareSummary` returns fallbackUsed=true on error, no crash
- [x] **Orphaned tool_use sanitization**: strips tool_use blocks from surviving assistant
  messages when their matching tool results were compressed
      `sanitizeAssistantMessageContent` removes unmatched tool_use, inserts placeholder if all removed
- [x] **Orphaned tool_result compression** (TECH-DEBT item 2 — resolved):
  `survivingAssistantToolUseIDs()` collects tool_use IDs from surviving assistant
  messages; tool results with no matching tool_use get marked compressed
- [x] Summary inserted as `is_summary=1` message with compressed turn range metadata
- [x] Sequence bisection for summary placement between head and tail
      Up to 1024 bisection attempts to avoid collision
- [x] Trigger conditions: NeedsCompressionPreflight (char-based), NeedsCompressionPostResponse
      (token-based), NeedsCompressionAfterProviderError (413/context_length_exceeded)
- [x] FTS5 index updated when content changes (via SQLite trigger on content column)
- [x] `report_store.go` persists ContextAssemblyReport to `context_reports` table
      Insert, Get, UpdateQuality methods with nil-safe guards
- [x] Test covers: full compression, fallback without summary, sequence collision,
  orphaned tool_use sanitization, orphaned tool_result compression, trigger checks,
  cascading compression (two rounds — old summary compressed, new summary covers full range)

### Cross-cutting
- [x] `go test -race ./internal/context/...` passes clean (verified 2026-04-01, 1.350s)
- [x] All compression tests use real SQLite databases (not mocks)
- [x] No nil pointer panics when optional components are nil
- [x] Token estimation is consistent (chars/4 approximation documented in GoDoc comments
      on both `approximateTokenCount` and `approximateTokensFromChars`)

---

## Audit Summary

**Date**: 2026-04-01
**Result**: 42/42 items — 40 PASS, 2 PARTIAL (spec-vs-checklist alignment), 0 FAIL

### Pass Rate by Epic
| Epic | Items | Pass | Partial | Fail |
|------|-------|------|---------|------|
| 01 — Context Assembly Types | 5 | 5 | 0 | 0 |
| 02 — Turn Analyzer | 4 | 3 | 1 | 0 |
| 03 — Query Extraction & Momentum | 4 | 4 | 0 | 0 |
| 04 — Retrieval Orchestrator | 7 | 7 | 0 | 0 |
| 05 — Budget Manager & Serialization | 6 | 5 | 1 | 0 |
| 06 — Context Assembly Pipeline | 5 | 5 | 0 | 0 |
| 07 — Compression Engine | 11 | 11 | 0 | 0 |
| Cross-cutting | 4 | 4 | 0 | 0 |

### Issues Fixed During Audit
1. **Added GoDoc comments** to `approximateTokenCount` (budget.go) and
   `approximateTokensFromChars` (compression.go) documenting the chars/4 heuristic
2. **Added assembler tests**: `TestContextAssemblerPropagatesRetrieverError` and
   `TestContextAssemblerHandlesNilOptionalComponents`
3. **Added cascading compression test**: `TestCompressionEngineCascadesTwoRoundsCompressingOldSummary`
   verifies two rounds of compression with old summary compressed and new summary
   covering full range

### Partial Items (Checklist-vs-Spec Alignment)
1. **Epic 02 signal types**: Checklist listed "question intent" and "debugging hints"
   which are not in the epic spec. The 6 spec-defined signal types are all implemented.
   No code change needed — checklist was aspirational.
2. **Epic 05 brain docs priority**: Checklist included "brain docs" in priority chain.
   Correctly deferred to v0.2 per epic spec. No code change needed.

### Strengths Noted
- All 7 epics fully implemented to v0.1 spec with no functional gaps
- Comprehensive test coverage: 43 tests across 9 test files, all passing
- Race detector clean with real concurrent goroutines
- Real SQLite databases used in all persistence tests (no mocks)
- Extensive nil-safety throughout the codebase
- Clean interface boundaries enabling future swapability (e.g., LLM-based analyzer)
- Deterministic serialization output for prompt cache stability
- FullContextPackage frozen flag for turn-level immutability
- Quality metrics instrumentation (AgentUsedSearchTool, ContextHitRate) built in from day one
