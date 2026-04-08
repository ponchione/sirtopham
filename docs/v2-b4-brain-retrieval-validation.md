# V2-B4 proactive brain retrieval validation

This is the maintained live validation package for the current v0.2 brain-retrieval contract.

What it proves:
- a fresh turn against the running app can answer a brain-only fact from proactive context assembly
- the stored context report shows the actual semantic queries, included brain hits, and non-zero brain budget
- the ordered `/api/metrics/conversation/:id/context/:turn/signals` endpoint exposes the signal/query flow used to make the retrieval decision

What it does not claim:
- this is not proof of semantic/index-backed brain retrieval
- this is not a guarantee that the model will never choose a reactive brain tool detour for other prompts
- this package is intentionally scoped to the current operator-facing truth: MCP/vault-backed keyword retrieval is live today

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

The validation package now carries three maintained prompt families:

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

The first scenario is the narrow no-detour canary. The second and third scenarios are the maintained broader live proofs for rationale/decision notes and prior-debugging/history notes. Those broader scenarios allow explicit brain tool detours when they happen, but they still fail closed unless the persisted proactive context report shows the expected brain hit, non-zero brain budget, and `prefer_brain_context` signal flow.

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
