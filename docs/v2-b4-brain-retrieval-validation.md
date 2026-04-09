# V2-B4 proactive brain retrieval validation

This is the maintained live validation package for the current v0.2+ brain-retrieval contract.

What it proves:
- a fresh turn against the running app can answer a brain-only fact from proactive context assembly
- the stored context report shows the actual semantic queries, included brain hits, and non-zero brain budget
- the ordered `/api/metrics/conversation/:id/context/:turn/signals` endpoint exposes the signal/query flow used to make the retrieval decision
- when a scenario expects structural retrieval, the matched `brain_results` hit can also be checked for `match_mode`, `match_sources`, and `graph_hop_depth`
- the package does not pin the canary to a specific lexical-vs-semantic source unless the local vault/runtime makes that stable on purpose

What it does not claim:
- this is not a blanket guarantee that semantic or graph retrieval will outperform lexical retrieval on every real vault
- this is not a guarantee that the model will never choose a reactive brain tool detour for other prompts
- this package is intentionally scoped to the current operator-facing truth: hybrid brain retrieval is now real at runtime, but live confidence still depends on the actual note corpus and graph quality

## Runtime assumptions

Primary validated runtime:
- app: `http://localhost:8092`
- config: `/tmp/my-website-runtime-8092.yaml`
- target project: `~/source/my-website`
- brain vault: `~/source/my-website/.brain/`
- expected note: `notes/runtime-brain-proof-apr-07.md`
- expected fact: `ORBIT LANTERN 642`

The note exists only in the brain vault, not in the repo code. That makes it a useful operator-facing proof that the answer came from brain retrieval rather than ordinary code RAG.

## Maintained scenarios

The validation package now carries six maintained prompt families:

1. `runtime-proof`
   - prompt: `What is the runtime brain proof canary phrase?`
   - expected note: `notes/runtime-brain-proof-apr-07.md`
   - expected answer evidence: `ORBIT LANTERN 642`

2. `rationale-layout`
   - prompt: `From our rationale notes, why did we choose the minimal content-first layout for the site? Answer in one short paragraph.`
   - expected note: `notes/minimal-content-first-layout-rationale.md`
   - expected answer evidence: `minimal content-first layout because`

3. `debug-history-vite`
   - prompt: `From our past debugging notes, what was the root cause of the vite rebuild loop and what was the fix? Answer in two sentences.`
   - expected note: `notes/past-debugging-vite-rebuild-loop.md`
   - expected answer evidence: `src/generated/index.ts`

4. `debug-history-lunar-hinge-graph`
   - prompt: `From our past debugging notes, what phrase sits behind LUNAR HINGE 91?`
   - expected note: `notes/past-debugging-deep-panel-fix.md`
   - expected answer evidence: `DEEP PANEL 23`
   - structural expectation: matched `brain_results` hit includes `match_sources: [.., "graph", ..]` with `graph_hop_depth >= 1`

5. `debug-history-saturn-rail-graph`
   - prompt: `From our past debugging notes, what follow-on fix canary phrase sits behind SATURN RAIL?`
   - expected note: `notes/past-debugging-saturn-rail-fix.md`
   - expected answer evidence: `PANEL LOCK 58`
   - structural expectation: matched `brain_results` hit includes `match_sources: [.., "graph", ..]` with `graph_hop_depth >= 1`

6. `layout-graph-saturn-rail`
   - prompt: `From our layout graph notes, what linked layout canary phrase sits behind SATURN RAIL?`
   - expected note: `notes/layout-graph-proof.md`
   - expected answer evidence: `PROSE FIRST 17`
   - structural expectation: matched `brain_results` hit includes `match_sources: [.., "graph", ..]` with `graph_hop_depth >= 1`, while `prefer_brain_context` remains present and code RAG stays out of the budget

The first scenario is the narrow no-detour canary. The second and third scenarios are the maintained broader live proofs for rationale/decision notes and prior-debugging/history notes. The fourth through sixth scenarios are maintained graph-aware proofs for the seeded bridge/fix and layout-graph pairs. The broader scenarios allow explicit brain tool detours when they happen, but they still fail closed unless the persisted proactive context report shows the expected brain hit, non-zero brain budget, and `prefer_brain_context` signal flow.

On the current validated `my-website` vault/runtime, the strict canary and the broader rationale/debug-history scenarios all match through the hybrid runtime path. The matched hit currently lands as `semantic` for the first three scenarios, while the maintained LUNAR HINGE, SATURN RAIL debug-history, and SATURN RAIL layout-graph canaries land as structural hybrid hits (`match_sources` includes `graph`, `graph_hop_depth = 1`) for their respective target notes. The latest full six-scenario package rerun on 2026-04-09 stayed green on the live `:8092` runtime.

The validator can now also enforce optional graph-aware expectations on the matched `brain_results` hit:
- `--expected-match-mode`
- `--expected-match-source`
- `--min-graph-hop-depth`

Use those only for scenarios whose target vault notes/links are stable enough to support a structural canary; do not overconstrain the general canary scenarios unless the local vault contract is intentionally fixed.

Current real-vault note: the first live pass on the validated `my-website` runtime had `Brain links indexed: 0`, so there was initially no structural canary to maintain. We then seeded linked validation notes and fixed parser/index truth so `.md` wikilink targets persist with exact document paths, and `sirtopham index brain --config /tmp/my-website-runtime-8092.yaml` now reports non-zero links. A follow-up proactive graph-debugging slice found the remaining blocker: `internal/context/brain_search.go` was skipping structural annotation whenever a direct semantic hit had already seeded `bestDepth = 0`, so one-hop graph evidence never attached to already-matched notes. The next slice tightened the reverse-edge policy too: direct bridge-note semantic seeds no longer get noisy `hybrid-backlink` promotion just because the linked fix note points back at them, while the intended fix-side target still keeps `hybrid-graph` annotation. With that cleanup in place, both the LUNAR HINGE 91 / DEEP PANEL 23 pair and the SATURN RAIL / PANEL LOCK 58 pair now yield stable proactive structural evidence in persisted `brain_results`, so this package maintains graph-aware scenarios for both pairs.

## Exact prompt

Use this as the default first-turn prompt:

`What is the runtime brain proof canary phrase?`

Expected answer shape:
- contains `ORBIT LANTERN 642`
- completes from assembled context without explicit tool detours for the current validated runtime
- the matching note path is then corroborated from `brain_results` in the stored context report

## Repeatable command

From the repo root:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario runtime-proof`

Broader rationale-family proof:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario rationale-layout`

Broader prior-debugging/history proof:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-vite`

Maintained graph-aware proof for the LUNAR HINGE bridge/fix pair:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-lunar-hinge-graph`

Maintained graph-aware proof for the SATURN RAIL bridge/fix pair:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario debug-history-saturn-rail-graph`

Maintained graph-aware proof for the SATURN RAIL layout-graph pair:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --scenario layout-graph-saturn-rail`

Optional looser mode for exploratory prompts that may still choose reactive note reads:

`python3 scripts/validate_brain_retrieval.py --base-url http://localhost:8092 --prompt "What is the runtime brain proof canary phrase for this project? Answer in one sentence and cite the source note path." --expected-note notes/runtime-brain-proof-apr-07.md --allow-tool-calls`

The default runtime-proof command is still the strictest canary because it requires the brain-only fact prompt to complete without explicit tool detours while still proving the proactive retrieval path through the persisted context report and signal stream. The rationale and debug-history scenarios are the maintained broader live proofs that V2-B1/V2-B2 now require.

## Passing conditions

The script must exit 0 and print JSON with `"status": "passed"`.

Required evidence in the output for every scenario:
- `assistant_text` includes the scenario's expected phrase
- `semantic_queries` is non-empty
- `brain_results` contains the scenario's expected note
- `budget_breakdown.brain > 0`
- `signal_stream` is non-empty and includes at least:
  - a `semantic_query` entry
  - a `flag` entry with `value: "prefer_brain_context"`

Optional structural evidence when you pass graph-aware flags:
- `matched_brain_hit.match_mode` matches `--expected-match-mode`
- `matched_brain_hit.match_sources` includes `--expected-match-source`
- `matched_brain_hit.graph_hop_depth >= --min-graph-hop-depth`

Useful corroboration:
- `tool_calls` is empty for the canary prompt, or at least does not need to carry the proof by itself
- `event_counts.context_debug == 1`
- `turn_number == 1` for the fresh conversation

## Manual spot checks

If you want to inspect the same run manually, use the printed `conversation_id` and `turn_number`:

`curl -s "http://localhost:8092/api/metrics/conversation/<conversation_id>/context/<turn_number>"`

`curl -s "http://localhost:8092/api/metrics/conversation/<conversation_id>/context/<turn_number>/signals"`

Things to confirm in the full context report:
- `needs.semantic_queries` is populated
- `brain_results` includes the runtime proof note and not just `_log.md`
- `budget_breakdown.brain` is non-zero
- code RAG is absent or clearly secondary for this prompt family
- for structural scenarios, inspect whether the matched hit carries the expected `match_mode`, `match_sources`, `graph_source_path`, and `graph_hop_depth`

Things to confirm in the signal stream:
- the ordered stream shows the same retrieval intent the analyzer produced
- the semantic queries visible there match the queries persisted on the report
- `prefer_brain_context` appears when the prompt is clearly brain-seeking

## Failure interpretation

If the answer is right only after explicit `brain_*` tool use, but the report still shows empty `brain_results` or `budget_breakdown.brain == 0`, treat that as a proactive retrieval regression, not a pass.

If the report shows the expected note in `brain_results` and non-zero brain budget, but the answer is wrong, treat that as an answer-quality or prompt-shaping issue rather than proof that retrieval is absent.

If the signal stream is empty while the full context report looks right, treat that as an observability regression in the narrow `/signals` endpoint or its persistence path.

## Why this package exists

Earlier work proved the individual pieces separately:
- reactive brain tools worked
- proactive brain retrieval could appear in context reports
- the inspector consumed the dedicated `/signals` endpoint

V2-B4 closes the packaging gap by keeping one durable, repeatable command that proves those pieces together against the live app.
