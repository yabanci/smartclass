# Push Notifications (FCM HTTP v1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistent device-token registration and FCM HTTP v1 push dispatch that fires whenever a notification is created.

**Architecture:** New `internal/devicetoken` package owns token CRUD; new `internal/pushnotif` package owns the FCM HTTP v1 client with no-op fallback when env vars are absent. `notification.Service` accepts a `Pusher` interface and fires pushes in a goroutine after persisting, so FCM latency never blocks callers.

**Tech Stack:** Go 1.25, pgx/v5, chi/v5, go-playground/validator/v10, zap, golang.org/x/oauth2/google (new dep), net/http, httptest, stretchr/testify.

---

## File Map

### New files
- `migrations/00016_device_tokens.sql` — goose Up/Down for device_tokens table + indexes
- `internal/devicetoken/model.go` — `Token` struct, `Platform` typed string + constants
- `internal/devicetoken/repository.go` — `Repository` interface
- `internal/devicetoken/postgres.go` — pgx UPSERT, delete, list
- `internal/devicetoken/service.go` — Register / Unregister / GetByUser
- `internal/devicetoken/dto.go` — RegisterRequest DTO with validator tags, TokenDTO response
- `internal/devicetoken/handler.go` — POST /me/device-tokens, DELETE /me/device-tokens/{token}
- `internal/devicetoken/devicetokentest/memrepo.go` — in-memory repo for tests
- `internal/devicetoken/service_test.go` — service unit tests
- `internal/devicetoken/handler_test.go` — handler unit tests
- `internal/pushnotif/client.go` — FCM HTTP v1 client + no-op + token cache
- `internal/pushnotif/pusher.go` — `NotificationPusher` wiring devicetoken.Service + Client
- `internal/pushnotif/client_test.go` — no-op path, JWT signing path, 404 token-invalid path

### Modified files
- `internal/notification/service.go` — add `Pusher` interface field, fire push in goroutine in `CreateForUser`/`CreateForClassroom`
- `internal/server/server.go` — add `DeviceTokenHandler` to `Deps`, register routes under `/me/device-tokens`
- `cmd/server/main.go` — construct pushnotif.Client + NotificationPusher, pass to notification.NewService; construct devicetoken handler and wire into server deps
- `go.mod` / `go.sum` — add `golang.org/x/oauth2`

---

## Task 1: Migration

**Files:**
- Create: `migrations/00016_device_tokens.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS device_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        TEXT        NOT NULL,
    platform     TEXT        NOT NULL CHECK (platform IN ('android', 'ios', 'web')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS device_tokens_user_token_uidx
    ON device_tokens(user_id, token);

CREATE INDEX IF NOT EXISTS device_tokens_user_idx
    ON device_tokens(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_tokens;
-- +goose StatementEnd
```

- [ ] **Step 2: Verify file is syntactically consistent with existing migrations**

Run: `ls migrations/ | sort` — confirm 00016 follows 00015 in sequence.

---

## Task 2: devicetoken model + repository interface

**Files:**
- Create: `internal/devicetoken/model.go`
- Create: `internal/devicetoken/repository.go`

- [ ] **Step 1: Write model.go**

```go
package devicetoken

import (
	"time"

	"github.com/google/uuid"
)

type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWeb     Platform = "web"
)

type Token struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Token      string
	Platform   Platform
	CreatedAt  time.Time
	LastSeenAt time.Time
}
```

- [ ] **Step 2: Write repository.go**

```go
package devicetoken

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("devicetoken: not found")

type Repository interface {
	Save(ctx context.Context, t *Token) error
	DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error)
}
```

- [ ] **Step 3: Verify package compiles**

Run from backend dir: `go build ./internal/devicetoken/...`

---

## Task 3: devicetoken postgres repo

**Files:**
- Create: `internal/devicetoken/postgres.go`

- [ ] **Step 1: Write postgres.go**

```go
package devicetoken

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Save upserts a device token. On conflict (user_id, token) it bumps last_seen_at.
func (r *PostgresRepository) Save(ctx context.Context, t *Token) error {
	const q = `
INSERT INTO device_tokens (id, user_id, token, platform, created_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, token)
DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
RETURNING id, created_at, last_seen_at`

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.LastSeenAt = now

	return r.pool.QueryRow(ctx, q,
		t.ID, t.UserID, t.Token, string(t.Platform), t.CreatedAt, t.LastSeenAt,
	).Scan(&t.ID, &t.CreatedAt, &t.LastSeenAt)
}

func (r *PostgresRepository) DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error {
	_, err := r.pool.Exec(ctx,
		"DELETE FROM device_tokens WHERE user_id=$1 AND token=$2",
		userID, token)
	return err
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error) {
	const q = `
SELECT id, user_id, token, platform, created_at, last_seen_at
FROM device_tokens
WHERE user_id=$1
ORDER BY last_seen_at DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Token
	for rows.Next() {
		tk := &Token{}
		var platform string
		if err := rows.Scan(&tk.ID, &tk.UserID, &tk.Token, &platform,
			&tk.CreatedAt, &tk.LastSeenAt); err != nil {
			return nil, err
		}
		tk.Platform = Platform(platform)
		out = append(out, tk)
	}
	return out, rows.Err()
}
```

- [ ] **Step 2: Build check**

Run: `go build ./internal/devicetoken/...`

---

## Task 4: devicetoken in-memory repo (for tests)

**Files:**
- Create: `internal/devicetoken/devicetokentest/memrepo.go`

- [ ] **Step 1: Write memrepo.go**

```go
package devicetokentest

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/devicetoken"
)

type MemRepo struct {
	mu    sync.Mutex
	items []*devicetoken.Token
}

func NewMemRepo() *MemRepo { return &MemRepo{} }

func (r *MemRepo) Save(_ context.Context, t *devicetoken.Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, existing := range r.items {
		if existing.UserID == t.UserID && existing.Token == t.Token {
			existing.LastSeenAt = now
			t.ID = existing.ID
			t.CreatedAt = existing.CreatedAt
			t.LastSeenAt = existing.LastSeenAt
			return nil
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.LastSeenAt = now
	c := *t
	r.items = append(r.items, &c)
	return nil
}

func (r *MemRepo) DeleteByToken(_ context.Context, userID uuid.UUID, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.items[:0]
	for _, t := range r.items {
		if !(t.UserID == userID && t.Token == token) {
			out = append(out, t)
		}
	}
	r.items = out
	return nil
}

func (r *MemRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*devicetoken.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*devicetoken.Token
	for _, t := range r.items {
		if t.UserID == userID {
			c := *t
			out = append(out, &c)
		}
	}
	return out, nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./internal/devicetoken/...`

---

## Task 5: devicetoken service + DTO

**Files:**
- Create: `internal/devicetoken/service.go`
- Create: `internal/devicetoken/dto.go`

- [ ] **Step 1: Write service.go**

```go
package devicetoken

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, userID uuid.UUID, token string, platform Platform) (*Token, error) {
	t := &Token{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	}
	if err := s.repo.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Unregister(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteByToken(ctx, userID, token)
}

func (s *Service) GetByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error) {
	return s.repo.ListByUser(ctx, userID)
}
```

- [ ] **Step 2: Write dto.go**

```go
package devicetoken

import "time"

// RegisterRequest is the JSON body for POST /me/device-tokens.
type RegisterRequest struct {
	Token    string `json:"token"    validate:"required,max=4096"`
	Platform string `json:"platform" validate:"required,oneof=android ios web"`
}

// TokenDTO is the JSON representation returned in responses.
type TokenDTO struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Platform   string    `json:"platform"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

func ToDTO(t *Token) TokenDTO {
	return TokenDTO{
		ID:         t.ID.String(),
		Token:      t.Token,
		Platform:   string(t.Platform),
		CreatedAt:  t.CreatedAt,
		LastSeenAt: t.LastSeenAt,
	}
}
```

- [ ] **Step 3: Build check**

Run: `go build ./internal/devicetoken/...`

---

## Task 6: devicetoken handler

**Files:**
- Create: `internal/devicetoken/handler.go`

- [ ] **Step 1: Write handler.go**

```go
package devicetoken

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/validation"
)

type Handler struct {
	svc    *Service
	valid  *validation.Validator
	bundle *i18n.Bundle
}

func NewHandler(svc *Service, valid *validation.Validator, bundle *i18n.Bundle) *Handler {
	return &Handler{svc: svc, valid: valid, bundle: bundle}
}

// Routes registers under an authenticated /me/device-tokens group.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.register)
	r.Delete("/{token}", h.unregister)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	var req RegisterRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	t, err := h.svc.Register(r.Context(), p.UserID, req.Token, Platform(req.Platform))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToDTO(t))
}

func (h *Handler) unregister(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	token := chi.URLParam(r, "token")
	if err := h.svc.Unregister(r.Context(), p.UserID, token); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.Empty(w, http.StatusNoContent)
}
```

- [ ] **Step 2: Build check**

Run: `go build ./internal/devicetoken/...`

---

## Task 7: devicetoken service_test.go

**Files:**
- Create: `internal/devicetoken/service_test.go`

- [ ] **Step 1: Write service_test.go**

```go
package devicetoken_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicetoken"
	"smartclass/internal/devicetoken/devicetokentest"
)

func TestService_Register_SavesToken(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	tok, err := svc.Register(context.Background(), userID, "tok-abc", devicetoken.PlatformAndroid)
	require.NoError(t, err)

	assert.Equal(t, userID, tok.UserID)
	assert.Equal(t, "tok-abc", tok.Token)
	assert.Equal(t, devicetoken.PlatformAndroid, tok.Platform)
	assert.NotEqual(t, uuid.Nil, tok.ID)
	assert.False(t, tok.CreatedAt.IsZero())
}

func TestService_Register_UpsertBumpsLastSeen(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	first, err := svc.Register(context.Background(), userID, "same-tok", devicetoken.PlatformIOS)
	require.NoError(t, err)

	second, err := svc.Register(context.Background(), userID, "same-tok", devicetoken.PlatformIOS)
	require.NoError(t, err)

	// Same token → same row, ID preserved.
	assert.Equal(t, first.ID, second.ID,
		"upsert must not create a second row for the same (user_id, token) pair")

	// last_seen_at must not regress.
	assert.False(t, second.LastSeenAt.Before(first.LastSeenAt),
		"re-registering the same token must bump last_seen_at")
}

func TestService_Unregister_RemovesToken(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	_, err := svc.Register(context.Background(), userID, "tok-del", devicetoken.PlatformWeb)
	require.NoError(t, err)

	require.NoError(t, svc.Unregister(context.Background(), userID, "tok-del"))

	tokens, err := svc.GetByUser(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, tokens,
		"after Unregister, ListByUser must return no tokens for this user")
}

func TestService_Unregister_Idempotent(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	err := svc.Unregister(context.Background(), userID, "nonexistent")
	assert.NoError(t, err,
		"unregistering a token that does not exist must not error — callers must be able to call it safely on logout")
}

func TestService_GetByUser_ScopedToUser(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	uid1 := uuid.New()
	uid2 := uuid.New()
	_, err := svc.Register(context.Background(), uid1, "tok-u1", devicetoken.PlatformAndroid)
	require.NoError(t, err)
	_, err = svc.Register(context.Background(), uid2, "tok-u2", devicetoken.PlatformWeb)
	require.NoError(t, err)

	tokens, err := svc.GetByUser(context.Background(), uid1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, "tok-u1", tokens[0].Token,
		"GetByUser must only return tokens belonging to the requested user")
}
```

- [ ] **Step 2: Run tests**

Run: `go test -race ./internal/devicetoken/...`
Expected: all tests pass.

---

## Task 8: devicetoken handler_test.go

**Files:**
- Create: `internal/devicetoken/handler_test.go`

- [ ] **Step 1: Write handler_test.go**

```go
package devicetoken_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicetoken"
	"smartclass/internal/devicetoken/devicetokentest"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/validation"
	"smartclass/internal/user"
)

func localesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

type dtHarness struct {
	router chi.Router
	userID uuid.UUID
}

func newDTHarness(t *testing.T) *dtHarness {
	t.Helper()
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	h := devicetoken.NewHandler(svc, validation.New(), bundle)

	uid := uuid.New()
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(),
				mw.Principal{UserID: uid, Role: string(user.RoleTeacher)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/me/device-tokens", h.Routes)
	return &dtHarness{router: r, userID: uid}
}

func postJSON(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDTHandler_Register_OK(t *testing.T) {
	h := newDTHarness(t)
	rec := postJSON(t, h.router, "/me/device-tokens", map[string]string{
		"token": "fcm-token-xyz", "platform": "android",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp struct {
		Data devicetoken.TokenDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "fcm-token-xyz", resp.Data.Token)
	assert.Equal(t, "android", resp.Data.Platform)
	assert.NotEmpty(t, resp.Data.ID)
}

func TestDTHandler_Register_InvalidPlatform(t *testing.T) {
	h := newDTHarness(t)
	rec := postJSON(t, h.router, "/me/device-tokens", map[string]string{
		"token": "fcm-token-xyz", "platform": "blackberry",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"platform must be one of android|ios|web — unknown values must 400")
}

func TestDTHandler_Register_MissingToken(t *testing.T) {
	h := newDTHarness(t)
	rec := postJSON(t, h.router, "/me/device-tokens", map[string]string{
		"platform": "ios",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"missing token field must return 400 validation error")
}

func TestDTHandler_Unregister_OK(t *testing.T) {
	h := newDTHarness(t)
	// Register first
	postJSON(t, h.router, "/me/device-tokens", map[string]string{
		"token": "tok-to-delete", "platform": "web",
	})

	req := httptest.NewRequest(http.MethodDelete,
		"/me/device-tokens/"+url.PathEscape("tok-to-delete"), nil)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func TestDTHandler_Unregister_NonExistent_Idempotent(t *testing.T) {
	h := newDTHarness(t)
	req := httptest.NewRequest(http.MethodDelete, "/me/device-tokens/ghost-token", nil)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code,
		"deleting a non-existent token must be idempotent — 204, not 404")
}
```

- [ ] **Step 2: Run tests**

Run: `go test -race ./internal/devicetoken/...`
Expected: all tests pass.

---

## Task 9: Add golang.org/x/oauth2 dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the dependency**

Run from backend dir:
```bash
cd /Users/arsenozhetov/Projects/pet/smartclass/.claude/worktrees/push-notifications/backend && go get golang.org/x/oauth2@latest
```

- [ ] **Step 2: Tidy**

Run: `go mod tidy`

- [ ] **Step 3: Verify go.mod has oauth2 in require block**

Run: `grep 'golang.org/x/oauth2' go.mod`
Expected: line like `golang.org/x/oauth2 v0.x.x`

---

## Task 10: pushnotif client

**Files:**
- Create: `internal/pushnotif/client.go`

- [ ] **Step 1: Write client.go**

```go
package pushnotif

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrInvalidToken is returned when FCM reports UNREGISTERED or INVALID_ARGUMENT
// for the device token. The caller should remove the token from the database.
var ErrInvalidToken = errors.New("pushnotif: invalid or unregistered FCM token")

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// Payload is the FCM HTTP v1 message body.
type Payload struct {
	Title string
	Body  string
	Data  map[string]string
}

// PushConfig holds project-level FCM configuration resolved from environment.
type PushConfig struct {
	ProjectID          string
	ServiceAccountJSON []byte // raw JSON credentials; nil = no-op mode
}

// ConfigFromEnv resolves PushConfig from environment variables.
// Priority: FIREBASE_SERVICE_ACCOUNT_JSON > FIREBASE_SERVICE_ACCOUNT_PATH.
// If FIREBASE_PROJECT_ID is empty or credentials are absent, returns a zero
// config which causes Client to operate in no-op mode.
func ConfigFromEnv() PushConfig {
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		return PushConfig{}
	}
	raw := []byte(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))
	if len(raw) == 0 {
		if path := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH"); path != "" {
			var err error
			raw, err = os.ReadFile(path)
			if err != nil {
				raw = nil
			}
		}
	}
	return PushConfig{ProjectID: projectID, ServiceAccountJSON: raw}
}

// Client sends FCM HTTP v1 messages. If ServiceAccountJSON is empty it becomes
// a no-op that logs at INFO level, so the service starts with zero config.
type Client struct {
	cfg    PushConfig
	log    *zap.Logger
	http   *http.Client

	mu          sync.Mutex
	tokenSource oauth2.TokenSource
}

// NewClient constructs a Client. cfg is typically obtained from ConfigFromEnv.
func NewClient(cfg PushConfig, log *zap.Logger) *Client {
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{cfg: cfg, log: log, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) isNoop() bool {
	return c.cfg.ProjectID == "" || len(c.cfg.ServiceAccountJSON) == 0
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokenSource == nil {
		creds, err := google.CredentialsFromJSON(ctx, c.cfg.ServiceAccountJSON, fcmScope)
		if err != nil {
			return "", fmt.Errorf("pushnotif: parse service account: %w", err)
		}
		c.tokenSource = creds.TokenSource
	}
	tok, err := c.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("pushnotif: oauth2 token: %w", err)
	}
	return tok.AccessToken, nil
}

// Send dispatches a push notification to a single FCM device token.
// Returns ErrInvalidToken when FCM reports the token is unregistered or invalid.
func (c *Client) Send(ctx context.Context, deviceToken string, p Payload) error {
	if c.isNoop() {
		c.log.Info("pushnotif: no-op (FCM not configured)",
			zap.String("token_prefix", safePrefix(deviceToken)),
			zap.String("title", p.Title))
		return nil
	}

	accessToken, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	dataStr := make(map[string]string, len(p.Data))
	for k, v := range p.Data {
		dataStr[k] = v
	}

	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]string{
				"title": p.Title,
				"body":  p.Body,
			},
			"data": dataStr,
		},
	})
	if err != nil {
		return fmt.Errorf("pushnotif: marshal: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send",
		c.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pushnotif: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pushnotif: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// FCM returns 404 for UNREGISTERED, 400 for INVALID_ARGUMENT on bad token.
	if resp.StatusCode == http.StatusNotFound ||
		(resp.StatusCode == http.StatusBadRequest &&
			strings.Contains(respStr, "INVALID_ARGUMENT")) {
		return ErrInvalidToken
	}

	return fmt.Errorf("pushnotif: FCM %d: %s", resp.StatusCode, respStr)
}

func safePrefix(token string) string {
	if len(token) > 8 {
		return token[:8] + "..."
	}
	return token
}
```

- [ ] **Step 2: Build check**

Run: `go build ./internal/pushnotif/...`

---

## Task 11: pushnotif client_test.go

**Files:**
- Create: `internal/pushnotif/client_test.go`

- [ ] **Step 1: Write client_test.go**

```go
package pushnotif_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"smartclass/internal/pushnotif"
)

func TestClient_Noop_WhenNotConfigured(t *testing.T) {
	// No project ID → no-op mode. Must not error.
	c := pushnotif.NewClient(pushnotif.PushConfig{}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{
		Title: "Hello", Body: "World",
	})
	assert.NoError(t, err, "no-op client must never return an error")
}

func TestClient_Noop_WhenOnlyProjectIDSet(t *testing.T) {
	// Project ID without credentials → no-op mode.
	c := pushnotif.NewClient(pushnotif.PushConfig{ProjectID: "proj", ServiceAccountJSON: nil}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.NoError(t, err)
}

// fakeServiceAccountJSON produces a minimal service-account JSON that
// google.CredentialsFromJSON will parse (using a freshly generated RSA key).
func fakeServiceAccountJSON(t *testing.T, tokenURL string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// x509.MarshalPKCS8PrivateKey returns DER; we need PEM.
	import_block := "-----BEGIN RSA PRIVATE KEY-----\n"
	_ = import_block // not actually needed below

	// We use the raw format that google.CredentialsFromJSON understands.
	// The token_uri points to our fake server so no real Google call happens.
	sa := map[string]any{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "key1",
		"private_key":                 rsaKeyToPEM(t, key),
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "123",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURL,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/test",
	}
	b, err := json.Marshal(sa)
	require.NoError(t, err)
	return b
}

func rsaKeyToPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	import_x509 "crypto/x509"
	import_pem "encoding/pem"
	import_bytes "bytes"

	der, err := import_x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	var buf import_bytes.Buffer
	require.NoError(t, import_pem.Encode(&buf, &import_pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	return buf.String()
}

func TestClient_ErrInvalidToken_On404(t *testing.T) {
	// Fake token endpoint — always returns a valid OAuth token.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenSrv.Close()

	// Fake FCM endpoint — returns 404 to simulate UNREGISTERED token.
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","message":"Requested entity was not found."}}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{
		ProjectID:          "test-project",
		ServiceAccountJSON: saJSON,
	}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(), fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "bad-device-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.ErrorIs(t, err, pushnotif.ErrInvalidToken,
		"FCM 404 must be mapped to ErrInvalidToken so callers can clean up stale tokens")
}

func TestClient_GenericError_OnFCM500(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	defer tokenSrv.Close()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{ProjectID: "proj", ServiceAccountJSON: saJSON}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(), fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "some-token", pushnotif.Payload{Title: "t", Body: "b"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, pushnotif.ErrInvalidToken,
		"FCM 500 must NOT be mapped to ErrInvalidToken — it is a transient server error, not a stale token")
	_ = time.Now() // suppress unused import
}
```

The test calls `pushnotif.NewClientWithHTTP` with a custom FCM URL template — add this constructor to `client.go`:

```go
// NewClientWithHTTP is used in tests to inject a custom HTTP client and FCM URL template.
// The urlTemplate must contain a single %s placeholder for the project ID.
func NewClientWithHTTP(cfg PushConfig, log *zap.Logger, httpClient *http.Client, urlTemplate string) *Client {
	c := NewClient(cfg, log)
	c.http = httpClient
	c.fcmURLTemplate = urlTemplate
	return c
}
```

And update `Client` struct and `Send` to use `c.fcmURLTemplate`:

In `client.go`, add field to `Client`:
```go
fcmURLTemplate string // empty = use default FCM URL
```

In `Send`, replace the URL construction line:
```go
fcmURL := c.fcmURLTemplate
if fcmURL == "" {
    fcmURL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
}
urlStr := fmt.Sprintf(fcmURL, c.cfg.ProjectID)
req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
```

- [ ] **Step 2: Fix rsaKeyToPEM import style**

The function inline-imports are pseudo-syntax. Rewrite `client_test.go` with proper top-level imports:

```go
package pushnotif_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"smartclass/internal/pushnotif"
)

func TestClient_Noop_WhenNotConfigured(t *testing.T) {
	c := pushnotif.NewClient(pushnotif.PushConfig{}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{Title: "Hello", Body: "World"})
	assert.NoError(t, err)
}

func TestClient_Noop_WhenOnlyProjectIDSet(t *testing.T) {
	c := pushnotif.NewClient(pushnotif.PushConfig{ProjectID: "proj"}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.NoError(t, err)
}

func fakeServiceAccountJSON(t *testing.T, tokenURL string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	sa := map[string]any{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "key1",
		"private_key":                 buf.String(),
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "123",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURL,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/test",
	}
	b, err := json.Marshal(sa)
	require.NoError(t, err)
	return b
}

func newFakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestClient_ErrInvalidToken_On404(t *testing.T) {
	tokenSrv := newFakeTokenServer(t)
	defer tokenSrv.Close()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","message":"Requested entity was not found."}}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{ProjectID: "test-project", ServiceAccountJSON: saJSON}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(),
		fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "bad-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.ErrorIs(t, err, pushnotif.ErrInvalidToken)
}

func TestClient_GenericError_OnFCM500(t *testing.T) {
	tokenSrv := newFakeTokenServer(t)
	defer tokenSrv.Close()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{ProjectID: "proj", ServiceAccountJSON: saJSON}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(),
		fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "some-token", pushnotif.Payload{Title: "t", Body: "b"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, pushnotif.ErrInvalidToken)
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/pushnotif/...`
Expected: all pass.

---

## Task 12: pushnotif NotificationPusher

**Files:**
- Create: `internal/pushnotif/pusher.go`

- [ ] **Step 1: Write pusher.go**

```go
package pushnotif

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TokenLister is implemented by devicetoken.Service.
type TokenLister interface {
	GetByUser(ctx context.Context, userID uuid.UUID) ([]tokenRecord, error)
}

// tokenRecord is a local alias so we don't import devicetoken here (would create
// a package cycle if devicetoken imports pushnotif for cleanup). Instead we
// define the minimal interface.
type tokenRecord interface {
	GetToken() string
}

// TokenService is the dependency interface for devicetoken.Service.
type TokenService interface {
	GetByUser(ctx context.Context, userID uuid.UUID) (interface{ Token() string; ID() string }, error)
}
```

The above approach creates unnecessary complexity with interfaces. Use a cleaner design: `NotificationPusher` accepts concrete `*devicetoken.Service` — both packages are internal, there's no cycle risk.

Replace with:

```go
package pushnotif

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/devicetoken"
)

// NotificationPusher fans out a push to all registered tokens for a user.
// On ErrInvalidToken, the stale token is removed from the database.
type NotificationPusher struct {
	client  *Client
	tokens  *devicetoken.Service
	log     *zap.Logger
}

func NewNotificationPusher(client *Client, tokens *devicetoken.Service, log *zap.Logger) *NotificationPusher {
	if log == nil {
		log = zap.NewNop()
	}
	return &NotificationPusher{client: client, tokens: tokens, log: log}
}

// Send dispatches p to all FCM tokens registered for userID.
// Stale tokens (ErrInvalidToken) are deleted; other errors are logged and skipped.
func (p *NotificationPusher) Send(ctx context.Context, userID uuid.UUID, payload Payload) error {
	toks, err := p.tokens.GetByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, t := range toks {
		if err := p.client.Send(ctx, t.Token, payload); err != nil {
			if errors.Is(err, ErrInvalidToken) {
				p.log.Info("pushnotif: removing stale token",
					zap.String("token_prefix", safePrefix(t.Token)))
				if delErr := p.tokens.Unregister(ctx, userID, t.Token); delErr != nil {
					p.log.Warn("pushnotif: failed to delete stale token", zap.Error(delErr))
				}
			} else {
				p.log.Warn("pushnotif: send failed", zap.Error(err),
					zap.Stringer("userID", userID))
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./internal/pushnotif/...`

---

## Task 13: Wire Pusher into notification.Service

**Files:**
- Modify: `internal/notification/service.go`

The change adds an optional `Pusher` interface to `Service`. `NewService` signature is backward-compatible: callers that don't call `WithPusher` get the nil-no-op behaviour.

- [ ] **Step 1: Add Pusher interface and field**

After the `MemberLookup` interface block, add:

```go
// Pusher sends a push notification to all FCM tokens registered for a user.
// Implementations must be safe for concurrent use.
type Pusher interface {
	Send(ctx context.Context, userID uuid.UUID, payload PushPayload) error
}

// PushPayload holds the content of a push notification.
type PushPayload struct {
	Title string
	Body  string
	Data  map[string]string
}
```

Add `pusher Pusher` field to `Service` struct.

Add `WithPusher` method:

```go
func (s *Service) WithPusher(p Pusher) *Service {
	if p != nil {
		s.pusher = p
	}
	return s
}
```

- [ ] **Step 2: Fire push after CreateForUser**

In `CreateForUser`, after the `s.publish(ctx, n)` call, add:

```go
if s.pusher != nil {
    go s.sendPush(n)
}
```

Add helper at bottom of service.go:

```go
func (s *Service) sendPush(n *Notification) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	payload := PushPayload{
		Title: n.Title,
		Body:  n.Message,
		Data:  map[string]string{"notificationId": n.ID.String(), "type": string(n.Type)},
	}
	if err := s.pusher.Send(ctx, n.UserID, payload); err != nil {
		s.log.Warn("notification: push failed", zap.Stringer("userID", n.UserID), zap.Error(err))
	}
}
```

- [ ] **Step 3: Fire push after CreateForClassroom too**

In `CreateForClassroom`, after the `for _, n := range items { s.publish(ctx, n) }` loop, add:

```go
if s.pusher != nil {
    for _, n := range items {
        n := n // capture loop var
        go s.sendPush(n)
    }
}
```

- [ ] **Step 4: Add `"time"` import if missing**

`sendPush` uses `context.WithTimeout` and `10*time.Second` — ensure `"time"` is in the import block. It already existed before; verify with `go build`.

- [ ] **Step 5: Build check**

Run: `go build ./internal/notification/...`

---

## Task 14: Wire NotificationPusher — PushPayload → pushnotif.Payload adapter

The `notification.Pusher.Send` receives `notification.PushPayload`. `pushnotif.NotificationPusher.Send` receives `pushnotif.Payload`. These need an adapter so the packages stay decoupled.

- [ ] **Step 1: Add adapter wrapper**

Add a file `internal/pushnotif/adapter.go`:

```go
package pushnotif

import (
	"context"

	"github.com/google/uuid"

	"smartclass/internal/notification"
)

// NotifPusher adapts NotificationPusher to notification.Pusher.
type NotifPusher struct {
	inner *NotificationPusher
}

func NewNotifPusher(inner *NotificationPusher) *NotifPusher {
	return &NotifPusher{inner: inner}
}

func (a *NotifPusher) Send(ctx context.Context, userID uuid.UUID, p notification.PushPayload) error {
	return a.inner.Send(ctx, userID, Payload{
		Title: p.Title,
		Body:  p.Body,
		Data:  p.Data,
	})
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

---

## Task 15: Wire into server.go and main.go

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add DeviceTokenHandler to Deps in server.go**

In the `Deps` struct, add:
```go
DeviceTokenHandler *devicetoken.Handler
```

Add import: `"smartclass/internal/devicetoken"`

In the authenticated routes group (after `r.Route("/users", ...)`), add:
```go
if d.DeviceTokenHandler != nil {
    r.Route("/me/device-tokens", d.DeviceTokenHandler.Routes)
}
```

- [ ] **Step 2: Wire everything in main.go**

Add imports:
```go
"smartclass/internal/devicetoken"
"smartclass/internal/pushnotif"
```

After `notificationRepo` construction, add:
```go
deviceTokenRepo := devicetoken.NewPostgresRepository(db.Pool)
deviceTokenSvc := devicetoken.NewService(deviceTokenRepo)

pushCfg := pushnotif.ConfigFromEnv()
pushClient := pushnotif.NewClient(pushCfg, logger)
notifPusher := pushnotif.NewNotifPusher(
    pushnotif.NewNotificationPusher(pushClient, deviceTokenSvc, logger))
```

Update the `notificationSvc` line:
```go
notificationSvc := notification.NewService(notificationRepo, classroomRepo, broker).
    WithLogger(logger).
    WithPusher(notifPusher)
```

Add handler:
```go
deviceTokenH := devicetoken.NewHandler(deviceTokenSvc, valid, bundle)
```

Add to `server.Deps`:
```go
DeviceTokenHandler: deviceTokenH,
```

- [ ] **Step 3: Full build check**

Run: `go build ./...`

---

## Task 16: Full test run + lint

- [ ] **Step 1: Run all tests with race detector**

Run: `go test -race ./...`
Expected: all PASS, no data races.

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Run golangci-lint**

Run: `golangci-lint run ./...`
Expected: clean.

- [ ] **Step 4: Run go mod tidy**

Run: `go mod tidy`
Expected: no changes to go.mod or go.sum after the tidy.

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| migration 00016_device_tokens.sql with required columns, UNIQUE index, user_id index, Down | Task 1 |
| model.go Token struct + Platform typed string | Task 2 |
| postgres.go Save UPSERT, DeleteByToken, ListByUser | Task 3 |
| service.go Register/Unregister/GetByUser | Task 5 |
| dto.go with validate tags required/max=4096/oneof | Task 5 |
| handler.go POST /me/device-tokens 201 + DELETE /me/device-tokens/:token 204 | Task 6 |
| service_test.go and handler_test.go | Tasks 7, 8 |
| pushnotif client.go FIREBASE_PROJECT_ID + JSON/path env reading | Task 10 |
| no-op mode when not configured | Task 10 |
| OAuth2 token caching via TokenSource | Task 10 |
| FCM HTTP v1 POST | Task 10 |
| ErrInvalidToken on 404/INVALID_ARGUMENT | Task 10 |
| NewClientWithHTTP for test injection | Task 10 |
| client_test.go: no-op, JWT signing, FCM 404 | Task 11 |
| NotificationPusher fan-out + stale token cleanup | Task 12 |
| notification.Pusher interface + WithPusher | Task 13 |
| push fires in goroutine after CreateForUser/CreateForClassroom | Task 13 |
| adapter pushnotif → notification.Pusher | Task 14 |
| server.go DeviceTokenHandler wiring | Task 15 |
| main.go full construction + graceful no-op when FCM not configured | Task 15 |
| golang.org/x/oauth2 dep added | Task 9 |
| go build / go vet / go test -race / golangci-lint clean | Task 16 |

**Placeholder scan:** No TBD/TODO/similar patterns in plan.

**Type consistency check:**
- `notification.PushPayload` defined in Task 13; used in Task 14 adapter — consistent.
- `pushnotif.Payload` defined in Task 10 `client.go`; used in Tasks 11, 12, 14 — consistent.
- `devicetoken.Service.GetByUser` returns `([]*Token, error)`; `NotificationPusher.Send` iterates `t.Token` (field access on struct) — consistent.
- `notification.Service.WithPusher` accepts `Pusher` interface (defined in same package in Task 13) — consistent.
- `pushnotif.NewNotifPusher` returns `*NotifPusher` which implements `notification.Pusher` — method signature `Send(ctx, uuid.UUID, notification.PushPayload) error` matches — consistent.
