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
	"smartclass/internal/platform/i18n"
)

func testBundle(t *testing.T) *i18n.Bundle {
	t.Helper()
	return i18n.NewBundle(i18n.EN)
}

func TestTicketHandler_ValidPrincipal_200(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := NewTicketHandler(store, testBundle(t))

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
	h := NewTicketHandler(store, testBundle(t))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ws/ticket", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"the route mounts inside the authenticated chi.Group, but if it's hit without principal — defense-in-depth — return 401")
}
