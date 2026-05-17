package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/notification"
	"smartclass/internal/notification/notificationtest"
	"smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/realtime"
	"smartclass/internal/user"
)

func notifLocalesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

type notifHarness struct {
	router chi.Router
	svc    *notification.Service
	userID uuid.UUID
}

func newNotifHarness(t *testing.T) *notifHarness {
	t.Helper()
	repo := notificationtest.NewMemRepo()
	svc := notification.NewService(repo, staticMembers{ids: nil}, realtime.Noop{})

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(notifLocalesDir(t)))
	h := notification.NewHandler(svc, bundle)

	uid := uuid.New()
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.WithPrincipalForTest(r.Context(),
				middleware.Principal{UserID: uid, Role: string(user.RoleTeacher)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/notifications", h.Routes)

	return &notifHarness{router: r, svc: svc, userID: uid}
}

func TestNotifHandler_List_PaginationClampsZeroLimit(t *testing.T) {
	h := newNotifHarness(t)
	for i := 0; i < 100; i++ {
		_, err := h.svc.CreateForUser(context.Background(), notification.Input{
			UserID: h.userID, Type: notification.TypeInfo, Title: "t", Message: "m",
		})
		require.NoError(t, err)
	}

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications?limit=0", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.LessOrEqual(t, len(resp.Data), 50,
		"limit=0 must clamp to the default page size, not return all 100 — "+
			"otherwise a malicious client can DoS by requesting 'no limit'")
}

func TestNotifHandler_List_AuthzScopedToCaller(t *testing.T) {
	h := newNotifHarness(t)

	// Insert a notification for someone OTHER than the principal user.
	other := uuid.New()
	_, err := h.svc.CreateForUser(context.Background(), notification.Input{
		UserID: other, Type: notification.TypeInfo, Title: "secret", Message: "private",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data,
		"the principal user has no notifications of their own, so the list must be empty — "+
			"another user's notification must NOT leak. Without scoping, every user sees every user's alerts")
}

func TestNotifHandler_MarkRead_NotFound_404(t *testing.T) {
	h := newNotifHarness(t)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notifications/"+uuid.NewString()+"/read", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"marking a non-existent notification as read must 404 — the handler must not 500 or 200-with-body")
}

func TestNotifHandler_MarkAllRead_Idempotent(t *testing.T) {
	h := newNotifHarness(t)
	for i := 0; i < 3; i++ {
		_, err := h.svc.CreateForUser(context.Background(), notification.Input{
			UserID: h.userID, Type: notification.TypeInfo, Title: "t", Message: "m",
		})
		require.NoError(t, err)
	}

	first := httptest.NewRecorder()
	h.router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, first.Code,
		"first MarkAllRead returns 200/204 — accept either as long as it's success")

	second := httptest.NewRecorder()
	h.router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, second.Code,
		"calling MarkAllRead twice must be idempotent — re-marking already-read items must not 500")
}
