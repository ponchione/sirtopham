Fresh-session handoff: Layer 6 Epic 07 complete (slices 1-3)

What was completed this session (1 commit, pushed to origin/main)
- `6af791c` — `feat(web): thinking blocks, tool call cards, markdown rendering, history loading (L6E07 slices 2-3)`

Current state — what exists
- Layers 0-5: fully implemented (tools, agent loop, context assembly, providers, conversations)
- Layer 6 Epics 01, 02, 04, 05: complete (HTTP server, REST API, WebSocket, serve command)
- Layer 6 Epic 06: COMPLETE (React scaffolding)
- Layer 6 Epic 07: COMPLETE (all 3 slices)
- Layer 6 Epic 03: NOT started (REST API for project/config/metrics)
- Layer 6 Epics 08-10: NOT started
- `make build` compiles frontend (Vite) → copies dist/ → builds Go binary with embed.FS
- `make test` — all packages pass
- Zero TypeScript errors

Layer 6 status map
```
  ✅ Epic 01 — HTTP Server Foundation
  ✅ Epic 02 — REST API: Conversations (6 endpoints)
  ⬚  Epic 03 — REST API: Project, Config & Metrics
  ✅ Epic 04 — WebSocket Handler
  ✅ Epic 05 — Serve Command (composition root)
  ✅ Epic 06 — React Scaffolding (Vite + React + TS + Tailwind + shadcn/ui)
  ✅ Epic 07 — Conversation UI (all 3 slices complete)
  ⬚  Epic 08 — Sidebar & Navigation
  ⬚  Epic 09 — Context Inspector (debug panel)
  ⬚  Epic 10 — Settings & Metrics UI
```

Epic 07 — what was built (complete)
Slice 1 (streaming chat plumbing):
  - useWebSocket hook: connect /api/ws, auto-reconnect with exponential backoff
  - useConversation hook: reducer-based state machine for messages + streaming
  - ConversationPage: message bubbles, streaming text cursor, status indicator,
    error banner, cancel button, auto-scroll
  - ConversationListPage: input bar → navigates to /c/new with initial message
  - URL updates to /c/:id on conversation_created event
  - Enter to send, Shift+Enter for newline, disabled during turn

Slice 2 (thinking + tool call visualization):
  - Block-based content model: ChatMessage.blocks[] with discriminated union
    (ThinkingBlock | ToolCallBlock | TextBlock)
  - Reducer handles thinking_start/delta/end, tool_call_start/output/end events
  - ThinkingBlock component: collapsible, streaming indicator, char count
  - ToolCallCard component: collapsible, tool name, JSON args, streaming output,
    result, duration, success/fail status
  - TurnUsageBadge: tokens in/out, duration, iteration count
  - turn_complete carries usage summary into state

Slice 3 (markdown + syntax highlighting + history + compressed):
  - react-markdown + remark-gfm for rich text rendering
  - react-syntax-highlighter (Prism/oneDark) for fenced code blocks
  - MarkdownContent component: headings, lists, tables, blockquotes, code
  - Load conversation history via REST GET /api/conversations/:id/messages
  - history.ts: messageViewsToChat() converts MessageView[] to ChatMessage[]
  - Compressed/summary message rendering: dashed border, greyed, [compressed] badge
  - System message rendering (centered, italic)

File map (frontend)
  web/src/hooks/use-conversation.ts  — block-based reducer, all event types
  web/src/hooks/use-websocket.ts     — WebSocket connection management
  web/src/pages/conversation.tsx     — main chat page with block rendering
  web/src/pages/conversation-list.tsx — conversation list / home page
  web/src/components/chat/
    thinking-block.tsx               — collapsible thinking section
    tool-call-card.tsx               — collapsible tool call card
    turn-usage-badge.tsx             — token/duration usage pill
    markdown-content.tsx             — markdown + syntax highlighting
  web/src/lib/history.ts             — REST MessageView[] → ChatMessage[]
  web/src/lib/api.ts                 — fetch wrapper
  web/src/types/events.ts            — WS event types
  web/src/types/api.ts               — REST API types

Important notes for next session
- shadcn/ui v4 uses @base-ui/react (NOT Radix). No `asChild` prop on Button
- `erasableSyntaxOnly` in tsconfig — no `public` constructor parameter properties
- .gitignore: `**/node_modules/` covers all nested node_modules (ts-analyzer too)
- .gitignore: `/lib/` and `/include/` root-anchored to avoid matching web/src/lib/
- .gitignore: `/sirtopham` root-anchored to avoid matching cmd/sirtopham/
- react-syntax-highlighter bundle is ~1MB — consider lazy import if bundle size matters

Development workflow
- Two terminals: `make dev-backend` + `make dev-frontend`
- Or production: `make build && ./bin/sirtopham serve --config sirtopham.yaml`

Next steps — recommended order
1. Epic 08: Sidebar & Navigation
   - Fetch conversation list from GET /api/conversations
   - Clickable conversation list in sidebar
   - New conversation button
   - Active conversation highlight
   - Responsive: hamburger menu on mobile

2. Epic 03: REST API for Project/Config/Metrics (independent, can parallel)

3. Epic 09: Context Inspector (debug panel)
   - Renders context_debug events
   - Token budget visualization

4. Epic 10: Settings & Metrics UI

Validation commands
- `git log --oneline -10`
- `make test` (all packages green)
- `make build && ./bin/sirtopham serve --config sirtopham.yaml`
- `cd web && npx tsc --noEmit` (zero TS errors)
- `make dev-frontend` (Vite dev server on :5173)
