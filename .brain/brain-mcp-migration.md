# Brain MCP Migration Spec

**Status:** Draft v2  
**Date:** 2026-04-02  
**Author:** Mitchell + Claude  
**Scope:** Replace Obsidian Local REST API with a custom Go MCP server for all brain operations  
**Supersedes:** v1 (mcpvault off-the-shelf approach)

---

## Motivation

The brain currently connects to Obsidian via the Local REST API plugin (`localhost:27124`). This works but carries three costs:

1. **Demonstrates nothing about MCP.** sirtopham's architecture claims MCP awareness as a future direction (doc 09, "v0.5+ MCP migration"). Pulling it forward to now means the working product demonstrates MCP capability — both client *and* server — which is a meaningfully stronger portfolio signal than "can consume a third-party MCP server."

2. **The REST API is a bespoke integration.** The `internal/brain` package hand-rolls HTTP requests, parses Obsidian-specific JSON response shapes, and manages auth headers. An MCP client is a general-purpose capability. An MCP server is an exposable product surface.

3. **External dependency on Obsidian running.** The REST API requires Obsidian to be open with the Local REST API plugin active. Direct filesystem access against the vault removes that requirement entirely — sirtopham can work with the brain headless, in CI, or while Obsidian is closed.

---

## Decision: Build a Custom Go MCP Server

### Options Evaluated

| Approach | Pros | Cons |
|----------|------|------|
| Off-the-shelf TS server (`mcpvault`, `cyanheads`) | Zero server code to write | Node.js subprocess, mixed runtime, only demonstrates MCP client side |
| Custom Go MCP server | Pure Go, both sides of MCP, exposable to external tools, no runtime deps | More code to write (but the vault ops are simple) |

### Decision: Custom Go server

The vault operations are not complex — they're file I/O, text search, YAML parsing, and directory listing. We already have most of this logic in `internal/brain/client.go`, just routed through HTTP instead of the filesystem. Rewriting as direct filesystem operations is straightforward.

Building the server gets three things the off-the-shelf approach can't:

- **Both sides of MCP.** Client *and* server, in Go, using the official SDK. That's the full protocol demonstrated end-to-end.
- **Pure Go runtime.** No Node.js subprocess, no npx cold starts, no npm cache. The React/Vite frontend needs Node for the build step, but the running sirtopham binary is pure Go. This keeps it that way.
- **Exposable from day one.** Doc 09 flags "MCP server exposure" as a v0.5+ direction — letting Claude Code and Codex query the project brain. With a custom server, that's a Cobra subcommand away, not a future project.

---

## Decision: Go MCP SDK

### `modelcontextprotocol/go-sdk` v1.3.0

The official Go SDK, maintained by the MCP org in collaboration with Google. 3.8k stars, 573 commits, v1.3.0 stable.

Key APIs we use:

| API | Purpose |
|-----|---------|
| `mcp.NewServer()` | Create the brain MCP server |
| `mcp.AddTool(server, tool, handler)` | Register each vault operation as an MCP tool |
| `mcp.NewClient()` | Create sirtopham's MCP client |
| `mcp.NewInMemoryTransports()` | In-process client↔server (no subprocess, no network) |
| `mcp.StdioTransport` | External exposure for `sirtopham brain-serve` command |
| `session.CallTool(ctx, params)` | How the brain tools invoke server-side operations |

---

## Architecture

### Current State

```
serve.go
  └─ brain.NewObsidianClient(url, apiKey)
       └─ RegisterBrainTools(registry, client, cfg)
            ├─ BrainRead    → client.ReadDocument()    → HTTP GET  /vault/{path}
            ├─ BrainSearch  → client.SearchKeyword()   → HTTP POST /search/simple
            ├─ BrainWrite   → client.WriteDocument()   → HTTP PUT  /vault/{path}
            └─ BrainUpdate  → client.ReadDocument()    → HTTP GET  (read)
                              client.WriteDocument()   → HTTP PUT  (write back)
```

### Target State

```
serve.go
  └─ brainmcp.Connect(ctx, vaultPath)
       │
       │  ┌────────────────────────────────────────────────────┐
       │  │  In-Process (NewInMemoryTransports)                │
       │  │                                                    │
       │  │  MCP Client ◄──── in-memory ────► MCP Server      │
       │  │  (brain.Backend)                  (vault package)  │
       │  └────────────────────────────────────────────────────┘
       │
       └─ RegisterBrainTools(registry, brainBackend, cfg)
            ├─ BrainRead    → brainBackend.ReadDocument()    → MCP call_tool "vault_read"
            ├─ BrainSearch  → brainBackend.SearchKeyword()   → MCP call_tool "vault_search"
            ├─ BrainWrite   → brainBackend.WriteDocument()   → MCP call_tool "vault_write"
            ├─ BrainUpdate  → brainBackend.PatchDocument()   → MCP call_tool "vault_patch"
            │                  (replace_section: read-modify-write via ReadDocument + WriteDocument)
            └─ BrainList    → brainBackend.ListDocuments()   → MCP call_tool "vault_list"


External exposure (separate command):

  $ sirtopham brain-serve --vault /path/to/vault

  ┌──────────────────────────────────────────────────────┐
  │  Claude Code / Codex / any MCP client                │
  │       │                                              │
  │       └─── stdio ───► MCP Server (same server code)  │
  │                        (vault package)               │
  └──────────────────────────────────────────────────────┘
```

### Three-Layer Design

#### Layer 1: `internal/brain/vault/` — Filesystem Operations

Pure Go. No MCP knowledge. No HTTP. Just functions that operate on an Obsidian vault directory.

```go
package vault

// Client provides direct filesystem access to an Obsidian vault.
type Client struct {
    root string // absolute path to vault directory
}

func New(vaultPath string) (*Client, error)

// Core operations — each maps 1:1 to what the brain tools need.
func (c *Client) ReadDocument(ctx context.Context, path string) (string, error)
func (c *Client) WriteDocument(ctx context.Context, path string, content string) error
func (c *Client) PatchDocument(ctx context.Context, path, operation, content string) error
func (c *Client) SearchKeyword(ctx context.Context, query string, maxResults int) ([]SearchHit, error)
func (c *Client) ListDocuments(ctx context.Context, directory string) ([]string, error)

// Enhanced operations — things the REST API couldn't do cleanly.
func (c *Client) ReadFrontmatter(ctx context.Context, path string) (map[string]any, error)
func (c *Client) WriteFrontmatter(ctx context.Context, path string, fields map[string]any) error
func (c *Client) ListTags(ctx context.Context, directory string) ([]string, error)
```

**Implementation notes:**

- `ReadDocument`: `os.ReadFile` on `filepath.Join(root, path)`. Validate path doesn't escape vault root (path traversal protection).
- `WriteDocument`: `os.WriteFile` with `os.MkdirAll` for parent directories. Atomic write via temp file + rename.
- `PatchDocument`: For `append`/`prepend`, read + modify + write. Same as current `brain_update` logic, but without the HTTP round-trip.
- `SearchKeyword`: Walk the vault directory, read `.md` files, score by keyword occurrence. Simple but effective for v0.1 — we're already planning LanceDB-backed semantic search for v0.2 which runs in-process regardless.
- `ListDocuments`: `filepath.WalkDir` with `.md` filter and `.obsidian/` exclusion.
- Frontmatter parsing: Split on `---` delimiters, `yaml.Unmarshal` the header block. We already have `gopkg.in/yaml.v3` in `go.mod`.

**This layer is independently useful and testable.** Tests hit a temp directory, no mocks needed.

#### Layer 2: `internal/brain/mcpserver/` — MCP Tool Registration

Wraps `vault.Client` operations as MCP tools using the Go SDK.

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// NewServer creates an MCP server exposing vault operations as tools.
func NewServer(vaultClient *vault.Client) *mcp.Server {
    server := mcp.NewServer(
        &mcp.Implementation{Name: "sirtopham-brain", Version: "v0.1.0"},
        nil,
    )

    mcp.AddTool(server, &mcp.Tool{
        Name:        "vault_read",
        Description: "Read a document from the Obsidian vault by path",
    }, func(ctx context.Context, req *mcp.CallToolRequest, input VaultReadInput) (*mcp.CallToolResult, any, error) {
        content, err := vaultClient.ReadDocument(ctx, input.Path)
        if err != nil {
            return &mcp.CallToolResult{IsError: true}, nil, err
        }
        return &mcp.CallToolResult{
            Content: []mcp.Content{&mcp.TextContent{Text: content}},
        }, nil, nil
    })

    // ... vault_write, vault_search, vault_patch, vault_list,
    //     vault_frontmatter_read, vault_frontmatter_write, vault_tags

    return server
}
```

**MCP tool inventory:**

| MCP Tool | Maps To | Description |
|----------|---------|-------------|
| `vault_read` | `vault.ReadDocument` | Read markdown content by vault-relative path |
| `vault_write` | `vault.WriteDocument` | Create or overwrite a document |
| `vault_patch` | `vault.PatchDocument` | Append, prepend, or replace content |
| `vault_search` | `vault.SearchKeyword` | Keyword search across the vault |
| `vault_list` | `vault.ListDocuments` | List documents in a directory |
| `vault_frontmatter_read` | `vault.ReadFrontmatter` | Read YAML frontmatter fields |
| `vault_frontmatter_write` | `vault.WriteFrontmatter` | Set/update frontmatter fields without touching body |
| `vault_tags` | `vault.ListTags` | List all tags used in the vault or a subdirectory |

The `vault_` prefix is deliberate — these are the server's tools, not sirtopham's agent-facing brain tools. The brain tools (`brain_read`, `brain_search`, etc.) call *through* the MCP client to reach these.

#### Layer 3: `internal/brain/mcpclient/` — MCP Client (Backend Implementation)

Connects to the MCP server and satisfies `brain.Backend`.

```go
package mcpclient

// Client implements brain.Backend by calling MCP tools on the brain server.
type Client struct {
    session *mcp.ClientSession
}

// Connect creates an in-process MCP brain server and connects to it.
func Connect(ctx context.Context, vaultPath string) (*Client, error) {
    // 1. Create vault.Client
    vc, err := vault.New(vaultPath)

    // 2. Create MCP server wrapping it
    server := mcpserver.NewServer(vc)

    // 3. Create in-memory transports
    t1, t2 := mcp.NewInMemoryTransports()

    // 4. Connect server to one end
    server.Connect(ctx, t1, nil)

    // 5. Connect client to the other end
    client := mcp.NewClient(
        &mcp.Implementation{Name: "sirtopham", Version: "v0.1.0"},
        nil,
    )
    session, err := client.Connect(ctx, t2, nil)

    return &Client{session: session}, nil
}

func (c *Client) ReadDocument(ctx context.Context, path string) (string, error) {
    res, err := c.session.CallTool(ctx, &mcp.CallToolParams{
        Name:      "vault_read",
        Arguments: map[string]any{"path": path},
    })
    // Extract text content from res.Content
}

// WriteDocument, SearchKeyword, ListDocuments, PatchDocument — same pattern
```

### Interface: `brain.Backend`

```go
// internal/brain/backend.go

package brain

import "context"

// Backend defines the operations the brain tools need from their backing store.
// Implementations:
//   - *mcpclient.Client (MCP, default) — in-process MCP client→server→filesystem
//   - *ObsidianClient (REST, legacy) — HTTP calls to Obsidian Local REST API
type Backend interface {
    ReadDocument(ctx context.Context, path string) (string, error)
    WriteDocument(ctx context.Context, path string, content string) error
    PatchDocument(ctx context.Context, path string, operation string, content string) error
    SearchKeyword(ctx context.Context, query string) ([]SearchHit, error)
    ListDocuments(ctx context.Context, directory string) ([]string, error)
}

// Closer is optionally implemented by backends that hold resources.
type Closer interface {
    Close() error
}
```

---

## Config Changes

### Before

```yaml
brain:
  enabled: true
  vault_path: /home/mitchell/vault
  obsidian_api_url: "http://localhost:27124"
  obsidian_api_key: "abc123"
```

### After

```yaml
brain:
  enabled: true
  vault_path: /home/mitchell/vault
  backend: mcp           # "mcp" (default, new) or "rest" (legacy)
  # Legacy REST fields — only used when backend: rest
  obsidian_api_url: "http://localhost:27124"
  obsidian_api_key: ""
```

The `backend` field defaults to `"mcp"`. Setting `"rest"` preserves the existing REST API path as a fallback during transition. No new config fields needed for MCP — the vault path is all the server needs.

---

## Wiring in `serve.go`

```go
// Brain backend — MCP (default) or REST (legacy).
var brainBackend brain.Backend
if cfg.Brain.Enabled {
    switch cfg.Brain.Backend {
    case "rest":
        // Legacy path: Obsidian REST API.
        apiURL := cfg.Brain.ObsidianAPIURL
        if apiURL == "" {
            apiURL = "http://localhost:27124"
        }
        brainBackend = brain.NewObsidianClient(apiURL, cfg.Brain.ObsidianAPIKey)
        logger.Info("brain backend: REST API", "url", apiURL)

    default: // "mcp" or empty
        // New default: in-process MCP server against vault filesystem.
        client, err := mcpclient.Connect(ctx, cfg.Brain.VaultPath)
        if err != nil {
            logger.Error("brain MCP server failed to start, disabling brain", "error", err)
            cfg.Brain.Enabled = false
        } else {
            brainBackend = client
            logger.Info("brain backend: MCP (in-process)", "vault", cfg.Brain.VaultPath)
            defer client.Close()
        }
    }
}
tool.RegisterBrainTools(registry, brainBackend, cfg.Brain)
```

---

## External Exposure: `sirtopham brain-serve`

New Cobra subcommand that runs the MCP server on stdio transport:

```go
// cmd/sirtopham/brain_serve.go

var brainServeCmd = &cobra.Command{
    Use:   "brain-serve",
    Short: "Run the project brain as a standalone MCP server (stdio)",
    Long:  "Exposes the Obsidian vault as an MCP server over stdin/stdout. " +
           "Connect from Claude Code, Codex, or any MCP client.",
    RunE: func(cmd *cobra.Command, args []string) error {
        vaultPath, _ := cmd.Flags().GetString("vault")
        vc, err := vault.New(vaultPath)
        if err != nil {
            return err
        }
        server := mcpserver.NewServer(vc)
        return server.Run(ctx, mcp.NewStdioTransport())
    },
}
```

**Claude Code config example:**

```json
{
  "mcpServers": {
    "sirtopham-brain": {
      "command": "sirtopham",
      "args": ["brain-serve", "--vault", "/path/to/vault"]
    }
  }
}
```

---

## brain_update: Upgrade Path

Current `brain_update` does read-modify-write in Go for three operations. The new `PatchDocument` on `vault.Client` handles all three natively on the filesystem:

| Operation | Current (REST) | New (vault.Client) |
|-----------|---------------|-------------------|
| `append` | HTTP GET → Go append → HTTP PUT | Read file → append → atomic write |
| `prepend` | HTTP GET → Go prepend → HTTP PUT | Read file → prepend after frontmatter → atomic write |
| `replace_section` | HTTP GET → Go section replace → HTTP PUT | Read file → heading-aware replace → atomic write |

Same logic, just without the HTTP round-trips. The `appendContent`, `prependContent`, and `replaceSectionContent` functions from `brain_update.go` move into `vault.Client.PatchDocument` — they're vault operations, not tool logic.

---

## File-by-File Change Inventory

### New Files

| File | Purpose |
|------|---------|
| `internal/brain/backend.go` | `Backend` interface definition |
| `internal/brain/vault/client.go` | Filesystem operations on Obsidian vault |
| `internal/brain/vault/client_test.go` | Tests against temp directory |
| `internal/brain/vault/search.go` | Keyword search implementation |
| `internal/brain/vault/search_test.go` | Search scoring/filtering tests |
| `internal/brain/vault/frontmatter.go` | YAML frontmatter read/write |
| `internal/brain/vault/doc.go` | Package doc |
| `internal/brain/mcpserver/server.go` | MCP tool registration wrapping vault.Client |
| `internal/brain/mcpserver/server_test.go` | MCP server tests (in-memory transport) |
| `internal/brain/mcpserver/doc.go` | Package doc |
| `internal/brain/mcpclient/client.go` | MCP client satisfying brain.Backend |
| `internal/brain/mcpclient/client_test.go` | End-to-end MCP client↔server tests |
| `internal/brain/mcpclient/doc.go` | Package doc |
| `cmd/sirtopham/brain_serve.go` | `brain-serve` Cobra subcommand |

### Modified Files

| File | Change |
|------|--------|
| `internal/brain/client.go` | Add `PatchDocument` method; verify satisfies `Backend` interface |
| `internal/brain/client_test.go` | Verify `ObsidianClient` still satisfies `Backend` |
| `internal/tool/brain_read.go` | Field type `*brain.ObsidianClient` → `brain.Backend` |
| `internal/tool/brain_search.go` | Same |
| `internal/tool/brain_write.go` | Same |
| `internal/tool/brain_update.go` | Same; `appendContent`/`prependContent`/`replaceSectionContent` move to vault package, tool calls `PatchDocument` for append/prepend, read-modify-write for replace_section |
| `internal/tool/register.go` | `RegisterBrainTools` accepts `brain.Backend` instead of `*brain.ObsidianClient` |
| `internal/tool/brain_test.go` | Replace `httptest.Server` mock with `fakeBackend` struct satisfying `brain.Backend` |
| `internal/config/config.go` | Add `Backend` field to `BrainConfig` (default: `"mcp"`) |
| `cmd/sirtopham/serve.go` | Branch on `cfg.Brain.Backend` to construct MCP or REST client |
| `cmd/sirtopham/root.go` | Register `brain-serve` subcommand |
| `go.mod` | Add `github.com/modelcontextprotocol/go-sdk` |

### Deleted Files

None. `internal/brain/client.go` (REST) is retained as a legacy `Backend` implementation.

### Unchanged Files

| File | Why |
|------|-----|
| `internal/context/*` | Brain types (`BrainHit`, `RetrievalResults.BrainHits`) are downstream of tool results |
| `internal/server/*` | Web API/metrics consume context assembly reports, not the brain client |
| `web/*` | Frontend unchanged |
| `internal/tool/brain_read.go` (schema) | Tool names, parameters, response format — all unchanged from the LLM's perspective |

---

## Testing Strategy

### Unit Tests: `internal/brain/vault/` (filesystem)

Tests create a temp directory, populate it with markdown files, and exercise each operation. No mocks — real filesystem I/O against a throwaway directory. Tests validate:

- Path traversal protection (reject `../` escapes)
- Atomic writes (no partial files on crash)
- Search scoring and ranking
- Frontmatter parsing round-trips
- `.obsidian/` directory exclusion
- Empty vault / missing directory handling

### Unit Tests: `internal/brain/mcpserver/` (MCP server)

Use `NewInMemoryTransports()` to connect a test client to the server. Call each tool, verify the response shape matches expectations. The vault client underneath points at a temp directory — still real I/O, but the test verifies the MCP protocol layer specifically.

### Unit Tests: `internal/brain/mcpclient/` (MCP client)

End-to-end through all three layers: test creates a temp vault → creates `vault.Client` → wraps in MCP server → connects MCP client → calls `Backend` methods. Verifies that the full round-trip (Go method → MCP call_tool → vault filesystem → MCP response → Go return) works correctly.

### Unit Tests: `internal/tool/brain_test.go` (simplified)

Replace the current `httptest.Server` mock (95 lines of HTTP handler logic mimicking the Obsidian REST API) with a `fakeBackend` struct:

```go
type fakeBackend struct {
    docs     map[string]string
    searches map[string][]brain.SearchHit
}

func (f *fakeBackend) ReadDocument(ctx context.Context, path string) (string, error) {
    content, ok := f.docs[path]
    if !ok {
        return "", fmt.Errorf("Document not found: %s", path)
    }
    return content, nil
}
// ... WriteDocument, SearchKeyword, ListDocuments, PatchDocument
```

This eliminates all HTTP coupling from tool tests. The 30 existing brain tool tests pass against `fakeBackend` with zero behavior change — they test the tool logic (frontmatter extraction, wikilink parsing, section replacement, error enrichment), not the transport.

### Manual Testing Checklist

Before declaring the migration complete:

1. `sirtopham serve` with default config — brain tools work in agent session via MCP
2. `sirtopham serve` with `backend: rest` — legacy REST path still works
3. `sirtopham serve` with Obsidian closed — MCP backend works (REST backend fails gracefully)
4. `sirtopham brain-serve --vault /path` — external MCP server starts, Claude Code can connect
5. Vault files written by MCP backend are valid Obsidian markdown (open in Obsidian, verify frontmatter renders, wikilinks resolve, graph view works)
6. Create, read, update, search cycle through a full agent session

---

## Migration Order

### Phase 1: Interface Extraction (no behavior change)

1. Define `brain.Backend` interface in `internal/brain/backend.go`
2. Add `PatchDocument` to `ObsidianClient` (delegates to read-modify-write, same as current `brain_update` logic)
3. Verify `*ObsidianClient` satisfies `Backend` (compile check)
4. Change all four brain tool structs from `*brain.ObsidianClient` to `brain.Backend`
5. Change `RegisterBrainTools` signature to accept `brain.Backend`
6. Update `serve.go` to pass the client as `brain.Backend`
7. Run all existing tests — everything passes, zero behavior change

**Gate:** All 207 tool tests + 9 brain client tests pass. No new packages, no new deps.

### Phase 2: Simplify Tool Tests

1. Implement `fakeBackend` in `brain_test.go`
2. Replace `httptest.Server` mock with `fakeBackend`
3. All 30 brain tool tests pass against `fakeBackend`
4. Validates that the `Backend` interface is correctly factored

**Gate:** Brain tool tests are HTTP-free. Coverage unchanged.

### Phase 3: Vault Filesystem Package

1. Implement `internal/brain/vault/client.go` — core operations
2. Implement `internal/brain/vault/search.go` — keyword search
3. Implement `internal/brain/vault/frontmatter.go` — YAML frontmatter
4. Move `appendContent`, `prependContent`, `replaceSectionContent` from `brain_update.go` into `vault.PatchDocument`
5. Write tests against temp directories

**Gate:** `vault` package fully tested independently. No MCP yet.

### Phase 4: MCP Server + Client

1. `go get github.com/modelcontextprotocol/go-sdk`
2. Implement `internal/brain/mcpserver/server.go` — register vault ops as MCP tools
3. Implement `internal/brain/mcpclient/client.go` — MCP client satisfying `brain.Backend`
4. Write end-to-end tests (client → server → vault → filesystem)
5. Add `Backend` config field to `BrainConfig`
6. Wire up in `serve.go` — branch on config

**Gate:** `sirtopham serve` works with both `backend: mcp` and `backend: rest`.

### Phase 5: External Exposure + Cleanup

1. Implement `cmd/sirtopham/brain_serve.go` — Cobra subcommand
2. Manual testing checklist
3. Update doc 09 to reflect MCP as default backend, `brain-serve` as new command
4. Update doc 00 index if needed

---

## Dependencies Added

| Dependency | Purpose | Size Impact |
|------------|---------|-------------|
| `github.com/modelcontextprotocol/go-sdk` | MCP client + server | Moderate (JSON-RPC, transport) |

### Runtime Dependencies

**None added.** The MCP server runs in-process. No Node.js, no subprocess, no external binary. `sirtopham brain-serve` is a mode of the existing binary, not a separate build artifact.

---

## What This Does NOT Change

- **Brain tool names and schemas** exposed to the LLM are unchanged. `brain_read`, `brain_search`, `brain_write`, `brain_update` — same names, same parameters, same response formats. The LLM sees zero difference.
- **Context assembly** is unchanged. `BrainHit` types, retrieval results, budget fitting — all downstream of tool execution.
- **Web UI** is unchanged. Context inspector, brain results display — all consume the same report types.
- **Vault structure and document format** are unchanged. Same markdown, same frontmatter, same wikilinks.
- **The REST API client** is not deleted. Retained as a `brain.Backend` implementation behind `backend: rest`.

---

## Open Questions

1. **Vault search quality.** The filesystem-based keyword search replaces Obsidian's built-in BM25 search. For v0.1 this is acceptable — search is keyword-based either way, and LanceDB semantic search (v0.2) will supersede it. Worth comparing result quality empirically during testing.

2. **File watching.** The vault package operates on files at call time. If someone edits a file in Obsidian between brain tool calls, sirtopham sees the updated content on the next read — this is correct. But should the MCP server emit `notifications/resources/updated` when vault files change? Out of scope for this migration but worth noting for future.

3. **Concurrent writes.** If the agent writes a brain document while the developer has the same file open in Obsidian, Obsidian detects external changes and prompts to reload. Same behavior as the REST API path. The atomic write (temp + rename) ensures Obsidian never sees a partial file.

4. **MCP server tool naming.** The spec uses `vault_read`, `vault_write`, etc. for the MCP server's tools. These are the *server-side* tool names. The *agent-facing* tool names remain `brain_read`, `brain_write`, etc. Two different naming layers — the agent never sees the MCP tool names.

---

## Future Leverage

This migration establishes patterns that pay forward:

- **MCP client infrastructure** is reusable for connecting to any MCP server (GitHub, Linear, Notion, etc.)
- **MCP server infrastructure** is reusable for exposing any sirtopham capability externally
- **`sirtopham brain-serve`** lets Claude Code and Codex query the project brain immediately — the "MCP server exposure" from doc 09 is delivered alongside this migration, not deferred to v0.5
- **The vault package** is useful independent of MCP — any future code that needs to read/write the Obsidian vault (indexing pipeline, convention extractor) uses it directly without going through the MCP layer
