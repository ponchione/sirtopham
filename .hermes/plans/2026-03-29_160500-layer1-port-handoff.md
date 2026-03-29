# Layer 1 port handoff — audit session plan

Goal
- All 7 RAG/graph subsystems are ported from agent-conductor. Next session should audit the whole port for correctness, consistency, and gaps.

Current git state
- Branch: `main`
- Worktree: clean
- Ahead of origin: 22 commits (nothing pushed)
- Remote: `git@github.com:ponchione/sirtopham.git`

Commits from this session (11 new)
- `b97b416` feat: add SQLite graph store with blast radius queries
- `bcf569e` feat: add multi-query semantic searcher with hop expansion
- `a87f1d6` feat: add LLM-backed describer for semantic code descriptions
- `ae94cf9` feat: add LanceDB vector store with full CRUD and search
- `37c71da` chore: add LanceDB native library and headers
- `3fe4924` feat: change Chunk Calls/CalledBy to FuncRef type
- `2d2fac4` feat: add tree-sitter parsers for Go, Python, TypeScript, and Markdown
- `2b6ac05` feat: add Go AST parser with call graph extraction
- `403bd93` feat: add FuncRef type and relationship fields to RawChunk
- `0c5aa3d` feat: add EmbedTexts and EmbedQuery to embedding client
- `2893c00` feat: add embedding client struct and HTTP transport

Ported subsystems

| # | Package | Source (old) | Tests | Interface satisfied |
|---|---------|-------------|-------|---------------------|
| 1 | `codeintel/embedder` | `rag/embedder.go` | 10 | `codeintel.Embedder` |
| 2 | `codeintel/goparser` | `rag/goparser.go` | 6 | `codeintel.Parser` |
| 3 | `codeintel/treesitter` | `rag/parser.go` + py/ts | 14 | `codeintel.Parser` |
| 4 | `vectorstore` | `rag/store.go` | 7 | `codeintel.Store` |
| 5 | `codeintel/describer` | `rag/describer.go` | 8 | `codeintel.Describer` |
| 6 | `codeintel/searcher` | `rag/searcher.go` | 5 | `codeintel.Searcher` |
| 7 | `codeintel/graph` | `graph/store.go` + blast | 8 | `codeintel.GraphStore` |

Total: 58 tests, all passing

Build requirements
- Standard `go test ./...` works for everything except `vectorstore`
- Vector store requires CGo flags for LanceDB:
  ```
  CGO_CFLAGS="-I./include"
  CGO_LDFLAGS="-L./lib/linux_amd64 -llancedb_go -lm -ldl -lpthread"
  LD_LIBRARY_PATH=./lib/linux_amd64
  ```

What was NOT ported (intentionally)
1. **Graph analyzers** — `go_analyzer.go`, `python_analyzer.go`, `ts_analyzer.go`, `resolver.go` (~2k LOC). These extract the Symbol/Edge data that feeds the graph store. Complex, have their own test suites, and need careful adaptation.
2. **Indexer pipeline** — `indexer.go` (~500 LOC). Three-pass pipeline (walk+parse → reverse call graph → enrich+embed+store). Depends on all other subsystems being wired.
3. **File hash cache** — `filehash.go` (~50 LOC). Trivial, port when indexer is needed.
4. **Agent-conductor-specific code** — WorkOrder search, context assembly adapters, CodeRef types. Not applicable to sirtopham.

Key design decisions made during port
1. **Chunk.Calls/CalledBy changed from `[]string` to `[]FuncRef`** — preserves package info from AST parsing.
2. **Describer returns `[]Description` + empty slice on LLM failure** — matches codeintel.Describer interface; indexing continues without descriptions.
3. **Searcher takes `[]string` queries, not single query** — multi-query dedup/re-rank is built in.
4. **Graph store uses internal `Symbol`/`Edge` types** — converts to `codeintel.GraphNode` at the interface boundary.
5. **Tree-sitter parser uses `ChunkTypeFallback` for sliding-window chunks** — distinct from `ChunkTypeSection`.
6. **ChunkTypeEnum added** — needed for TypeScript enum declarations.

Audit checklist for next session
- [ ] Read each ported file and compare against old source for missed logic
- [ ] Check all interface implementations compile with `go vet ./...`
- [ ] Run `go test -race ./...` to check for race conditions
- [ ] Verify no TODO/FIXME left from port
- [ ] Check if graph analyzers should be ported now or deferred
- [ ] Check if doc status markers need updating
- [ ] Decide on push timing

Suggested next-session prompt
"Read `.hermes/plans/2026-03-29_160500-layer1-port-handoff.md` and run the audit checklist."
