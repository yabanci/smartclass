package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/hasher"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/platform/validation"
	"smartclass/internal/user"
	"smartclass/internal/user/usertest"
)

func localesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

type fixture struct {
	handler *user.Handler
	issuer  *tokens.JWT
	repo    *usertest.MemRepo
	hash    *hasher.Bcrypt
	bundle  *i18n.Bundle
	userID  uuid.UUID
}

func setupFixture(t *testing.T) *fixture {
	t.Helper()
	repo := usertest.NewMemRepo()
	h := hasher.NewBcrypt(4)
	iss := tokens.NewJWT("test-secret-key-1234567890", time.Minute, time.Hour, "test")
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))

	pwHash, err := h.Hash("password1")
	require.NoError(t, err)
	u := &user.User{
		ID:           uuid.New(),
		Email:        "me@example.com",
		PasswordHash: pwHash,
		FullName:     "Me",
		Role:         user.RoleTeacher,
		Language:     "en",
	}
	require.NoError(t, repo.Create(context.Background(), u))

	svc := user.NewService(repo, h)
	handler := user.NewHandler(svc, validation.New(), bundle)

	return &fixture{handler: handler, issuer: iss, repo: repo, hash: h, bundle: bundle, userID: u.ID}
}

func (f *fixture) router() http.Handler {
	r := chi.NewRouter()
	r.Use(mw.Language)
	r.Group(func(r chi.Router) {
		r.Use(mw.Authn(f.issuer, f.bundle))
		r.Route("/users", f.handler.Routes)
	})
	return r
}

func (f *fixture) bearer(t *testing.T) string {
	t.Helper()
	pair, err := f.issuer.Issue(f.userID, string(user.RoleTeacher))
	require.NoError(t, err)
	return "Bearer " + pair.Access
}

func doJSON(t *testing.T, h http.Handler, method, path, auth string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Me_RequiresAuth(t *testing.T) {
	f := setupFixture(t)
	rec := doJSON(t, f.router(), http.MethodGet, "/users/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Me_ReturnsProfile(t *testing.T) {
	f := setupFixture(t)
	rec := doJSON(t, f.router(), http.MethodGet, "/users/me", f.bearer(t), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data user.ProfileDTO
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "me@example.com", resp.Data.Email)
	assert.Equal(t, "teacher", resp.Data.Role)
}

func TestHandler_UpdateMe(t *testing.T) {
	f := setupFixture(t)
	rec := doJSON(t, f.router(), http.MethodPatch, "/users/me", f.bearer(t), map[string]any{
		"fullName": "Updated",
		"language": "ru",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data user.ProfileDTO
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp.Data.FullName)
	assert.Equal(t, "ru", resp.Data.Language)
}

func TestHandler_ChangePassword(t *testing.T) {
	f := setupFixture(t)

	t.Run("ok", func(t *testing.T) {
		rec := doJSON(t, f.router(), http.MethodPost, "/users/me/password", f.bearer(t), map[string]any{
			"currentPassword": "password1",
			"newPassword":     "brand-new-pass",
		})
		assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	})

	t.Run("wrong current", func(t *testing.T) {
		rec := doJSON(t, f.router(), http.MethodPost, "/users/me/password", f.bearer(t), map[string]any{
			"currentPassword": "nope",
			"newPassword":     "brand-new-pass-2",
		})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
