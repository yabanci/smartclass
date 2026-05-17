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
	// Register first.
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
