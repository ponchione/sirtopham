# Session handoff — sodoryard migration

**Date:** 2026-04-11
**Branch:** main
**Cwd:** /home/gernsback/source/sodoryard

> Read this cold. Everything you need to orient yourself is in here. If anything in this doc disagrees with current repo state, trust the repo and update this doc before acting.

---

## What this project is

Migrating `ponchione/sirtopham` (single-binary coding harness) into the `ponchione/sodoryard` monorepo. The GitHub repo has been renamed; the local directory is `/home/gernsback/source/sodoryard`; the git remote points at `git@github.com:ponchione/sodoryard.git`.

Target monorepo layout (all in place as of this handoff):

- **Tidmouth** — headless engine harness (`cmd/tidmouth/`)
- **SirTopham** — chain orchestrator (`cmd/sirtopham/`)
- **Yard** — operator-facing CLI for project bootstrap (`cmd/yard/`, partial — see "Phase 5b ready" below)
- **Knapford** — web dashboard (`cmd/knapford/`, placeholder until Phase 6)

The full migration roadmap is `sodor-migration-roadmap.md`. Read phases 0–7 before touching code.

---

## Current state of all phases

| Phase | Status | Tag |
|---|---|---|
| 0 — prep | ✅ done | `v0.1-pre-sodor` |
| 1 — monorepo restructure | ✅ done | `v0.2-monorepo-structure` |
| 2 — headless run command | ✅ done | (no separate tag) |
| 3 — SirTopham orchestrator | ✅ done | `v0.4-orchestrator` |
| 4 — system prompts | 🛠 deferred (handled out-of-band) | — |
| 5a — yard paths rename | ✅ done | `v0.2.1-yard-paths` |
| **5b — yard init** | 📋 spec + plan ready, NOT executed | (will be `v0.5-yard-init`) |
| 6 — Knapford dashboard | 🛠 deferred (waiting on Phase 4) | — |
| **7 — yard containerization** | 📋 spec + plan ready, NOT executed | (will be `v0.7-containerization`) |

Phases 5b and 7 both have committed specs and committed implementation plans. They are ready to execute task-by-task with no further design work needed.

---

## Phase 5b — ready to execute

**Spec:** `docs/specs/16-yard-init.md` (386 lines)
**Plan:** `docs/plans/2026-04-11-phase-5b-yard-init-implementation-plan.md` (2365 lines)

**What it ships:**
- New `cmd/yard/` top-level binary (`yard init` subcommand only)
- New `internal/initializer/` package — embedded `templates/init/` via `//go:embed all:templates/init`, substitution helpers, the moved Obsidian config writer / gitignore patcher / database bootstrap from `cmd/tidmouth/init.go`, the `Run()` orchestrator
- Rewritten `templates/init/yard.yaml.example` with all 13 `agent_roles` entries seeded with `{{SODORYARD_AGENTS_DIR}}` placeholders
- Deletion of `cmd/tidmouth/init.go` and `cmd/tidmouth/init_test.go` (the existing init logic moves wholesale to the new package)
- Makefile `yard:` target with the same FTS5/lancedb cgo wiring as `tidmouth:` and `sirtopham:`

**Locked decisions** (do not re-litigate during execution):
1. New top-level `cmd/yard` binary (not a Tidmouth subcommand)
2. All 13 agent roles seeded with `{{SODORYARD_AGENTS_DIR}}` placeholders
3. `templates/init/` is canonical source of truth, embedded via `go:embed`
4. Default provider: `codex` / `gpt-5.4-mini`
5. `cmd/tidmouth/init.go` deleted outright (no deprecation alias)
6. Exactly two substitutions at copy time: `{{PROJECT_ROOT}}` and `{{PROJECT_NAME}}`
7. Idempotent re-run is a no-op

**Smoke verification:** plan task 12 runs `yard init` against a fresh `/tmp/yard-init-smoke-*` directory, confirms the file tree, hand-substitutes `{{SODORYARD_AGENTS_DIR}}` via `sed`, and runs a real `sirtopham chain` against the freshly initialized project end-to-end.

---

## Phase 7 — ready to execute

**Spec:** `docs/specs/17-yard-containerization.md` (407 lines)
**Plan:** `docs/plans/2026-04-11-phase-7-yard-containerization-implementation-plan.md` (1288 lines)

**Depends on:** Phase 5b must be fully landed and tagged `v0.5-yard-init` first. Phase 7's plan assumes `cmd/yard/main.go`, `cmd/yard/init.go`, and `internal/initializer/` already exist.

**What it ships:**
- New `cmd/yard/install.go` subcommand — substitutes `{{SODORYARD_AGENTS_DIR}}` in `yard.yaml` (the placeholder Phase 5b's `yard init` deliberately leaves manual). Reads `--sodoryard-agents-dir` flag or `SODORYARD_AGENTS_DIR` env var. Idempotent.
- New `internal/initializer/install.go` + tests — the substitution function (testable in isolation).
- New `Dockerfile` — three-stage build: `node:20-bookworm-slim` (frontend) → `golang:1.22-bookworm` (binaries with corrected lancedb rpath) → `debian:bookworm-slim` (runtime with `liblancedb_go.so` at `/usr/local/lib/` + `ldconfig` + agent prompts at `/opt/yard/agents/`).
- New `docker-compose.yaml` at the repo root — `yard` service plus a profile-gated `knapford` placeholder. Both share the existing external `llm-net` network with `ops/llm/docker-compose.yml`.
- New `.dockerignore` — keeps host artifacts out of the build context. **Intentionally does NOT exclude `templates/init/`** (the Go builder embeds it via `go:embed`).

**Locked decisions** (do not re-litigate during execution):
1. Headless-only Phase 7 — Knapford service is a profile-gated placeholder until Phase 6
2. `yard install` is the only command that performs the agents-dir substitution; `yard init` stays unchanged
3. `debian:bookworm-slim` runtime base (no alpine, no distroless, no scratch — hard glibc constraint from lancedb)
4. amd64 only (no `linux_arm64` lancedb library exists)
5. Two compose files, share `llm-net` external network — `ops/llm/docker-compose.yml` is NOT modified
6. `liblancedb_go.so` staged at `/usr/local/lib/` + `ldconfig` + builder rpath rebuild (belt-and-suspenders)
7. Image filesystem: binaries at `/usr/local/bin/`, agent prompts at `/opt/yard/agents/`, project bind-mounted at `/project`
8. Tag is `v0.7-containerization`, NOT `v1.0-sodor` (`v1.0-sodor` is reserved for Phase 6 + Phase 7 shipped together)

**Smoke verification:** plan task 11 runs a real `sirtopham chain` inside the container against a `/tmp/yard-container-smoke-*` host bind mount, with `~/.sirtopham/auth.json` mounted read-only as `/root/.sirtopham/auth.json`. Confirms the container can talk to the codex API, the bind mount works in both directions, the lancedb cgo binary loads, and a freshly `yard init`'d + `yard install`'d project is immediately usable end-to-end.

---

## Deferred (not in this session)

### Phase 4 — system prompts

The 13 agent prompt files in `agents/` are operational stubs (13–27 lines each) inherited from earlier phases. Phase 4 expands them into production prompts: identity & boundaries, brain interaction protocol, tool usage guidance, quality criteria, output expectations.

**Status:** the user is handling Phase 4 out-of-band with a dedicated prompt agent. Do NOT touch `agents/` files in this repo unless the user explicitly asks. Phase 4 may have landed by the time the next session starts.

### Phase 6 — Knapford dashboard

Web dashboard that consumes `.brain/`, `.yard/yard.db`, and the chain state to render conversations, chain timelines, brain explorer, review queue, agent drilldown, analytics. Roadmap calls this the largest phase; needs decomposition into per-epic specs.

**Status:** deferred until Phase 4 prompts are ready for dogfooding. Phase 6 is the natural dogfooding target for the railway — once chains produce useful artifacts, Knapford gives them a place to display.

The Phase 7 docker-compose.yaml has a profile-gated `knapford` service slot waiting for Phase 6 to fill in. Once Phase 6 ships, the profile gate gets removed and `knapford` becomes a default-on service.

---

## Recent commits

```
HEAD  6c08d0f  chore(plans): delete audit fix followup plan, bug already fixed
      6e2851b  chore(docs): delete historical layer0-6 build epic docs (360 files)
      108021c  chore(docs): delete stale operator boundary docs
      3e3049f  chore(plans): delete completed-phase implementation plans
      4733d25  chore: remove empty README.md
      87e84c4  docs(plans): add Phase 7 yard containerization implementation plan
      898aab1  docs(spec17): add Phase 7 yard containerization design spec
      aeeb2ae  docs(plans): add Phase 5b yard init implementation plan
      4b9ff35  docs(spec16): add Phase 5b yard init design spec
      ee685a1  docs(tech-debt): log three orchestrator follow-ups surfaced by phase 3 verification
      103e491  fix(build): give sirtopham the same FTS5 and lancedb cgo flags as tidmouth
      a259c3f  docs(plans): land phase 3 sirtopham orchestrator implementation plan and handoff
      5f9809e  docs(spec15): align chain orchestrator spec with shipped phase 3 implementation
      e71cd26  docs(agents): expand orchestrator system prompt for phase 3 smoke test
      a5aebe0  feat(sirtopham): wire orchestrator CLI on top of chain store and spawn tools
      b0ffeca  feat(spawn): add spawn_agent and chain_complete tools with subprocess driver
      48078d4  feat(role,tool,agent): add custom-tool factory and ErrChainComplete sentinel
      d93e5f0  feat(chain): add chain store with state transitions and limits
      fd2b82b  feat(db): add chains/steps/events schema and sqlc bindings
      ac5e9ad  feat(receipt): add shared brain receipt parser package   ← v0.4-orchestrator starts here
      619f5a7  docs: add phase 5a post-review cleanup to handoff commit stack
```

- Working tree: clean
- `make test`: green (42 packages)
- `make all`: green (4 binaries: tidmouth, sirtopham, yard, knapford — wait, **yard does not build yet** because Phase 5b hasn't been executed; `make all` produces 3 binaries: tidmouth, sirtopham, knapford)
- Tags: `v0.1-pre-sodor`, `v0.2-monorepo-structure`, `v0.2.1-yard-paths`, `v0.4-orchestrator`
- **Not pushed.** User pushes manually.

---

## Operational notes

### Hard rules

- **Per-step commits** — don't batch multi-task work into one mega-commit.
- **Do not push** — the user pushes manually.
- **Do not skip git hooks** unless the user explicitly asks.
- **Do not touch `agents/`** — Phase 4 prompts are being handled out-of-band.
- **Do not touch `cmd/sirtopham/` or `internal/{chain,spawn,receipt}/`** beyond the TECH-DEBT items unless the user explicitly asks — Phase 3 just shipped and is stable.
- **Do not modify `ops/llm/docker-compose.yml`** in Phase 7 — it's an independent compose file with its own lifecycle.

### Pre-flight checks before any smoke test

Local llama.cpp services (only needed if dogfooding against local LLM):

```bash
curl -s --max-time 3 http://localhost:12434/v1/models | head -c 80
curl -s --max-time 3 http://localhost:12435/v1/models | head -c 80
```

Both should return `{"models":[...]}`. If down: `cd ops/llm && docker compose up -d`.

Codex auth:

```bash
./bin/tidmouth auth status
```

Should show `codex (codex): healthy` with non-expired tokens. As of 2026-04-11 the tokens expire 2026-04-13 — re-auth via `codex auth` if needed.

### Where to find things

- **Current product specs:** `docs/specs/01-15` (numbered) plus `docs/specs/16-yard-init.md` and `docs/specs/17-yard-containerization.md`. The `00-index.md` in the same directory is stale (only references 01–09) and should be either updated or deleted in a future session.
- **Ready-to-execute implementation plans:** `docs/plans/2026-04-11-phase-5b-yard-init-implementation-plan.md` and `docs/plans/2026-04-11-phase-7-yard-containerization-implementation-plan.md`. Older plans for completed phases were deleted during this session's cleanup; recover from git history if needed (`git log --all --diff-filter=D -- docs/plans/`).
- **Roadmap:** `sodor-migration-roadmap.md` (overall phase plan, still authoritative).
- **Tech debt:** `TECH-DEBT.md` (R5/R6/R7 are open Phase 3 follow-ups; R1–R4 are pre-existing).
- **Operator boundary docs:** previously at `docs/agent-role-conductor-boundary.md` and `docs/agent-roles-and-brain-conventions.md` — both deleted as stale this session. The current truth is in `docs/specs/14_Agent_Roles_and_Brain_Conventions.md` and `docs/specs/15-chain-orchestrator.md`.
- **Conductor v1 reference:** `conductor-v1-extraction.md` at the repo root — historical reference for what was migrated from the archived `ponchione/agent-conductor` repo. Useful background for understanding why Phase 3 looks the way it does.
- **Live validation procedures:** `MANUAL_LIVE_VALIDATION.md` at the repo root — referenced by TECH-DEBT R1.

### Tools and services this project uses

- **llama.cpp** at `localhost:12434` (qwen-coder-7b, code completion) and `localhost:12435` (nomic-embed-code, embeddings). Managed via `ops/llm/docker-compose.yml`. GPU-required (CUDA images). Optional — only needed if dogfooding against local inference rather than codex/anthropic.
- **Codex auth** stored in `~/.sirtopham/auth.json`. Status check: `./bin/tidmouth auth status`. Imports from `~/.codex/auth.json` if its own store is empty.
- **LanceDB** via `lib/linux_amd64/liblancedb_go.so`. Tests need the env vars set by `make test`. Phase 7 stages this into `/usr/local/lib/` inside the runtime image.
- **sqlc** generates `internal/db/*.sql.go` from `internal/db/query/*.sql`. If you change SQL, regenerate (`sqlc generate` from `sqlc.yaml`).

### Next session

The next session should pick one of:

1. **Execute Phase 5b** — work through `docs/plans/2026-04-11-phase-5b-yard-init-implementation-plan.md` task by task. Use either the subagent-driven-development skill (recommended, two-stage review per task) or the executing-plans skill (inline batched execution). End state: `bin/yard` builds, `tidmouth init` no longer exists, smoke chain passes against a freshly initialized project, tag `v0.5-yard-init`.

2. **Execute Phase 7** — only after Phase 5b is fully landed and tagged. Work through `docs/plans/2026-04-11-phase-7-yard-containerization-implementation-plan.md` task by task. End state: `Dockerfile` and `docker-compose.yaml` at the repo root, `yard install` command exists, container smoke chain passes end-to-end with the codex auth bind mount, tag `v0.7-containerization`.

3. **TECH-DEBT cleanup** — work through R5/R6/R7 (Phase 3 orchestrator follow-ups) if Phase 5b/7 execution isn't a priority right now. Each is small (one to two commits) and surfaces from the Phase 3 verification this session.

The user has explicitly said Phase 4 (prompts) and Phase 6 (Knapford) are NOT for this conversation series — Phase 4 is being handled out-of-band, Phase 6 waits for Phase 4 to mature.

---

## When in doubt

- Trust the repo over this doc. If git status, the actual code, or `make test` says something different from this doc, the doc is wrong — fix the doc before acting.
- Per-step commits with clear messages are always safe.
- Don't push. Don't skip hooks. Don't expand scope.
