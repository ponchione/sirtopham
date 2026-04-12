# Phase 7 Yard Containerization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a multi-stage Dockerfile, a root-level `docker-compose.yaml`, and a new `yard install` command so the railway runs in a container against any mounted project. Phase 7 is headless-only — Knapford has a placeholder slot in compose, real Knapford lands in Phase 6.

**Architecture:** Three-stage Dockerfile (node frontend builder → go builder with corrected lancedb rpath → debian-slim runtime with `liblancedb_go.so` at `/usr/local/lib/` + `ldconfig`). New `cmd/yard/install.go` subcommand resolves the `{{SODORYARD_AGENTS_DIR}}` placeholder Phase 5b deliberately leaves manual; Dockerfile sets `SODORYARD_AGENTS_DIR=/opt/yard/agents` so `yard install` inside the container is a single zero-flag invocation. Two compose files (existing `ops/llm/docker-compose.yml` for inference services, new root `docker-compose.yaml` for the railway) coexist on the external `llm-net` network with independent lifecycles.

**Tech Stack:** `debian:bookworm-slim` runtime, `golang:1.22-bookworm` builder, `node:20-bookworm-slim` frontend builder, Docker Compose v2, lancedb cgo (glibc, amd64-only), `embed.FS` (already in use from Phase 5b).

**Spec:** [`docs/specs/17-yard-containerization.md`](../specs/17-yard-containerization.md)

---

## Required reading before starting

Read these in order:

1. `AGENTS.md` — repo conventions and hard rules
2. `docs/specs/17-yard-containerization.md` — the design spec this plan implements
3. `docs/specs/16-yard-init.md` — Phase 5b spec (this plan is the sibling of Phase 5b's plan; the `yard install` command introduced here resolves the placeholder Phase 5b leaves manual)
4. `Makefile` — current build environment and the host-absolute rpath that Phase 7 fixes inside the Dockerfile
5. `ops/llm/docker-compose.yml` — the existing LLM compose this plan coexists with (NOT modified)
6. `internal/initializer/substitute.go` (after Phase 5b lands) — the substitution helper that `install.go` mirrors
7. `cmd/yard/init.go` (after Phase 5b lands) — the cobra subcommand pattern that `install.go` follows
8. `lib/linux_amd64/liblancedb_go.so` (just `ls -la`, don't try to read the binary) — the shared library Phase 7 stages into `/usr/local/lib/` inside the runtime image

After reading, run `make build && make test` to confirm the baseline is green before touching anything. **Phase 5b must be fully landed and tagged `v0.5-yard-init` before Phase 7 starts** — this plan assumes `cmd/yard/main.go`, `cmd/yard/init.go`, and `internal/initializer/` already exist.

---

## Locked decisions (do not re-litigate)

These are fixed for Phase 7. If implementation reveals one is wrong, stop and ask before changing.

1. New `cmd/yard/install.go` subcommand. Reads `--sodoryard-agents-dir` flag OR `SODORYARD_AGENTS_DIR` env var, in that priority order.
2. `yard install` substitutes `{{SODORYARD_AGENTS_DIR}}` only — does not touch any other placeholder, does not validate the path exists, does not write a backup.
3. `yard install` is destructive (overwrites yard.yaml in place) and idempotent (re-running on an already-substituted file is a no-op).
4. Multi-stage Dockerfile: `node:20-bookworm-slim` → `golang:1.22-bookworm` → `debian:bookworm-slim`.
5. Runtime image stages `liblancedb_go.so` at `/usr/local/lib/`, runs `ldconfig`, AND the Go builder rebuilds with `-Wl,-rpath,/usr/local/lib`. Both mechanisms are belt-and-suspenders.
6. Image filesystem: binaries at `/usr/local/bin/`, agent prompts at `/opt/yard/agents/`, project bind-mounted at `/project` (also `WORKDIR`).
7. Root `docker-compose.yaml` declares `yard` and `knapford` services. Both share the existing external `llm-net` network. `ops/llm/docker-compose.yml` is **not** modified.
8. `knapford` service uses Compose **profiles** (`profiles: [knapford]`) so it does NOT start with a default `docker compose up`. Operators who want to run the placeholder explicitly use `docker compose --profile knapford up knapford`. Once Phase 6 lands, the profile gate is removed.
9. `WORKDIR /project`, no `ENTRYPOINT`, default `CMD ["yard", "--help"]`. Operators run `docker compose run --rm yard <command>` to invoke specific subcommands.
10. amd64 only. `debian:bookworm-slim` only (no alpine, no distroless, no scratch). No registry push, no `:latest` tag — local `ponchione/yard:dev` only.
11. Phase 5b is **not** retroactively updated. The Phase 5b plan that already committed uses `sed` for its smoke story; that stays. Phase 7's `yard install` is a parallel option, not a Phase 5b modification.
12. Tag is `v0.7-containerization`, NOT `v1.0-sodor` (the `v1.0-sodor` tag is reserved for Phase 6 + Phase 7 shipped together).

---

## File structure

**New files:**

```
Dockerfile                                  # repo root, 3-stage build
docker-compose.yaml                         # repo root, yard + knapford-placeholder
.dockerignore                               # repo root, keep host artifacts out

cmd/yard/
└── install.go                              # cobra wrapper for `yard install`

internal/initializer/
├── install.go                              # Install() substitution function
└── install_test.go                         # unit tests (substitution + env var + idempotency)
```

**Modified files:**

```
docs/specs/17-yard-containerization.md      # only if anything drifts during impl (Task 12)
```

**Unchanged files** (explicitly listed so the implementer doesn't touch them):

```
ops/llm/docker-compose.yml                  # the existing LLM compose, untouched
Makefile                                    # no new make targets in Phase 7
cmd/yard/main.go                            # adds install registration only via cobra (Task 2 step 2.3 edit)
cmd/yard/init.go                            # untouched
internal/initializer/initializer.go         # untouched
templates/init/yard.yaml.example            # untouched (Phase 5b owns it)
```

Each new file has one responsibility:
- `Dockerfile` — multi-stage build
- `docker-compose.yaml` — service declarations
- `.dockerignore` — build context filter
- `cmd/yard/install.go` — CLI surface for install (cobra wiring only)
- `internal/initializer/install.go` — the substitution function (logic, no CLI)
- `internal/initializer/install_test.go` — unit tests for the function

---

## Checkpoints

| Checkpoint | Tasks | Proof |
|---|---|---|
| CP1: yard install command (host-only) | 1, 2, 3 | `bin/yard install --sodoryard-agents-dir <path>` works against a tempdir; tests pass |
| CP2: Dockerfile foundation | 4, 5 | `docker compose build yard` succeeds, image is created locally |
| CP3: compose + verification | 6, 7, 8 | All four binaries load inside the container; ldconfig finds lancedb |
| CP4: end-to-end smoke | 9, 10, 11 | `yard init && yard install` inside container produces a working tree; sirtopham chain runs end-to-end |
| CP5: cleanup + tag | 12, 13 | Spec aligned with shipped impl; `v0.7-containerization` tagged |

If you finish a session mid-checkpoint, update `NEXT_SESSION_HANDOFF.md` with the current checkpoint, the failing command/test, and the next unresolved sub-step.

---

## Task 1: Add `internal/initializer/install.go` with `Install()` function

**Files:**
- Create: `internal/initializer/install.go`
- Create: `internal/initializer/install_test.go`

**Background:** the substitution function reads a yard.yaml file from disk, replaces `{{SODORYARD_AGENTS_DIR}}` with a provided value, and writes the file back. Idempotent — running against an already-substituted file is a no-op. Returns a structured result so the CLI layer can print operator-friendly status.

- [ ] **Step 1.1: Write the failing tests**

Create `internal/initializer/install_test.go` with:

```go
package initializer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSubstitutesAgentsDir(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	original := `agent_roles:
  coder:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
  planner:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/planner.md
`
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Install(InstallOptions{
		ConfigPath:        yamlPath,
		SodoryardAgentsDir: "/opt/yard/agents",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Substitutions != 2 {
		t.Errorf("expected 2 substitutions, got %d", result.Substitutions)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := `agent_roles:
  coder:
    system_prompt: /opt/yard/agents/coder.md
  planner:
    system_prompt: /opt/yard/agents/planner.md
`
	if string(got) != want {
		t.Errorf("substitution mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	if err := os.WriteFile(yamlPath, []byte("system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if result.Substitutions != 0 {
		t.Errorf("expected 0 substitutions on re-run, got %d", result.Substitutions)
	}
	second, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("file content changed across runs:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestInstallErrorsWhenConfigMissing(t *testing.T) {
	_, err := Install(InstallOptions{
		ConfigPath:         filepath.Join(t.TempDir(), "nonexistent.yaml"),
		SodoryardAgentsDir: "/opt/yard/agents",
	})
	if err == nil {
		t.Errorf("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "yard.yaml") && !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention the missing file, got: %v", err)
	}
}

func TestInstallErrorsWhenAgentsDirEmpty(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	if err := os.WriteFile(yamlPath, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: ""})
	if err == nil {
		t.Errorf("expected error for empty SodoryardAgentsDir, got nil")
	}
}

func TestInstallLeavesOtherPlaceholdersAlone(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "yard.yaml")
	original := `project_root: /home/user/myapp
foo: {{SOME_OTHER_PLACEHOLDER}}
system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
`
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Install(InstallOptions{ConfigPath: yamlPath, SodoryardAgentsDir: "/opt/yard/agents"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "{{SOME_OTHER_PLACEHOLDER}}") {
		t.Errorf("expected unrelated placeholder to be preserved, got:\n%s", got)
	}
	if !strings.Contains(string(got), "/opt/yard/agents/coder.md") {
		t.Errorf("expected agents-dir substitution, got:\n%s", got)
	}
}
```

- [ ] **Step 1.2: Run tests to verify they fail**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: FAIL with `undefined: Install` and `undefined: InstallOptions`.

- [ ] **Step 1.3: Implement `internal/initializer/install.go`**

Create `internal/initializer/install.go` with:

```go
package initializer

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// InstallOptions configure a single Install() call.
type InstallOptions struct {
	// ConfigPath is the path to the yard.yaml file to substitute. Required.
	ConfigPath string

	// SodoryardAgentsDir is the absolute path to the sodoryard install's
	// agents/ directory. This value replaces every occurrence of
	// {{SODORYARD_AGENTS_DIR}} in the config file. Required.
	SodoryardAgentsDir string
}

// InstallResult describes what Install() did.
type InstallResult struct {
	// Substitutions is the number of {{SODORYARD_AGENTS_DIR}} occurrences
	// that were replaced. Zero means the file was already fully substituted
	// and the call was a no-op.
	Substitutions int

	// ConfigPath is the absolute path to the file that was modified
	// (or would have been modified, if Substitutions == 0).
	ConfigPath string
}

// installPlaceholder is the literal token that gets substituted.
const installPlaceholder = "{{SODORYARD_AGENTS_DIR}}"

// Install reads opts.ConfigPath, replaces every occurrence of
// {{SODORYARD_AGENTS_DIR}} with opts.SodoryardAgentsDir, and writes the
// result back. Idempotent: running on an already-substituted file is a
// no-op (no occurrences left to replace).
//
// Errors:
//   - opts.SodoryardAgentsDir is empty
//   - opts.ConfigPath does not exist
//   - reading or writing the file fails
//
// Install does NOT validate that opts.SodoryardAgentsDir exists on disk.
// Install does NOT touch any placeholder other than {{SODORYARD_AGENTS_DIR}}.
// Install does NOT write a backup of the original file.
func Install(opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.SodoryardAgentsDir) == "" {
		return nil, errors.New("install: SodoryardAgentsDir is required")
	}
	if strings.TrimSpace(opts.ConfigPath) == "" {
		return nil, errors.New("install: ConfigPath is required")
	}

	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("install: read %s: %w", opts.ConfigPath, err)
	}

	original := string(data)
	count := strings.Count(original, installPlaceholder)
	if count == 0 {
		return &InstallResult{Substitutions: 0, ConfigPath: opts.ConfigPath}, nil
	}

	updated := strings.ReplaceAll(original, installPlaceholder, opts.SodoryardAgentsDir)
	if err := os.WriteFile(opts.ConfigPath, []byte(updated), 0o644); err != nil {
		return nil, fmt.Errorf("install: write %s: %w", opts.ConfigPath, err)
	}

	return &InstallResult{Substitutions: count, ConfigPath: opts.ConfigPath}, nil
}
```

- [ ] **Step 1.4: Run tests to verify they pass**

Run: `make test 2>&1 | grep -E "internal/initializer|FAIL" | head -20`

Expected: passing tests, no FAIL lines.

- [ ] **Step 1.5: Commit**

```bash
git add internal/initializer/install.go internal/initializer/install_test.go
git commit -m "feat(initializer): add Install() for {{SODORYARD_AGENTS_DIR}} substitution

Phase 7 task 1 — Install() reads a yard.yaml, replaces every
occurrence of {{SODORYARD_AGENTS_DIR}} with a caller-provided
value, and writes the file back. Idempotent (re-running on an
already-substituted file is a no-op, returns Substitutions=0).
Errors on empty agents dir or missing config file. Does not
touch any other placeholder, does not validate the agents-dir
path on disk, does not write a backup.

This is the substitution Phase 5b's yard init deliberately
left manual. Phase 7 task 2 wires it into a cobra subcommand
exposed as 'yard install'.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add `cmd/yard/install.go` with the cobra subcommand

**Files:**
- Create: `cmd/yard/install.go`
- Modify: `cmd/yard/main.go` (register the new subcommand)

**Background:** thin cobra wrapper that resolves the agents-dir from flag or env var, calls `internal/initializer.Install()`, and prints the result. No business logic.

- [ ] **Step 2.1: Create `cmd/yard/install.go`**

Create `cmd/yard/install.go` with:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/initializer"
)

func newInstallCmd() *cobra.Command {
	var sodoryardAgentsDir string
	var configFilename string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Substitute {{SODORYARD_AGENTS_DIR}} in yard.yaml",
		Long: `Resolve the {{SODORYARD_AGENTS_DIR}} placeholder that 'yard init'
leaves in the generated yard.yaml.

The agents directory is resolved in this order:
  1. The --sodoryard-agents-dir flag value (if set)
  2. The SODORYARD_AGENTS_DIR environment variable (if set)
  3. Error: no agents directory provided

The substitution is destructive (overwrites yard.yaml in place)
and idempotent (re-running on an already-substituted file is a
no-op).

Inside the official yard Docker image, SODORYARD_AGENTS_DIR is
preset to /opt/yard/agents so 'yard install' works with no flags.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, sodoryardAgentsDir, configFilename)
		},
	}
	cmd.Flags().StringVar(&sodoryardAgentsDir, "sodoryard-agents-dir", "", "Absolute path to sodoryard's agents/ directory (overrides SODORYARD_AGENTS_DIR env var)")
	cmd.Flags().StringVar(&configFilename, "config", "yard.yaml", "Path to the yard.yaml file to substitute")
	return cmd
}

func runInstall(cmd *cobra.Command, sodoryardAgentsDir, configFilename string) error {
	if sodoryardAgentsDir == "" {
		sodoryardAgentsDir = os.Getenv("SODORYARD_AGENTS_DIR")
	}
	if sodoryardAgentsDir == "" {
		return fmt.Errorf("no agents directory provided: pass --sodoryard-agents-dir or set SODORYARD_AGENTS_DIR")
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Installing yard config in %s\n", configFilename)
	_, _ = fmt.Fprintf(out, "  agents dir: %s\n", sodoryardAgentsDir)

	result, err := initializer.Install(initializer.InstallOptions{
		ConfigPath:         configFilename,
		SodoryardAgentsDir: sodoryardAgentsDir,
	})
	if err != nil {
		return err
	}

	if result.Substitutions == 0 {
		_, _ = fmt.Fprintln(out, "  no substitutions made (already installed)")
	} else {
		_, _ = fmt.Fprintf(out, "  substituted %d {{SODORYARD_AGENTS_DIR}} occurrences\n", result.Substitutions)
	}
	_, _ = fmt.Fprintln(out, "Done.")
	return nil
}
```

- [ ] **Step 2.2: Register the new command in `cmd/yard/main.go`**

Open `cmd/yard/main.go`. Find the `AddCommand` call inside `newRootCmd()`:

```go
rootCmd.AddCommand(newInitCmd())
```

Replace with:

```go
rootCmd.AddCommand(
	newInitCmd(),
	newInstallCmd(),
)
```

- [ ] **Step 2.3: Build and verify the new command exists**

Run: `make yard && ./bin/yard install --help`

Expected: clean build, `yard install --help` prints the long description, the `--sodoryard-agents-dir` and `--config` flags are listed.

- [ ] **Step 2.4: Commit**

```bash
git add cmd/yard/install.go cmd/yard/main.go
git commit -m "feat(yard): add yard install subcommand

Phase 7 task 2 — new cobra subcommand cmd/yard/install.go
that wraps internal/initializer.Install(). Resolves the agents
directory from --sodoryard-agents-dir flag or
SODORYARD_AGENTS_DIR env var, calls Install(), prints the
substitution count.

Inside the Phase 7 Docker image, SODORYARD_AGENTS_DIR is
preset to /opt/yard/agents so 'yard install' works with no
flags. On the host the operator passes the flag once per
project bootstrap.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Verify `yard install` works on host (CP1 gate)

**Files:** none (verification only)

**Background:** before touching Docker at all, prove the new command works on the host using a tempdir and a fake yard.yaml.

- [ ] **Step 3.1: Create a tempdir and fake yard.yaml**

Run:

```bash
SMOKE_DIR="/tmp/yard-install-smoke-$(date +%s)"
mkdir -p "$SMOKE_DIR"
cat > "$SMOKE_DIR/yard.yaml" << 'EOF'
project_root: /tmp/example
agent_roles:
  coder:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
  planner:
    system_prompt: {{SODORYARD_AGENTS_DIR}}/planner.md
EOF
echo "Smoke dir: $SMOKE_DIR"
cat "$SMOKE_DIR/yard.yaml"
```

Expected: the file contains two `{{SODORYARD_AGENTS_DIR}}` placeholders.

- [ ] **Step 3.2: Run `yard install` with the flag**

Run:

```bash
cd "$SMOKE_DIR"
/home/gernsback/source/sodoryard/bin/yard install --sodoryard-agents-dir /home/gernsback/source/sodoryard/agents
cat yard.yaml
```

Expected: command prints `substituted 2 {{SODORYARD_AGENTS_DIR}} occurrences`, the yaml now contains `/home/gernsback/source/sodoryard/agents/coder.md` and `/home/gernsback/source/sodoryard/agents/planner.md`.

- [ ] **Step 3.3: Run `yard install` again (idempotency check)**

Run:

```bash
/home/gernsback/source/sodoryard/bin/yard install --sodoryard-agents-dir /home/gernsback/source/sodoryard/agents
```

Expected: command prints `no substitutions made (already installed)`. File is unchanged (verify with `md5sum yard.yaml` before and after).

- [ ] **Step 3.4: Run `yard install` with the env var instead of the flag**

Reset the file and re-run:

```bash
cat > "$SMOKE_DIR/yard.yaml" << 'EOF'
system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
EOF
SODORYARD_AGENTS_DIR=/home/gernsback/source/sodoryard/agents /home/gernsback/source/sodoryard/bin/yard install
cat yard.yaml
```

Expected: substitution happened via the env var, output says `substituted 1 {{SODORYARD_AGENTS_DIR}} occurrences`.

- [ ] **Step 3.5: Run `yard install` with neither flag nor env var (error path)**

Reset the file and run:

```bash
cat > "$SMOKE_DIR/yard.yaml" << 'EOF'
system_prompt: {{SODORYARD_AGENTS_DIR}}/coder.md
EOF
unset SODORYARD_AGENTS_DIR
/home/gernsback/source/sodoryard/bin/yard install
echo "exit: $?"
```

Expected: error message `no agents directory provided: pass --sodoryard-agents-dir or set SODORYARD_AGENTS_DIR`, exit code non-zero.

- [ ] **Step 3.6: Clean up**

```bash
cd /home/gernsback/source/sodoryard
rm -rf "$SMOKE_DIR"
```

- [ ] **Step 3.7: No commit for this task**

This is verification only. Proceed to Task 4 if all sub-steps passed.

---

## Task 4: Add `.dockerignore` at the repo root

**Files:**
- Create: `.dockerignore`

**Background:** prevents host build artifacts and per-project state from entering the Docker build context. The build context is everything under the repo root by default; without `.dockerignore`, the host's `bin/`, `web/node_modules/`, `.brain/`, `.yard/`, and a few multi-GB GGUF model files would be sent to the Docker daemon every build.

- [ ] **Step 4.1: Create `.dockerignore`**

Create `.dockerignore` at the repo root with:

```
.git/
bin/
web/node_modules/
webfs/dist/
.brain/
.yard/
.sirtopham/
*.log
*.tmp
docs/
ops/llm/models/
.idea/
.vscode/
```

**Important:** do **not** add `templates/init/` or anything under it. The Go builder stage embeds it via `//go:embed all:templates/init` (from Phase 5b spec 16 §3.3) and excluding it would produce a yard binary with an empty embed FS that can't bootstrap projects.

- [ ] **Step 4.2: Verify the dockerignore is well-formed**

Run: `cat .dockerignore`

Expected: 13 lines, no extra whitespace, no comments.

- [ ] **Step 4.3: Commit**

```bash
git add .dockerignore
git commit -m "build: add .dockerignore for Phase 7 Docker build context

Phase 7 task 4 — keep host build artifacts (bin/, webfs/dist/,
node_modules/), per-project state (.brain/, .yard/, .sirtopham/),
multi-GB GGUF model files (ops/llm/models/), and docs out of
the Docker build context.

Intentionally NOT excluded: templates/init/ — the Go builder
stage embeds it via //go:embed all:templates/init from spec 16
and would produce a broken yard binary if it were excluded.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Add the multi-stage `Dockerfile`

**Files:**
- Create: `Dockerfile`

**Background:** three-stage build per spec 17 §3.7. Stage 1 builds the React frontend, stage 2 builds the four Go binaries with the corrected lancedb rpath, stage 3 is the slim runtime image with binaries + lib + agent prompts.

- [ ] **Step 5.1: Create `Dockerfile`**

Create `Dockerfile` at the repo root with:

```dockerfile
# syntax=docker/dockerfile:1.6

# ─── Stage 1: frontend builder ──────────────────────────────────────
# Builds the React frontend that tidmouth embeds via go:embed.
# Output is /web/dist which the Go stage copies to webfs/dist/ before
# compiling, so the embed picks it up.
FROM node:20-bookworm-slim AS frontend-builder

WORKDIR /web

# Copy package manifests first for layer cache friendliness.
COPY web/package.json web/package-lock.json* ./
RUN npm install

# Copy the rest of the frontend source.
COPY web/ ./

# Build. Output goes to /web/dist.
RUN npm run build


# ─── Stage 2: Go builder ────────────────────────────────────────────
# Compiles the four Go binaries (tidmouth, sirtopham, yard, knapford)
# with sqlite_fts5 + lancedb cgo wiring. Rebuilds rpath to point at
# the runtime image's library location (/usr/local/lib) so the
# binaries find liblancedb_go.so without env var gymnastics.
FROM golang:1.22-bookworm AS go-builder

WORKDIR /workspace

# Copy go.mod and go.sum first for layer cache friendliness.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source tree (everything not excluded by .dockerignore).
COPY . .

# Copy the frontend build output to the location tidmouth's
# webfs/embed.go expects.
COPY --from=frontend-builder /web/dist ./webfs/dist

# Build the four binaries with the corrected rpath. The CGO_LDFLAGS
# rpath points at /usr/local/lib because that's where the runtime
# stage stages liblancedb_go.so.
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/workspace/lib/linux_amd64 -llancedb_go -lm -ldl -lpthread -Wl,-rpath,/usr/local/lib"

RUN go build -tags sqlite_fts5 -o /out/tidmouth ./cmd/tidmouth
RUN go build -tags sqlite_fts5 -o /out/sirtopham ./cmd/sirtopham
RUN go build -tags sqlite_fts5 -o /out/yard ./cmd/yard
RUN go build -o /out/knapford ./cmd/knapford


# ─── Stage 3: runtime ───────────────────────────────────────────────
# Slim debian image with glibc + the four binaries + lancedb shared
# library + agent prompts. No Go toolchain, no Node, no source.
FROM debian:bookworm-slim AS runtime

# ca-certificates: needed for HTTPS calls to provider APIs (codex,
# anthropic). tini: PID 1 init for clean signal handling.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        tini \
    && rm -rf /var/lib/apt/lists/*

# Stage liblancedb_go.so at the standard site-installed library path.
# ldconfig updates the linker cache so binaries find it via the
# normal search path, in addition to the embedded rpath.
COPY --from=go-builder /workspace/lib/linux_amd64/liblancedb_go.so /usr/local/lib/
RUN ldconfig

# Install the four binaries.
COPY --from=go-builder /out/tidmouth /usr/local/bin/tidmouth
COPY --from=go-builder /out/sirtopham /usr/local/bin/sirtopham
COPY --from=go-builder /out/yard /usr/local/bin/yard
COPY --from=go-builder /out/knapford /usr/local/bin/knapford

# Install the 13 agent prompts at the canonical container location.
# yard install reads SODORYARD_AGENTS_DIR (set below) when invoked
# inside the container, so the substitution lands at this path.
COPY --from=go-builder /workspace/agents /opt/yard/agents

# Tell yard install where the agents directory lives. Operators
# inside the container do not need to pass --sodoryard-agents-dir.
ENV SODORYARD_AGENTS_DIR=/opt/yard/agents

# Bind-mounted project lives at /project; make it the working
# directory so a bare 'yard init' operates on the mounted project.
WORKDIR /project

# tini as PID 1 means signals propagate cleanly. Default command is
# yard --help so a bare 'docker run' shows the help text.
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["yard", "--help"]
```

- [ ] **Step 5.2: Build the image**

Run: `docker build -t ponchione/yard:dev . 2>&1 | tail -30`

Expected: clean build through all three stages. Final output ends with `naming to docker.io/ponchione/yard:dev`.

If the build fails:
- Check that `.dockerignore` does not exclude `templates/init/`
- Check that `lib/linux_amd64/liblancedb_go.so` exists in the build context
- Check that `web/package.json` and the frontend source are copied correctly
- Check that the Go builder finds `webfs/dist` after the frontend stage copies it

- [ ] **Step 5.3: Verify the image was created**

Run: `docker images ponchione/yard:dev`

Expected: one row showing the new image with size > 100MB (binaries + lib + agents).

- [ ] **Step 5.4: Commit**

```bash
git add Dockerfile
git commit -m "build: add Phase 7 multi-stage Dockerfile

Phase 7 task 5 — three-stage Dockerfile per spec 17:

  1. node:20-bookworm-slim — builds the React frontend that
     tidmouth embeds via go:embed
  2. golang:1.22-bookworm — compiles the four Go binaries with
     sqlite_fts5 + lancedb cgo, rebuilds rpath to point at
     /usr/local/lib so the runtime image finds liblancedb_go.so
     via the standard linker search path
  3. debian:bookworm-slim — runtime image with glibc, ca-certs,
     tini, the four binaries at /usr/local/bin/, the lancedb
     shared library at /usr/local/lib/ (ldconfig'd), the 13
     agent prompts at /opt/yard/agents/, SODORYARD_AGENTS_DIR
     env var preset, WORKDIR /project, default CMD yard --help

The runtime image has no Go toolchain, no Node, no source —
just what the operator actually executes.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Add `docker-compose.yaml` at the repo root

**Files:**
- Create: `docker-compose.yaml`

**Background:** declares the `yard` and `knapford` services. Both share the existing external `llm-net` network so they can reach the LLM inference services in `ops/llm/docker-compose.yml` when those are also running. `knapford` is profile-gated so a default `docker compose up` does not start it.

- [ ] **Step 6.1: Verify the `llm-net` network exists**

Run: `docker network ls | grep llm-net`

Expected: a row showing the `llm-net` network. If it does not exist, create it once:

```bash
docker network create llm-net
```

This is a one-time setup. The compose file declares the network as `external` so neither compose file is responsible for creating it.

- [ ] **Step 6.2: Create `docker-compose.yaml`**

Create `docker-compose.yaml` at the repo root with:

```yaml
services:
  yard:
    build:
      context: .
      dockerfile: Dockerfile
    image: ponchione/yard:dev
    networks:
      - llm-net
    volumes:
      - ${PROJECT_DIR:-./}:/project
    environment:
      - YARD_PROJECT=/project
      - SODORYARD_AGENTS_DIR=/opt/yard/agents

  knapford:
    image: ponchione/yard:dev
    command: ["knapford"]
    networks:
      - llm-net
    volumes:
      - ${PROJECT_DIR:-./}:/project
    ports:
      - "8080:8080"
    environment:
      - YARD_PROJECT=/project
    profiles:
      - knapford

networks:
  llm-net:
    external: true
```

- [ ] **Step 6.3: Validate the compose file**

Run: `docker compose config 2>&1 | head -40`

Expected: the resolved compose configuration is printed, no parse errors. Both services are present, `llm-net` is referenced as external.

- [ ] **Step 6.4: Commit**

```bash
git add docker-compose.yaml
git commit -m "build: add Phase 7 root docker-compose.yaml

Phase 7 task 6 — root-level docker-compose.yaml declaring the
yard service (build from local Dockerfile, mount PROJECT_DIR
at /project) and a profile-gated knapford service slot. Both
share the existing external llm-net network with
ops/llm/docker-compose.yml so the railway can reach local LLM
inference services when they're up.

knapford is gated behind 'profiles: [knapford]' so a default
'docker compose up' does not start it. Once Phase 6 ships a
real Knapford web service, the profile gate is removed.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Verify all four binaries load inside the container

**Files:** none (verification only)

**Background:** the lancedb cgo binary is the most likely failure point because the rpath rewrite is the most novel change. Verify all four binaries load and print help.

- [ ] **Step 7.1: Run `yard --help` inside the container**

Run: `docker compose run --rm yard yard --help`

Expected: the cobra help for `yard` (including the `init` and `install` subcommands) prints. No `liblancedb_go.so` errors.

- [ ] **Step 7.2: Run `tidmouth --help` inside the container**

Run: `docker compose run --rm yard tidmouth --help`

Expected: tidmouth's cobra help prints. No cgo errors.

- [ ] **Step 7.3: Run `sirtopham --help` inside the container**

Run: `docker compose run --rm yard sirtopham --help`

Expected: sirtopham's cobra help prints. No cgo errors.

- [ ] **Step 7.4: Run `knapford` inside the container (placeholder behavior)**

Run: `docker compose run --rm yard knapford`

Expected: prints the placeholder string (something like `knapford placeholder`) and exits 0. This is the placeholder slot behavior — Phase 6 will replace this binary with a real long-running web service.

- [ ] **Step 7.5: No commit for this task**

Verification only. Proceed to Task 8.

---

## Task 8: Verify lancedb library is reachable

**Files:** none (verification only)

**Background:** the runtime image stages `liblancedb_go.so` at `/usr/local/lib/` and runs `ldconfig`. Verify both the file is present and the linker cache knows about it.

- [ ] **Step 8.1: List the lib in the image**

Run: `docker compose run --rm yard ls -la /usr/local/lib/liblancedb_go.so`

Expected: file exists, non-zero size, executable.

- [ ] **Step 8.2: Verify ldconfig found it**

Run: `docker compose run --rm yard ldconfig -p | grep lancedb`

Expected: a line like `liblancedb_go.so (libc6,x86-64) => /usr/local/lib/liblancedb_go.so`. If this is empty, `ldconfig` did not pick up the library — re-check that the Dockerfile runs `ldconfig` AFTER the `COPY` of the .so.

- [ ] **Step 8.3: Verify the binaries link against it**

Run: `docker compose run --rm yard ldd /usr/local/bin/tidmouth | grep lancedb`

Expected: a line like `liblancedb_go.so => /usr/local/lib/liblancedb_go.so (0x...)`.

- [ ] **Step 8.4: No commit for this task**

Verification only. Proceed to Task 9.

---

## Task 9: Verify the Knapford profile starts and exits cleanly

**Files:** none (verification only)

**Background:** the `knapford` service is profile-gated. Confirm that `docker compose up` does NOT start it by default, and that `docker compose --profile knapford up knapford` runs the placeholder binary.

- [ ] **Step 9.1: Confirm default `up` skips knapford**

Run:

```bash
docker compose up -d
docker compose ps
```

Expected: the `yard` service is listed (or not — yard is `restart: no` and exits after CMD; the point is no error). The `knapford` service is NOT listed.

- [ ] **Step 9.2: Tear down**

Run: `docker compose down`

- [ ] **Step 9.3: Start with the knapford profile explicitly**

Run: `docker compose --profile knapford up knapford 2>&1 | tail -10`

Expected: the knapford container starts, prints the placeholder string, and exits. If the operator's host has port 8080 in use, this will fail with a port binding error — that's fine for Phase 7 (Phase 6 will need to handle the port collision).

- [ ] **Step 9.4: Tear down**

Run: `docker compose --profile knapford down`

- [ ] **Step 9.5: No commit for this task**

Verification only. Proceed to Task 10.

---

## Task 10: End-to-end smoke — `yard init && yard install` inside the container

**Files:** none (verification only)

**Background:** the operator's primary entrypoint to the railway via the container. Verify the two-command bootstrap produces a working project on a bind-mounted directory.

- [ ] **Step 10.1: Create a fresh smoke directory**

Run:

```bash
SMOKE_DIR="/tmp/yard-container-smoke-$(date +%s)"
mkdir -p "$SMOKE_DIR"
cd "$SMOKE_DIR"
echo "Smoke dir: $SMOKE_DIR"
```

- [ ] **Step 10.2: Run `yard init` inside the container**

Run:

```bash
PROJECT_DIR=$(pwd) docker compose -f /home/gernsback/source/sodoryard/docker-compose.yaml run --rm yard yard init
ls -la
```

Expected:
- `yard.yaml` exists
- `.yard/yard.db` exists
- `.brain/specs/.gitkeep`, `.brain/architecture/.gitkeep`, etc. exist
- `.gitignore` contains `.yard/` and `.brain/`
- The container exited 0

- [ ] **Step 10.3: Verify the yard.yaml has the placeholder unsubstituted (yard init should NOT have done it)**

Run: `grep -c "{{SODORYARD_AGENTS_DIR}}" yard.yaml`

Expected: 13 (one per agent role). Spec 16 §3.4 explicitly says yard init does not substitute this placeholder.

- [ ] **Step 10.4: Run `yard install` inside the container**

Run:

```bash
PROJECT_DIR=$(pwd) docker compose -f /home/gernsback/source/sodoryard/docker-compose.yaml run --rm yard yard install
grep -c "{{SODORYARD_AGENTS_DIR}}" yard.yaml
grep -c "/opt/yard/agents/" yard.yaml
```

Expected:
- Output says `substituted 13 {{SODORYARD_AGENTS_DIR}} occurrences`
- 0 remaining `{{SODORYARD_AGENTS_DIR}}` placeholders
- 13 occurrences of `/opt/yard/agents/`

This proves the container's `SODORYARD_AGENTS_DIR=/opt/yard/agents` env var is being read by `yard install` correctly.

- [ ] **Step 10.5: Run `yard install` again (idempotency)**

Run:

```bash
PROJECT_DIR=$(pwd) docker compose -f /home/gernsback/source/sodoryard/docker-compose.yaml run --rm yard yard install
```

Expected: output says `no substitutions made (already installed)`.

- [ ] **Step 10.6: No commit for this task**

Verification only. Proceed to Task 11.

---

## Task 11: End-to-end smoke — real sirtopham chain inside the container

**Files:** none (verification only)

**Background:** the ultimate proof: a freshly initialized + installed project runs a full sirtopham chain from inside the container. Same shape as the Phase 3 smoke chain that verified Phase 3 in this session, but running in the container against a host bind mount.

**Important:** this requires codex auth. The container does NOT have your ChatGPT credentials. You'll need to either:
- Mount `~/.sirtopham/auth.json` into the container (`-v $HOME/.sirtopham:/root/.sirtopham:ro`), OR
- Set `ANTHROPIC_API_KEY` in the env and switch the smoke yard.yaml to anthropic, OR
- Wait until a future phase fixes container auth, and accept that this smoke step is incomplete in Phase 7

For Phase 7 acceptance, the **mount approach** is the right move because it matches what the host smoke chain proved. Phase 7 done definition includes this mount.

- [ ] **Step 11.1: Re-run the smoke setup with the auth mount**

Continue in `$SMOKE_DIR` from Task 10.

Run:

```bash
PROJECT_DIR=$(pwd) docker compose -f /home/gernsback/source/sodoryard/docker-compose.yaml run --rm \
  -v $HOME/.sirtopham:/root/.sirtopham:ro \
  yard sirtopham chain \
    --config /project/yard.yaml \
    --task "Spawn correctness-auditor once with task 'list the brain receipts directory and write a brief receipt at receipts/correctness-auditor/yard-container-smoke-step-001.md', then call chain_complete with status=success and a one-sentence summary." \
    --chain-id "yard-container-smoke" \
    --max-steps 3 \
    --max-duration 5m
```

Expected:
- The chain completes
- The orchestrator emits `yard-container-smoke` as the success line
- Exit code 0
- No `liblancedb_go.so` errors
- No `database is closed` errors fatal to the run

- [ ] **Step 11.2: Verify both receipts exist on the host**

Run:

```bash
ls .brain/receipts/orchestrator/
ls .brain/receipts/correctness-auditor/
```

Expected: each directory contains the corresponding receipt file. **The receipts must be visible on the host filesystem** because the container wrote them to `/project/.brain/receipts/...` and `/project` is the bind mount of the host smoke directory.

This is the proof that:
- The container can talk to the codex API (via the mounted credential store)
- The bind mount works in both directions (yard.yaml read in, receipts written out)
- The lancedb cgo binary works in the runtime image
- The orchestrator and the spawned engine binary both work inside the container
- A freshly-`yard init`'d + `yard install`'d project is immediately usable end-to-end

- [ ] **Step 11.3: Clean up the smoke directory**

```bash
cd /home/gernsback/source/sodoryard
rm -rf "$SMOKE_DIR"
```

- [ ] **Step 11.4: No commit for this task**

Verification only. If everything passed, proceed to Task 12. If anything failed, stop and diagnose before tagging.

---

## Task 12: Update spec 17 if anything drifted during implementation

**Files:**
- Modify (only if needed): `docs/specs/17-yard-containerization.md`

**Background:** if implementation revealed a decision in spec 17 was wrong or incomplete, update the spec before tagging so the doc matches what shipped. Common cases: a flag name changed, a path moved, an unexpected dependency was needed (e.g., `tini` was added to the runtime image — that should be reflected in §3.7 if it isn't).

- [ ] **Step 12.1: Diff what shipped vs what spec 17 says**

Skim spec 17 §3 (decisions) and §3.7 (Dockerfile description) and §3.8 (compose shape). Compare against:
- The actual Dockerfile from Task 5
- The actual docker-compose.yaml from Task 6
- The actual install.go from Tasks 1-2

For each difference: either fix the spec to match what shipped, or fix the implementation to match the spec. **Prefer fixing the spec** unless the divergence is clearly a Phase 7 acceptance bug.

- [ ] **Step 12.2: Commit any spec updates**

If spec 17 was updated:

```bash
git add docs/specs/17-yard-containerization.md
git commit -m "docs(spec17): align with shipped Phase 7 implementation

Phase 7 task 12 — bring spec 17 in line with what actually
shipped during Phase 7 implementation. <list the specific
drifts being corrected>

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

If no updates were needed: no commit, proceed to Task 13.

---

## Task 13: Tag `v0.7-containerization`

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

Expected: all four binaries (tidmouth, sirtopham, yard, knapford) present.

- [ ] **Step 13.3: Confirm `docker compose build yard` is idempotent**

Run: `docker compose build yard 2>&1 | tail -5`

Expected: build completes (likely fast due to Docker layer cache).

- [ ] **Step 13.4: Tag the release**

Run:

```bash
git tag -a v0.7-containerization -m "Phase 7 — yard containerization

Ship a multi-stage Dockerfile, root docker-compose.yaml, and
the yard install command so the railway runs in a container
against any mounted project.

- New cmd/yard/install.go subcommand: substitutes the
  {{SODORYARD_AGENTS_DIR}} placeholder Phase 5b yard init
  leaves manual. Reads --sodoryard-agents-dir flag or
  SODORYARD_AGENTS_DIR env var. Idempotent.
- New internal/initializer/install.go: the substitution
  function (testable in isolation, no CLI dependency).
- New Dockerfile: 3-stage multi-stage build (node frontend,
  go-builder with corrected lancedb rpath, debian-slim runtime
  with /usr/local/lib + ldconfig + /opt/yard/agents).
- New docker-compose.yaml: yard service + profile-gated
  knapford placeholder slot. Both share existing external
  llm-net network with ops/llm/docker-compose.yml.
- New .dockerignore: keep host artifacts out of build context,
  intentionally NOT excluding templates/init/ (go:embed dep).

Verified live by yard-container-smoke against /tmp/...:
yard init + yard install + sirtopham chain end-to-end inside
the container, both receipts visible on the host bind mount,
no liblancedb_go.so errors, no cgo issues.

NOT v1.0-sodor — that tag is reserved for Phase 6 + Phase 7
shipped together."

git tag -l 'v*'
```

Expected: new tag listed alongside `v0.1-pre-sodor`, `v0.2-monorepo-structure`, `v0.2.1-yard-paths`, `v0.4-orchestrator`, `v0.5-yard-init`.

- [ ] **Step 13.5: Update `NEXT_SESSION_HANDOFF.md`**

Edit `NEXT_SESSION_HANDOFF.md` to:
1. Add a "Phase 7 complete" section noting the tag and what shipped
2. Update the "Next task" section to point at Phase 6 (Knapford), the only remaining migration phase
3. Note that Phase 4 prompts are still being handled out-of-band and may have landed since this session

Commit the handoff update:

```bash
git add NEXT_SESSION_HANDOFF.md
git commit -m "docs: update handoff for Phase 7 completion

Phase 7 (yard containerization) shipped in this stack, tagged
v0.7-containerization. Only remaining migration phase is
Phase 6 (Knapford dashboard), waiting on the Phase 4 prompt
work to mature."
```

---

## Final acceptance summary

Phase 7 is done when **all** of the following are true (mirrors spec 17 §6):

- [x] `Dockerfile` exists at the repo root
- [x] `docker compose build yard` produces an image without errors
- [x] `docker-compose.yaml` exists at the repo root with the yard + knapford service definitions
- [x] `.dockerignore` exists at the repo root
- [x] `cmd/yard/install.go` exists, `yard install --help` prints meaningful usage
- [x] `internal/initializer/install.go` and `install_test.go` exist with full unit test coverage
- [x] `make all` builds bin/yard with the new install subcommand
- [x] `docker compose run --rm yard ldconfig -p | grep lancedb` returns a hit
- [x] `docker compose run --rm yard tidmouth --help` works (proves the cgo binary loads)
- [x] `docker compose run --rm yard sirtopham --help` works
- [x] `docker compose run --rm yard yard --help` works
- [x] End-to-end smoke: `yard init && yard install` in a fresh `/tmp/yard-container-smoke-*` produces a substituted yard.yaml
- [x] End-to-end smoke continued: `sirtopham chain` inside the container produces both an orchestrator and an engine receipt visible on the host bind mount
- [x] `docker compose --profile knapford up knapford` starts the placeholder Knapford container
- [x] Spec 17 is unchanged or has been updated to match anything that drifted during implementation
- [x] Phase 7 commit stack is tagged `v0.7-containerization` (NOT `v1.0-sodor`)

---

## If you get stuck

Stop only if one of these is true:
- A referenced file/package no longer exists and there is no obvious replacement
- The implementation would require changing one of the locked decisions in this plan or in spec 17
- Live runtime behavior contradicts the plan in a way that narrow edits cannot fix
- The smoke chain in Task 11 fails (this means Phase 7 broke something Phase 3 or Phase 5b left working — diagnose before continuing)
- The Docker build in Task 5 fails on the lancedb cgo step (most likely failure point — check rpath, library staging order, glibc version compatibility)

Otherwise, adapt locally and continue.

When handing off mid-stream, update `NEXT_SESSION_HANDOFF.md` with:
- current checkpoint (CP1–CP5)
- exact files touched
- exact failing command/test or Docker build step
- whether the failure is code, schema, environment, Dockerfile, or compose
- the next smallest unresolved step
