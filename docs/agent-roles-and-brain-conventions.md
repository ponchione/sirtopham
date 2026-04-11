# Agent roles and `.brain/` conventions

This document describes the checked-in SirTopham role prompts, the canonical `agent_roles` wiring in `sirtopham.yaml`, and the `.brain/` layout expected by the spec-13/spec-14 headless workflow.

## What SirTopham supports in this repo

Today, SirTopham provides these pieces on its own:
- checked-in role prompt assets under `agents/`
- checked-in canonical `agent_roles` config in `sirtopham.yaml`
- role-scoped tool registration for `sirtopham run --role ...`
- role-scoped brain write-path enforcement
- headless `run` execution for a single role session

Today, SirTopham does not provide orchestrator custom tools such as `spawn_agent` or `chain_complete`. Those remain external-conductor responsibilities.

## `.brain/` layout

The intended vault structure is:

```text
.brain/
├── specs/
├── architecture/
├── epics/
├── tasks/
├── plans/
├── receipts/
├── logs/
├── conventions/
└── _log.md
```

Recommended subdirectories:
- `specs/`: human-owned feature requirements
- `architecture/`: system design notes, diagrams, invariants
- `epics/`: feature-level decomposition output
- `tasks/`: ordered task documents produced from epics
- `plans/`: implementation plans for individual tasks
- `receipts/`: machine-readable completion records per role/run
- `logs/`: append-oriented operational notes per role/run
- `conventions/`: coding standards, testing strategy, repo conventions
- `_log.md`: global brain activity log maintained by the harness

## Ownership and expected writers

| Path | Primary owner | Typical writers |
| --- | --- | --- |
| `specs/**` | human | human, docs-arbiter for factual corrections only |
| `architecture/**` | human | human, docs-arbiter |
| `conventions/**` | human | human |
| `epics/**` | epic-decomposer | epic-decomposer, orchestrator if explicitly allowed |
| `tasks/**` | task-decomposer | task-decomposer, orchestrator if explicitly allowed |
| `plans/**` | planner | planner, coder, orchestrator if explicitly allowed |
| `receipts/**` | per-role | the role writing its own receipt |
| `logs/**` | per-role | the role writing its own logs |
| `_log.md` | SirTopham harness | harness only |

SirTopham enforces only write-path policy. Brain reads are intentionally unrestricted.

## Naming conventions

Recommended document paths:
- `epics/{slug}/epic.md`
- `tasks/{epic-slug}/{NN-task-slug}.md`
- `plans/{epic-slug}/{NN-task-slug}.md`
- `receipts/{role}/{chain-id}--{task-slug}.md`

A few canonical checked-in roles intentionally use shorter receipt/log directory names to match spec-14 examples:
- `correctness-auditor` -> `receipts/correctness/**`, `logs/correctness/**`
- `quality-auditor` -> `receipts/quality/**`, `logs/quality/**`
- `performance-auditor` -> `receipts/performance/**`, `logs/performance/**`
- `security-auditor` -> `receipts/security/**`, `logs/security/**`
- `integration-auditor` -> `receipts/integration/**`, `logs/integration/**`
- `test-writer` -> `receipts/tests/**`, `logs/tests/**`
- `docs-arbiter` -> `receipts/arbiter/**`, `logs/arbiter/**`

The harness does not hard-code receipt naming. The configured role write paths are the effective constraint.

## Canonical role/tool mapping

The checked-in `agent_roles` block uses this mapping:

| Role | Tools |
| --- | --- |
| `orchestrator` | `brain` plus external `custom_tools` |
| `epic-decomposer` | `brain` |
| `task-decomposer` | `brain` |
| `planner` | `brain`, `search` |
| `coder` | `brain`, `file`, `git`, `shell`, `search` |
| auditor roles | `brain`, `file:read`, `git` |
| `test-writer` | `brain`, `file`, `git`, `shell` |
| `resolver` | `brain`, `file`, `git`, `shell` |
| `docs-arbiter` | `brain` |

`file:read` is the read-only file group. It exposes `file_read` without `file_write` or `file_edit`.

## What SirTopham enforces at runtime

For `sirtopham run --role <name>` today:
- the role must exist in `agent_roles`
- the role prompt path is resolved relative to project root
- tool groups are scoped by role
- brain writes and brain updates are restricted by `brain_write_paths` and `brain_deny_paths`
- `custom_tools` remain unsupported by plain SirTopham role execution and fail clearly in the role builder

## Operator notes

- Keep `.brain/specs/`, `.brain/architecture/`, and `.brain/conventions/` human-readable and durable. Other role outputs can be regenerated.
- Prefer receipt bodies that summarize what changed, what was validated, and what should happen next.
- If you run roles manually, choose roles whose tool sets match what SirTopham can actually provide locally.
- Plain `sirtopham run --role orchestrator ...` is expected to fail unless an external conductor injects the orchestrator custom tools.

## Related docs

- `docs/specs/13_Headless_Run_Command.md`
- `docs/specs/14_Agent_Roles_and_Brain_Conventions.md`
- `docs/agent-role-conductor-boundary.md`
