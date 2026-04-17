package auth_test

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/auth"
	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/platform/validation"
	"smartclass/internal/user/usertest"
)

func localesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

func newHandler(t *testing.T) (*auth.Handler, *auth.Service) {
	t.Helper()
	repo := usertest.NewMemRepo()
	h := hasher.NewBcrypt(4)
	iss := tokens.NewJWT("test-secret-key-1234567890", time.Minute, time.Hour, "test")
	svc := auth.NewService(repo, h, iss)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))

	return auth.NewHandler(svc, validation.New(), bundle), svc
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func doRequest(t *testing.T, h *auth.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Route("/auth", h.Routes)

	var reader *bytes.Reader
	if body != nil {
		reader = mustJSON(t, body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Register(t *testing.T) {
	h, _ := newHandler(t)

	t.Run("201 on valid input", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/register", map[string]any{
			"email":    "new@example.com",
			"password": "password1",
			"fullName": "New User",
			"role":     "teacher",
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		var resp struct {
			Data struct {
				User struct {
					Email string `json:"email"`
					Role  string `json:"role"`
				}
				Tokens struct {
					AccessToken  string `json:"accessToken"`
					RefreshToken string `json:"refreshToken"`
				}
			}
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "new@example.com", resp.Data.User.Email)
		assert.Equal(t, "teacher", resp.Data.User.Role)
		assert.NotEmpty(t, resp.Data.Tokens.AccessToken)
		assert.NotEmpty(t, resp.Data.Tokens.RefreshToken)
	})

	t.Run("400 on validation error", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/register", map[string]any{
			"email":    "not-an-email",
			"password": "short",
			"fullName": "",
			"role":     "nobody",
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("409 on duplicate email", func(t *testing.T) {
		body := map[string]any{
			"email":    "dup@example.com",
			"password": "password1",
			"fullName": "Dup",
			"role":     "admin",
		}
		_ = doRequest(t, h, http.MethodPost, "/auth/register", body)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestHandler_LoginAndRefresh(t *testing.T) {
	h, svc := newHandler(t)

	_, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "li@example.com", Password: "password1", FullName: "Li", Role: "teacher",
	})
	require.NoError(t, err)

	t.Run("login ok", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/login", map[string]any{
			"email": "li@example.com", "password": "password1",
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("login wrong pass", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/login", map[string]any{
			"email": "li@example.com", "password": "nope",
		})
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("refresh ok", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/login", map[string]any{
			"email": "li@example.com", "password": "password1",
		})
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Data struct {
				Tokens struct {
					RefreshToken string `json:"refreshToken"`
				}
			}
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Data.Tokens.RefreshToken)

		rec2 := doRequest(t, h, http.MethodPost, "/auth/refresh", map[string]any{
			"refreshToken": resp.Data.Tokens.RefreshToken,
		})
		assert.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	})

	t.Run("refresh garbage", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/refresh", map[string]any{
			"refreshToken": "xxx",
		})
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
