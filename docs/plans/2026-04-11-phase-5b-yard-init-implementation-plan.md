# Phase 5b Yard Init Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `yard init` as a top-level command in a new `cmd/yard` binary that bootstraps any project for railway use, replacing the existing `tidmouth init` command and reconciling the parallel `templates/init/` source-of-truth.

**Architecture:** Add a new fourth binary `cmd/yard/` (cobra root + thin `init` subcommand) that delegates all work to a new `internal/initializer/` package. The package uses `//go:embed templates/init/* templates/init/**/*` to bake the canonical template tree into the binary at build time, performs minimal `{{PROJECT_ROOT}}` / `{{PROJECT_NAME}}` substitution at copy time, and reuses the existing SQLite schema bootstrap (`appdb.InitIfNeeded`). The current `cmd/tidmouth/init.go` implementation is split: salvageable parts (Obsidian config writer, gitignore patcher, database init) move into `internal/initializer/`, then the original files are deleted outright with no deprecation alias.

**Tech Stack:** Go 1.22+, cobra, `embed.FS`, `internal/db` (sqlc + sqlite_fts5), `internal/config`. No new third-party dependencies.

**Spec:** [`docs/specs/16-yard-init.md`](../specs/16-yard-init.md)

---

## Required reading before starting

Read these in order:

1. `AGENTS.md` — repo conventions and hard rules
2. `docs/specs/16-yard-init.md` — the design spec this plan implements
3. `cmd/tidmouth/init.go` — the implementation being split apart and replaced
4. `cmd/tidmouth/init_test.go` — the tests being moved
5. `templates/init/yard.yaml.example` — the current 2-role template, being rewritten
6. `internal/db/init.go` — `OpenDB`, `InitIfNeeded`, the helpers init reuses
7. `cmd/sirtopham/main.go` — reference for cobra root + subcommand wiring (the closest analog to what `cmd/yard` will look like)
8. `Makefile` — to understand the existing `tidmouth:` and `sirtopham:` build patterns

After reading, run `make build && make test` to confirm the baseline is green before touching anything.

---

## Locked decisions (do not re-litigate)

These are fixed for Phase 5b. If implementation reveals one is wrong, stop and ask before changing.

1. New top-level `cmd/yard` binary, **not** a Tidmouth subcommand.
2. Exactly one command in this phase: `yard init`. No `yard run`, `yard chain`, `yard status`, etc.
3. All 13 agent roles are seeded into the generated `yard.yaml`, every time, with `{{SODORYARD_AGENTS_DIR}}/<role>.md` placeholder paths.
4. `templates/init/` is the canonical source, embedded via `//go:embed`. The inline `generateConfigYAML()` function dies.
5. Default provider is `codex` / `gpt-5.4-mini`. Anthropic is not seeded.
6. `cmd/tidmouth/init.go` and `cmd/tidmouth/init_test.go` are **deleted outright**. No deprecation alias.
7. Substitution at copy time is exactly two tokens: `{{PROJECT_ROOT}}` (absolute path from `os.Getwd()`) and `{{PROJECT_NAME}}` (basename). Every other `{{...}}` token stays as a placeholder for the operator.
8. `yard init` is non-interactive, has no `--force` / `--reset` flag, and always operates on `os.Getwd()`.
9. Re-running `yard init` against an already-initialized project is an idempotent no-op.
10. The new `yard:` Makefile target uses the same `CGO_BUILD_ENV` and `GOFLAGS_DB` as `tidmouth:` and `sirtopham:`.

---

## File structure

**New files:**

```
cmd/yard/
├── main.go                       # cobra root, ~30 lines
└── init.go                       # init subcommand wrapper, ~50 lines

internal/initializer/
├── initializer.go                # Run() entrypoint that orchestrates everything
├── templates.go                  # //go:embed declaration + walkers
├── substitute.go                 # placeholder substitution at copy time
├── obsidian.go                   # Obsidian config writer (moved from cmd/tidmouth/init.go)
├── gitignore.go                  # .gitignore patcher (moved from cmd/tidmouth/init.go)
├── database.go                   # SQLite bootstrap (moved from cmd/tidmouth/init.go)
├── initializer_test.go           # integration test against tempdir
├── substitute_test.go            # placeholder substitution unit tests
├── obsidian_test.go              # obsidian config unit tests
├── gitignore_test.go             # gitignore patching unit tests
└── database_test.go              # database init unit tests
```

**Modified files:**

```
templates/init/yard.yaml.example  # rewritten with all 13 agent_roles + {{PLACEHOLDERS}}
cmd/tidmouth/main.go              # remove newInitCmd registration
Makefile                          # add yard: target, add yard to all:
```

**Deleted files:**

```
cmd/tidmouth/init.go
cmd/tidmouth/init_test.go
```

Each file has one responsibility:
- `cmd/yard/*` — CLI surface only, no business logic
- `internal/initializer/initializer.go` — orchestration (calls the helpers in the right order)
- `internal/initializer/{templates,substitute,obsidian,gitignore,database}.go` — one helper per file, each independently testable

---

## Checkpoints

| Checkpoint | Tasks | Proof |
|---|---|---|
| CP1: template + embed + substitution | 1, 2, 3 | new `internal/initializer` compiles, embed FS contains the rewritten template, substitute_test passes |
| CP2: helpers moved | 4, 5, 6 | obsidian, gitignore, database helpers all live in internal/initializer with passing tests; cmd/tidmouth/init.go still compiles by referencing the new package |
| CP3: Run() entrypoint | 7 | initializer.Run() bootstraps a tempdir from scratch; integration test passes |
| CP4: cmd/yard binary | 8, 9, 10 | `make yard` produces `bin/yard`; `bin/yard init` works against a tempdir |
| CP5: cleanup + verify | 11, 12, 13 | `tidmouth init` no longer exists; live smoke test in /tmp passes; `v0.5-yard-init` tagged |

If you finish a session mid-checkpoint, update `NEXT_SESSION_HANDOFF.md` with the current checkpoint, the failing command/test, and the next unresolved sub-step.

---

## Task 1: Rewrite `templates/init/yard.yaml.example` with all 13 agent_roles

**Files:**
- Modify: `templates/init/yard.yaml.example`

**Background:** the current template has only `coder` and `correctness-auditor` seeded with `/path/to/sodoryard/agents/...` paths. Phase 5b requires all 13 roles with `{{PLACEHOLDER}}` syntax. The provider stays codex/gpt-5.4-mini (already correct in the template).

- [ ] **Step 1.1: Replace the template file content**

Overwrite `templates/init/yard.yaml.example` with the following content **exactly**:

```yaml
# yard.yaml — generated by `yard init`
#
# Project: {{PROJECT_NAME}}
#
# This file was created by `yard init`. Two tokens were substituted at
# generation time: {{PROJECT_ROOT}} and {{PROJECT_NAME}}. The remaining
# {{PLACEHOLDERS}} below need a one-time hand substitution before the
# railway can run against this project — most notably:
#
#   {{SODORYARD_AGENTS_DIR}}  → absolute path to your sodoryard install's
#                                agents/ directory
#                                (e.g. /home/you/source/sodoryard/agents)
#
# Find-and-replace works fine. After substitution, run:
#
#   tidmouth index --config yard.yaml
#   sirtopham chain --config yard.yaml --task "..."
#
# See docs/specs/13_Headless_Run_Command.md and
# docs/specs/14_Agent_Roles_and_Brain_Conventions.md for the full
# config schema documentation.

project_root: {{PROJECT_ROOT}}
log_level: info
log_format: text

server:
  host: localhost
  port: 8090
  dev_mode: false
  open_browser: false

routing:
  default:
    provider: codex
    model: gpt-5.4-mini

providers:
  codex:
    type: codex
    model: gpt-5.4-mini

index:
  include:
    - "**/*.go"
    - "**/*.py"
    - "**/*.ts"
    - "**/*.tsx"
    - "**/*.js"
    - "**/*.jsx"
    - "**/*.sql"
    - "**/*.md"
    - "**/*.yaml"
    - "**/*.yml"
    - "**/*.json"
    - "**/*.html"
    - "**/*.css"
  exclude:
    - "**/.git/**"
    - "**/.yard/**"
    - "**/.brain/**"
    - "**/node_modules/**"
    - "**/vendor/**"
    - "**/dist/**"
    - "**/build/**"
    - "**/coverage/**"
    - "**/.next/**"
    - "**/.turbo/**"
    - "**/*.min.js"

brain:
  enabled: true
  vault_path: .brain
  log_brain_queries: true

# Agent roles — all 13 are seeded by `yard init`. Each system_prompt path
# uses the {{SODORYARD_AGENTS_DIR}} placeholder; substitute once with your
# sodoryard install path and every role will resolve.
#
# Roles:
#   orchestrator         — dispatches engines via spawn_agent + chain_complete
#   coder                — implements code changes from a plan
#   planner              — turns tasks into implementation plans
#   test-writer          — writes tests for required behavior
#   resolver             — fixes auditor findings
#   correctness-auditor  — validates implementation against requirements
#   integration-auditor  — validates inter-component behavior
#   performance-auditor  — validates performance against budgets
#   security-auditor     — validates against security expectations
#   quality-auditor      — validates code quality and conventions
#   docs-arbiter         — owns conventions and documentation drift
#   epic-decomposer      — turns specs into feature-level epics
#   task-decomposer      — turns epics into ordered tasks

agent_roles:
  orchestrator:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/orchestrator.md
    tools:
      - brain
    custom_tools:
      - spawn_agent
      - chain_complete
    brain_write_paths:
      - "receipts/orchestrator/**"
      - "logs/orchestrator/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 50
    max_tokens: 500000

  coder:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
    tools:
      - brain
      - file
      - git
      - shell
      - search
    brain_write_paths:
      - "receipts/coder/**"
      - "logs/coder/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 100
    max_tokens: 1000000

  planner:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/planner.md
    tools:
      - brain
      - file:read
      - search
    brain_write_paths:
      - "plans/**"
      - "receipts/planner/**"
      - "logs/planner/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
    max_turns: 30
    max_tokens: 300000

  test-writer:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/test-writer.md
    tools:
      - brain
      - file
      - git
      - shell
      - search
    brain_write_paths:
      - "receipts/test-writer/**"
      - "logs/test-writer/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 50
    max_tokens: 500000

  resolver:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/resolver.md
    tools:
      - brain
      - file
      - git
      - shell
      - search
    brain_write_paths:
      - "receipts/resolver/**"
      - "logs/resolver/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 80
    max_tokens: 800000

  correctness-auditor:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/correctness-auditor.md
    tools:
      - brain
      - file:read
      - git
      - search
    brain_write_paths:
      - "receipts/correctness-auditor/**"
      - "logs/correctness-auditor/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  integration-auditor:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/integration-auditor.md
    tools:
      - brain
      - file:read
      - git
      - search
      - shell
    brain_write_paths:
      - "receipts/integration-auditor/**"
      - "logs/integration-auditor/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  performance-auditor:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/performance-auditor.md
    tools:
      - brain
      - file:read
      - git
      - search
      - shell
    brain_write_paths:
      - "receipts/performance-auditor/**"
      - "logs/performance-auditor/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  security-auditor:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/security-auditor.md
    tools:
      - brain
      - file:read
      - git
      - search
    brain_write_paths:
      - "receipts/security-auditor/**"
      - "logs/security-auditor/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  quality-auditor:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/quality-auditor.md
    tools:
      - brain
      - file:read
      - git
      - search
    brain_write_paths:
      - "receipts/quality-auditor/**"
      - "logs/quality-auditor/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  docs-arbiter:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/docs-arbiter.md
    tools:
      - brain
      - file:read
      - search
    brain_write_paths:
      - "conventions/**"
      - "receipts/docs-arbiter/**"
      - "logs/docs-arbiter/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "epics/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  epic-decomposer:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/epic-decomposer.md
    tools:
      - brain
      - file:read
      - search
    brain_write_paths:
      - "epics/**"
      - "receipts/epic-decomposer/**"
      - "logs/epic-decomposer/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "tasks/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

  task-decomposer:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/task-decomposer.md
    tools:
      - brain
      - file:read
      - search
    brain_write_paths:
      - "tasks/**"
      - "receipts/task-decomposer/**"
      - "logs/task-decomposer/**"
    brain_deny_paths:
      - "specs/**"
      - "architecture/**"
      - "conventions/**"
      - "epics/**"
      - "plans/**"
    max_turns: 30
    max_tokens: 300000

local_services:
  enabled: false
  mode: manual
  provider: docker-compose
  compose_file: ./ops/llm/docker-compose.yml
  project_dir: ./ops/llm
  required_networks:
    - llm-net
  auto_create_networks: true
  startup_timeout_seconds: 180
  healthcheck_interval_seconds: 2
  services:
    qwen-coder:
      base_url: http://localhost:12434
      health_path: /health
      models_path: /v1/models
      required: true
    nomic-embed:
      base_url: http://localhost:12435
      health_path: /health
      models_path: /v1/models
      required: true

embedding:
  base_url: http://localhost:12435
  model: nomic-embed-code
  batch_size: 32
  timeout_seconds: 30
  query_prefix: "Represent this query for searching relevant code: "
```

- [ ] **Step 1.2: Verify the template parses as valid YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('templates/init/yard.yaml.example').read()); print('OK')"`

Expected: `OK`. If you see a YAML parse error, fix it before continuing — substitution doesn't run yet so the `{{PLACEHOLDERS}}` are part of the YAML at parse time and need to be in valid string positions.

- [ ] **Step 1.3: Commit**

```bash
git add templates/init/yard.yaml.example
git commit -m "feat(templates/init): seed all 13 agent_roles in yard.yaml template

Phase 5b task 1 — rewrite the templates/init/yard.yaml.example
file to contain all 13 agent_roles with {{PLACEHOLDER}} syntax
for the agent prompt paths. The previous template had only
coder and correctness-auditor seeded.

This template is the canonical source — Phase 5b task 2 will
embed it into the yard binary via go:embed.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add `internal/initializer/templates.go` with `go:embed` declaration

**Files:**
- Create: `internal/initializer/templates.go`
- Create: `internal/initializer/templates_test.go`

**Background:** the embed FS holds the entire `templates/init/` tree (the `yard.yaml.example` file plus the `brain/<section>/.gitkeep` files). The templates package exposes a typed walker that returns the file path inside the embed and its content. Substitution and file writing happen in later tasks.

- [ ] **Step 2.1: Write the failing test**

Create `internal/initializer/templates_test.go` with:

```go
package initializer

import (
	"strings"
	"testing"
)

func TestEmbeddedTemplatesContainsYardYaml(t *testing.T) {
	content, err := readEmbeddedFile("templates/init/yard.yaml.example")
	if err != nil {
		t.Fatalf("readEmbeddedFile: %v", err)
	}
	if !strings.Contains(string(content), "{{PROJECT_ROOT}}") {
		t.Fatalf("expected embedded yard.yaml.example to contain {{PROJECT_ROOT}} placeholder")
	}
	if !strings.Contains(string(content), "agent_roles:") {
		t.Fatalf("expected embedded yard.yaml.example to contain agent_roles section")
	}
	if !strings.Contains(string(content), "orchestrator:") {
		t.Fatalf("expected embedded yard.yaml.example to contain orchestrator role")
	}
}

func TestEmbeddedTemplatesContainsBrainGitkeeps(t *testing.T) {
	wantSections := []string{"architecture", "conventions", "epics", "logs", "plans", "receipts", "specs", "tasks"}
	for _, section := range wantSections {
		path := "templates/init/brain/" + section + "/.gitkeep"
		if _, err := readEmbeddedFile(path); err != nil {
			t.Errorf("expected embedded %s to exist: %v", path, err)
		}
	}
}

func TestListBrainSectionDirs(t *testing.T) {
	dirs, err := listBrainSectionDirs()
	if err != nil {
		t.Fatalf("listBrainSectionDirs: %v", err)
	}
	want := []string{"architecture", "conventions", "epics", "logs", "plans", "receipts", "specs", "tasks"}
	if len(dirs) != len(want) {
		t.Fatalf("listBrainSectionDirs returned %d dirs, want %d: %v", len(dirs), len(want), dirs)
	}
	for i, w := range want {
		if dirs[i] != w {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i], w)
		}
	}
}
```

- [ ] **Step 2.2: Run test to verify it fails**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: FAIL with `package github.com/ponchione/sodoryard/internal/initializer is not in std` (or similar — the package doesn't exist yet).

- [ ] **Step 2.3: Implement `internal/initializer/templates.go`**

Create `internal/initializer/templates.go` with:

```go
// Package initializer creates the on-disk artifacts a project needs to be
// usable by the railway: yard.yaml, .yard/, .brain/, .gitignore. The
// templates/init/ tree is embedded into the binary at build time so the
// initializer has no runtime filesystem dependency.
package initializer

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// all: prefix is required so go:embed includes .gitkeep files (without it,
// files starting with `.` are skipped). One directive covers the whole tree
// including any future template files added under templates/init/.
//go:embed all:templates/init
var templateFS embed.FS

// readEmbeddedFile returns the bytes of a file inside the embedded templates
// tree. The path is the same path you would use in `go:embed`.
func readEmbeddedFile(path string) ([]byte, error) {
	data, err := templateFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", path, err)
	}
	return data, nil
}

// listBrainSectionDirs returns the names of the railway brain section
// directories that templates/init/brain/ declares, sorted alphabetically.
// These are the directories `yard init` creates under `<project>/.brain/`.
func listBrainSectionDirs() ([]string, error) {
	entries, err := fs.ReadDir(templateFS, "templates/init/brain")
	if err != nil {
		return nil, fmt.Errorf("read embedded brain dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// yardYamlTemplatePath is the path to the yard.yaml template inside the
// embedded filesystem. Centralised so callers don't string-literal it.
const yardYamlTemplatePath = "templates/init/yard.yaml.example"

// templatesPathTrimPrefix is the prefix the embed FS adds to every entry.
// Stripped when reporting paths to the operator.
const templatesPathTrimPrefix = "templates/init/"

// stripTemplatePrefix returns the path with the templates/init/ prefix
// removed, suitable for joining onto a destination project root.
func stripTemplatePrefix(p string) string {
	return strings.TrimPrefix(p, templatesPathTrimPrefix)
}
```

- [ ] **Step 2.4: Run test to verify it passes**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: `ok  github.com/ponchione/sodoryard/internal/initializer ...` and no FAIL lines.

- [ ] **Step 2.5: Commit**

```bash
git add internal/initializer/templates.go internal/initializer/templates_test.go
git commit -m "feat(initializer): embed templates/init/ via go:embed

Phase 5b task 2 — new internal/initializer package houses the
//go:embed declaration for the templates/init/ tree. Two readers:
readEmbeddedFile returns the bytes for any embedded path,
listBrainSectionDirs enumerates the railway brain section dirs
the template declares (architecture, conventions, epics, logs,
plans, receipts, specs, tasks).

Tests cover the yard.yaml.example placeholder presence, the
8 .gitkeep files, and the section listing.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Add `internal/initializer/substitute.go` for `{{PLACEHOLDER}}` substitution

**Files:**
- Create: `internal/initializer/substitute.go`
- Create: `internal/initializer/substitute_test.go`

**Background:** at copy time, two tokens get substituted in the yard.yaml template — `{{PROJECT_ROOT}}` and `{{PROJECT_NAME}}`. Every other `{{...}}` token (notably `{{SODORYARD_AGENTS_DIR}}`) is left as-is. Substitution is exact-string replacement, no regex, no escaping rules.

- [ ] **Step 3.1: Write the failing tests**

Create `internal/initializer/substitute_test.go` with:

```go
package initializer

import (
	"strings"
	"testing"
)

func TestSubstituteReplacesProjectRootAndName(t *testing.T) {
	in := "project_root: {{PROJECT_ROOT}}\nname: {{PROJECT_NAME}}\n"
	out := substituteTemplate(in, SubstitutionValues{
		ProjectRoot: "/home/user/myapp",
		ProjectName: "myapp",
	})
	if !strings.Contains(out, "project_root: /home/user/myapp\n") {
		t.Errorf("PROJECT_ROOT not substituted: %s", out)
	}
	if !strings.Contains(out, "name: myapp\n") {
		t.Errorf("PROJECT_NAME not substituted: %s", out)
	}
}

func TestSubstituteLeavesOtherPlaceholdersAlone(t *testing.T) {
	in := "system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md\n"
	out := substituteTemplate(in, SubstitutionValues{
		ProjectRoot: "/home/user/myapp",
		ProjectName: "myapp",
	})
	if !strings.Contains(out, "{{SODORYARD_AGENTS_DIR}}/coder.md") {
		t.Errorf("expected {{SODORYARD_AGENTS_DIR}} placeholder to be preserved, got: %s", out)
	}
}

func TestSubstituteIsIdempotent(t *testing.T) {
	in := "project_root: {{PROJECT_ROOT}}\n"
	values := SubstitutionValues{ProjectRoot: "/x/y", ProjectName: "y"}
	once := substituteTemplate(in, values)
	twice := substituteTemplate(once, values)
	if once != twice {
		t.Errorf("substitution not idempotent: once=%q twice=%q", once, twice)
	}
}

func TestSubstituteReplacesMultipleOccurrences(t *testing.T) {
	in := "{{PROJECT_ROOT}}/foo {{PROJECT_ROOT}}/bar"
	out := substituteTemplate(in, SubstitutionValues{ProjectRoot: "/p", ProjectName: "p"})
	if out != "/p/foo /p/bar" {
		t.Errorf("expected both occurrences substituted, got: %q", out)
	}
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: FAIL with `undefined: substituteTemplate` and `undefined: SubstitutionValues`.

- [ ] **Step 3.3: Implement `internal/initializer/substitute.go`**

Create `internal/initializer/substitute.go` with:

```go
package initializer

import "strings"

// SubstitutionValues holds the values that get substituted into a template
// at copy time. Two fields, two corresponding {{PLACEHOLDERS}}.
type SubstitutionValues struct {
	// ProjectRoot is the absolute path to the project being initialized.
	// Substituted for {{PROJECT_ROOT}}.
	ProjectRoot string

	// ProjectName is the basename of ProjectRoot. Used in the yaml header
	// comment. Substituted for {{PROJECT_NAME}}.
	ProjectName string
}

// substituteTemplate performs exact-string replacement of the two known
// placeholders in s. Other {{...}} tokens are left untouched.
func substituteTemplate(s string, v SubstitutionValues) string {
	s = strings.ReplaceAll(s, "{{PROJECT_ROOT}}", v.ProjectRoot)
	s = strings.ReplaceAll(s, "{{PROJECT_NAME}}", v.ProjectName)
	return s
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: `ok  github.com/ponchione/sodoryard/internal/initializer` with no FAIL lines.

- [ ] **Step 3.5: Commit**

```bash
git add internal/initializer/substitute.go internal/initializer/substitute_test.go
git commit -m "feat(initializer): add placeholder substitution at copy time

Phase 5b task 3 — substituteTemplate replaces exactly two
tokens in template content: {{PROJECT_ROOT}} and {{PROJECT_NAME}}.
Every other {{...}} placeholder (notably {{SODORYARD_AGENTS_DIR}})
is left as-is for the operator to substitute by hand.

Pure string replacement, no regex, no escaping. Idempotent.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Move Obsidian config writer to `internal/initializer/obsidian.go`

**Files:**
- Create: `internal/initializer/obsidian.go`
- Create: `internal/initializer/obsidian_test.go`
- Modify: `cmd/tidmouth/init.go:255-295` (the `initBrainVault` function — partially)

**Background:** `cmd/tidmouth/init.go:255` defines `initBrainVault` which writes the `.brain/.obsidian/{app,appearance,community-plugins,core-plugins}.json` files. That JSON content lives in two package-level vars (`obsidianAppJSON`, `obsidianCorePluginsJSON`). All of this moves to `internal/initializer/obsidian.go`. The Tidmouth file keeps its own copy temporarily (Task 11 deletes it), so this task is purely additive — no deletions yet.

- [ ] **Step 4.1: Write the failing test**

Create `internal/initializer/obsidian_test.go` with:

```go
package initializer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureObsidianConfigCreatesAllFiles(t *testing.T) {
	brainDir := filepath.Join(t.TempDir(), ".brain")
	if err := os.MkdirAll(brainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := EnsureObsidianConfig(brainDir); err != nil {
		t.Fatalf("EnsureObsidianConfig: %v", err)
	}

	wantFiles := []string{"app.json", "appearance.json", "community-plugins.json", "core-plugins.json"}
	for _, name := range wantFiles {
		path := filepath.Join(brainDir, ".obsidian", name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected %s to be non-empty", path)
		}
	}

	// app.json should contain vimMode: false
	appData, err := os.ReadFile(filepath.Join(brainDir, ".obsidian", "app.json"))
	if err != nil {
		t.Fatalf("ReadFile app.json: %v", err)
	}
	var app map[string]any
	if err := json.Unmarshal(appData, &app); err != nil {
		t.Fatalf("Unmarshal app.json: %v", err)
	}
	if app["vimMode"] != false {
		t.Errorf("expected app.json vimMode=false, got %v", app["vimMode"])
	}
}

func TestEnsureObsidianConfigSkipsExistingFiles(t *testing.T) {
	brainDir := filepath.Join(t.TempDir(), ".brain")
	obsDir := filepath.Join(brainDir, ".obsidian")
	if err := os.MkdirAll(obsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	customApp := []byte(`{"vimMode":true,"customField":"keep me"}`)
	if err := os.WriteFile(filepath.Join(obsDir, "app.json"), customApp, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := EnsureObsidianConfig(brainDir); err != nil {
		t.Fatalf("EnsureObsidianConfig: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(obsDir, "app.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(customApp) {
		t.Errorf("expected existing app.json to be preserved, got: %s", got)
	}
}
```

- [ ] **Step 4.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: FAIL with `undefined: EnsureObsidianConfig`.

- [ ] **Step 4.3: Implement `internal/initializer/obsidian.go`**

Create `internal/initializer/obsidian.go` with:

```go
package initializer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// obsidianAppJSON is the minimal Obsidian app.json config the railway seeds.
var obsidianAppJSON = map[string]any{
	"vimMode": false,
}

// obsidianCorePluginsJSON lists the core Obsidian plugins to enable so the
// vault is usable for browsing receipts/specs/plans/etc out of the box.
var obsidianCorePluginsJSON = []string{
	"file-explorer",
	"global-search",
	"graph",
	"outline",
	"page-preview",
}

// obsidianFiles maps each .obsidian/<name>.json filename to the value that
// gets JSON-marshaled and written when the file does not already exist.
var obsidianFiles = map[string]any{
	"app.json":               obsidianAppJSON,
	"appearance.json":        map[string]any{},
	"community-plugins.json": []string{},
	"core-plugins.json":      obsidianCorePluginsJSON,
}

// EnsureObsidianConfig writes the .obsidian/ config files into the given
// brain directory. Files that already exist are left untouched. The brain
// directory itself must exist before calling this — initializer.Run() makes
// it as part of the brain mkdir step.
func EnsureObsidianConfig(brainDir string) error {
	obsidianDir := filepath.Join(brainDir, ".obsidian")
	if err := os.MkdirAll(obsidianDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", obsidianDir, err)
	}

	for name, content := range obsidianFiles {
		fp := filepath.Join(obsidianDir, name)
		if _, err := os.Stat(fp); err == nil {
			continue // already exists
		}
		data, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", name, err)
		}
		if err := os.WriteFile(fp, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 4.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: passing tests, no FAIL lines.

- [ ] **Step 4.5: Commit**

```bash
git add internal/initializer/obsidian.go internal/initializer/obsidian_test.go
git commit -m "feat(initializer): add EnsureObsidianConfig for .brain/.obsidian/

Phase 5b task 4 — move the Obsidian config maps and writer
logic out of cmd/tidmouth/init.go into internal/initializer.
Same behavior: writes app.json, appearance.json,
community-plugins.json, core-plugins.json into .brain/.obsidian/
when they don't already exist; leaves existing files untouched.

cmd/tidmouth/init.go retains a copy until Phase 5b task 11
(the deletion task).

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Move `.gitignore` patcher to `internal/initializer/gitignore.go`

**Files:**
- Create: `internal/initializer/gitignore.go`
- Create: `internal/initializer/gitignore_test.go`

**Background:** `cmd/tidmouth/init.go:298-354` defines `patchGitignore` and `gitignoreContains`. They append `.yard/` and `.brain/` to `.gitignore` if not present. Move both to `internal/initializer/gitignore.go`. Keep the original copy in tidmouth for now.

- [ ] **Step 5.1: Write the failing tests**

Create `internal/initializer/gitignore_test.go` with:

```go
package initializer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreEntriesCreatesFileWhenMissing(t *testing.T) {
	projectRoot := t.TempDir()
	added, err := EnsureGitignoreEntries(projectRoot)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}
	if len(added) != 2 {
		t.Errorf("expected 2 entries added, got %d: %v", len(added), added)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{".yard/", ".brain/"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected .gitignore to contain %q, got:\n%s", want, data)
		}
	}
}

func TestEnsureGitignoreEntriesAppendsToExistingFile(t *testing.T) {
	projectRoot := t.TempDir()
	existing := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	added, err := EnsureGitignoreEntries(projectRoot)
	if err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}
	if len(added) != 2 {
		t.Errorf("expected 2 entries added, got %d", len(added))
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "node_modules/") || !strings.Contains(got, "dist/") {
		t.Errorf("expected existing entries preserved, got:\n%s", got)
	}
	if !strings.Contains(got, ".yard/") || !strings.Contains(got, ".brain/") {
		t.Errorf("expected new entries appended, got:\n%s", got)
	}
}

func TestEnsureGitignoreEntriesIsIdempotent(t *testing.T) {
	projectRoot := t.TempDir()
	if _, err := EnsureGitignoreEntries(projectRoot); err != nil {
		t.Fatalf("first call: %v", err)
	}
	added, err := EnsureGitignoreEntries(projectRoot)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("second call should add nothing, got %d: %v", len(added), added)
	}
}
```

- [ ] **Step 5.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: FAIL with `undefined: EnsureGitignoreEntries`.

- [ ] **Step 5.3: Implement `internal/initializer/gitignore.go`**

Create `internal/initializer/gitignore.go` with:

```go
package initializer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

// gitignoreEntries is the list of paths the railway needs excluded from
// git. Order matters: it's the order they get appended to a fresh
// .gitignore.
var gitignoreEntries = []string{
	appconfig.StateDirName + "/", // ".yard/"
	".brain/",
}

// EnsureGitignoreEntries appends the railway entries (.yard/, .brain/) to
// the project's .gitignore file if they're not already present. Creates
// the file if it doesn't exist. Returns the list of entries that were
// actually added (empty if all were already present).
func EnsureGitignoreEntries(projectRoot string) ([]string, error) {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, entry := range gitignoreEntries {
		if !gitignoreContains(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil, nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()

	// Make sure we start on a new line.
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return nil, fmt.Errorf("write newline: %w", err)
		}
	}

	if _, err := f.WriteString("\n# yard (auto-generated)\n"); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}
	for _, entry := range toAdd {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return nil, fmt.Errorf("write entry %s: %w", entry, err)
		}
	}

	return toAdd, nil
}

// gitignoreContains reports whether the .gitignore file content already
// contains the given entry on its own line. Tolerates trailing slash drift.
func gitignoreContains(content, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == strings.TrimSuffix(entry, "/") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: passing tests, no FAIL lines.

- [ ] **Step 5.5: Commit**

```bash
git add internal/initializer/gitignore.go internal/initializer/gitignore_test.go
git commit -m "feat(initializer): add EnsureGitignoreEntries for .yard/ and .brain/

Phase 5b task 5 — move the .gitignore patcher logic out of
cmd/tidmouth/init.go into internal/initializer. Same behavior:
appends .yard/ and .brain/ to .gitignore if not already present,
creates the file if missing, returns the list of entries actually
added so the caller can print operator-friendly status.

cmd/tidmouth/init.go retains a copy until Phase 5b task 11.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Move database init logic to `internal/initializer/database.go`

**Files:**
- Create: `internal/initializer/database.go`
- Create: `internal/initializer/database_test.go`

**Background:** `cmd/tidmouth/init.go:210-252` defines `initDatabase` which opens the SQLite db, runs `appdb.InitIfNeeded`, runs the two upgrade helpers, and inserts/upserts the project record. Move all of that to `internal/initializer/database.go`. The function signature changes slightly so the caller can pass the same `(projectRoot, projectName, stateDir)` triple that initializer.Run will compute once.

- [ ] **Step 6.1: Write the failing test**

Create `internal/initializer/database_test.go` with:

```go
//go:build sqlite_fts5
// +build sqlite_fts5

package initializer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	appdb "github.com/ponchione/sodoryard/internal/db"
)

func TestEnsureDatabaseCreatesSchema(t *testing.T) {
	projectRoot := t.TempDir()
	stateDir := filepath.Join(projectRoot, appconfig.StateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	created, err := EnsureDatabase(context.Background(), projectRoot, "myproject", stateDir)
	if err != nil {
		t.Fatalf("EnsureDatabase: %v", err)
	}
	if !created {
		t.Errorf("expected created=true on first run")
	}

	dbPath := filepath.Join(stateDir, appconfig.StateDBName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected %s to exist: %v", dbPath, err)
	}

	// Verify the schema is queryable.
	db, err := appdb.OpenDB(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	var name string
	row := db.QueryRowContext(context.Background(), "SELECT name FROM projects WHERE id = ?", projectRoot)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("project record query: %v", err)
	}
	if name != "myproject" {
		t.Errorf("project name = %q, want %q", name, "myproject")
	}
}

func TestEnsureDatabaseIsIdempotent(t *testing.T) {
	projectRoot := t.TempDir()
	stateDir := filepath.Join(projectRoot, appconfig.StateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if _, err := EnsureDatabase(context.Background(), projectRoot, "myproject", stateDir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	created, err := EnsureDatabase(context.Background(), projectRoot, "myproject", stateDir)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if created {
		t.Errorf("expected created=false on second run")
	}
}
```

- [ ] **Step 6.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: FAIL with `undefined: EnsureDatabase`.

- [ ] **Step 6.3: Implement `internal/initializer/database.go`**

Create `internal/initializer/database.go` with:

```go
package initializer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	appdb "github.com/ponchione/sodoryard/internal/db"
)

// EnsureDatabase opens the project's .yard/yard.db file, initialises the
// schema if needed, runs the schema upgrade helpers, and ensures a project
// record exists. Returns true if the schema was created from scratch on
// this call, false if the database was already initialised.
//
// stateDir must already exist on disk — initializer.Run() creates it before
// calling EnsureDatabase.
func EnsureDatabase(ctx context.Context, projectRoot, projectName, stateDir string) (bool, error) {
	dbPath := filepath.Join(stateDir, appconfig.StateDBName)

	database, err := appdb.OpenDB(ctx, dbPath)
	if err != nil {
		return false, fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	created, err := appdb.InitIfNeeded(ctx, database)
	if err != nil {
		return false, fmt.Errorf("init database schema: %w", err)
	}
	if err := appdb.EnsureMessageSearchIndexesIncludeTools(ctx, database); err != nil {
		return false, fmt.Errorf("upgrade message search indexes: %w", err)
	}
	if err := appdb.EnsureContextReportsIncludeTokenBudget(ctx, database); err != nil {
		return false, fmt.Errorf("upgrade context report token budget storage: %w", err)
	}
	if err := appdb.EnsureChainSchema(ctx, database); err != nil {
		return false, fmt.Errorf("ensure chain schema: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = database.ExecContext(ctx, `
INSERT INTO projects(id, name, root_path, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	root_path = excluded.root_path,
	updated_at = excluded.updated_at
`, projectRoot, projectName, projectRoot, now, now)
	if err != nil {
		return false, fmt.Errorf("ensure project record: %w", err)
	}

	return created, nil
}
```

Note: `EnsureChainSchema` is included here (it wasn't in the old `cmd/tidmouth/init.go`) because Phase 3 added it, and freshly initialised projects need the chain tables for sirtopham to work.

- [ ] **Step 6.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: passing tests, no FAIL lines.

- [ ] **Step 6.5: Commit**

```bash
git add internal/initializer/database.go internal/initializer/database_test.go
git commit -m "feat(initializer): add EnsureDatabase for .yard/yard.db bootstrap

Phase 5b task 6 — move the database initialisation logic out
of cmd/tidmouth/init.go into internal/initializer. Same behavior
plus one fix: EnsureChainSchema is now called as part of init,
so freshly initialised projects work with sirtopham out of the
box (Phase 3 added the chain tables but the old tidmouth init
predated that and never called the migration).

cmd/tidmouth/init.go retains a copy until Phase 5b task 11.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Add `internal/initializer/initializer.go` with the `Run()` entrypoint

**Files:**
- Create: `internal/initializer/initializer.go`
- Create: `internal/initializer/initializer_test.go`

**Background:** this is the orchestration layer — the function `cmd/yard/init.go` will call. It pulls together the helpers from Tasks 2–6 and walks them in the right order: render config, mkdir state dir, init database, mkdir lancedb dirs, mkdir brain dirs (from embed), write Obsidian config, patch gitignore. It returns a structured `Report` so the CLI layer can print operator-friendly status without re-walking.

- [ ] **Step 7.1: Write the failing integration test**

Create `internal/initializer/initializer_test.go` with:

```go
//go:build sqlite_fts5
// +build sqlite_fts5

package initializer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitializesEmptyDirectory(t *testing.T) {
	projectRoot := t.TempDir()

	report, err := Run(context.Background(), Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report == nil {
		t.Fatalf("Run returned nil report")
	}

	// Files that must exist after init.
	wantPaths := []string{
		"yard.yaml",
		".yard",
		".yard/yard.db",
		".yard/lancedb/code",
		".yard/lancedb/brain",
		".brain",
		".brain/.obsidian/app.json",
		".brain/notes",
		".brain/specs/.gitkeep",
		".brain/architecture/.gitkeep",
		".brain/epics/.gitkeep",
		".brain/tasks/.gitkeep",
		".brain/plans/.gitkeep",
		".brain/receipts/.gitkeep",
		".brain/logs/.gitkeep",
		".brain/conventions/.gitkeep",
		".gitignore",
	}
	for _, p := range wantPaths {
		full := filepath.Join(projectRoot, p)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}

	// yard.yaml content checks.
	configData, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile yard.yaml: %v", err)
	}
	got := string(configData)

	// PROJECT_ROOT was substituted.
	if !strings.Contains(got, "project_root: "+projectRoot+"\n") {
		t.Errorf("expected project_root substituted to %s, got:\n%s", projectRoot, got)
	}
	// PROJECT_NAME (basename) was substituted.
	wantName := filepath.Base(projectRoot)
	if !strings.Contains(got, "Project: "+wantName) {
		t.Errorf("expected PROJECT_NAME substituted to %s, got:\n%s", wantName, got)
	}
	// SODORYARD_AGENTS_DIR placeholder is preserved.
	if !strings.Contains(got, "{{SODORYARD_AGENTS_DIR}}/coder.md") {
		t.Errorf("expected {{SODORYARD_AGENTS_DIR}} placeholder to be preserved")
	}
	// All 13 roles are present in agent_roles.
	wantRoles := []string{
		"orchestrator:", "coder:", "planner:", "test-writer:", "resolver:",
		"correctness-auditor:", "integration-auditor:", "performance-auditor:",
		"security-auditor:", "quality-auditor:", "docs-arbiter:",
		"epic-decomposer:", "task-decomposer:",
	}
	for _, role := range wantRoles {
		if !strings.Contains(got, "  "+role) {
			t.Errorf("expected agent_roles to contain %q", role)
		}
	}

	// .gitignore has the railway entries.
	gitignoreData, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore: %v", err)
	}
	for _, want := range []string{".yard/", ".brain/"} {
		if !strings.Contains(string(gitignoreData), want) {
			t.Errorf("expected .gitignore to contain %q", want)
		}
	}
}

func TestRunIsIdempotent(t *testing.T) {
	projectRoot := t.TempDir()

	if _, err := Run(context.Background(), Options{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Capture file contents after first run.
	firstYaml, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile after first run: %v", err)
	}
	firstGitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore after first run: %v", err)
	}

	// Re-run.
	report, err := Run(context.Background(), Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if report == nil {
		t.Fatal("nil report on re-run")
	}

	// Content of files must not change.
	secondYaml, err := os.ReadFile(filepath.Join(projectRoot, "yard.yaml"))
	if err != nil {
		t.Fatalf("ReadFile after second run: %v", err)
	}
	if string(firstYaml) != string(secondYaml) {
		t.Errorf("yard.yaml content changed across runs")
	}
	secondGitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile .gitignore after second run: %v", err)
	}
	if string(firstGitignore) != string(secondGitignore) {
		t.Errorf(".gitignore content changed across runs")
	}
}

func TestRunRequiresProjectRoot(t *testing.T) {
	if _, err := Run(context.Background(), Options{}); err == nil {
		t.Errorf("expected error for empty ProjectRoot, got nil")
	}
}
```

- [ ] **Step 7.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -20`

Expected: FAIL with `undefined: Run` and `undefined: Options`.

- [ ] **Step 7.3: Implement `internal/initializer/initializer.go`**

Create `internal/initializer/initializer.go` with:

```go
package initializer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

// Options configure a single initializer.Run() call.
type Options struct {
	// ProjectRoot is the absolute path to the directory being initialized.
	// Required.
	ProjectRoot string

	// ConfigFilename overrides the generated config filename. If empty,
	// the canonical "yard.yaml" is used. Provided as an escape hatch for
	// tests and unusual operator setups.
	ConfigFilename string
}

// Report describes what Run() did. Each entry is one operator-visible
// action — created, skipped, or modified.
type Report struct {
	Entries []ReportEntry
}

// ReportEntry is one line of init output.
type ReportEntry struct {
	Kind    string // "config", "mkdir", "database", "vault", "gitignore"
	Path    string // operator-relative path (relative to ProjectRoot)
	Status  string // "created", "skipped", "added <details>"
	Details string // optional extra information for "added" entries
}

// Run bootstraps a project for railway use. It is safe to re-run against
// an already-initialized project — every step is idempotent and
// existing files are preserved.
//
// Run does not change the process working directory.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if strings.TrimSpace(opts.ProjectRoot) == "" {
		return nil, fmt.Errorf("initializer: ProjectRoot is required")
	}
	projectRoot := opts.ProjectRoot
	configFilename := opts.ConfigFilename
	if configFilename == "" {
		configFilename = appconfig.ConfigFilename
	}
	projectName := filepath.Base(projectRoot)
	stateDir := filepath.Join(projectRoot, appconfig.StateDirName)

	report := &Report{}

	// 1. Generate yard.yaml from the embedded template.
	configEntry, err := writeConfigFile(projectRoot, projectName, configFilename)
	if err != nil {
		return nil, err
	}
	report.Entries = append(report.Entries, configEntry)

	// 2. mkdir state dir.
	if entry, err := mkdirRelative(projectRoot, appconfig.StateDirName, "mkdir"); err != nil {
		return nil, err
	} else {
		report.Entries = append(report.Entries, entry)
	}

	// 3. Initialize database.
	created, err := EnsureDatabase(ctx, projectRoot, projectName, stateDir)
	if err != nil {
		return nil, err
	}
	dbStatus := "schema created"
	if !created {
		dbStatus = "already initialized, skipped"
	}
	report.Entries = append(report.Entries, ReportEntry{
		Kind:   "database",
		Path:   filepath.Join(appconfig.StateDirName, appconfig.StateDBName),
		Status: dbStatus,
	})

	// 4. mkdir lancedb directories under state dir.
	for _, sub := range []string{filepath.Join("lancedb", "code"), filepath.Join("lancedb", "brain")} {
		if entry, err := mkdirRelative(projectRoot, filepath.Join(appconfig.StateDirName, sub), "mkdir"); err != nil {
			return nil, err
		} else {
			report.Entries = append(report.Entries, entry)
		}
	}

	// 5. mkdir .brain/ root.
	if entry, err := mkdirRelative(projectRoot, ".brain", "mkdir"); err != nil {
		return nil, err
	} else {
		report.Entries = append(report.Entries, entry)
	}

	// 6. Write .obsidian config.
	if err := EnsureObsidianConfig(filepath.Join(projectRoot, ".brain")); err != nil {
		return nil, err
	}
	report.Entries = append(report.Entries, ReportEntry{
		Kind:   "vault",
		Path:   filepath.Join(".brain", ".obsidian") + "/",
		Status: "obsidian config ready",
	})

	// 7. mkdir .brain/notes (operator's free-form notes).
	if entry, err := mkdirRelative(projectRoot, filepath.Join(".brain", "notes"), "mkdir"); err != nil {
		return nil, err
	} else {
		report.Entries = append(report.Entries, entry)
	}

	// 8. mkdir .brain/<section>/ for each railway section.
	sections, err := listBrainSectionDirs()
	if err != nil {
		return nil, err
	}
	for _, section := range sections {
		// Create the directory.
		dir := filepath.Join(".brain", section)
		if entry, err := mkdirRelative(projectRoot, dir, "mkdir"); err != nil {
			return nil, err
		} else {
			report.Entries = append(report.Entries, entry)
		}
		// Place a .gitkeep so empty railway sections survive `git add`.
		gitkeepPath := filepath.Join(projectRoot, dir, ".gitkeep")
		if _, err := os.Stat(gitkeepPath); err != nil {
			if err := os.WriteFile(gitkeepPath, nil, 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", gitkeepPath, err)
			}
		}
	}

	// 9. Patch .gitignore.
	added, err := EnsureGitignoreEntries(projectRoot)
	if err != nil {
		return nil, err
	}
	gitignoreStatus := "already has entries, skipped"
	gitignoreDetails := ""
	if len(added) > 0 {
		gitignoreStatus = "added"
		gitignoreDetails = strings.Join(added, ", ")
	}
	report.Entries = append(report.Entries, ReportEntry{
		Kind:    "gitignore",
		Path:    ".gitignore",
		Status:  gitignoreStatus,
		Details: gitignoreDetails,
	})

	return report, nil
}

// writeConfigFile renders the embedded yard.yaml template into the project
// root, performing the two known substitutions. Returns a ReportEntry that
// describes what happened.
func writeConfigFile(projectRoot, projectName, configFilename string) (ReportEntry, error) {
	configPath := filepath.Join(projectRoot, configFilename)
	if _, err := os.Stat(configPath); err == nil {
		return ReportEntry{Kind: "config", Path: configFilename, Status: "already exists, skipped"}, nil
	}

	raw, err := readEmbeddedFile(yardYamlTemplatePath)
	if err != nil {
		return ReportEntry{}, err
	}
	rendered := substituteTemplate(string(raw), SubstitutionValues{
		ProjectRoot: projectRoot,
		ProjectName: projectName,
	})
	if err := os.WriteFile(configPath, []byte(rendered), 0o644); err != nil {
		return ReportEntry{}, fmt.Errorf("write %s: %w", configPath, err)
	}
	return ReportEntry{Kind: "config", Path: configFilename, Status: "created"}, nil
}

// mkdirRelative creates the given subpath under projectRoot, recording
// whether the directory was newly created or already existed. Used by
// Run() for every directory it makes.
func mkdirRelative(projectRoot, rel, kind string) (ReportEntry, error) {
	full := filepath.Join(projectRoot, rel)
	if info, err := os.Stat(full); err == nil && info.IsDir() {
		return ReportEntry{Kind: kind, Path: rel, Status: "already exists"}, nil
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return ReportEntry{}, fmt.Errorf("create %s: %w", rel, err)
	}
	return ReportEntry{Kind: kind, Path: rel, Status: "created"}, nil
}
```

- [ ] **Step 7.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "initializer|FAIL" | head -30`

Expected: all `internal/initializer` tests pass with no FAIL lines. The integration test exercises the full Run() against a tempdir.

- [ ] **Step 7.5: Commit**

```bash
git add internal/initializer/initializer.go internal/initializer/initializer_test.go
git commit -m "feat(initializer): add Run() entrypoint that orchestrates init

Phase 5b task 7 — internal/initializer.Run() ties together the
helpers from Tasks 2-6 in the right order: render yard.yaml from
the embedded template, mkdir .yard/, init the SQLite schema,
mkdir lancedb dirs, mkdir .brain/ + .obsidian + notes + 8 section
dirs, write .gitkeeps, patch .gitignore. Returns a structured
Report so the CLI layer can print operator status without
re-walking.

Integration tests cover: empty directory bootstrap, idempotent
re-run, and the empty-ProjectRoot rejection path.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Add `cmd/yard/main.go` with the cobra root command

**Files:**
- Create: `cmd/yard/main.go`

**Background:** new top-level binary. Mirrors the `cmd/sirtopham/main.go` pattern (root command with `SilenceUsage: true`, exit code propagation, version string). Only one subcommand registered: `init`.

- [ ] **Step 8.1: Create `cmd/yard/main.go`**

Create `cmd/yard/main.go` with:

```go
// Command yard is the operator-facing CLI for railway project bootstrap
// and (in future phases) other top-level operator workflows. Today its
// only subcommand is `init`.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "yard",
		Short:        "Yard — railway project operator CLI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "yard %s\n", version)
			return nil
		},
	}
	rootCmd.AddCommand(newInitCmd())
	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if coded, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(coded.ExitCode())
		}
		os.Exit(1)
	}
}
```

- [ ] **Step 8.2: Verify the file compiles standalone (will fail until Task 9)**

Run: `go build ./cmd/yard 2>&1 | head -10`

Expected: `undefined: newInitCmd` — this is expected; Task 9 adds the file. Do not commit yet.

---

## Task 9: Add `cmd/yard/init.go` with the init subcommand

**Files:**
- Create: `cmd/yard/init.go`

**Background:** thin wrapper that calls `internal/initializer.Run()` and prints the report. No business logic in this file.

- [ ] **Step 9.1: Create `cmd/yard/init.go`**

Create `cmd/yard/init.go` with:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/initializer"
)

func newInitCmd() *cobra.Command {
	var configFilename string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the current directory for railway use",
		Long: `Bootstrap the current directory for the railway:
  - Generate yard.yaml with all 13 agent roles seeded
  - Create .yard/ with initialized SQLite database and lancedb roots
  - Create .brain/ vault with Obsidian config and the 8 railway section dirs
  - Patch .gitignore with .yard/ and .brain/ entries

Safe to re-run — never overwrites existing files or data.

After init, edit yard.yaml to substitute {{SODORYARD_AGENTS_DIR}}
with the absolute path to your sodoryard install's agents/ dir.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), cmd, configFilename)
		},
	}
	cmd.Flags().StringVar(&configFilename, "config", "", "Override the generated config filename (default: yard.yaml)")
	return cmd
}

func runInit(ctx context.Context, cmd *cobra.Command, configFilename string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Initializing yard in %s\n\n", projectRoot)

	report, err := initializer.Run(ctx, initializer.Options{
		ProjectRoot:    projectRoot,
		ConfigFilename: configFilename,
	})
	if err != nil {
		return err
	}

	for _, e := range report.Entries {
		switch e.Status {
		case "added":
			_, _ = fmt.Fprintf(out, "  %-10s %s (added %s)\n", e.Kind, e.Path, e.Details)
		default:
			_, _ = fmt.Fprintf(out, "  %-10s %s (%s)\n", e.Kind, e.Path, e.Status)
		}
	}

	_, _ = fmt.Fprintln(out, "\nDone.")
	_, _ = fmt.Fprintln(out, "Next steps:")
	_, _ = fmt.Fprintln(out, "  1. Edit yard.yaml — replace {{SODORYARD_AGENTS_DIR}} with the absolute")
	_, _ = fmt.Fprintln(out, "     path to your sodoryard install's agents/ directory.")
	_, _ = fmt.Fprintln(out, "  2. Confirm the provider block matches your auth setup")
	_, _ = fmt.Fprintln(out, "     (default is codex via ~/.sirtopham/auth.json).")
	_, _ = fmt.Fprintln(out, "  3. Run `tidmouth index` to populate the code search index.")
	_, _ = fmt.Fprintln(out, "  4. Run `sirtopham chain --task \"...\"` to start your first chain.")
	return nil
}
```

- [ ] **Step 9.2: Verify the binary builds**

Run: `go build -tags sqlite_fts5 ./cmd/yard 2>&1 | head -10`

Expected: clean build, no output. (No `bin/yard` produced yet because we're not using `-o bin/yard`.)

- [ ] **Step 9.3: Commit Tasks 8 and 9 together**

```bash
git add cmd/yard/main.go cmd/yard/init.go
git commit -m "feat(yard): add cmd/yard binary with init subcommand

Phase 5b tasks 8-9 — new top-level cmd/yard binary, mirrors the
cmd/sirtopham/main.go pattern. Only one command registered:
yard init, which delegates to internal/initializer.Run() and
prints an operator-facing report.

The new binary is the canonical entry point for project
bootstrap. Phase 5b task 11 deletes cmd/tidmouth/init.go.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Add the `yard:` Makefile target

**Files:**
- Modify: `Makefile`

**Background:** mirror the `sirtopham:` target precisely so the new binary uses the same FTS5 + lancedb cgo wiring. Add `yard` to `all:` so `make all` builds it.

- [ ] **Step 10.1: Update the Makefile**

Find this block in `Makefile:11-15`:

```makefile
.PHONY: all build tidmouth sirtopham knapford test dev-backend dev-frontend dev frontend-deps frontend-build frontend-typecheck clean

# `make all` builds every monorepo binary. `make build` is an alias for
# `make tidmouth` to preserve the single-binary workflow during Phase 1/2.
all: tidmouth sirtopham knapford
```

Replace with:

```makefile
.PHONY: all build tidmouth sirtopham knapford yard test dev-backend dev-frontend dev frontend-deps frontend-build frontend-typecheck clean

# `make all` builds every monorepo binary. `make build` is an alias for
# `make tidmouth` to preserve the single-binary workflow during Phase 1/2.
all: tidmouth sirtopham knapford yard
```

Then find this block in `Makefile:32-35`:

```makefile
# knapford: web dashboard (Phase 6 placeholder for now).
knapford:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/knapford ./cmd/knapford
```

Append immediately after it:

```makefile

# yard: operator-facing CLI for project bootstrap. Same SQLite (FTS5) and
# lancedb cgo wiring as tidmouth/sirtopham because internal/initializer
# opens the same .yard/yard.db database.
yard:
	mkdir -p $(BIN_DIR)
	$(CGO_BUILD_ENV) go build $(GOFLAGS_DB) -o $(BIN_DIR)/yard ./cmd/yard
```

- [ ] **Step 10.2: Build the new binary**

Run: `make yard`

Expected: builds without errors, produces `bin/yard`.

Verify:

```bash
ls -la bin/yard && ./bin/yard --help
```

Expected: binary exists, `yard --help` prints the cobra usage with `init` listed as a subcommand.

- [ ] **Step 10.3: Run `make all` to confirm all four binaries build**

Run: `make all 2>&1 | tail -15 && ls bin/`

Expected: all four binaries (tidmouth, sirtopham, knapford, yard) present in `bin/`.

- [ ] **Step 10.4: Commit**

```bash
git add Makefile
git commit -m "build: add yard binary target with FTS5 and lancedb cgo flags

Phase 5b task 10 — Makefile yard: target mirrors the sirtopham:
target's CGO_BUILD_ENV and GOFLAGS_DB usage so the new binary
links against sqlite_fts5 and lancedb the same way the other
two database-touching binaries do. Added yard to .PHONY and to
the 'all' target.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: Delete `cmd/tidmouth/init.go` and `cmd/tidmouth/init_test.go`

**Files:**
- Delete: `cmd/tidmouth/init.go`
- Delete: `cmd/tidmouth/init_test.go`
- Modify: `cmd/tidmouth/main.go`

**Background:** all the salvageable logic from `cmd/tidmouth/init.go` is now in `internal/initializer/`. The old file is dead. Delete it, delete its test, remove the cobra registration. After this lands, `tidmouth init` returns `unknown command "init"` from cobra.

- [ ] **Step 11.1: Find the registration in `cmd/tidmouth/main.go`**

Run: `grep -n "newInitCmd" cmd/tidmouth/main.go`

Expected output: one or more lines showing `newInitCmd` referenced. Note the line numbers.

- [ ] **Step 11.2: Remove the registration**

Open `cmd/tidmouth/main.go`. Find the line that calls `newInitCmd(...)` inside the `AddCommand` block (or wherever the init command is registered) and delete it. The exact text depends on the file's current shape — look for something like:

```go
rootCmd.AddCommand(
    ...
    newInitCmd(&configPath),
    ...
)
```

Remove the `newInitCmd(&configPath),` line. If `newInitCmd` is registered standalone (`rootCmd.AddCommand(newInitCmd(&configPath))`), delete that statement entirely.

- [ ] **Step 11.3: Delete the init source files**

Run:

```bash
git rm cmd/tidmouth/init.go cmd/tidmouth/init_test.go
```

- [ ] **Step 11.4: Build and test**

Run: `make tidmouth 2>&1 | tail -5`

Expected: clean build of tidmouth. If there are unused-import errors in `cmd/tidmouth/main.go` after removing the registration, delete the orphaned imports too.

Run: `make test 2>&1 | grep -E "cmd/tidmouth|FAIL" | head -20`

Expected: cmd/tidmouth tests pass (now without the init test).

- [ ] **Step 11.5: Verify `tidmouth init` no longer exists**

Run: `./bin/tidmouth init 2>&1 | head -5`

Expected: `Error: unknown command "init" for "tidmouth"` (or similar cobra unknown-command output). Exit code non-zero.

- [ ] **Step 11.6: Commit**

```bash
git add cmd/tidmouth/main.go
git commit -m "refactor(tidmouth): remove init subcommand, replaced by yard init

Phase 5b task 11 — cmd/tidmouth/init.go and cmd/tidmouth/init_test.go
are deleted. The salvageable logic (Obsidian config writer,
gitignore patcher, database bootstrap) now lives in
internal/initializer/, and the operator entry point is
'yard init' as of Phase 5b task 9.

No deprecation alias. Per the repo's house rule, removed code
gets deleted outright rather than left as a backwards-compat
shim. Anyone who types 'tidmouth init' gets cobra's
'unknown command' error and re-learns 'yard init'.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Live smoke test against a fresh `/tmp/` directory

**Files:** none (verification only)

**Background:** acceptance criteria 6, 7, and 8 from spec 16 require a live end-to-end smoke test, not just unit tests. The Phase 3 verification this session proved that "tests pass" and "the binary actually works" are different gates.

- [ ] **Step 12.1: Create a fresh smoke directory**

Run:

```bash
SMOKE_DIR="/tmp/yard-init-smoke-$(date +%s)"
mkdir -p "$SMOKE_DIR"
cd "$SMOKE_DIR"
echo "Smoke dir: $SMOKE_DIR"
```

Expected: directory created. Note the path for cleanup.

- [ ] **Step 12.2: Run `yard init` in the smoke directory**

Run:

```bash
/home/gernsback/source/sodoryard/bin/yard init
```

Expected stdout: starts with `Initializing yard in /tmp/yard-init-smoke-*`, prints a list of `kind  path  (status)` lines for the config, mkdir entries, database, vault, gitignore, and ends with `Done.` plus the four "Next steps" lines.

- [ ] **Step 12.3: Verify the produced file tree**

Run:

```bash
ls -la
ls -la .yard/ .brain/ .brain/.obsidian/ .brain/specs/
cat .gitignore
```

Expected:
- `yard.yaml` exists
- `.yard/yard.db` exists
- `.yard/lancedb/{code,brain}/` exist
- `.brain/.obsidian/{app,appearance,community-plugins,core-plugins}.json` exist
- `.brain/notes/` exists
- `.brain/{architecture,conventions,epics,logs,plans,receipts,specs,tasks}/.gitkeep` all exist
- `.gitignore` contains `.yard/` and `.brain/`

- [ ] **Step 12.4: Verify yard.yaml content has substitutions and placeholders**

Run:

```bash
grep "project_root:" yard.yaml
grep "Project: " yard.yaml
grep -c "{{SODORYARD_AGENTS_DIR}}" yard.yaml
grep -c "system_prompt:" yard.yaml
```

Expected:
- `project_root: /tmp/yard-init-smoke-*` (substituted)
- `# Project: yard-init-smoke-*` (substituted)
- 13 occurrences of `{{SODORYARD_AGENTS_DIR}}` (one per role, preserved as placeholder)
- 13 occurrences of `system_prompt:` (one per role)

- [ ] **Step 12.5: Verify idempotency on re-run**

Run:

```bash
/home/gernsback/source/sodoryard/bin/yard init
```

Expected: every `kind  path  (status)` line shows `(already exists, skipped)`, `(already initialized, skipped)`, `(already has entries, skipped)` etc. No error. Exit code 0.

Verify the file content is unchanged:

```bash
md5sum yard.yaml .gitignore .brain/.obsidian/app.json
```

Expected: same checksums as before the second `yard init`.

- [ ] **Step 12.6: Verify tidmouth init no longer works**

Run:

```bash
/home/gernsback/source/sodoryard/bin/tidmouth init 2>&1 | head -5
```

Expected: `Error: unknown command "init" for "tidmouth"` from cobra. Exit code non-zero.

- [ ] **Step 12.7: Substitute `{{SODORYARD_AGENTS_DIR}}` and run a real chain against the smoke dir**

Run:

```bash
sed -i "s|{{SODORYARD_AGENTS_DIR}}|/home/gernsback/source/sodoryard/agents|g" yard.yaml
grep -c "system_prompt: /home/gernsback/source/sodoryard/agents/" yard.yaml
```

Expected: `13` (every role's system_prompt now points at a real file).

Run:

```bash
PATH="/home/gernsback/source/sodoryard/bin:$PATH" \
  /home/gernsback/source/sodoryard/bin/sirtopham chain \
    --config "$(pwd)/yard.yaml" \
    --task "Spawn correctness-auditor once with task 'list the brain receipts directory and write a brief receipt at receipts/correctness-auditor/yard-init-smoke-step-001.md', then call chain_complete with status=success and a one-sentence summary." \
    --chain-id "yard-init-smoke" \
    --max-steps 3 \
    --max-duration 5m
```

Expected: chain completes, `yard-init-smoke` printed as the success line, exit 0.

- [ ] **Step 12.8: Verify the chain produced both receipts**

Run:

```bash
ls .brain/receipts/orchestrator/
ls .brain/receipts/correctness-auditor/
```

Expected: each directory contains the corresponding receipt file. This is the proof that a freshly `yard init`'d project is immediately usable end-to-end with no operator intervention beyond the one find-and-replace in step 12.7.

- [ ] **Step 12.9: Clean up the smoke directory (optional)**

Run:

```bash
cd /home/gernsback/source/sodoryard
rm -rf "$SMOKE_DIR"
```

- [ ] **Step 12.10: No commit for this task**

This is verification only. If everything passed, proceed to Task 13. If anything failed, stop and diagnose before tagging.

---

## Task 13: Tag `v0.5-yard-init`

**Files:** none (tag only)

- [ ] **Step 13.1: Confirm the working tree is clean and tests are green**

Run:

```bash
git status --short
make test 2>&1 | grep -E "FAIL|ok\s+github" | head -5
```

Expected: empty git status, no FAIL lines.

- [ ] **Step 13.2: Confirm `make all` produces all four binaries**

Run: `make all 2>&1 | tail -5 && ls bin/`

Expected: all four binaries present.

- [ ] **Step 13.3: Tag the release**

Run:

```bash
git tag -a v0.5-yard-init -m "Phase 5b — yard init

Ship 'yard init' as the canonical operator command for railway
project bootstrap.

- New cmd/yard top-level binary (init subcommand only)
- New internal/initializer package — embedded templates,
  substitution, Obsidian config, gitignore patcher, database
  bootstrap, Run() orchestrator
- templates/init/yard.yaml.example rewritten with all 13
  agent_roles seeded with {{SODORYARD_AGENTS_DIR}} placeholders
- cmd/tidmouth/init.go and init_test.go deleted outright
- Makefile yard: target mirrors the sirtopham: cgo wiring

Verified live by yard-init-smoke against a fresh /tmp dir:
empty bootstrap, idempotent re-run, end-to-end sirtopham chain
against the freshly initialized project."

git tag -l 'v*'
```

Expected: new tag listed alongside `v0.1-pre-sodor`, `v0.2-monorepo-structure`, `v0.2.1-yard-paths`, `v0.4-orchestrator`.

- [ ] **Step 13.4: Update `NEXT_SESSION_HANDOFF.md`**

This is a separate concern but should land before any other phase starts. Edit `NEXT_SESSION_HANDOFF.md` to:
1. Move "Phase 3" out of the "Next task" section
2. Add a "Phase 3 complete" section noting the tag and commit range
3. Add a "Phase 5b complete" section noting the tag and what shipped
4. Update the "Next task" section to point at Phase 6 (Knapford) or Phase 7 (containerization), whichever the user picks next

Commit the handoff update separately:

```bash
git add NEXT_SESSION_HANDOFF.md
git commit -m "docs: update handoff for Phase 3 + Phase 5b completion

Phase 3 (sirtopham orchestrator) shipped in commits ac5e9ad
through ee685a1, tagged v0.4-orchestrator.

Phase 5b (yard init) shipped in this stack, tagged v0.5-yard-init.

The next migration target is either Phase 6 (Knapford dashboard)
or Phase 7 (containerization stubs)."
```

---

## Final acceptance summary

Phase 5b is done when **all** of the following are true (mirrors spec 16 §9):

- [x] `make all` builds `bin/yard` alongside `bin/tidmouth`, `bin/sirtopham`, `bin/knapford`
- [x] `make test` is green, including `internal/initializer/` tests
- [x] `cmd/tidmouth/init.go` and `cmd/tidmouth/init_test.go` no longer exist
- [x] `tidmouth init` returns `unknown command` from cobra
- [x] `internal/initializer/` exists and houses all init logic
- [x] `templates/init/yard.yaml.example` contains all 13 `agent_roles` with `{{SODORYARD_AGENTS_DIR}}` placeholders, embedded via `go:embed`
- [x] `yard init` in an empty `/tmp/yard-init-smoke-*` produces the full file tree, exits 0, prints the operator report
- [x] Re-running `yard init` is a no-op (every entry shows `skipped`)
- [x] After hand-substitution of `{{SODORYARD_AGENTS_DIR}}`, `sirtopham chain` succeeds against the freshly initialized project end-to-end
- [x] `docs/specs/16-yard-init.md` is unchanged or has been updated to match anything that drifted during implementation
- [x] Phase 5b commit stack is tagged `v0.5-yard-init`

---

## If you get stuck

Stop only if one of these is true:
- A referenced file/package no longer exists and there is no obvious replacement
- The implementation would require changing one of the locked decisions in this plan or in spec 16
- Live runtime behavior contradicts the plan in a way that narrow edits cannot fix
- The smoke chain in Task 12.7 fails (this means Phase 5b broke something Phase 3 left working — diagnose before continuing)

Otherwise, adapt locally and continue.

When handing off mid-stream, update `NEXT_SESSION_HANDOFF.md` with:
- current checkpoint (CP1–CP5)
- exact files touched
- exact failing command/test
- whether the failure is code, schema, environment, or template syntax
- the next smallest unresolved step
