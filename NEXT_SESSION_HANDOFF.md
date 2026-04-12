# Session handoff — sodoryard migration

**Date:** 2026-04-12
**Branch:** main
**Cwd:** /home/gernsback/source/sodoryard

> Read this cold. Everything you need to orient yourself is in here. If anything in this doc disagrees with current repo state, trust the repo and update this doc before acting.

---

## What this project is

Migrating `ponchione/sirtopham` (single-binary coding harness) into the `ponchione/sodoryard` monorepo. The GitHub repo has been renamed; the local directory is `/home/gernsback/source/sodoryard`; the git remote points at `git@github.com:ponchione/sodoryard.git`.

Target monorepo layout (all in place as of this handoff):

- **Tidmouth** — headless engine harness (`cmd/tidmouth/`)
- **SirTopham** — chain orchestrator (`cmd/sirtopham/`)
- **Yard** — operator-facing CLI (`cmd/yard/` — `yard init` + `yard install`)
- **Knapford** — web dashboard (`cmd/knapford/`, placeholder until Phase 6)

The full migration roadmap is `sodor-migration-roadmap.md`.

---

## Current state of all phases

| Phase | Status | Tag |
|---|---|---|
| 0 — prep | done | `v0.1-pre-sodor` |
| 1 — monorepo restructure | done | `v0.2-monorepo-structure` |
| 2 — headless run command | done | (no separate tag) |
| 3 — SirTopham orchestrator | done | `v0.4-orchestrator` |
| 4 — system prompts | **done** (landed this session) | — |
| 5a — yard paths rename | done | `v0.2.1-yard-paths` |
| 5b — yard init | done | `v0.5-yard-init` |
| 6 — Knapford dashboard | deferred | — |
| **7 — yard containerization** | **done** | `v0.7-containerization` |

---

## Phase 7 — complete

**Tag:** `v0.7-containerization`
**Commit range:** `249e28f..d1503fa` (tech debt fixes + Phase 7 implementation)

**What shipped:**
- `yard install` subcommand — substitutes `{{SODORYARD_AGENTS_DIR}}` in yard.yaml from flag or env var
- Three-stage Dockerfile: `node:22-slim` (frontend) → `golang:1.25-trixie` (Go binaries with corrected lancedb rpath) → `debian:trixie-slim` (runtime with codex CLI, lancedb at `/usr/local/lib/`, agents at `/opt/yard/agents/`)
- Root `docker-compose.yaml` — `yard` service + profile-gated `knapford` placeholder, both on `llm-net`
- `.dockerignore` — keeps host artifacts out of build context

**Implementation deviations from plan:**
1. **Trixie instead of Bookworm** — `liblancedb_go.so` requires GLIBC_2.38 (`__isoc23_strtol`, `__isoc23_sscanf`) which Bookworm's glibc 2.36 cannot satisfy. All three stages switched to Trixie/testing-based images.
2. **Codex CLI in runtime** — the codex provider shells out to the `codex` binary for auth token management. Node.js + npm + `@openai/codex` installed in the runtime image.
3. **Go 1.25** — project uses go 1.25.5 (go.mod), not 1.22 as the plan assumed.

**Also shipped (tech debt):**
- R5: drain in-flight sub-call writes before DB close (no more "sql: database is closed" on clean exit)
- R6: only register YAML-configured providers (no more spurious anthropic/openrouter registration)
- R7: real chain metrics in orchestrator receipts (was hardcoded zeros)

**Verified live:** container smoke test — `yard init` + `yard install` + `sirtopham chain` end-to-end inside the container. Both receipts visible on host bind mount. Receipt frontmatter shows real metrics (`turns_used: 1`, `tokens_used: 5966`, `duration_seconds: 3`).

---

## Phase 4 — complete

**What shipped:** production agent prompts (13 files, ~5KB each) with Thomas & Friends engine names:

| Role | Engine | File |
|---|---|---|
| Orchestrator | Sir Topham Hatt | `sirtophamhatt.md` |
| Planner | Gordon | `gordon.md` |
| Epic Decomposer | Edward | `edward.md` |
| Task Decomposer | Emily | `emily.md` |
| Coder | Thomas | `thomas.md` |
| Correctness Auditor | Percy | `percy.md` |
| Quality Auditor | James | `james.md` |
| Performance Auditor | Spencer | `spencer.md` |
| Security Auditor | Diesel | `diesel.md` |
| Integration Auditor | Toby | `toby.md` |
| Test Writer | Rosie | `rosie.md` |
| Resolver | Victor | `victor.md` |
| Docs Arbiter | Harold | `harold.md` |

---

## Deferred

### Phase 6 — Knapford dashboard

Web dashboard that consumes `.brain/`, `.yard/yard.db`, and chain state. The Phase 7 docker-compose.yaml has a profile-gated `knapford` service slot ready. Once Phase 6 ships, the profile gate is removed and `knapford` becomes a default-on service.

**Status:** the largest remaining phase. Needs decomposition into per-epic specs. Phase 4 prompts are now ready for dogfooding.

---

## Recent commits

```
HEAD  d1503fa  fix(docker): install codex CLI in runtime image
      04ebf10  build: add Phase 7 root docker-compose.yaml
      46e6d45  build: add Phase 7 multi-stage Dockerfile
      8885238  build: add .dockerignore for Phase 7 Docker build context
      69fff1c  feat(yard): add yard install subcommand for agents-dir substitution
      25602ab  fix(sirtopham): drain in-flight sub-call writes before DB close
      a2d7f1d  fix(sirtopham): only register YAML-configured providers
      249e28f  fix(spawn): populate real chain metrics in orchestrator receipt
      8592baa  feat(agents): replace stubs with production prompts, rename to engine names
      1198039  docs: update handoff for Phase 5b completion
```

- Working tree at handoff time: intended clean; trust `git status` for the current local checkout state.
- `make test`: green
- `make all`: green (4 binaries: tidmouth, sirtopham, knapford, yard)
- `docker compose build yard`: green
- Tags: `v0.1-pre-sodor`, `v0.2-monorepo-structure`, `v0.2.1-yard-paths`, `v0.4-orchestrator`, `v0.5-yard-init`, `v0.7-containerization`
- **Not pushed.** User pushes manually.

---

## Operational notes

### Hard rules

- **Per-step commits** — don't batch multi-task work into one mega-commit.
- **Do not push** — the user pushes manually.
- **Do not skip git hooks** unless the user explicitly asks.

### Running the containerized railway

```bash
# Build the image
docker compose build yard

# Initialize a project
PROJECT_DIR=/path/to/project docker compose run --rm yard yard init
PROJECT_DIR=/path/to/project docker compose run --rm yard yard install

# Run a chain (needs codex auth mounted)
PROJECT_DIR=/path/to/project docker compose run --rm \
  -v ~/.sirtopham:/root/.sirtopham:ro \
  yard sirtopham chain --config /project/yard.yaml --task "..."
```

### Where to find things

- **Templates:** `internal/initializer/templates/init/` (moved from repo-root `templates/init/` during Phase 5b)
- **Agent prompts:** `agents/` — 13 engine-named `.md` files
- **Specs:** `docs/specs/16-yard-init.md`, `docs/specs/17-yard-containerization.md`, `docs/specs/18-unified-yard-cli.md`
- **Roadmap:** `sodor-migration-roadmap.md`
- **Tech debt:** `TECH-DEBT.md` (R5/R6/R7 closed this session; R1-R4 remain)

### Codex auth

Tokens in `~/.sirtopham/auth.json` expire 2026-04-13. Re-auth via `codex auth` if needed.

---

## Next session — Phase 8: Unified `yard` CLI

**Execute immediately.** No further design work needed.

**Spec:** `docs/specs/18-unified-yard-cli.md`
**Plan:** `docs/plans/2026-04-12-phase-8-unified-yard-cli-implementation-plan.md` (3350 lines, 16 tasks)

**Execution method:** Use `superpowers:subagent-driven-development` — dispatch a fresh subagent per task, review between tasks.

**What it ships:** All 19 operator-facing commands under the `yard` binary. `internal/runtime/` package with extracted runtime builders. Legacy binaries (`tidmouth`, `sirtopham`) continue building unchanged. Tag: `v0.8-unified-cli`.

**After Phase 8:** The operator workflow becomes `yard serve`, `yard chain start`, `yard index`, `yard brain index`, etc. — one CLI, one `--help`. Then dogfood on a real project.

### After Phase 8

1. **Daily-driver dogfooding** — point `yard` at a real project, run the full workflow (init → install → index → serve → chain), prove it works end-to-end through the UI.

2. **Phase 6 — Knapford features folded into `yard serve`** — chain timelines, brain explorer, analytics added to the existing web UI rather than a separate binary. Epic decomposition needed first.

3. **TECH-DEBT R1/R2/R3** — daily-driver validation, brain retrieval quality, index freshness UX.

---

## When in doubt

- Trust the repo over this doc.
- Per-step commits with clear messages are always safe.
- Don't push. Don't skip hooks. Don't expand scope.
