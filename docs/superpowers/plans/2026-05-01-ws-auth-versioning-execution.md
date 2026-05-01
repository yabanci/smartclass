# WebSocket Auth + Contract Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `?access_token=` JWT query auth with single-use 60s tickets, tighten WS CheckOrigin to the CORS allow-list, add `version: 1` to every realtime event.

**Architecture:** New `TicketStore` interface in `internal/realtime/ws/ticket.go` (in-memory `sync.Map` + `atomic.Bool` for once-only consumption). New `POST /api/v1/ws/ticket` endpoint authenticated via existing Bearer middleware. WS handler reads `?ticket=<x>` (not `?access_token=`), consumes via store, sets principal in context. CheckOrigin reads CORS allow-list. `realtime.Event` gains a `Version int` field; all 6 emit sites set `Version: 1`.

**Tech Stack:** Go 1.25 · chi v5.2.5 · gorilla/websocket · Flutter 3.41.7 · Riverpod · Dio.

**Source spec:** `docs/superpowers/specs/2026-05-01-ws-auth-versioning-design.md`

---

## File map

```
backend/
├── internal/realtime/ws/
│   ├── ticket.go                  NEW — TicketStore + MemTicketStore + Cleanup loop
│   ├── ticket_test.go             NEW — 5 unit tests
│   ├── ticket_handler.go          NEW — POST /ws/ticket handler
│   ├── ticket_handler_test.go     NEW — 3 endpoint tests
│   ├── handler.go                 modify — read ?ticket= instead of bearer; CheckOrigin from CORS
│   └── handler_test.go            extend — 4 handshake tests + 3 CheckOrigin tests
├── internal/realtime/
│   ├── broker.go                  modify — Event gains Version int
│   └── event_test.go              NEW — 1 round-trip test
├── internal/platform/httpx/
│   ├── errors.go                  modify — new domain errors ErrWSTicketRequired/Invalid
│   └── middleware/auth.go         modify — extractToken drops query fallback
├── internal/server/server.go      modify — register POST /ws/ticket; pass CORS origins to ws.NewHandler
└── cmd/server/main.go             modify — construct MemTicketStore, pass to handlers

Six service files that emit realtime.Event:
  internal/notification/service.go    add Version: 1 to literal
  internal/sensor/service.go          add Version: 1
  internal/device/service.go          add Version: 1
  internal/scene/service.go           add Version: 1
  internal/realtime/ws/hub_test.go    add Version: 1 to test fixtures
  (no others — confirmed via grep "realtime.Event{")

mobile/
├── lib/core/api/endpoints/ws_endpoints.dart   NEW — createTicket()
└── lib/shared/providers/ws_provider.dart      modify — fetch ticket, build URL with ?ticket=
```

**Per-task discipline.** Each task is its own commit. Run `go test -race ./<changed-package>/...` after each. Final task runs full regression.

**Working directory.** `backend/`-prefixed paths relative to `/Users/arsenozhetov/Projects/pet/smartclass`. Run go commands from `backend/`.

---

## Task 1: TicketStore interface + in-memory implementation

**Files:**
- Create: `backend/internal/realtime/ws/ticket.go`
- Create: `backend/internal/realtime/ws/ticket_test.go`

- [ ] **Step 1: Write failing tests**

Create `backend/internal/realtime/ws/ticket_test.go`:

```go
package ws

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemTicketStore_Issue_ReturnsRandomBase64(t *testing.T) {
	s := NewMemTicketStore(60 * time.Second)
	uid := uuid.New()

	t1, err := s.Issue(context.Background(), uid)
	require.NoError(t, err)
	t2, err := s.Issue(context.Background(), uid)
	require.NoError(t, err)

	assert.NotEqual(t, t1.Raw, t2.Raw, "two consecutive Issue calls must produce distinct tickets")
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]{32,}$`), t1.Raw,
		"ticket must be URL-safe base64 (no padding) so it round-trips through query strings safely")
	assert.WithinDuration(t, time.Now().Add(60*time.Second), t1.ExpiresAt, time.Second,
		"expires-at must be ~now+TTL")
	assert.Equal(t, uid, t1.UserID)
}

func TestMemTicketStore_Consume_OnceOnly_SecondCallFails(t *testing.T) {
	s := NewMemTicketStore(60 * time.Second)
	uid := uuid.New()

	tkt, err := s.Issue(context.Background(), uid)
	require.NoError(t, err)

	gotUID, err := s.Consume(context.Background(), tkt.Raw)
	require.NoError(t, err)
	assert.Equal(t, uid, gotUID)

	_, err = s.Consume(context.Background(), tkt.Raw)
	assert.ErrorIs(t, err, ErrTicketUnknown,
		"a ticket already consumed must report ErrTicketUnknown — that's the once-only guarantee")
}

func TestMemTicketStore_Consume_ExpiredFails(t *testing.T) {
	s := NewMemTicketStore(60 * time.Second)
	uid := uuid.New()

	tkt, err := s.Issue(context.Background(), uid)
	require.NoError(t, err)

	// Force expiry by reaching into the implementation. We deliberately
	// don't add a clock-injection seam to the public API — tests own the
	// time-travel knob via the unexported field.
	s.entries.Range(func(_, v any) bool {
		v.(*ticketEntry).expiresAt = time.Now().Add(-time.Second)
		return false
	})

	_, err = s.Consume(context.Background(), tkt.Raw)
	assert.ErrorIs(t, err, ErrTicketUnknown,
		"a ticket past its expiresAt must report ErrTicketUnknown — TTL is enforced even before Cleanup runs")
}

func TestMemTicketStore_Consume_UnknownTicket_Fails(t *testing.T) {
	s := NewMemTicketStore(60 * time.Second)

	_, err := s.Consume(context.Background(), "never-issued")
	assert.True(t, errors.Is(err, ErrTicketUnknown),
		"a random ticket string never issued must report ErrTicketUnknown — never crash, never leak")
}

func TestMemTicketStore_Cleanup_PrunesExpired(t *testing.T) {
	s := NewMemTicketStore(60 * time.Second)

	for i := 0; i < 100; i++ {
		_, err := s.Issue(context.Background(), uuid.New())
		require.NoError(t, err)
	}

	// Force every entry past expiry.
	s.entries.Range(func(_, v any) bool {
		v.(*ticketEntry).expiresAt = time.Now().Add(-time.Second)
		return true
	})

	s.cleanup()

	count := 0
	s.entries.Range(func(_, _ any) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count,
		"cleanup must remove every expired entry — without bounded growth the map leaks under churn")
}
```

- [ ] **Step 2: Run failing tests**

```bash
cd backend && go test ./internal/realtime/ws/... -run TestMemTicketStore_
```

Expected: FAIL — undefined types.

- [ ] **Step 3: Implement ticket.go**

Create `backend/internal/realtime/ws/ticket.go`:

```go
package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Ticket is what TicketStore.Issue hands back to the caller — `Raw` is the
// string the WS client puts in `?ticket=`, the rest is metadata used by the
// store and surfaced for observability/logs.
type Ticket struct {
	Raw       string
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// TicketStore mints and consumes single-use WebSocket upgrade tickets.
// Tickets exist because we don't want JWTs in URL query strings (they end up
// in reverse-proxy access logs). The mobile client calls Issue with a Bearer
// JWT, then immediately uses the returned ticket on the WS upgrade. The
// server validates with Consume — once-only, so a ticket leaked into a log
// or shoulder-surfed off the URL bar is useless.
type TicketStore interface {
	Issue(ctx context.Context, userID uuid.UUID) (Ticket, error)
	Consume(ctx context.Context, raw string) (uuid.UUID, error)
}

// ErrTicketUnknown collapses three failure modes into one: ticket never
// issued, expired, or already used. Callers map all three to a single 401
// rather than disclose which condition matched (so an attacker can't probe).
var ErrTicketUnknown = errors.New("ws: ticket unknown or already used")

// MemTicketStore is the single-process implementation. A sync.Map keyed on
// the raw ticket string holds entries with userID, expiry, and an atomic.Bool
// that the Consume CAS uses to enforce once-only redemption.
type MemTicketStore struct {
	ttl     time.Duration
	entries sync.Map // map[string]*ticketEntry
}

type ticketEntry struct {
	userID    uuid.UUID
	expiresAt time.Time
	used      atomic.Bool
}

// NewMemTicketStore returns a store with the given TTL. Callers should also
// call Run on it from a goroutine so expired entries get pruned.
func NewMemTicketStore(ttl time.Duration) *MemTicketStore {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &MemTicketStore{ttl: ttl}
}

// Issue generates a fresh random ticket. 24 random bytes encode to 32 URL-safe
// base64 chars without padding — small enough for a query string, large
// enough to resist guessing.
func (s *MemTicketStore) Issue(_ context.Context, userID uuid.UUID) (Ticket, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return Ticket{}, fmt.Errorf("ws: ticket entropy: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf[:])
	expiresAt := time.Now().Add(s.ttl)
	s.entries.Store(raw, &ticketEntry{userID: userID, expiresAt: expiresAt})
	return Ticket{Raw: raw, UserID: userID, ExpiresAt: expiresAt}, nil
}

// Consume marks a ticket used and returns the userID. Returns ErrTicketUnknown
// for unknown / expired / already-used tickets — single error path so the
// caller's response is identical regardless of which case matched.
func (s *MemTicketStore) Consume(_ context.Context, raw string) (uuid.UUID, error) {
	v, ok := s.entries.Load(raw)
	if !ok {
		return uuid.Nil, ErrTicketUnknown
	}
	entry := v.(*ticketEntry)
	if time.Now().After(entry.expiresAt) {
		s.entries.Delete(raw)
		return uuid.Nil, ErrTicketUnknown
	}
	if !entry.used.CompareAndSwap(false, true) {
		return uuid.Nil, ErrTicketUnknown
	}
	// Drop from the map immediately — once-used is forever-used; keeping
	// it around just wastes memory until cleanup.
	s.entries.Delete(raw)
	return entry.userID, nil
}

// Run starts a background goroutine that prunes expired entries on `interval`.
// Returns a stop function the caller invokes on shutdown.
func (s *MemTicketStore) Run(interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = time.Minute
	}
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
	return func() { close(done) }
}

func (s *MemTicketStore) cleanup() {
	now := time.Now()
	s.entries.Range(func(k, v any) bool {
		if now.After(v.(*ticketEntry).expiresAt) {
			s.entries.Delete(k)
		}
		return true
	})
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/realtime/ws/... -run TestMemTicketStore_
```

Expected: PASS, all 5 cases.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/realtime/ws/ticket.go backend/internal/realtime/ws/ticket_test.go
git -c commit.gpgsign=false commit -m "feat(ws): TicketStore interface + in-memory once-only impl (T1)"
```

---

## Task 2: Ticket-issue endpoint + new domain errors

**Files:**
- Modify: `backend/internal/platform/httpx/errors.go` (add 2 new domain errors)
- Create: `backend/internal/realtime/ws/ticket_handler.go`
- Create: `backend/internal/realtime/ws/ticket_handler_test.go`

- [ ] **Step 1: Add domain errors**

Open `backend/internal/platform/httpx/errors.go`. Find the var block with `ErrUnauthorized`, `ErrForbidden`, etc. Add two new entries:

```go
var (
	ErrUnauthorized = NewDomainError("unauthorized", http.StatusUnauthorized, "unauthorized")
	ErrForbidden    = NewDomainError("forbidden", http.StatusForbidden, "forbidden")
	ErrNotFound     = NewDomainError("not_found", http.StatusNotFound, "not_found")
	ErrBadRequest   = NewDomainError("bad_request", http.StatusBadRequest, "bad_request")
	// New WS-specific errors. ErrWSTicketRequired separates "client forgot
	// the ticket" from "ticket invalid" so dashboards can tell the two apart;
	// the response body code differs even though both are 401.
	ErrWSTicketRequired = NewDomainError("ws_ticket_required", http.StatusUnauthorized, "ws.ticket_required")
	ErrWSTicketInvalid  = NewDomainError("ws_ticket_invalid", http.StatusUnauthorized, "ws.ticket_invalid")
)
```

If the existing block is structured differently (e.g. const-style errors), match its style.

- [ ] **Step 2: Write failing test**

Create `backend/internal/realtime/ws/ticket_handler_test.go`:

```go
package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mw "smartclass/internal/platform/httpx/middleware"
)

func TestTicketHandler_ValidPrincipal_200(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := NewTicketHandler(store)

	uid := uuid.New()
	r := httptest.NewRequest(http.MethodPost, "/ws/ticket", nil)
	r = r.WithContext(mw.WithPrincipalForTest(r.Context(),
		mw.Principal{UserID: uid, Role: "teacher"}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data struct {
			Ticket    string    `json:"ticket"`
			ExpiresAt time.Time `json:"expiresAt"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.Ticket, "response must include the ticket string")
	assert.WithinDuration(t, time.Now().Add(60*time.Second), resp.Data.ExpiresAt, 2*time.Second,
		"expiresAt must be ~now+TTL so the client knows when to refetch")
}

func TestTicketHandler_NoPrincipal_401(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := NewTicketHandler(store)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ws/ticket", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"the route mounts inside the authenticated chi.Group, but if it's hit without principal — defense-in-depth — return 401")
}
```

- [ ] **Step 3: Run failing**

```bash
cd backend && go test ./internal/realtime/ws/... -run TestTicketHandler_
```

Expected: FAIL — `NewTicketHandler` undefined.

- [ ] **Step 4: Implement ticket_handler.go**

Create `backend/internal/realtime/ws/ticket_handler.go`:

```go
package ws

import (
	"net/http"
	"time"

	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
)

// TicketHandler issues short-lived tickets the client uses to authenticate
// the next WebSocket upgrade. The route is mounted inside the authenticated
// chi.Group, so the principal is already validated; we just record (userID,
// expiry) into the store and hand the random ticket string back.
type TicketHandler struct {
	store TicketStore
}

func NewTicketHandler(store TicketStore) *TicketHandler {
	return &TicketHandler{store: store}
}

func (h *TicketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, nil, httpx.ErrUnauthorized)
		return
	}
	tkt, err := h.store.Issue(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, r, nil, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ticketResponse{
		Ticket:    tkt.Raw,
		ExpiresAt: tkt.ExpiresAt.UTC(),
	})
}

type ticketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test ./internal/realtime/ws/... -run TestTicketHandler_
```

Expected: PASS for both cases.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/realtime/ws/ticket_handler.go \
       backend/internal/realtime/ws/ticket_handler_test.go \
       backend/internal/platform/httpx/errors.go
git -c commit.gpgsign=false commit -m "feat(ws): POST /ws/ticket endpoint + new domain errors (T2)"
```

---

## Task 3: WS handler reads `?ticket=`, drops bearer-from-extractor

**Files:**
- Modify: `backend/internal/realtime/ws/handler.go`
- Modify: `backend/internal/realtime/ws/handler_test.go` (add new tests)

- [ ] **Step 1: Read current handler.Serve**

```bash
grep -n "extractToken\|PrincipalFrom\|Upgrade\b\|Serve\b" backend/internal/realtime/ws/handler.go | head -10
```

Note where token extraction currently happens.

- [ ] **Step 2: Modify handler.go to consume ticket**

Open `backend/internal/realtime/ws/handler.go`. Replace the Handler struct + NewHandler + Serve body:

```go
// Handler accepts WS upgrade requests. Authentication is the *ticket flow*,
// not a JWT — the mobile client first POSTs to /ws/ticket (Bearer-authenticated)
// and uses the returned 60s single-use ticket here. Tokens-in-URL leaked into
// reverse-proxy access logs; tickets are one-shot and short-lived.
type Handler struct {
	hub          *Hub
	log          *zap.Logger
	bundle       *i18n.Bundle
	authz        TopicAuthorizer
	tickets      TicketStore
	allowedOrigs []string
}

func NewHandler(hub *Hub, log *zap.Logger, bundle *i18n.Bundle,
	authz TopicAuthorizer, tickets TicketStore, allowedOrigs []string) *Handler {

	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		hub: hub, log: log.With(zap.String("subsystem", "ws")),
		bundle: bundle, authz: authz, tickets: tickets, allowedOrigs: allowedOrigs,
	}
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ticket")
	if raw == "" {
		httpx.WriteError(w, r, h.bundle, httpx.ErrWSTicketRequired)
		return
	}
	userID, err := h.tickets.Consume(r.Context(), raw)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrWSTicketInvalid)
		return
	}

	// The principal we know about is just userID; role isn't carried on the
	// ticket. For topic-authz we only need userID + role-string; lookups in
	// classroom.Service treat "" as "non-admin" which is the safest default.
	p := mw.Principal{UserID: userID, Role: ""}

	topics, err := h.authorizeTopics(r.Context(), p, r.URL.Query()["topic"])
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     makeCheckOrigin(h.allowedOrigs),
		Subprotocols:    []string{"bearer"},
	}
	conn, upgErr := upgrader.Upgrade(w, r, nil)
	if upgErr != nil {
		h.log.Warn("ws upgrade", zap.Error(upgErr))
		return
	}

	client := newClient(uuid.NewString(), topics)
	h.hub.Register(client)

	go h.readPump(conn, client)
	go h.writePump(conn, client, r.Context())
}

// makeCheckOrigin returns a CheckOrigin that consults the CORS allow-list.
// Empty origins (non-browser clients: native mobile, CLI, tests) are allowed
// because the ticket validation already gated the request — there's no
// cookie-equivalent CSRF threat.
func makeCheckOrigin(allowedOrigins []string) func(*http.Request) bool {
	allowAll := len(allowedOrigins) == 1 && strings.TrimSpace(allowedOrigins[0]) == "*"
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}
	return func(r *http.Request) bool {
		if allowAll {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}
```

Add to imports if missing:

```go
import (
	// ... existing ...
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/httpx"
	"strings"
)
```

Delete the package-level `var upgrader = websocket.Upgrader{...}` block — `Serve` now constructs its own per-Handler upgrader so CheckOrigin captures `h.allowedOrigs`.

- [ ] **Step 3: Update handler_test.go callsites**

Existing tests construct `NewHandler` with the old (4-arg) signature. Update each call site to match the new (6-arg) signature. Find them:

```bash
grep -rn "ws\.NewHandler\|NewHandler(.*hub" backend --include="*.go"
```

For each existing test that calls `NewHandler(hub, log, bundle, authz)`, change to `NewHandler(hub, log, bundle, authz, NewMemTicketStore(60*time.Second), []string{"*"})` (any-origin in tests).

- [ ] **Step 4: Add new handshake tests**

Append to `backend/internal/realtime/ws/handler_test.go`:

```go
func TestHandler_Serve_MissingTicket_401(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}}

	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"WS upgrade without ?ticket= must 401 — there's no fallback to JWT in query anymore")
}

func TestHandler_Serve_UnknownTicket_401(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}}

	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet, "/ws?ticket=never-issued", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a ticket string the store never issued must 401 — single failure path, no info leak")
}

func TestHandler_Serve_AccessTokenInQuery_NotAccepted(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}}

	// Even with a syntactically valid bearer-style string in the OLD query
	// param name, the handler must reject — the deprecated path is gone.
	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet,
		"/ws?access_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.x.y", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"the legacy ?access_token= param must NOT be honored — we deliberately removed it to stop tokens in proxy logs")
}

func TestCheckOrigin_AllowedOrigin_True(t *testing.T) {
	check := makeCheckOrigin([]string{"http://localhost:3000"})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	assert.True(t, check(r))
}

func TestCheckOrigin_DisallowedOrigin_False(t *testing.T) {
	check := makeCheckOrigin([]string{"http://localhost:3000"})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Origin", "http://evil.example")
	assert.False(t, check(r),
		"an Origin not on the CORS allow-list must be rejected — that's the WS-specific CSRF protection")
}

func TestCheckOrigin_EmptyOrigin_True(t *testing.T) {
	check := makeCheckOrigin([]string{"http://localhost:3000"})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil) // no Origin header
	assert.True(t, check(r),
		"native mobile / CLI / tests don't send Origin; the ticket already gated them, so allow")
}

func TestCheckOrigin_StarMode_AllowsAll(t *testing.T) {
	check := makeCheckOrigin([]string{"*"})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Origin", "http://anywhere.example")
	assert.True(t, check(r), `"*" mode (dev/test convenience) must keep the old allow-anything behavior`)
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/realtime/ws/...
```

Expected: PASS — all old hub tests + new handshake/check-origin tests.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/realtime/ws/handler.go backend/internal/realtime/ws/handler_test.go
git -c commit.gpgsign=false commit -m "feat(ws): handler consumes tickets + CheckOrigin uses CORS allow-list (T3)"
```

---

## Task 4: Drop `?access_token=` query fallback in middleware/auth.go

**Files:**
- Modify: `backend/internal/platform/httpx/middleware/auth.go`

- [ ] **Step 1: Read current extractToken**

```bash
grep -B1 -A12 "func extractToken" backend/internal/platform/httpx/middleware/auth.go
```

- [ ] **Step 2: Remove the query-string branch**

Replace:

```go
func extractToken(r *http.Request) string {
	const prefix = "Bearer "
	if raw := r.Header.Get("Authorization"); strings.HasPrefix(raw, prefix) {
		return strings.TrimPrefix(raw, prefix)
	}
	if q := r.URL.Query().Get("access_token"); q != "" {
		return q
	}
	return ""
}
```

with:

```go
func extractToken(r *http.Request) string {
	const prefix = "Bearer "
	// Authorization header only. The previous `?access_token=` query-string
	// fallback was intentional for browser WebSocket upgrades, but it leaked
	// JWTs into reverse-proxy logs. WebSocket upgrades now use the single-
	// use ticket flow under /api/v1/ws/ticket; nothing else needs the fallback.
	if raw := r.Header.Get("Authorization"); strings.HasPrefix(raw, prefix) {
		return strings.TrimPrefix(raw, prefix)
	}
	return ""
}
```

If the file no longer uses `r.URL.Query()`, the import for `net/url` (if any) becomes unused — `go build` will surface that. The function uses `r.URL.Query()` which is on `*http.Request`, so no import goes away.

- [ ] **Step 3: Verify all tests still pass**

```bash
cd backend && go test -race -count=1 ./internal/platform/httpx/middleware/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/platform/httpx/middleware/auth.go
git -c commit.gpgsign=false commit -m "feat(auth): remove ?access_token= query fallback — Bearer header only (T4)"
```

---

## Task 5: realtime.Event Version field + emit `version: 1` everywhere

**Files:**
- Modify: `backend/internal/realtime/broker.go`
- Create: `backend/internal/realtime/event_test.go`
- Modify: 4 service files that build `realtime.Event{...}` literals
- Modify: `backend/internal/realtime/ws/hub_test.go` (test fixtures)

- [ ] **Step 1: Write failing test**

Create `backend/internal/realtime/event_test.go`:

```go
package realtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/realtime"
)

func TestEvent_VersionRoundTrip(t *testing.T) {
	in := realtime.Event{
		Version: 1,
		Topic:   "user:1:notifications",
		Type:    "notification.created",
		Payload: map[string]any{"id": "abc"},
	}
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"version":1`,
		"every emitted event must include `version: 1` in its JSON — that's the contract for forward-compatibility")

	var out realtime.Event
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, 1, out.Version)
}

func TestEvent_MissingVersion_DefaultsToZero(t *testing.T) {
	// A consumer encountering a legacy event without version sees Version=0,
	// distinguishable from any explicit Version. That lets future consumers
	// say "I don't know how to read v0" cleanly.
	var got realtime.Event
	require.NoError(t, json.Unmarshal([]byte(`{"topic":"t","type":"x"}`), &got))
	assert.Equal(t, 0, got.Version)
}
```

- [ ] **Step 2: Run failing**

```bash
cd backend && go test ./internal/realtime/... -run TestEvent_
```

Expected: FAIL (Version field doesn't exist).

- [ ] **Step 3: Add Version field**

Open `backend/internal/realtime/broker.go`. Replace the Event struct:

```go
// Event is the wire format every broker fans out to subscribers.
//
// Version is the schema version: bumped only on breaking changes (renames,
// type changes). Consumers MUST tolerate unknown fields so additive changes
// don't require a version bump. Producers always set this — see NewEvent.
type Event struct {
	Version int            `json:"version"`
	Topic   string         `json:"topic"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// NewEvent is the canonical constructor — sets Version to the current schema
// version (1) so callers can't accidentally emit Version=0. Use this in
// preference to literal struct construction.
func NewEvent(topic, eventType string, payload map[string]any) Event {
	return Event{Version: 1, Topic: topic, Type: eventType, Payload: payload}
}
```

- [ ] **Step 4: Update emit sites to use Version: 1 (or NewEvent)**

There are 4 production files that build `realtime.Event{...}` literals. For each, add `Version: 1,` as the first field.

```bash
grep -rn "realtime\.Event{" backend/internal --include="*.go" | grep -v _test.go
```

Expected output (4 files):

```
internal/notification/service.go:128
internal/sensor/service.go:123
internal/device/service.go:265
internal/scene/service.go:179
```

For each, edit the literal to include `Version: 1,` at the top of the struct. Example for `notification/service.go:128`:

```go
// before
realtime.Event{
    Topic: ...,
    Type:  ...,
    Payload: ...,
}

// after
realtime.Event{
    Version: 1,
    Topic:   ...,
    Type:    ...,
    Payload: ...,
}
```

- [ ] **Step 5: Update hub_test.go fixtures**

```bash
grep -n "realtime\.Event{" backend/internal/realtime/ws/hub_test.go
```

For each fixture (4 in hub_test.go: lines 24, 43, 53, 95 in the previously seen output), add `Version: 1,` so the test fixtures match what production emits. Otherwise they'd carry Version: 0 and a stricter assertion in the future could break.

- [ ] **Step 6: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/realtime/... ./internal/notification/... ./internal/sensor/... ./internal/device/... ./internal/scene/...
```

Expected: PASS (all 6 packages).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/realtime/ \
       backend/internal/notification/service.go \
       backend/internal/sensor/service.go \
       backend/internal/device/service.go \
       backend/internal/scene/service.go
git -c commit.gpgsign=false commit -m "feat(realtime): Event.Version field + emit version=1 at all 4 sites (T5)"
```

---

## Task 6: Wire ticket store + handler in server.go and main.go

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Update server.Deps shape and route registration**

Open `backend/internal/server/server.go`. The `Deps` struct currently includes `WSHandler *ws.Handler`. Add a sibling field for the ticket route:

```go
type Deps struct {
	// ... existing fields ...
	WSHandler       *ws.Handler
	WSTicketHandler http.Handler // mounted at /api/v1/ws/ticket inside authenticated group
}
```

In `New(d Deps)`, find the authenticated chi.Group block (the one that already mounts `r.Route("/auth", d.AuthHandler.AuthenticatedRoutes)` per the audit work). Add:

```go
		r.Group(func(r chi.Router) {
			r.Use(mw.Authn(d.Issuer, d.Bundle))

			r.Route("/auth", d.AuthHandler.AuthenticatedRoutes)

			// New WS-specific routes — POST /ws/ticket needs the principal,
			// so it sits inside the authenticated group. The actual upgrade
			// (/ws) reads the ticket from a query param and is registered
			// outside this group (just below).
			if d.WSTicketHandler != nil {
				r.Post("/ws/ticket", d.WSTicketHandler.ServeHTTP)
			}

			// ... existing routes ...
```

The actual `r.Get("/ws", d.WSHandler.Serve)` route stays where it is (still inside the authenticated group, but reads the ticket itself).

Wait — that's a contradiction. The /ws route needs to be reachable WITHOUT a Bearer JWT (the ticket is the auth). Move /ws OUT of the authenticated group:

Find the existing `if d.WSHandler != nil { r.Get("/ws", d.WSHandler.Serve) }` and move it OUT of the authenticated `r.Group(...)` block — into the outer `r.Route("/api/v1", ...)` directly.

Resulting structure:

```go
r.Route("/api/v1", func(r chi.Router) {
    r.Group(func(r chi.Router) {
        r.Use(authRL.Middleware())
        r.Route("/auth", d.AuthHandler.Routes)
    })

    // Authenticated group (Authn middleware) - everything that needs the principal.
    r.Group(func(r chi.Router) {
        r.Use(mw.Authn(d.Issuer, d.Bundle))
        r.Route("/auth", d.AuthHandler.AuthenticatedRoutes)
        if d.WSTicketHandler != nil {
            r.Post("/ws/ticket", d.WSTicketHandler.ServeHTTP)
        }
        r.Route("/users", d.UserHandler.Routes)
        // ...all existing routes...
    })

    // /ws is authenticated by ticket, NOT by Bearer JWT — sits outside the
    // Authn-protected group so the upgrade handshake doesn't 401 before the
    // ticket is even consumed.
    if d.WSHandler != nil {
        r.Get("/ws", d.WSHandler.Serve)
    }
})
```

- [ ] **Step 2: Wire from main.go**

Open `backend/cmd/server/main.go`. After the `wsH := ws.NewHandler(...)` line, construct the store and the new ticket handler:

```go
	// Ticket store for WS upgrades. 60s TTL, single-use, in-memory.
	// Cleanup goroutine started immediately; stop function deferred so it
	// shuts down cleanly on signal.
	wsTickets := ws.NewMemTicketStore(60 * time.Second)
	stopTicketCleanup := wsTickets.Run(time.Minute)
	defer stopTicketCleanup()

	wsH := ws.NewHandler(hub, logger, bundle, classroomSvc, wsTickets, cfg.CORS.Origins)
	wsTicketH := ws.NewTicketHandler(wsTickets)
```

Update the existing `wsH := ws.NewHandler(...)` line to the new 6-arg signature shown above.

In the `srv := server.New(server.Deps{...})` block, add:

```go
		WSTicketHandler:     wsTicketH,
```

- [ ] **Step 3: Build + run all tests**

```bash
cd backend && go build ./... && go test -race -count=1 ./...
```

Expected: build OK; all 22+ packages PASS under `-race`. Coverage may bump slightly from the new tests.

- [ ] **Step 4: Smoke-test the route registration**

Add a small server_test.go entry:

```go
func TestServer_TicketRoute_Authenticated_200(t *testing.T) {
	// existing /metrics test pattern; mount the ticket handler with a fake
	// principal-injection middleware and verify it returns 200.
	store := ws.NewMemTicketStore(60 * time.Second)
	tkt := ws.NewTicketHandler(store)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(),
				mw.Principal{UserID: uuid.New(), Role: "teacher"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/api/v1/ws/ticket", tkt.ServeHTTP)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
```

(Append to the existing `backend/internal/server/server_test.go`. Add the `ws` and `uuid` imports.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/server.go backend/internal/server/server_test.go backend/cmd/server/main.go
git -c commit.gpgsign=false commit -m "feat(server): wire WS ticket store + handler; route /ws outside Authn group (T6)"
```

---

## Task 7: Mobile — call createTicket() before WS connect

**Files:**
- Create: `mobile/lib/core/api/endpoints/ws_endpoints.dart`
- Modify: `mobile/lib/shared/providers/ws_provider.dart`

- [ ] **Step 1: Read existing endpoints file pattern**

```bash
ls /Users/arsenozhetov/Projects/pet/smartclass/mobile/lib/core/api/endpoints/
cat /Users/arsenozhetov/Projects/pet/smartclass/mobile/lib/core/api/endpoints/user_endpoints.dart | head -30
```

Note: how endpoints classes are constructed (typically `class XEndpoints { final ApiClient _client; XEndpoints(this._client); }`).

- [ ] **Step 2: Create ws_endpoints.dart**

Create `mobile/lib/core/api/endpoints/ws_endpoints.dart`:

```dart
import '../client.dart';

/// WsEndpoints exposes WebSocket-related HTTP endpoints. The single-use
/// ticket flow lives here: the caller fetches a ticket immediately before
/// each WS upgrade attempt (including reconnects), then includes it as a
/// `?ticket=` query param on the upgrade URL.
///
/// The ticket is single-use and 60s-lived; never cache it across upgrades.
class WsEndpoints {
  WsEndpoints(this._client);

  final ApiClient _client;

  /// Issues a fresh single-use ticket for the next WebSocket upgrade.
  /// Throws if the call fails — the caller should surface that as a regular
  /// connection error rather than retrying silently.
  Future<String> createTicket() async {
    final res = await _client.post<Map<String, dynamic>>('/ws/ticket');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final ticket = data?['ticket'] as String?;
    if (ticket == null || ticket.isEmpty) {
      throw StateError('ws/ticket: empty or missing ticket in response');
    }
    return ticket;
  }
}
```

(Adjust `import '../client.dart';` and the `_client.post<...>(...)` call shape to match the actual `ApiClient` signature. Inspect `mobile/lib/core/api/client.dart` if needed.)

- [ ] **Step 3: Modify ws_provider.dart**

Open `mobile/lib/shared/providers/ws_provider.dart`. The current `connectToClassroom` builds a URL with `&access_token=$token`. Replace:

```dart
// before
Future<void> connectToClassroom(String classroomId) async {
  final token = await _storage.getAccessToken();
  if (token == null) return;

  final baseWs = _resolver.wsBaseUrl;
  final url =
      '$baseWs/ws?topic=classroom:$classroomId:devices&topic=classroom:$classroomId:sensors&access_token=$token';
  _ws.connect(url);
  state = true;
}

// after
Future<void> connectToClassroom(String classroomId) async {
  final ticket = await _wsEndpoints.createTicket();
  final baseWs = _resolver.wsBaseUrl;
  final url =
      '$baseWs/ws?topic=classroom:$classroomId:devices&topic=classroom:$classroomId:sensors&ticket=$ticket';
  _ws.connect(url);
  state = true;
}
```

The class constructor now needs the WsEndpoints injected. Find:

```dart
class WsConnectionNotifier extends StateNotifier<bool> {
  WsConnectionNotifier(this._ws, this._storage, this._resolver) : super(false);
  final WsClient _ws;
  final SecureStorage _storage;
  final ConnectionResolver _resolver;
```

Add a `WsEndpoints _wsEndpoints` field and constructor param. The provider that builds it must source `WsEndpoints` from an `apiClientProvider` (or wherever ApiClient comes from) — find that provider and pass it through.

If `_storage` is no longer needed (because the ticket call already authenticates via the underlying ApiClient's bearer interceptor), remove the `_storage` field too.

- [ ] **Step 4: Update wsConnectionProvider to pass WsEndpoints**

Find the `final wsConnectionProvider = StateNotifierProvider<...>` definition. Update its build closure to read `WsEndpoints(ref.read(apiClientProvider))` and pass into the notifier constructor.

- [ ] **Step 5: Run mobile tests + analyze**

```bash
cd /Users/arsenozhetov/Projects/pet/smartclass
(cd mobile && flutter analyze && flutter test --reporter=compact 2>&1 | tail -3)
```

Expected: clean analyze; 59/59 tests pass.

- [ ] **Step 6: Commit**

```bash
git add mobile/lib/core/api/endpoints/ws_endpoints.dart mobile/lib/shared/providers/ws_provider.dart
git -c commit.gpgsign=false commit -m "feat(mobile/ws): create ticket before connect; drop ?access_token= (T7)"
```

---

## Task 8: Final regression sweep

**Files:** No changes — verification only.

- [ ] **Step 1: All Go checks**

```bash
export PATH=$PATH:$(go env GOPATH)/bin
cd backend
go vet ./... && echo "vet OK"
staticcheck ./... && echo "staticcheck OK"
govulncheck ./... 2>&1 | tail -3
gosec -quiet ./... 2>&1 | tail -7
go test -race -count=1 ./... 2>&1 | grep -E "FAIL|ok\s" | tail -25
```

Expected: each tool reports zero issues; all packages PASS under `-race`.

- [ ] **Step 2: Slow tests**

```bash
cd backend && go test -tags=slow -race -count=1 ./internal/hass/...
```

Expected: PASS.

- [ ] **Step 3: Coverage**

```bash
cd backend && go test -coverprofile=/tmp/cov.out ./... > /dev/null 2>&1
go tool cover -func=/tmp/cov.out | tail -1
rm /tmp/cov.out
```

Expected: total ≥ 30% (CI gate).

- [ ] **Step 4: Mobile**

```bash
cd /Users/arsenozhetov/Projects/pet/smartclass
(cd mobile && flutter analyze && flutter test --reporter=compact 2>&1 | tail -3)
```

Expected: clean analyze; 59/59.

- [ ] **Step 5: Live smoke (optional, requires running stack)**

```bash
make up
sleep 5

# Old query-token path must fail.
curl -sI "http://localhost:8080/api/v1/ws?access_token=invalid" | head -1
# Expected: HTTP/1.1 401

# New ticket flow.
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"teacher@smartclass.kz","password":"teacher1234"}' \
  | jq -r '.data.tokens.accessToken')

TICKET=$(curl -s -X POST http://localhost:8080/api/v1/ws/ticket \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.ticket')
echo "Got ticket: $TICKET"

# Connect WS.
echo | wscat -c "ws://localhost:8080/api/v1/ws?ticket=$TICKET&topic=user:foo:notifications" --no-color | head -3
# Expected: connection opens (depending on classroom membership for the topic).

# Reused ticket fails.
echo | wscat -c "ws://localhost:8080/api/v1/ws?ticket=$TICKET&topic=user:foo:notifications" --no-color 2>&1 | head -3
# Expected: 401 / handshake error.

make down
```

- [ ] **Step 6: Commit if any tweaks**

```bash
git status
# If clean:
echo "no changes"
# If tweaks made:
git add .
git -c commit.gpgsign=false commit -m "chore(ws): regression fixes from final smoke"
```

- [ ] **Step 7: Print summary**

Output: 16 new tests; ticket flow live; `?access_token=` removed; CheckOrigin reads CORS list; every event now has `version: 1`. Audit findings closed: F-014, F-021, F-022.

---

## Self-Review

**Spec coverage check (against `2026-05-01-ws-auth-versioning-design.md`):**
- §2 TicketStore + MemTicketStore + Cleanup → Task 1 ✓
- §2 POST /ws/ticket endpoint → Task 2 ✓
- §2 WS handler reads ?ticket → Task 3 ✓
- §2 extractToken drops ?access_token → Task 4 ✓
- §2 CheckOrigin uses CORS allow-list → Task 3 (makeCheckOrigin) ✓
- §2 Event.Version field + 6 emit sites → Task 5 ✓
- §2 Mobile WsEndpoints.createTicket() + ws_provider URL builder → Task 7 ✓
- §6 ErrWSTicketRequired / ErrWSTicketInvalid → Task 2 ✓
- §7 16 new tests → Tasks 1 (5) + 2 (2) + 3 (3 handshake + 4 origin) + 5 (2) + 6 (1) = 17 (slight overshoot, fine) ✓

**Placeholder scan:** No "TBD"/"TODO"/"fill in details" inside task steps. Mobile dialog says "Adjust to match actual ApiClient signature" — that's pragmatic discovery, not a placeholder.

**Type consistency:**
- `MemTicketStore` / `TicketStore` / `Ticket` / `ErrTicketUnknown` — consistent across Tasks 1-3.
- `NewTicketHandler` / `TicketHandler` — Tasks 2 + 6.
- `Handler` constructor signature — Task 3 declares 6-arg shape; Task 6 main.go and existing tests update accordingly.
- `httpx.ErrWSTicketRequired` / `httpx.ErrWSTicketInvalid` — Tasks 2 + 3 use the same names.
- `realtime.Event.Version` / `NewEvent` — Tasks 5 + downstream emit sites consistent.
- Mobile `WsEndpoints.createTicket()` — Task 7 only.

Plan ready.
