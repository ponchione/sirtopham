# SirTopham / conductor boundary for agent roles

This repo intentionally stops at SirTopham-side role execution. It does not implement the external conductor.

## SirTopham owns

In this repository, SirTopham owns:
- loading `agent_roles` from config
- resolving checked-in prompt files from project root
- building the tool registry for a single role run
- enforcing brain write allow/deny paths
- executing one headless `sirtopham run` session
- writing or validating the run receipt contract

## Conductor owns

Out of scope for this repo, and still expected from the external conductor:
- `spawn_agent`
- `chain_complete`
- multi-step sequencing and loop control
- resolver retry limits
- role fan-out / parallel auditor execution
- deterministic reindex triggers before or after roles
- cross-role scheduling policy and chain-wide budgeting

## Important operator consequence

The checked-in config includes an `orchestrator` role because the prompt path, role wiring, and write policy are part of the SirTopham contract.

However, plain local execution still behaves like this:
- `sirtopham run --role coder ...` can run entirely inside SirTopham
- `sirtopham run --role correctness-auditor ...` can run entirely inside SirTopham
- `sirtopham run --role orchestrator ...` is expected to fail unless the external conductor provides the orchestrator custom tools

That failure mode is intentional and truthful: the role exists for config compatibility, but the custom orchestration tools are not implemented in this repo.

## Why the split exists

Keeping conductor behavior out of SirTopham keeps the harness focused on:
- context assembly
- tool dispatch
- brain access
- provider interaction
- receipt production

That keeps orchestration policy, retries, and cross-agent coordination in one external layer instead of duplicating it inside the harness.

## Related docs

- `docs/agent-roles-and-brain-conventions.md`
- `docs/specs/13_Headless_Run_Command.md`
- `docs/specs/14_Agent_Roles_and_Brain_Conventions.md`
