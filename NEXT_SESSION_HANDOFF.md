Fresh-session handoff: Layer 6 backend complete — frontend is next

What was completed (7 commits, all pushed to origin/main)
- `b6fffcb` — `docs: add Layer 6 backend implementation plan`
- `fc17d80` — `feat(server): add HTTP server foundation with middleware and static serving (L6E01)`
- `bc3aae0` — `feat(server): add REST API for conversations (L6E02)`
- `35ba0ff` — `feat(server): add WebSocket handler for agent event streaming (L6E04)`
- `3e2836e` — `feat(cmd): wire serve command as composition root (L6E05)`
- `85a7166` — `chore: ignore binary artifact`
- `cd6422b` — `docs: update handoff — Layer 6 backend complete`

Current state — what exists
- Layers 0-5: fully implemented (tools, agent loop, context assembly, providers, conversations)
- Layer 6 Epics 01, 02, 04, 05: complete (HTTP server, REST API, WebSocket, serve command)
- Layer 6 Epic 03: NOT started (REST API for project/config/metrics)
- Layer 6 Epics 06-10: NOT started (React frontend)
- `sirtopham serve` is a fully wired composition root — all backend layers connected
- 28 tests in internal/server/ pass with -race
- Binary builds: `make build` → bin/sirtopham

Layer 6 status map
```
  ✅ Epic 01 — HTTP Server Foundation
  ✅ Epic 02 — REST API: Conversations (6 endpoints)
  ⬚  Epic 03 — REST API: Project, Config & Metrics
  ✅ Epic 04 — WebSocket Handler
  ✅ Epic 05 — Serve Command (composition root)
  ⬚  Epic 06 — React Scaffolding (Vite + React + TS + Tailwind + shadcn/ui)
  ⬚  Epic 07 — Conversation UI (chat interface)
  ⬚  Epic 08 — Sidebar & Navigation
  ⬚  Epic 09 — Context Inspector (debug panel)
  ⬚  Epic 10 — Settings & Metrics UI
```

Dependency graph for remaining work
```
  Epic 03 (REST API: Project/Config/Metrics) — independent, can start now
       │
  Epic 06 (React Scaffolding) — needs Epic 05 (done), can start now
       │
  ┌────┼──────────┐
  │    │           │
Epic 07  Epic 08  Epic 10     ← parallel after 06
(Chat)  (Sidebar) (Settings)
  │       │
  └──┬────┘
     │
  Epic 09 (Context Inspector) ← needs 07 + 08
```

Next steps — recommended order

1. Epic 06: React Scaffolding (HIGH PRIORITY — unblocks all frontend work)
   - Init Vite + React + TypeScript in web/
   - Configure Tailwind + shadcn/ui
   - Vite dev server proxy for /api and /api/ws
   - Makefile: `make build` compiles frontend then Go binary with embed.FS
   - App shell: sidebar placeholder + main content area, dark theme
   - TypeScript types for WebSocket events (ServerMessage, ClientMessage)
   - API client utility (thin fetch wrapper)
   - Read: docs/layer6/06-react-scaffolding/epic-06-react-scaffolding.md

2. Epic 07: Conversation UI (first working chat in browser)
   - Message list component, user input, streaming token display
   - Tool call visualization (collapsible blocks)
   - Thinking indicator
   - WebSocket connection lifecycle
   - Read: docs/layer6/07-conversation-ui/epic-07-conversation-ui.md

3. Epic 03: REST API for Project/Config/Metrics (can parallel with frontend)
   - GET /api/project, /api/project/tree, /api/project/file
   - GET/PUT /api/config
   - GET /api/providers
   - GET /api/metrics/conversation/:id
   - GET /api/metrics/conversation/:id/context/:turn
   - Read: docs/layer6/03-rest-api-project-config-metrics/epic-03-rest-api-project-config-metrics.md

4. Epic 08: Sidebar & Navigation
5. Epic 09: Context Inspector (debug panel — the v0.1 differentiator)
6. Epic 10: Settings & Metrics UI

Other pending work (non-Layer 6)
- Agent loop batch dispatch refactor (replace single-call adapter with native batch)
- Obsidian Client & Brain Tools (v0.2 scope)
- Semantic search tool (requires embedding service running)

Important architecture decisions locked
- Go stdlib net/http with Go 1.22+ patterns (no framework)
- nhooyr.io/websocket v1.8.17 for WebSocket (uses direct Hijacker assertion)
- statusWriter implements http.Hijacker for WS upgrade through middleware
- Server.ListenAddr() blocks on ready channel (race-free)
- ConversationService and AgentService are narrow interfaces in server package
- embed.FS serves web/dist/ in prod, SPA fallback to index.html for client routing

Validation commands
- `git log --oneline -10`
- `make test` (28 packages, all green)
- `make build && ./bin/sirtopham serve --config sirtopham.yaml --dev`
- `curl http://localhost:8090/api/health`
- `curl http://localhost:8090/api/conversations`

Read first for Epic 06
- docs/layer6/06-react-scaffolding/epic-06-react-scaffolding.md
- docs/specs/07-web-interface-and-streaming.md (frontend stack + WS protocol)
- docs/specs/05-agent-loop.md §Streaming to the Web UI (event types)
- internal/server/server.go (embed.FS wiring)
- internal/server/websocket.go (ServerMessage/ClientMessage types)
- Makefile (build target to extend)
