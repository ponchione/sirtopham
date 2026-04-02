# Brain Viewer Spec

**Status:** Draft  
**Date:** 2026-04-02  
**Author:** Mitchell + Claude  
**Scope:** Read-only brain explorer in sirtopham's web UI  
**Depends on:** Brain MCP Migration (for `internal/brain/vault/` package)

---

## Overview

A read-only brain explorer built into sirtopham's web UI. Not an Obsidian replacement — a viewer that shows you what the brain looks like from sirtopham's perspective. Three components: document viewer, wikilink graph, and vault browser. Obsidian stays as the editor.

The goal is answering two questions without switching to Obsidian: "what does the agent know about this project?" and "how are these brain documents connected?"

---

## What This Is

- **Document viewer.** Click a brain document path anywhere in the UI (context inspector, conversation, search results) → see rendered markdown inline. Frontmatter displayed as structured metadata. Wikilinks are clickable within the viewer.
- **Wikilink graph.** Interactive force-directed graph showing how brain documents connect via `[[wikilinks]]`. Click a node to open that document in the viewer. Visual clusters reveal knowledge topology.
- **Vault browser.** File tree of the brain vault with search. Replaces the need to open Obsidian just to see what's in the vault.
- **"Open in Obsidian" links.** Every document in the viewer has a one-click link to open it in Obsidian for editing. `obsidian://open?vault=...&file=...` URI scheme.

## What This Is Not

- Not an editor. No write operations from the web UI. Editing happens in Obsidian.
- Not a full knowledge management app. No canvas, no plugins, no themes, no mobile sync.
- Not a replacement for Obsidian. The two tools share the same vault files. sirtopham reads. Obsidian reads and writes.

---

## Architecture

### Data Flow

```
Vault filesystem (markdown files on disk)
       │
       ├──► vault.Client (internal/brain/vault/)
       │         │
       │         ├──► Brain REST endpoints (new)
       │         │         │
       │         │         └──► React brain viewer components
       │         │
       │         └──► MCP server (existing from MCP migration)
       │
       └──► Obsidian (editor, graph view, plugins)
```

The vault package from the MCP migration provides all the filesystem operations the viewer needs. The web UI gets new REST endpoints that call into `vault.Client` directly — no MCP round-trip needed for the viewer, since the Go server has direct access.

### New REST Endpoints

```
GET /api/brain/documents              List all brain documents (path, title, tags, updated_at)
GET /api/brain/documents/*path        Read a single document (rendered content + frontmatter + wikilinks)
GET /api/brain/search?q=...           Search brain documents
GET /api/brain/graph                  Wikilink graph (nodes + edges)
GET /api/brain/tags                   All tags with document counts
GET /api/brain/stats                  Vault stats (doc count, total size, tag distribution)
```

All read-only. No PUT, POST, DELETE. These endpoints are lightweight — they call `vault.Client` methods that are already built for the MCP migration.

### Endpoint Details

#### `GET /api/brain/documents`

Returns a flat list of all `.md` files in the vault with extracted metadata.

```json
{
  "documents": [
    {
      "path": "architecture/provider-design.md",
      "title": "Provider Design",
      "tags": ["architecture", "provider"],
      "frontmatter": {"status": "active", "created": "2026-03-28", "author": "agent"},
      "size_bytes": 4200,
      "updated_at": "2026-03-30T14:22:00Z"
    }
  ],
  "total": 47
}
```

Optional query params: `?tag=debugging`, `?dir=architecture/`, `?sort=updated`.

#### `GET /api/brain/documents/*path`

Returns document content plus parsed metadata.

```json
{
  "path": "debugging/lancedb-cgo-gotchas.md",
  "title": "LanceDB CGo Nil Slice Segfault",
  "content_markdown": "# LanceDB CGo Nil Slice Segfault\n\n...",
  "frontmatter": {"created": "2026-03-28", "tags": ["debugging", "cgo", "lancedb"]},
  "outgoing_links": ["architecture/provider-design", "conventions/error-handling"],
  "incoming_links": ["debugging/oauth-token-refresh-race"],
  "obsidian_uri": "obsidian://open?vault=brain-vault&file=debugging%2Flancedb-cgo-gotchas"
}
```

The `outgoing_links` come from wikilink extraction (already implemented in `brain_read.go`). The `incoming_links` come from scanning the vault for references to this document's basename.

#### `GET /api/brain/graph`

Returns the full wikilink graph for visualization.

```json
{
  "nodes": [
    {"id": "architecture/provider-design.md", "title": "Provider Design", "tags": ["architecture"], "size": 4200}
  ],
  "edges": [
    {"source": "debugging/lancedb-cgo-gotchas.md", "target": "architecture/provider-design.md", "label": "provider-design"}
  ]
}
```

Built by walking all `.md` files and extracting wikilinks. Cached in memory, rebuilt when any file changes or on explicit refresh.

Optional param: `?root=architecture/provider-design.md&depth=2` for ego-centric subgraphs.

#### `GET /api/brain/search?q=...`

Keyword search with snippets.

```json
{
  "query": "cgo segfault",
  "results": [
    {
      "path": "debugging/lancedb-cgo-gotchas.md",
      "title": "LanceDB CGo Nil Slice Segfault",
      "snippet": "...nil slice passed through CGo boundary causes segfault...",
      "score": 0.85
    }
  ]
}
```

Backed by `vault.SearchKeyword`.

#### `GET /api/brain/tags`

```json
{
  "tags": [
    {"name": "debugging", "count": 12},
    {"name": "architecture", "count": 8},
    {"name": "convention", "count": 5}
  ]
}
```

#### `GET /api/brain/stats`

```json
{
  "document_count": 47,
  "total_size_bytes": 198400,
  "tag_count": 15,
  "orphan_count": 3,
  "most_connected": "architecture/provider-design.md",
  "last_modified": "2026-04-02T09:15:00Z"
}
```

---

## UI Components

### Route: `/brain`

New top-level route in the React app. Three-panel layout:

```
┌──────────┬─────────────────────────────┬──────────────┐
│          │                             │              │
│  Vault   │     Document Viewer         │    Graph     │
│  Browser │     (rendered markdown)     │    Panel     │
│          │                             │   (d3-force) │
│  - tree  │  frontmatter bar            │              │
│  - search│  wikilinks → clickable      │  click node  │
│  - tags  │  "Open in Obsidian" button  │  → opens doc │
│          │                             │              │
└──────────┴─────────────────────────────┴──────────────┘
```

The graph panel collapses on smaller screens. The vault browser collapses into a sheet/drawer on mobile.

### Vault Browser (left panel)

- **File tree.** Collapsible directory listing from `GET /api/brain/documents`. Grouped by directory. Click a file → opens in document viewer.
- **Search bar.** Calls `GET /api/brain/search?q=...`. Shows results inline, replacing the tree temporarily.
- **Tag filter.** Chips from `GET /api/brain/tags`. Click a tag → filters tree to documents with that tag.
- **Stats footer.** Document count, last modified time from `GET /api/brain/stats`.

### Document Viewer (center panel)

- **Markdown rendering.** Use `react-markdown` with `remark-gfm` for GitHub-flavored markdown (tables, task lists, strikethrough). Code blocks with syntax highlighting via `rehype-highlight` or `shiki`.
- **Frontmatter bar.** Structured display above the content: tags as colored chips, status badge, created/author/date fields. Not raw YAML — parsed and formatted.
- **Wikilink rendering.** `[[target]]` links rendered as clickable internal links. Click → navigates to that document within the viewer. Wikilinks that resolve to existing documents get a different style than broken links (vault browser can verify existence).
- **Backlinks section.** Below the content: "Referenced by" list showing `incoming_links` from the API response. Click → navigates to the referencing document.
- **"Open in Obsidian" button.** Top-right of viewer. Uses `obsidian_uri` from the API response. Opens the document in Obsidian for editing. Falls back gracefully if Obsidian isn't running (URI scheme just doesn't open anything — no error handling needed).
- **Breadcrumb.** Shows vault path: `brain / debugging / lancedb-cgo-gotchas.md`. Each segment is clickable → navigates vault browser to that directory.

### Graph Panel (right panel)

- **Force-directed graph.** `d3-force` (already available — d3 is in the artifact libraries list for React). Nodes are documents, edges are wikilinks. Node size by connection count. Node color by primary tag or directory.
- **Interaction.** Click node → opens document in viewer. Hover → shows title tooltip. Drag to rearrange. Scroll to zoom. Double-click → center and zoom on that node's neighborhood.
- **Ego view.** When a document is open in the viewer, the graph highlights that node and its direct connections. Dims everything else. Shows the local neighborhood without losing global context.
- **Orphan indicator.** Documents with zero wikilinks (incoming or outgoing) shown in a distinct color or clustered at the edge. These are candidates for linking or deletion.
- **Cluster detection.** Visually, the force layout will naturally cluster densely-connected documents. No algorithmic clustering needed — d3-force handles this with link distance and charge parameters.

### Context Inspector Integration

The existing context inspector already shows brain results with `vault_path` and `title`. Two additions:

- **Clickable paths.** Brain result paths in the context inspector become links. Click → navigates to `/brain?doc=debugging/lancedb-cgo-gotchas.md`, which opens the brain viewer focused on that document.
- **"View in Brain" button.** Small icon button next to each brain result in the inspector. Same navigation.

### Conversation Integration

When brain tool results appear in conversation messages (brain_read, brain_search responses), document paths become clickable links to the brain viewer. Same pattern as the context inspector integration.

---

## Implementation Plan

### Dependencies

| Dependency | Purpose | Status |
|------------|---------|--------|
| `internal/brain/vault/` | Filesystem operations | Built during MCP migration |
| `react-markdown` | Markdown rendering | npm install |
| `remark-gfm` | GFM tables, task lists | npm install |
| `rehype-highlight` | Code syntax highlighting | npm install (or `shiki` for better quality) |
| `d3` | Force-directed graph | Already available in React artifact libraries |

### New Go Files

| File | Purpose |
|------|---------|
| `internal/server/brainapi.go` | Brain REST endpoint handlers |
| `internal/server/brainapi_test.go` | Endpoint tests |

### New React Files

| File | Purpose |
|------|---------|
| `web/src/pages/brain.tsx` | Brain explorer page (three-panel layout) |
| `web/src/components/brain/vault-browser.tsx` | Left panel: file tree, search, tags |
| `web/src/components/brain/document-viewer.tsx` | Center panel: markdown renderer, frontmatter, backlinks |
| `web/src/components/brain/graph-panel.tsx` | Right panel: d3-force wikilink graph |
| `web/src/components/brain/frontmatter-bar.tsx` | Structured frontmatter display |
| `web/src/hooks/use-brain-document.ts` | Fetch + cache a single brain document |
| `web/src/hooks/use-brain-graph.ts` | Fetch + cache the wikilink graph |
| `web/src/hooks/use-brain-search.ts` | Search with debounce |
| `web/src/types/brain.ts` | TypeScript types for brain API responses |

### Modified Files

| File | Change |
|------|--------|
| `web/src/main.tsx` | Add `/brain` and `/brain/*` routes |
| `web/src/components/layout/sidebar.tsx` | Add "Brain" nav link |
| `web/src/components/inspector/context-inspector.tsx` | Make brain result paths clickable |
| `cmd/sirtopham/serve.go` | Register brain API handler |

---

## Build Phases

### Phase 1: REST Endpoints

1. Implement `internal/server/brainapi.go` — all six endpoints
2. Wire up in `serve.go` — pass `vault.Client` to handler constructor
3. Test endpoints against a fixture vault

**Gate:** `curl` against all endpoints returns expected JSON.

### Phase 2: Vault Browser + Document Viewer

1. Add `/brain` route, page scaffold, TypeScript types
2. Implement vault browser (file tree, search)
3. Implement document viewer (markdown rendering, frontmatter bar)
4. Wire up wikilink click navigation
5. Add "Open in Obsidian" button
6. Add backlinks section

**Gate:** Can browse vault, read any document, click wikilinks to navigate, open in Obsidian.

### Phase 3: Graph Visualization

1. Implement graph panel with d3-force
2. Wire up click-to-navigate (graph node → document viewer)
3. Implement ego view (highlight active document's neighborhood)
4. Style nodes by tag/directory, size by connection count

**Gate:** Graph renders, nodes are clickable, ego view highlights on document selection.

### Phase 4: Cross-UI Integration

1. Make brain paths clickable in context inspector
2. Make brain paths clickable in conversation messages
3. Add "Brain" link to sidebar navigation

**Gate:** Any brain reference anywhere in the UI is one click from the brain viewer.

---

## Obsidian URI Scheme

The "Open in Obsidian" button uses Obsidian's URI protocol:

```
obsidian://open?vault={vault_name}&file={encoded_path}
```

- `vault_name`: The vault folder name (last segment of `cfg.Brain.VaultPath`).
- `file`: URL-encoded vault-relative path without `.md` extension (Obsidian convention).

Example: `obsidian://open?vault=brain-vault&file=debugging%2Flancedb-cgo-gotchas`

The Go API includes the pre-built URI in the document response so the frontend doesn't need to construct it. If Obsidian isn't installed or the vault name doesn't match, the link simply does nothing — no error handling required.

---

## Design Notes

**Markdown rendering fidelity.** The viewer doesn't need to match Obsidian's rendering pixel-for-pixel. It needs to render standard markdown, GFM extensions, YAML frontmatter, and wikilinks. Obsidian-specific syntax (callouts, Dataview queries, Templater macros) renders as plain text — acceptable for a read-only viewer.

**Graph performance.** For a personal brain vault (likely under 200 documents), d3-force handles the full graph comfortably. If the vault grows past ~500 nodes, add a pagination or clustering strategy. Not a v0.1 concern.

**Caching.** The graph endpoint is the most expensive (walks entire vault, extracts all wikilinks). Cache the graph in-memory with a TTL or invalidation on vault writes. Document reads are cheap (single file read) and don't need caching.

**No real-time sync.** The viewer reads files on request. If you edit a document in Obsidian, the viewer shows the update on next navigation or refresh. No file watcher, no WebSocket push. Simple and sufficient for a single-user tool.

---

## What This Enables

- **Debugging context assembly.** See exactly what brain document the agent retrieved. Click from context inspector → read the full document → understand why the agent made a particular decision.
- **Vault health monitoring.** Orphan documents, missing backlinks, tag distribution. The graph immediately shows which documents are disconnected.
- **Session review.** After the agent writes a brain document during a session, view it immediately without switching to Obsidian. Verify it captured the right information.
- **Onboarding.** When starting a new session, browse the brain to see what institutional knowledge already exists. The graph reveals the knowledge topology at a glance.

---

## Open Questions

1. **Obsidian callout syntax.** Obsidian uses `> [!note]` callout blocks. These render as blockquotes in standard markdown. Worth adding a custom remark plugin to render them with styling? Low priority but would improve visual fidelity.

2. **Embedded images.** Brain documents may reference images in the vault. The viewer would need to serve vault images via a new endpoint (`GET /api/brain/media/*path`). Defer until someone actually puts images in brain docs.

3. **Mermaid diagrams.** If brain documents contain mermaid code blocks, should the viewer render them? A mermaid remark plugin exists. Nice-to-have, not blocking.

4. **Graph layout persistence.** Should the graph remember node positions between sessions? d3-force randomizes initial positions. LocalStorage could persist the layout. Low priority.
