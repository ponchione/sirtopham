Fresh-session handoff: LAYER 6 COMPLETE — all 10 epics done

What was completed this session (3 commits, pushed to origin/main)
- `6af791c` — `feat(web): thinking blocks, tool call cards, markdown rendering, history loading (L6E07 slices 2-3)`
- `80b81c7` — `feat(web): sidebar with conversation list, navigation, and mobile responsive layout (L6E08)`
- `584beb2` — `feat(api): REST endpoints for project, config, providers, and metrics (L6E03)`
- `47fa85b` — `feat(web): context inspector debug panel, settings page, conversation metrics (L6E09+E10)`

Current state — LAYER 6 COMPLETE
- Layers 0-5: fully implemented (tools, agent loop, context assembly, providers, conversations)
- Layer 6 Epics 01-10: ALL COMPLETE
- `make build` compiles frontend (Vite) → copies dist/ → builds Go binary with embed.FS
- `make test` — all packages pass
- Zero TypeScript errors

Layer 6 status map
```
  ✅ Epic 01 — HTTP Server Foundation
  ✅ Epic 02 — REST API: Conversations (6 endpoints)
  ✅ Epic 03 — REST API: Project, Config & Metrics (8 endpoints)
  ✅ Epic 04 — WebSocket Handler
  ✅ Epic 05 — Serve Command (composition root)
  ✅ Epic 06 — React Scaffolding (Vite + React + TS + Tailwind + shadcn/ui)
  ✅ Epic 07 — Conversation UI (streaming chat, thinking, tools, markdown)
  ✅ Epic 08 — Sidebar & Navigation (conversation list, mobile responsive)
  ✅ Epic 09 — Context Inspector (debug panel with all sections)
  ✅ Epic 10 — Settings & Metrics UI (settings page, conversation metrics)
```

Full REST API surface
  GET    /api/health
  GET    /api/conversations
  POST   /api/conversations
  GET    /api/conversations/:id
  DELETE /api/conversations/:id
  GET    /api/conversations/:id/messages
  GET    /api/conversations/search?q=
  GET    /api/project
  GET    /api/project/tree?depth=
  GET    /api/project/file?path=
  GET    /api/config
  PUT    /api/config
  GET    /api/providers
  GET    /api/metrics/conversation/:id
  GET    /api/metrics/conversation/:id/context/:turn
  WS     /api/ws

Frontend routes
  /           — Home (new conversation input)
  /c/:id      — Conversation view (chat + inspector + metrics)
  /settings   — Settings (providers, model selector, project info)

Frontend file map
  web/src/main.tsx                              — Router (3 routes)
  web/src/pages/
    conversation-list.tsx                       — Home page
    conversation.tsx                            — Chat page + inspector + metrics
    settings.tsx                                — Settings page
  web/src/components/layout/
    root-layout.tsx                             — Sidebar state + hamburger
    sidebar.tsx                                 — Conv list, nav, mobile
  web/src/components/chat/
    thinking-block.tsx                          — Collapsible thinking
    tool-call-card.tsx                          — Collapsible tool call
    turn-usage-badge.tsx                        — Token/duration pill
    markdown-content.tsx                        — Markdown + syntax highlight
    conversation-metrics.tsx                    — Token/tool/quality metrics
  web/src/components/inspector/
    context-inspector.tsx                       — Full inspector panel
    collapsible-section.tsx                     — Reusable collapsible
    budget-bar.tsx                              — Token budget stacked bar
  web/src/hooks/
    use-conversation.ts                         — Block-based reducer
    use-conversation-list.ts                    — REST list fetch
    use-conversation-metrics.ts                 — REST metrics fetch
    use-context-report.ts                       — REST/WS context reports
    use-providers.ts                            — REST providers fetch
    use-project-info.ts                         — REST project info fetch
    use-websocket.ts                            — WebSocket connection
  web/src/lib/
    api.ts                                      — Fetch wrapper
    history.ts                                  — MessageView → ChatMessage
    utils.ts                                    — cn() utility
  web/src/types/
    api.ts                                      — REST conversation types
    events.ts                                   — WebSocket event types
    metrics.ts                                  — Metrics/config/provider types

Backend handler files
  internal/server/server.go                     — HTTP server
  internal/server/api.go                        — JSON helpers
  internal/server/conversations.go              — Conversation CRUD
  internal/server/websocket.go                  — WebSocket streaming
  internal/server/project.go                    — Project info/tree/file
  internal/server/configapi.go                  — Config/providers
  internal/server/metrics.go                    — Metrics/context reports

Important notes for next session
- shadcn/ui v4 uses @base-ui/react (NOT Radix). No `asChild` prop on Button
- `erasableSyntaxOnly` in tsconfig — no `public` constructor parameter properties
- PUT /api/config is runtime-only, NOT persisted to sirtopham.yaml
- react-syntax-highlighter bundle is ~1MB — consider lazy import if size matters
- `make test` not `go test ./...` — Makefile has CGo linker flags for lancedb
- Card and Input UI components exist but are unused (available for future)

Development workflow
- Two terminals: `make dev-backend` + `make dev-frontend`
- Or production: `make build && ./bin/sirtopham serve --config sirtopham.yaml`

Validation commands
- `git log --oneline -10`
- `make test` (all packages green)
- `make build && ./bin/sirtopham serve --config sirtopham.yaml`
- `cd web && npx tsc --noEmit` (zero TS errors)
