     1|# Next session handoff
     2|
     3|Date: 2026-04-09
     4|Repo: /home/gernsback/source/sirtopham
     5|Branch: main
     6|Status: brain-system rebuild work is now in progress. Phase 0 contract/alignment slices, Phase 1 Tasks 1.1-1.4, the first narrow Phase 2 freshness-contract follow-through, Phase 2 Task 2.1 brain chunk modeling, Phase 2 Task 2.2 semantic brain indexing orchestration, Phase 2 Task 2.3 explicit freshness/reporting, Phase 3 Task 3.1 proactive hybrid runtime brain retrieval, Phase 3 Task 3.2 reactive hybrid `brain_search` parity, Phase 3 Task 3.3 graph/backlink expansion, `brain_read` derived-backlink follow-through, and the first ranking/explainability label + report/inspector follow-through are landed locally but not yet committed.
     7|
     8|## Read this first next session
     9|Start with these files in this order:
    10|1. `BRAIN_SYSTEM_AUDIT_AND_REBUILD.md`
    11|2. `docs/plans/2026-04-09-brain-system-rebuild-implementation-plan.md`
    12|3. `NEXT_SESSION_HANDOFF.md`
    13|4. `TECH-DEBT.md`
    14|
    15|Then inspect current changes:
    16|- `git status --short --branch`
    17|- `git diff --stat`
    18|
    19|## What this session completed
    20|
    21|### 1. Phase 0 serializer/spec-alignment slice
    22|Completed:
    23|- moved `Project Brain` before `Relevant Code` in assembled context
    24|- upgraded brain serialization from one-line bullets to richer note blocks with:
    25|  - title heading
    26|  - path
    27|  - match mode
    28|  - tags when present
    29|  - multiline excerpt block
    30|
    31|Touched files:
    32|- `internal/context/serializer.go`
    33|- `internal/context/serializer_test.go`
    34|
    35|### 2. Phase 0 brain-config contract slice
    36|Completed:
    37|- made `brain.max_brain_tokens` real in budget fitting
    38|- made `brain.brain_relevance_threshold` real in proactive brain retrieval
    39|- wired both through the live `serve` construction path
    40|- fixed a regression where an early candidate with only filtered-out brain hits blocked later fallback candidates
    41|
    42|Touched files:
    43|- `cmd/sirtopham/serve.go`
    44|- `internal/context/budget.go`
    45|- `internal/context/budget_test.go`
    46|- `internal/context/retrieval.go`
    47|- `internal/context/retrieval_test.go`
    48|
    49|### 3. Phase 0 report/signal coverage follow-through
    50|Completed:
    51|- strengthened context report persistence coverage for:
    52|  - `brain_results`
    53|  - `prefer_brain_context`
    54|  - semantic query retention alongside brain-aware needs
    55|
    56|Touched file:
    57|- `internal/context/assembler_test.go`
    58|
    59|### 4. Phase 1 Task 1.1 parser foundation
    60|Completed:
    61|- added canonical parser package:
    62|  - `internal/brain/parser/document.go`
    63|  - `internal/brain/parser/document_test.go`
    64|- parser now produces a richer document model with:
    65|  - `Path`
    66|  - `Title`
    67|  - `Content`
    68|  - `Body`
    69|  - `ContentHash`
    70|  - `Tags`
    71|  - `Frontmatter`
    72|  - `Wikilinks []ParsedLink`
    73|  - `Headings []Heading`
    74|  - `TokenCount`
    75|  - `UpdatedAt` / `HasUpdatedAt`
    76|- parser supports:
    77|  - YAML frontmatter extraction
    78|  - title extraction from first H1, fallback to filename
    79|  - merged frontmatter + inline tags
    80|  - wikilink parsing including `[[target|display]]`
    81|  - heading extraction
    82|  - SHA-256 content hashing via existing `codeintel.ContentHash(...)`
    83|  - optional file-mod-time fallback for `UpdatedAt`
    84|- `internal/brain/analysis/parse.go` now delegates to the canonical parser instead of maintaining a separate parser
    85|- analysis-side flattened wikilinks are deduped by target
    86|- fence handling was hardened after review:
    87|  - ignores headings/tags inside fenced code blocks
    88|  - supports both ``` and ~~~
    89|  - tracks actual opener length/type
    90|  - requires valid closing fences
    91|  - rejects invalid shorter/annotated closers from ending the fence early
    92|
    93|Touched files:
    94|- `internal/brain/parser/document.go`
    95|- `internal/brain/parser/document_test.go`
    96|- `internal/brain/analysis/parse.go`
    97|- `internal/brain/analysis/lint_test.go`
    98|
    99|### 5. Phase 1 Task 1.2 DB metadata/query support
   100|Completed:
   101|- added focused sqlite integration coverage for:
   102|  - upsert/replace one `brain_documents` row by `(project_id, path)`
   103|  - delete/rewrite `brain_links` rows for one source document
   104|  - list brain docs for a project
   105|  - fetch brain doc metadata by path
   106|- added sqlc query source for derived brain metadata persistence in `internal/db/query/brain.sql`
   107|- regenerated sqlc output in `internal/db/brain.sql.go`
   108|- locked the upsert contract so `created_at` stays stable on replacement while mutable metadata updates in place
   109|
   110|Touched files:
   111|- `internal/db/query/brain.sql`
   112|- `internal/db/brain.sql.go`
   113|- `internal/db/schema_integration_test.go`
   114|
   115|### 6. Phase 1 Task 1.3 derived-state indexer service
   116|Completed:
   117|- added `internal/brain/indexer/indexer.go` with a narrow full-rebuild materialization path
   118|- indexer now:
   119|  - lists vault markdown documents from the brain backend
   120|  - skips operational `_log.md`
   121|  - parses each note through `internal/brain/parser`
   122|  - upserts `brain_documents`
   123|  - rewrites outgoing `brain_links`
   124|  - deletes stale `brain_documents` rows and their outgoing links when notes disappear from the vault
   125|  - preserves `created_at` across rebuilds while updating mutable metadata and `updated_at`
   126|- added focused sqlite-backed indexer tests for:
   127|  - indexing docs + links + `_log.md` exclusion
   128|  - link rewrite + stale-doc deletion across rebuilds
   129|- added one more sqlc query for stale-doc cleanup:
   130|  - `DeleteBrainDocumentByPath`
   131|
   132|Touched files:
   133|- `internal/brain/indexer/indexer.go`
   134|- `internal/brain/indexer/indexer_test.go`
   135|- `internal/db/query/brain.sql`
   136|- `internal/db/brain.sql.go`
   137|
   138|### 7. Phase 1 Task 1.4 explicit brain reindex command wiring
   139|Completed:
   140|- added an explicit operator-visible `sirtopham index brain` path instead of folding brain rebuild into ordinary code indexing implicitly
   141|- command now loads project config, reuses the shared MCP/vault brain backend wiring, ensures the SQLite project record exists, and runs the landed `internal/brain/indexer` rebuild against the current project
   142|- plain-text command output now reports:
   143|  - brain documents indexed
   144|  - brain links indexed
   145|  - brain documents deleted
   146|- `--json` output is supported for machine-readable command use
   147|- added focused command tests covering:
   148|  - config handoff into the brain reindex path
   149|  - human-readable summary output
   150|  - JSON output
   151|
   152|Touched files:
   153|- `cmd/sirtopham/index.go`
   154|- `cmd/sirtopham/index_test.go`
   155|
   156|### 8. Phase 2 freshness-contract follow-through (narrow explicit-reminder slice)
   157|Completed:
   158|- chose the explicit-reminder contract for now instead of silent auto-refresh or hidden stale-state bookkeeping
   159|- `brain_write` success output now tells the operator the derived brain index is stale and names the exact refresh command: `sirtopham index brain`
   160|- `brain_update` success output now does the same while preserving the content preview
   161|- added focused failing tests first for both write and update success paths, then implemented the minimum follow-through
   162|- kept the workflow unsurprising: vault write/update succeeds immediately, and the operator gets an explicit reindex reminder rather than implicit background magic
   163|
   164|Touched files:
   165|- `internal/tool/brain_format.go`
   166|- `internal/tool/brain_write.go`
   167|- `internal/tool/brain_update.go`
   168|- `internal/tool/brain_test.go`
   169|
   170|### 9. Phase 2 Task 2.1 brain chunk model slice
   171|Completed:
   172|- added a new `internal/brain/chunks` package with a narrow chunk model kept separate from code chunks
   173|- introduced a heading-aware `BuildDocument(...)` chunking path over parsed brain documents
   174|- chunk model now carries provenance needed for later semantic indexing:
   175|  - stable chunk id
   176|  - chunk index
   177|  - document path
   178|  - document title
   179|  - tags
   180|  - section heading
   181|  - line range
   182|  - document content hash
   183|  - document updated-at metadata when available
   184|- chunking contract currently is:
   185|  - short documents use a single chunk
   186|  - long documents split at level-2 (`##`) headings
   187|  - nested headings stay inside the parent level-2 section chunk
   188|  - long documents without level-2 headings fall back to a single chunk
   189|- added focused chunk tests first, then implemented the minimum model/build path
   190|
   191|Touched files:
   192|- `internal/brain/chunks/chunks.go`
   193|- `internal/brain/chunks/chunks_test.go`
   194|
   195|### 10. Phase 2 Task 2.2 semantic brain indexing orchestration
   196|Completed:
   197|- added `internal/brain/indexer/semantic.go` with a dedicated semantic rebuild path built on the new brain chunk model
   198|- semantic rebuild now:
   199|  - lists vault documents from the brain backend
   200|  - skips operational `_log.md`
   201|  - parses notes and builds heading-aware brain chunks
   202|  - embeds those chunks with the configured embedding runtime
   203|  - writes them into the separate brain LanceDB path rather than the code vectorstore path
   204|  - clears/replaces semantic chunks for currently indexed documents
   205|  - deletes stale semantic chunks for previously indexed docs that disappeared from the vault
   206|- wired `sirtopham index brain` to run both:
   207|  - derived SQLite metadata/link rebuild
   208|  - semantic brain chunk rebuild
   209|- command summary / JSON output now includes:
   210|  - `semantic_chunks_indexed`
   211|  - `semantic_documents_deleted`
   212|- added focused semantic indexer tests first for:
   213|  - chunk upsert + stale deletion behavior
   214|  - clean embedder failure behavior without partial store writes
   215|
   216|Touched files:
   217|- `internal/brain/indexer/semantic.go`
   218|- `internal/brain/indexer/semantic_test.go`
   219|- `internal/brain/indexer/indexer.go`
   220|- `cmd/sirtopham/index.go`
   221|- `cmd/sirtopham/index_test.go`
   222|
   223|### 11. Phase 3 Tasks 3.1-3.2 hybrid brain retrieval parity
   224|Completed:
   225|- added `internal/context/brain_search.go` with a narrow hybrid runtime searcher that merges:
   226|  - MCP/vault keyword hits
   227|  - semantic brain-vector hits from the separate brain LanceDB store
   228|  - SQLite brain-document metadata enrichment (`title`, `tags`)
   229|- expanded proactive context-side brain retrieval to use the richer interface/result shape instead of raw keyword hits only
   230|- proactive `BrainHit` records now preserve:
   231|  - lexical score
   232|  - semantic score
   233|  - final score
   234|  - match mode
   235|  - match sources
   236|  - section heading
   237|  - tags
   238|- wired `cmd/sirtopham/serve.go` to open the separate brain LanceDB store at runtime and reuse the same hybrid searcher for:
   239|  - proactive context assembly
   240|  - reactive `brain_search` semantic/auto modes
   241|- reactive `brain_search` now:
   242|  - keeps `mode=keyword` deterministic and keyword-only
   243|  - uses the hybrid runtime path for real `mode=semantic` and `mode=auto`
   244|  - preserves fallback notice behavior only when no runtime hybrid searcher is available
   245|  - shows semantic/hybrid match mode inline in returned titles for operator visibility
   246|  - prefers indexed tag metadata for tag filtering when runtime results already carry tags, with backend read fallback only when needed
   247|- updated metrics/frontend types so richer brain-result fields remain representable in the web client
   248|- rewrote the handoff recommendation so the next session starts at graph/backlink expansion instead of repeating hybrid-retrieval work
   249|
   250|Touched files:
   251|- `internal/context/interfaces.go`
   252|- `internal/context/types.go`
   253|- `internal/context/retrieval.go`
   254|- `internal/context/retrieval_test.go`
   255|- `internal/context/brain_search.go`
   256|- `internal/context/brain_search_test.go`
   257|- `internal/tool/brain_search.go`
   258|- `internal/tool/register.go`
   259|- `internal/tool/brain_test.go`
   260|- `cmd/sirtopham/serve.go`
   261|- `web/src/types/metrics.ts`
   262|- `NEXT_SESSION_HANDOFF.md`
   263|- `TECH-DEBT.md`
   264|
   265|## Reviews completed this session
   266|- Phase 0 combined slice (`Task 0.4` + minimal `Phase 4.1` follow-through): spec review PASS
   267|- Phase 0 combined slice: final quality review APPROVED after fixing retrieval fallback bug
   268|- Phase 1 Task 1.1: spec review PASS
   269|- Phase 1 Task 1.1: final quality review APPROVED after fence-handling and analysis-dedupe follow-ups
   270|
   271|## Validation run this session
   272|Passed:
   273|- `go test -tags sqlite_fts5 ./internal/context -run 'TestMarkdownSerializerBrainAppearsBeforeCode|TestMarkdownSerializerBrainIncludesRichKnowledgeContent|TestMarkdownSerializerGroupsChunksAnnotatesSeenFilesAndIsDeterministic|TestMarkdownSerializerHandlesEmptyBudgetResult'`
   274|- `go test -tags sqlite_fts5 ./internal/context -run 'TestPriorityBudgetManagerHonorsMaxBrainTokens|TestRetrievalOrchestratorHonorsBrainRelevanceThreshold'`
   275|- `go test -tags sqlite_fts5 ./internal/context -run 'TestRetrievalOrchestratorFallsBackWhenEarlyBrainHitsAreFilteredOut|TestRetrievalOrchestratorHonorsBrainRelevanceThreshold|TestPriorityBudgetManagerHonorsMaxBrainTokens'`
   276|- `go test -tags sqlite_fts5 ./internal/context`
   277|- `go test -tags sqlite_fts5 ./internal/context ./internal/server`
   278|- `go test -tags sqlite_fts5 ./internal/context`
   279|- `go test -tags sqlite_fts5 ./internal/server`
   280|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/tool -run 'TestBrainSearch'`
   281|- `go test -tags sqlite_fts5 ./internal/brain/parser ./internal/brain/analysis`
   282|- `go test -tags sqlite_fts5 ./internal/brain/...`
   283|- `go test -tags sqlite_fts5 ./internal/brain/indexer -run 'TestIndexerRebuildProjectIndexesDocsLinksAndSkipsOperationalLog|TestIndexerRebuildProjectRewritesLinksAndDeletesMissingDocuments'`
   284|- `go test -tags sqlite_fts5 ./internal/db -run 'TestBrainDocumentQueriesUpsertListAndFetchByPath|TestBrainLinkQueriesDeleteAndRewriteForSourceDocument'`
   285|- `go test -tags sqlite_fts5 ./internal/db`
   286|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./cmd/sirtopham -run 'TestIndex'`
   287|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/tool -run 'TestBrainWriteSuccess|TestBrainUpdateAppend'`
   288|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/tool`
   289|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/brain/chunks`
   290|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/brain/indexer -run 'TestSemanticIndexer'`
   291|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./cmd/sirtopham -run 'TestIndex'`
   292|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/brain/chunks ./internal/brain/indexer`
   293|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/brain/...`
   294|- `CGO_ENABLED=1 CGO_LDFLAGS="-L$PWD/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread" LD_LIBRARY_PATH="$PWD/lib/linux_amd64" go test -tags sqlite_fts5 ./internal/db`
   295|
   296|Known verification limit:
   297|- `go test -tags sqlite_fts5 ./cmd/sirtopham`
   298|- still fails here due existing LanceDB native linker issues (`undefined reference to simple_lancedb_*`), not because of the brain-rebuild slices above
   299|
   300|## Current working tree
   301|At handoff time the tree contains these relevant tracked modifications:
   302|- `NEXT_SESSION_HANDOFF.md`
   303|- `cmd/sirtopham/index.go`
   304|- `cmd/sirtopham/index_test.go`
   305|- `cmd/sirtopham/serve.go`
   306|- `internal/brain/analysis/lint_test.go`
   307|- `internal/brain/analysis/parse.go`
   308|- `internal/context/assembler_test.go`
   309|- `internal/context/budget.go`
   310|- `internal/context/budget_test.go`
   311|- `internal/context/retrieval.go`
   312|- `internal/context/retrieval_test.go`
   313|- `internal/context/serializer.go`
   314|- `internal/context/serializer_test.go`
   315|- `internal/db/schema_integration_test.go`
   316|- `internal/tool/brain_format.go`
   317|- `internal/tool/brain_test.go`
   318|- `internal/tool/brain_update.go`
   319|- `internal/tool/brain_write.go`
   320|- `internal/brain/chunks/chunks.go`
   321|- `internal/brain/chunks/chunks_test.go`
   322|- `internal/brain/indexer/semantic.go`
   323|- `internal/brain/indexer/semantic_test.go`
   324|
   325|Untracked but relevant to keep:
   326|- `BRAIN_SYSTEM_AUDIT_AND_REBUILD.md`
   327|- `docs/plans/2026-04-09-brain-system-rebuild-implementation-plan.md`
   328|- `internal/brain/parser/`
   329|- `internal/brain/indexer/`
   330|- `internal/db/query/brain.sql`
   331|- `internal/db/brain.sql.go`
   332|
   333|   333|Nothing was pushed.
   334|   334|
   335|   335|### 14. Live-validation/doc-truth slice
   336|   336|Completed against the real local runtime rather than a mocked test harness:
   337|   337|- started the repo-owned local llama.cpp services and confirmed health/models on:
   338|   338|  - `http://localhost:12434` (`Qwen2.5-Coder-7B-Instruct-Q6_K_L.gguf`)
   339|   339|  - `http://localhost:12435` (`nomic-embed-code.Q8_0.gguf`)
   340|   340|- built the current tree with `make build`
   341|   341|- created `/tmp/my-website-runtime-8092.yaml` for the real target project/root/vault:
   342|   342|  - project: `~/source/my-website`
   343|   343|  - app: `http://localhost:8092`
   344|   344|  - provider/model: `codex` / `gpt-5.4-mini`
   345|   345|  - brain vault: `~/source/my-website/.brain`
   346|   346|- ran `./bin/sirtopham index --config /tmp/my-website-runtime-8092.yaml`
   347|   347|- ran `./bin/sirtopham index brain --config /tmp/my-website-runtime-8092.yaml`
   348|   348|  - result: `Brain documents indexed: 4`, `Brain links indexed: 0`, `Brain semantic chunks indexed: 12`
   349|   349|- started `./bin/sirtopham serve --config /tmp/my-website-runtime-8092.yaml`
   350|   350|- confirmed live runtime surfaces:
   351|   351|  - `/api/config` reported `default_provider=codex`, `default_model=gpt-5.4-mini`
   352|   352|  - `/api/project` reported `brain_index.status=clean`
   353|   353|
   354|   354|Validation evidence:
   355|   355|- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario runtime-proof`
   356|   356|  - initially failed only because the maintained package still pinned the canary to `expected_match_mode=keyword`
   357|   357|  - the real live hit on this runtime was `notes/runtime-brain-proof-apr-07.md` with `match_mode=semantic`, `match_sources=["semantic"]`, `budget_breakdown.brain=380`, and zero tool calls
   358|   358|  - after removing that stale keyword-only assumption from the package, the canary passed
   359|   359|- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario rationale-layout`
   360|   360|  - passed
   361|   361|  - matched `notes/minimal-content-first-layout-rationale.md`
   362|   362|  - current live matched hit was also `semantic`
   363|   363|- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-vite`
   364|   364|  - passed
   365|   365|  - matched `notes/past-debugging-vite-rebuild-loop.md`
   366|   366|  - current live matched hit was also `semantic`
   367|   367|
   368|   368|Doc/package truth updated in this slice:
   369|   369|- `scripts/validate_brain_retrieval.py` no longer claims the canary proves keyword-only retrieval and no longer hard-codes `runtime-proof` to `expected_match_mode=keyword`
   370|   370|- `docs/v2-b4-brain-retrieval-validation.md` now says the package proves hybrid runtime behavior and records the current real-runtime fact that the maintained `my-website` scenarios presently land as semantic hits
   371|   371|- the same doc now records the latest real-vault graph truth too: we seeded linked validation notes and fixed `.md` wikilink-target preservation in the parser/index path, but there is still no maintained graph-aware validation scenario because the attempted live prompts do not yet surface stable structural hits in proactive `brain_results`
   372|   372|
   373|   373|### 15. Follow-through after continuing in the same session
   374|   374|Completed:
   375|   375|- seeded real linked notes under `~/source/my-website/.brain/notes/` to make the vault actually carry graph edges for validation:
   376|   376|  - `layout-nav-bridge.md`
   377|   377|  - `layout-graph-bridge.md`
   378|   378|  - `layout-graph-proof.md`
   379|   379|  - `past-debugging-saturn-rail-bridge.md`
   380|   380|  - `past-debugging-saturn-rail-fix.md`
   381|   381|  - `past-debugging-lunar-hinge-bridge.md`
   382|   382|  - `past-debugging-deep-panel-fix.md`
   383|   383|- reran `./bin/sirtopham index brain --config /tmp/my-website-runtime-8092.yaml`
   384|   384|  - latest result: `Brain documents indexed: 11`, `Brain links indexed: 4`, `Brain semantic chunks indexed: 19`
   385|   385|- found and fixed a real parser/index mismatch that blocked live structural lookup:
   386|   386|  - `internal/brain/parser/document.go` had been stripping `.md` from wikilink targets
   387|   387|  - that made `brain_links.target_path` disagree with `brain_documents.path`
   388|   388|  - added parser regression coverage in `internal/brain/parser/document_test.go`
   389|   389|- validation/tests run after the fix:
   390|   390|  - `go test -tags sqlite_fts5 ./internal/brain/parser`
   391|   391|  - `go test -tags sqlite_fts5 ./internal/brain/indexer ./internal/context ./internal/tool`
   392|   392|  - `make build`
   393|   393|
Latest live evidence after the proactive graph-debugging slice:
- traced the live `LUNAR HINGE 91` failure back to `internal/context/brain_search.go`
  - root cause: `applyGraphLink(...)` returned early whenever a candidate note already existed in `bestDepth` at depth 0 from a direct semantic hit
  - effect: one-hop structural evidence never attached to notes that were already matched semantically, so persisted proactive `brain_results` stayed semantic-only even when `brain_links` clearly connected the bridge note to the fix note
- landed a narrow guard fix so direct depth-0 hits can still be annotated by one-hop graph evidence without reopening deeper-cycle pollution on later hops
- added focused regression coverage in `internal/context/brain_search_test.go` for the real shape that failed live: a direct semantic hit on `notes/past-debugging-deep-panel-fix.md` now upgrades to structural-hybrid metadata when linked from the `LUNAR HINGE 91` bridge note
- added a maintained graph-aware validation scenario to `scripts/validate_brain_retrieval.py` / `docs/v2-b4-brain-retrieval-validation.md`:
  - `debug-history-lunar-hinge-graph`
  - prompt: `From our past debugging notes, what phrase sits behind LUNAR HINGE 91?`
  - expects `notes/past-debugging-deep-panel-fix.md`
  - requires `match_sources` to include `graph` and `graph_hop_depth >= 1`
- fresh live validation on the rebuilt `:8092` runtime now passes:
  - matched hit: `notes/past-debugging-deep-panel-fix.md`
  - `match_mode: hybrid-graph`
  - `match_sources: ["graph", "semantic"]`
  - `graph_source_path: notes/past-debugging-lunar-hinge-bridge.md`
  - `graph_hop_depth: 1`

### 16. Graph-selection follow-through after the first structural canary
Completed:
- compared the live `debug-history-lunar-hinge-graph` output with SATURN RAIL and layout-family prompts against the rebuilt `:8092` runtime
- confirmed the noisy part of the old policy: direct semantic bridge notes were picking up reverse-edge `hybrid-backlink` metadata too easily, which made bridge notes look structurally promoted even when the real value was the fix-side target note
- added a focused failing regression in `internal/context/brain_search_test.go` proving a direct bridge-note semantic seed should stay `semantic` while the linked fix note still upgrades to `hybrid-graph`
- tightened `internal/context/brain_search.go` so depth-1 reverse-edge `backlink` annotation no longer applies to already-direct semantic seeds, while target/fix-side graph promotion still works
- rebuilt/restarted the real `:8092` runtime and revalidated:
  - `debug-history-lunar-hinge-graph` still passes, but the bridge note now stays `match_mode: semantic`
  - the SATURN RAIL bridge/fix pair now also passes as a maintained graph-aware live scenario for `notes/past-debugging-saturn-rail-fix.md` / `PANEL LOCK 58`
- updated `scripts/validate_brain_retrieval.py` and `docs/v2-b4-brain-retrieval-validation.md` to carry the second maintained graph-aware scenario: `debug-history-saturn-rail-graph`

Validation run for this slice:
- `go test -tags sqlite_fts5 ./internal/context -run 'TestHybridBrainSearcherAnnotatesDirectSemanticHitsWithGraphEvidence|TestHybridBrainSearcherExpandsBacklinksAndGraphHopsFromBrainLinks|TestBrainMatchModeDistinguishesBacklinkGraphAndHybridStructuralResults'`
- `make build`
- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-lunar-hinge-graph`
- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-saturn-rail-graph`

Important live evidence from the same comparison pass:
- the layout-family exploratory prompt (`From our layout graph notes, what linked layout canary phrase sits behind SATURN RAIL?`) answered correctly but still failed the maintained validator contract because:
  - `prefer_brain_context` never fired
  - `signal_stream` only carried the semantic query
  - code RAG stayed active (`budget_breakdown.rag = 2287`)
- this means the graph-selection cleanup is good enough for the debugging-family canaries, but the layout-family prompt is still routed more like ordinary code/layout search than explicit brain-seeking retrieval

### 17. Layout-family brain-intent routing follow-through
Completed:
- added a narrow layout-intent analyzer pass in `internal/context/analyzer.go`
  - new phrase family: `layout graph notes`, `layout rationale notes`
  - emitted signal: `brain_seeking_intent` with value `layout`
  - still keeps the phrase list narrow so generic layout/code questions are not hijacked
- added focused tests first for:
  - positive layout-graph prompt routing in `internal/context/analyzer_test.go`
  - negative generic layout/code prompts staying code-oriented in `internal/context/analyzer_test.go`
  - analyzer + retrieval follow-through in `internal/context/retrieval_test.go` proving the layout-graph prompt now skips semantic code RAG when brain context is preferred
- rebuilt/restarted the real `:8092` runtime and reran live validation
- added a maintained validation-package scenario for this prompt family:
  - `layout-graph-saturn-rail`
  - expected note: `notes/layout-graph-proof.md`
  - expected phrase: `PROSE FIRST 17`

Validation run for this slice:
- `go test -tags sqlite_fts5 ./internal/context -run 'TestBrainSeekingLayoutIntentPrefersBrainContext|TestBrainSeekingLayoutIntentIgnoresGenericLayoutCodeQuestions|TestRetrievalOrchestratorSkipsSemanticSearchForLayoutGraphBrainPrompt'`
- `go test -tags sqlite_fts5 ./internal/context`
- `make build`
- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario layout-graph-saturn-rail`
- `python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-saturn-rail-graph`

Live outcome after the fix:
- the layout-family SATURN RAIL prompt now passes the maintained validator contract
- persisted evidence now shows:
  - `brain_results` include `notes/layout-graph-proof.md`
  - `match_sources` include `graph`
  - `signal_stream` includes `brain_seeking_intent(layout)` and `prefer_brain_context`
  - `budget_breakdown.rag = 0`

## Exact recommended next step
The maintained six-scenario validation package was rerun after the latest graph/layout follow-through work and stayed green end to end on the live `:8092` runtime.

Latest package result:
- `runtime-proof`: passed
- `rationale-layout`: passed
- `debug-history-vite`: passed
- `debug-history-lunar-hinge-graph`: passed
- `debug-history-saturn-rail-graph`: passed
- `layout-graph-saturn-rail`: passed

Implication:
- do not spend the next slice on more retrieval/analyzer routing churn unless a fresh maintained validator run starts failing
- choose the next slice from a different brain-quality area, with ranking/selection quality or broader real-vault note coverage as the best current candidates

Lowest-churn starting point:
- rerun the six maintained validator scenarios against the current `:8092` runtime only when you need a regression check
- otherwise move directly to the next non-routing slice
