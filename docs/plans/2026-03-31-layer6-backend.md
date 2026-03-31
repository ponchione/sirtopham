# Layer 6 Backend Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Build the Go HTTP/WebSocket backend — server foundation, REST API, WebSocket streaming, and the `sirtopham serve` composition root — so the agent is usable via curl/wscat before any frontend exists.

**Architecture:** Thin HTTP adapter layer over existing packages. ConversationManager (Layer 5) handles all DB logic. AgentLoop (Layer 5) handles turn execution and event emission. The server subscribes an EventSink to the agent loop and forwards events over WebSocket. The serve command is the composition root that wires everything.

**Tech Stack:** Go stdlib `net/http` with Go 1.22+ pattern matching (no framework), `nhooyr.io/websocket` for WebSocket, `embed.FS` for static assets, cobra CLI (already in use).

**Test command:** `make test` (NOT `go test ./...` — Makefile has CGo linker flags for lancedb)

---

## Dependency Graph

```
Epic 01 (HTTP Server Foundation)
  ├── Epic 02 (REST API Conversations)  ── parallel
  ├── Epic 04 (WebSocket Handler)       ── parallel
  └── Epic 05 (Serve Command)           ── after 01+02+04
```

---

## Epic 01: HTTP Server Foundation

### Task 1.1: Add WebSocket dependency + Server struct + listener

**Objective:** Create the server package with a Server struct that starts/stops an HTTP listener.

**Files:**
- Modify: `go.mod` (add `nhooyr.io/websocket`)
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Implementation:**

```go
// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server is the HTTP server for the sirtopham web interface.
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	logger     *slog.Logger
	host       string
	port       int
	devMode    bool
}

// Config holds server configuration.
type Config struct {
	Host    string
	Port    int
	DevMode bool
}

// New creates a new Server.
func New(cfg Config, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{
		mux:    mux,
		logger: logger,
		host:   cfg.Host,
		port:   cfg.Port,
		devMode: cfg.DevMode,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:           mux, // middleware wraps this later
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	return s
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

// Start begins listening. Blocks until the server stops or context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr())
	if err != nil {
		return fmt.Errorf("server listen: %w", err)
	}
	s.logger.Info("server listening", "addr", s.Addr(), "dev_mode", s.devMode)

	errCh := make(chan error, 1)
	go func() { errCh <- s.httpServer.Serve(ln) }()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		return s.Shutdown()
	}
}

// Shutdown gracefully shuts down the server with a 10-second deadline.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.logger.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
```

**Test:**

```go
// internal/server/server_test.go
package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/ponchione/sirtopham/internal/server"
)

func TestServerStartAndShutdown(t *testing.T) {
	s := server.New(server.Config{Host: "127.0.0.1", Port: 0}, slog.Default())
	// Port 0 won't work with our Addr() approach, so pick a high port
	s = server.New(server.Config{Host: "127.0.0.1", Port: 18923}, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify it's listening
	resp, err := http.Get("http://127.0.0.1:18923/")
	if err != nil {
		t.Fatalf("server not reachable: %v", err)
	}
	resp.Body.Close()

	// Shutdown
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Steps:**
1. `go get nhooyr.io/websocket`
2. Create `internal/server/server.go` and `internal/server/server_test.go`
3. Run `make test` — verify pass
4. `git add -A && git commit -m "feat(server): add Server struct with start/shutdown (L6E01)"`

---

### Task 1.2: Router setup, health check, and route registration

**Objective:** Add health check endpoint and a method for external packages to register routes.

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Implementation additions to server.go:**

```go
// HandleFunc registers a handler on the server's mux.
// Pattern uses Go 1.22+ syntax: "GET /api/foo", "POST /api/bar/{id}".
func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Handle registers an http.Handler on the server's mux.
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// registerCoreRoutes sets up routes that are always present.
func (s *Server) registerCoreRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
```

Call `s.registerCoreRoutes()` at the end of `New()`.

**Tests:** Add `TestHealthEndpoint` that starts server, GETs `/api/health`, asserts 200 + `{"status":"ok"}`.

**Steps:**
1. Update `server.go` with HandleFunc/Handle/registerCoreRoutes/handleHealth
2. Add health check test
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add health check and route registration (L6E01)"`

---

### Task 1.3: Middleware chain (logging, panic recovery, CORS)

**Objective:** Wrap the mux with request logging, panic recovery, and dev-mode CORS.

**Files:**
- Create: `internal/server/middleware.go`
- Create: `internal/server/middleware_test.go`
- Modify: `internal/server/server.go` (apply middleware in New)

**Implementation:**

```go
// internal/server/middleware.go
package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// requestLogger logs method, path, status, and duration.
func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// panicRecovery catches panics in handlers and returns 500.
func panicRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered", "error", fmt.Sprintf("%v", err), "path", r.URL.Path)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors adds permissive CORS headers for dev mode (Vite on localhost:5173).
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

In `server.go`, update `New()` to build the middleware chain:

```go
handler := http.Handler(mux)
handler = requestLogger(logger, handler)
handler = panicRecovery(logger, handler)
if cfg.DevMode {
    handler = cors(handler)
}
s.httpServer.Handler = handler
```

**Tests:**
- `TestPanicRecovery`: handler panics → returns 500
- `TestCORSDevMode`: dev mode → CORS headers present
- `TestCORSProdMode`: prod mode → no CORS headers

**Steps:**
1. Create middleware.go and middleware_test.go
2. Update server.go to apply middleware chain
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add logging, panic recovery, CORS middleware (L6E01)"`

---

### Task 1.4: embed.FS static serving with SPA fallback

**Objective:** Serve compiled frontend from `embed.FS` in prod mode, with SPA fallback to index.html for client-side routing.

**Files:**
- Create: `internal/server/static.go`
- Create: `internal/server/static_test.go`
- Modify: `internal/server/server.go` (register static handler)

**Implementation:**

```go
// internal/server/static.go
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

// staticHandler serves embedded frontend files with SPA fallback.
// Non-API requests that don't match a static file get index.html.
func staticHandler(logger *slog.Logger, frontendFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(frontendFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve static for API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the exact file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Strip leading slash for fs.Open
		filePath := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(frontendFS, filePath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for unknown paths
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

In `server.go`, accept an optional `fs.FS` parameter:

```go
type Config struct {
	Host       string
	Port       int
	DevMode    bool
	FrontendFS fs.FS // nil if no embedded frontend (dev mode)
}
```

Register static handler at the end of `registerCoreRoutes()` if `FrontendFS` is non-nil:

```go
if s.frontendFS != nil {
    s.mux.Handle("/", staticHandler(s.logger, s.frontendFS))
}
```

**Tests:** Use `fstest.MapFS` to create an in-memory FS with index.html, test:
- `/` → serves index.html
- `/style.css` → serves the CSS file
- `/some/route` → SPA fallback → index.html
- `/api/health` → NOT intercepted by static handler

**Steps:**
1. Create static.go and static_test.go
2. Update server.go Config struct and registerCoreRoutes
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add embed.FS static serving with SPA fallback (L6E01)"`

---

## Epic 02: REST API — Conversations

### Task 2.1: JSON helpers and API error handling

**Objective:** Create shared helpers for JSON response writing and error responses used by all handlers.

**Files:**
- Create: `internal/server/api.go`
- Create: `internal/server/api_test.go`

**Implementation:**

```go
// internal/server/api.go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON decodes the request body into v. Returns false + writes 400 on error.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, logger *slog.Logger) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		logger.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
```

**Tests:** Test writeJSON output, writeError format, decodeJSON with valid/invalid bodies.

**Steps:**
1. Create api.go and api_test.go
2. Run `make test`
3. `git add -A && git commit -m "feat(server): add JSON helpers and error handling (L6E02)"`

---

### Task 2.2: Conversation handler struct + list/create endpoints

**Objective:** Create the conversation handler with `GET /api/conversations` and `POST /api/conversations`.

**Files:**
- Create: `internal/server/conversations.go`
- Create: `internal/server/conversations_test.go`

**Implementation:**

The handler depends on an interface (not the concrete Manager) so it's testable:

```go
// internal/server/conversations.go
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ponchione/sirtopham/internal/conversation"
)

// ConversationService is the interface the conversation handlers need.
// Satisfied by *conversation.Manager.
type ConversationService interface {
	Create(ctx context.Context, projectID string, opts ...conversation.CreateOption) (*conversation.Conversation, error)
	Get(ctx context.Context, conversationID string) (*conversation.Conversation, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]conversation.ConversationSummary, error)
	Delete(ctx context.Context, conversationID string) error
}

// ConversationHandler handles conversation REST endpoints.
type ConversationHandler struct {
	service   ConversationService
	projectID string
	logger    *slog.Logger
}

// NewConversationHandler creates a new handler and registers routes on the server.
func NewConversationHandler(s *Server, svc ConversationService, projectID string, logger *slog.Logger) *ConversationHandler {
	h := &ConversationHandler{service: svc, projectID: projectID, logger: logger}
	s.HandleFunc("GET /api/conversations", h.handleList)
	s.HandleFunc("POST /api/conversations", h.handleCreate)
	s.HandleFunc("GET /api/conversations/{id}", h.handleGet)
	s.HandleFunc("DELETE /api/conversations/{id}", h.handleDelete)
	return h
}

func (h *ConversationHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	convos, err := h.service.List(r.Context(), h.projectID, limit, offset)
	if err != nil {
		h.logger.Error("list conversations", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}
	writeJSON(w, http.StatusOK, convos)
}

func (h *ConversationHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &req, h.logger) {
			return
		}
	}

	var opts []conversation.CreateOption
	if req.Title != "" {
		opts = append(opts, conversation.WithTitle(req.Title))
	}
	if req.Model != "" {
		opts = append(opts, conversation.WithModel(req.Model))
	}
	if req.Provider != "" {
		opts = append(opts, conversation.WithProvider(req.Provider))
	}

	c, err := h.service.Create(r.Context(), h.projectID, opts...)
	if err != nil {
		h.logger.Error("create conversation", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create conversation")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}
```

**Tests:** Use a mock ConversationService. Test:
- GET /api/conversations → 200 + JSON array
- GET /api/conversations?limit=5&offset=10 → passes params correctly
- POST /api/conversations with body → 201 + created conversation
- POST /api/conversations with empty body → 201 (no options)

**Steps:**
1. Create conversations.go and conversations_test.go
2. Run `make test`
3. `git add -A && git commit -m "feat(server): add list and create conversation endpoints (L6E02)"`

---

### Task 2.3: Get conversation + delete endpoints

**Objective:** Add `GET /api/conversations/{id}` and `DELETE /api/conversations/{id}`.

**Files:**
- Modify: `internal/server/conversations.go`
- Modify: `internal/server/conversations_test.go`

**Implementation:**

```go
func (h *ConversationHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.service.Get(r.Context(), id)
	if err != nil {
		// Check if not found (conversation.Manager returns a specific error)
		h.logger.Error("get conversation", "error", err, "id", id)
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ConversationHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.Delete(r.Context(), id); err != nil {
		h.logger.Error("delete conversation", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to delete conversation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Tests:**
- GET /api/conversations/{id} → 200 with conversation
- GET /api/conversations/{nonexistent} → 404
- DELETE /api/conversations/{id} → 204

**Steps:**
1. Add handleGet and handleDelete implementations
2. Add tests
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add get and delete conversation endpoints (L6E02)"`

---

### Task 2.4: Messages endpoint

**Objective:** Add `GET /api/conversations/{id}/messages` returning all messages in sequence order.

**Files:**
- Modify: `internal/server/conversations.go` (add ConversationService.GetMessages to interface + handler)
- Modify: `internal/server/conversations_test.go`

**Implementation:**

The interface needs a GetMessages method. Check what conversation.Manager exposes — it has HistoryManager.ReconstructHistory. We need to check if there's a simpler "get all messages" method or if we need to add a thin wrapper. The handler:

```go
// Add to ConversationService interface:
// GetMessages(ctx context.Context, conversationID string) ([]Message, error)
// The exact type depends on what conversation.Manager exposes.

func (h *ConversationHandler) handleMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msgs, err := h.service.GetMessages(r.Context(), id)
	if err != nil {
		h.logger.Error("get messages", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}
```

Register: `s.HandleFunc("GET /api/conversations/{id}/messages", h.handleMessages)`

**Note to implementer:** Check `internal/conversation/manager.go` and the sqlc queries for a `ListMessages` or `GetMessagesByConversation` query. If one doesn't exist, add a sqlc query first. The messages should include `is_compressed` and `is_summary` flags per the spec.

**Steps:**
1. Check existing message retrieval methods, add sqlc query if needed
2. Add GetMessages to ConversationService interface
3. Add handleMessages
4. Add tests
5. Run `make test`
6. `git add -A && git commit -m "feat(server): add messages endpoint (L6E02)"`

---

### Task 2.5: FTS5 search endpoint

**Objective:** Add `GET /api/conversations/search?q=<query>` for full-text search.

**Files:**
- Modify: `internal/server/conversations.go`
- Modify: `internal/server/conversations_test.go`

**Implementation:**

```go
// Add to ConversationService interface (or a separate SearchService):
// Search(ctx context.Context, projectID, query string) ([]SearchResult, error)

func (h *ConversationHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	results, err := h.service.Search(r.Context(), h.projectID, q)
	if err != nil {
		h.logger.Error("search conversations", "error", err, "query", q)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, results)
}
```

**IMPORTANT:** Register this BEFORE the `{id}` route so `/api/conversations/search` doesn't get captured by `GET /api/conversations/{id}`. Go 1.22 mux handles this correctly since `search` is a literal match vs `{id}` wildcard.

Register: `s.HandleFunc("GET /api/conversations/search", h.handleSearch)`

**Note to implementer:** Check if `conversation.Manager` already has a `Search` method using FTS5. The sqlc query from spec uses `messages_fts MATCH ?` with `snippet()`. If not present, add the sqlc query.

**Steps:**
1. Check/add Search method on conversation.Manager
2. Add handleSearch
3. Add test with mock
4. Run `make test`
5. `git add -A && git commit -m "feat(server): add FTS5 search endpoint (L6E02)"`

---

## Epic 04: WebSocket Handler

### Task 4.1: WebSocket upgrade + read/write goroutines

**Objective:** Accept WebSocket connections at `/api/ws`, upgrade, and set up read/write goroutine structure.

**Files:**
- Create: `internal/server/websocket.go`
- Create: `internal/server/websocket_test.go`

**Implementation:**

```go
// internal/server/websocket.go
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/ponchione/sirtopham/internal/agent"
)

// AgentService is the interface the WebSocket handler needs.
type AgentService interface {
	RunTurn(ctx context.Context, req agent.RunTurnRequest) (*agent.TurnResult, error)
	Subscribe(sink agent.EventSink)
	Unsubscribe(sink agent.EventSink)
	Cancel()
}

// WebSocketHandler handles WebSocket connections for streaming agent events.
type WebSocketHandler struct {
	agent     AgentService
	convSvc   ConversationService
	projectID string
	logger    *slog.Logger
}

// NewWebSocketHandler creates a handler and registers the WS route.
func NewWebSocketHandler(s *Server, agentSvc AgentService, convSvc ConversationService, projectID string, logger *slog.Logger) *WebSocketHandler {
	h := &WebSocketHandler{
		agent:     agentSvc,
		convSvc:   convSvc,
		projectID: projectID,
		logger:    logger,
	}
	s.HandleFunc("/api/ws", h.handleWS)
	return h
}

func (h *WebSocketHandler) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// In dev mode, Vite dev server connects from different origin
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logger.Error("websocket accept failed", "error", err)
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Channel for events from agent → client
	eventCh := make(chan agent.Event, 64)
	sink := &channelSink{ch: eventCh}

	// Read loop (client → server)
	go h.readLoop(ctx, cancel, conn, sink)

	// Write loop (server → client) + heartbeat
	h.writeLoop(ctx, conn, eventCh)
}
```

**Steps:**
1. Create websocket.go with upgrade logic and stub read/write loops
2. Create websocket_test.go with basic upgrade test using httptest
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add WebSocket upgrade and connection handler (L6E04)"`

---

### Task 4.2: ChannelSink + event write loop + heartbeat

**Objective:** Implement the ChannelSink (EventSink → channel bridge) and the write loop that forwards events as JSON to the WebSocket client. Add 30s ping/pong heartbeat.

**Files:**
- Modify: `internal/server/websocket.go`

**Implementation:**

```go
// channelSink bridges agent.EventSink to a channel for the write loop.
type channelSink struct {
	ch     chan agent.Event
	closed bool
	mu     sync.Mutex
}

func (s *channelSink) Emit(e agent.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- e:
	default:
		// Drop event if channel full (slow client)
	}
}

func (s *channelSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	close(s.ch)
}

// writeLoop sends events and heartbeats to the WebSocket client.
func (h *WebSocketHandler) writeLoop(ctx context.Context, conn *websocket.Conn, eventCh <-chan agent.Event) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-eventCh:
			if !ok {
				return
			}
			// Wrap event with type field for client dispatch
			msg := map[string]any{
				"type":      evt.EventType(),
				"timestamp": evt.Timestamp(),
				"data":      evt,
			}
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				h.logger.Debug("websocket write error", "error", err)
				return
			}
		case <-ticker.C:
			if err := conn.Ping(ctx); err != nil {
				h.logger.Debug("websocket ping failed", "error", err)
				return
			}
		}
	}
}
```

**Steps:**
1. Add channelSink and writeLoop
2. Add test for channelSink behavior (emit, close, drop on full)
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add ChannelSink and event write loop (L6E04)"`

---

### Task 4.3: Read loop — message, cancel, model_override

**Objective:** Handle client→server messages: `message` (triggers RunTurn), `cancel` (cancels current turn), `model_override` (changes model for next turn).

**Files:**
- Modify: `internal/server/websocket.go`

**Implementation:**

```go
// ClientMessage represents a message from the WebSocket client.
type ClientMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
	Model          string `json:"model,omitempty"`
	Provider       string `json:"provider,omitempty"`
}

func (h *WebSocketHandler) readLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, sink *channelSink) {
	defer cancel()
	var turnActive sync.Mutex

	for {
		var msg ClientMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			h.logger.Debug("websocket read error", "error", err)
			return
		}

		switch msg.Type {
		case "message":
			// One turn at a time
			if !turnActive.TryLock() {
				wsjson.Write(ctx, conn, map[string]string{
					"type":  "error",
					"error": "a turn is already in progress",
				})
				continue
			}

			go func() {
				defer turnActive.Unlock()
				h.handleMessage(ctx, conn, sink, msg)
			}()

		case "cancel":
			h.agent.Cancel()

		default:
			h.logger.Warn("unknown client message type", "type", msg.Type)
		}
	}
}

func (h *WebSocketHandler) handleMessage(ctx context.Context, conn *websocket.Conn, sink *channelSink, msg ClientMessage) {
	// Subscribe sink to receive events
	h.agent.Subscribe(sink)
	defer h.agent.Unsubscribe(sink)

	// Create conversation if needed, or use existing
	convID := msg.ConversationID
	if convID == "" {
		c, err := h.convSvc.Create(ctx, h.projectID)
		if err != nil {
			h.logger.Error("create conversation", "error", err)
			wsjson.Write(ctx, conn, map[string]string{"type": "error", "error": "failed to create conversation"})
			return
		}
		convID = c.ID
		// Tell client the new conversation ID
		wsjson.Write(ctx, conn, map[string]any{"type": "conversation_created", "conversation_id": convID})
	}

	req := agent.RunTurnRequest{
		ConversationID: convID,
		Message:        msg.Content,
	}

	_, err := h.agent.RunTurn(ctx, req)
	if err != nil {
		h.logger.Error("run turn", "error", err, "conversation_id", convID)
		// Error events are emitted by the agent loop itself
	}
}
```

**Steps:**
1. Add ClientMessage, readLoop, handleMessage
2. Add tests with mock AgentService
3. Run `make test`
4. `git add -A && git commit -m "feat(server): add WebSocket read loop with message/cancel handling (L6E04)"`

---

## Epic 05: Serve Command (Composition Root)

### Task 5.1: Wire serve command — init sequence

**Objective:** Replace the stub `serve` command with the full init sequence that wires all layers.

**Files:**
- Modify: `cmd/sirtopham/main.go`
- May create: `cmd/sirtopham/serve.go` (if cleaner to separate)

**Implementation outline:**

```go
// Init sequence inside serveCmd.RunE:
// 1. Load config
cfg, err := appconfig.Load()

// 2. Set up logger
logger := slog.New(...)

// 3. Open + init DB
db, err := sql.Open("sqlite3", dbPath)
appdb.Init(ctx, db)
queries := db.New(db)

// 4. Build provider router
router := router.New(cfg.Routing, cfg.Providers, logger)

// 5. Build tool registry + executor
registry := tool.NewRegistry()
// register all tools...
executor := tool.NewExecutor(registry, logger)

// 6. Build conversation manager
convManager := conversation.NewManager(queries, logger)

// 7. Build agent loop
agentLoop := agent.NewAgentLoop(agent.AgentLoopDeps{
    ConversationManager: convManager,
    ProviderRouter:      router,
    ToolExecutor:        tool.NewAgentLoopAdapter(executor),
    // ...
})

// 8. Build HTTP server
srv := server.New(server.Config{
    Host:    cfg.Server.Host,
    Port:    cfg.Server.Port,
    DevMode: cfg.Server.DevMode,
})

// 9. Register handlers
server.NewConversationHandler(srv, convManager, projectID, logger)
server.NewWebSocketHandler(srv, agentLoop, convManager, projectID, logger)

// 10. Start server
srv.Start(ctx)
```

**Note to implementer:** Check the exact constructor signatures for each component. The config fields, dependency injection, and interface satisfaction all need to match. This is the integration point where mismatches surface.

**Steps:**
1. Implement serve command init sequence
2. Verify `make build` succeeds (compilation check)
3. `git add -A && git commit -m "feat(cmd): wire serve command with full init sequence (L6E05)"`

---

### Task 5.2: Graceful shutdown + signal handling

**Objective:** Handle SIGINT/SIGTERM for ordered teardown: HTTP drain → agent cancel → DB close.

**Files:**
- Modify: `cmd/sirtopham/serve.go` (or main.go)

**Implementation:**

```go
// Signal handling
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Start server (blocks until ctx cancelled)
if err := srv.Start(ctx); err != nil {
    logger.Error("server error", "error", err)
}

// Ordered teardown
agentLoop.Cancel()
agentLoop.Close()
db.Close()
logger.Info("shutdown complete")
```

**Steps:**
1. Add signal handling and ordered shutdown
2. Manual test: start server, Ctrl-C, verify clean shutdown in logs
3. `git add -A && git commit -m "feat(cmd): add graceful shutdown with signal handling (L6E05)"`

---

### Task 5.3: Startup logging + browser launch

**Objective:** Log startup info (providers, project root, listen address) and optionally open browser.

**Files:**
- Modify: `cmd/sirtopham/serve.go`

**Implementation:**

```go
// Startup logging
logger.Info("sirtopham starting",
    "version", version,
    "project", cfg.ProjectRoot,
    "listen", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
    "dev_mode", cfg.Server.DevMode,
)

// Browser launch (non-blocking, best-effort)
if cfg.Server.OpenBrowser && !cfg.Server.DevMode {
    url := fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
    go func() {
        time.Sleep(500 * time.Millisecond) // let server start
        exec.Command("xdg-open", url).Start() // linux
    }()
}
```

**Steps:**
1. Add startup logging and browser launch
2. Run `make build` to verify compilation
3. `git add -A && git commit -m "feat(cmd): add startup logging and browser launch (L6E05)"`

---

### Task 5.4: Flag overrides (--port, --host, --dev)

**Objective:** Add cobra flags that override config values.

**Files:**
- Modify: `cmd/sirtopham/serve.go`

**Implementation:**

```go
serveCmd.Flags().IntVar(&portOverride, "port", 0, "override server port")
serveCmd.Flags().StringVar(&hostOverride, "host", "", "override server host")
serveCmd.Flags().BoolVar(&devOverride, "dev", false, "enable dev mode")

// In RunE, after loading config:
if portOverride > 0 {
    cfg.Server.Port = portOverride
}
if hostOverride != "" {
    cfg.Server.Host = hostOverride
}
if devOverride {
    cfg.Server.DevMode = true
}
```

**Steps:**
1. Add flag definitions and override logic
2. Run `make test` for final full-suite pass
3. `git add -A && git commit -m "feat(cmd): add --port, --host, --dev flag overrides (L6E05)"`

---

## Bonus: Agent Loop Batch Dispatch Refactor

### Task B.1: Refactor agent loop to use batch dispatch

**Objective:** Replace the single-call ToolExecutor interface with batch dispatch, eliminating the AgentLoopAdapter.

**Files:**
- Modify: `internal/agent/loop.go` (change ToolExecutor interface to batch)
- Modify: `internal/agent/loop_test.go` (update mocks)
- Delete: `internal/tool/adapter.go`
- Delete: `internal/tool/adapter_test.go`

**Current state:**
```go
// Current single-call interface
type ToolExecutor interface {
    Execute(ctx context.Context, call provider.ToolCall) (*provider.ToolResult, error)
}
```

**Target:**
```go
// Batch interface matching tool.Executor.Execute signature
type ToolExecutor interface {
    Execute(ctx context.Context, calls []provider.ToolCall) ([]*provider.ToolResult, error)
}
```

Then update the iteration loop to pass all tool calls at once instead of ranging over them one by one.

**Note:** This is independent of Layer 6 and can be done anytime. The existing tool.Executor already supports batch with purity-based concurrency. This just removes the adapter shim.

**Steps:**
1. Update ToolExecutor interface to batch signature
2. Update iteration loop to pass full slice
3. Update all tests (mocks)
4. Delete adapter.go and adapter_test.go
5. Update serve command (pass executor directly, no adapter wrapping)
6. Run `make test`
7. `git add -A && git commit -m "refactor(agent): use native batch tool dispatch, remove adapter"`

---

## Summary

| Task  | Epic | Description                                   | Est. |
|-------|------|-----------------------------------------------|------|
| 1.1   | 01   | Server struct + listener                      | S    |
| 1.2   | 01   | Health check + route registration             | S    |
| 1.3   | 01   | Middleware (logging, panic, CORS)             | S    |
| 1.4   | 01   | embed.FS + SPA fallback                       | M    |
| 2.1   | 02   | JSON helpers + error handling                 | S    |
| 2.2   | 02   | List + create conversation endpoints          | M    |
| 2.3   | 02   | Get + delete endpoints                        | S    |
| 2.4   | 02   | Messages endpoint                             | M    |
| 2.5   | 02   | FTS5 search endpoint                          | M    |
| 4.1   | 04   | WebSocket upgrade + goroutine structure       | M    |
| 4.2   | 04   | ChannelSink + write loop + heartbeat          | M    |
| 4.3   | 04   | Read loop (message/cancel handling)           | M    |
| 5.1   | 05   | Serve command init sequence                   | L    |
| 5.2   | 05   | Graceful shutdown + signals                   | S    |
| 5.3   | 05   | Startup logging + browser launch              | S    |
| 5.4   | 05   | Flag overrides                                | S    |
| B.1   | —    | Batch dispatch refactor                       | M    |
