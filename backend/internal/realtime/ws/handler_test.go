package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"smartclass/internal/classroom"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
)

func handlerTestBundle() *i18n.Bundle { return i18n.NewBundle(i18n.EN) }
func nopLog() *zap.Logger             { return zap.NewNop() }

// fakeAuthz lets us drive the topic-authorization decision tree without a
// live classroom service or DB. allowedClassrooms enumerates which classroom
// IDs the principal may subscribe to; everything else is denied.
type fakeAuthz struct {
	allowedClassrooms map[uuid.UUID]struct{}
}

func (f fakeAuthz) Authorize(_ context.Context, _ classroom.Principal, classroomID uuid.UUID, _ bool) error {
	if _, ok := f.allowedClassrooms[classroomID]; ok {
		return nil
	}
	return errors.New("not a member")
}

func TestAuthorizeTopics_AlwaysIncludesOwnNotificationTopic(t *testing.T) {
	h := &Handler{authz: fakeAuthz{}}
	uid := uuid.New()
	got, err := h.authorizeTopics(context.Background(), mw.Principal{UserID: uid, Role: "teacher"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"user:" + uid.String() + ":notifications"}, got,
		"a connecting client must always be subscribed to its own user-notification topic, "+
			"even if it asked for nothing else — that's how the server pushes account-level events")
}

func TestAuthorizeTopics_AcceptsClassroomTopicWhenAuthorized(t *testing.T) {
	uid := uuid.New()
	classroomID := uuid.New()
	h := &Handler{authz: fakeAuthz{allowedClassrooms: map[uuid.UUID]struct{}{classroomID: {}}}}

	got, err := h.authorizeTopics(context.Background(),
		mw.Principal{UserID: uid, Role: "teacher"},
		[]string{"classroom:" + classroomID.String() + ":devices"})
	require.NoError(t, err)
	assert.Contains(t, got, "classroom:"+classroomID.String()+":devices")
}

func TestAuthorizeTopics_RejectsClassroomTopicForOtherTenant(t *testing.T) {
	uid := uuid.New()
	mineID := uuid.New()
	theirsID := uuid.New()
	h := &Handler{authz: fakeAuthz{allowedClassrooms: map[uuid.UUID]struct{}{mineID: {}}}}

	_, err := h.authorizeTopics(context.Background(),
		mw.Principal{UserID: uid, Role: "teacher"},
		[]string{"classroom:" + theirsID.String() + ":devices"})
	require.Error(t, err,
		"subscribing to another tenant's classroom topic must fail closed — "+
			"otherwise a teacher silently observes another classroom's realtime events")
}

func TestAuthorizeTopics_RejectsForeignUserNotificationTopic(t *testing.T) {
	mine := uuid.New()
	other := uuid.New()
	h := &Handler{authz: fakeAuthz{}}

	_, err := h.authorizeTopics(context.Background(),
		mw.Principal{UserID: mine, Role: "teacher"},
		[]string{"user:" + other.String() + ":notifications"})
	require.Error(t, err,
		"the only `user:...` topic a client may subscribe to is its own; otherwise "+
			"any teacher could read another user's push-notification stream")
}

func TestAuthorizeTopics_RejectsUnknownTopicShape(t *testing.T) {
	h := &Handler{authz: fakeAuthz{}}
	_, err := h.authorizeTopics(context.Background(),
		mw.Principal{UserID: uuid.New(), Role: "teacher"},
		[]string{"random:garbage:42"})
	require.Error(t, err,
		"the topic allowlist is strict: anything outside user: / classroom: prefixes must be rejected, "+
			"so future event categories can't accidentally bypass authorization")
}

func TestAuthorizeTopics_RejectsMalformedClassroomTopic(t *testing.T) {
	h := &Handler{authz: fakeAuthz{}}
	_, err := h.authorizeTopics(context.Background(),
		mw.Principal{UserID: uuid.New(), Role: "teacher"},
		[]string{"classroom:not-a-uuid:devices"})
	require.Error(t, err,
		"a classroom topic with a non-UUID id must be rejected before it reaches the authorizer")
}

// --- Ticket-flow handshake tests ---

func TestHandler_Serve_MissingTicket_401(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}, bundle: handlerTestBundle()}

	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"WS upgrade without ?ticket= must 401 — there's no fallback to JWT in query anymore")
}

func TestHandler_Serve_UnknownTicket_401(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}, bundle: handlerTestBundle()}

	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet, "/ws?ticket=never-issued", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a ticket string the store never issued must 401 — single failure path, no info leak")
}

func TestHandler_Serve_AccessTokenInQuery_NotAccepted(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{}, bundle: handlerTestBundle()}

	// Even with a syntactically valid bearer-style string in the OLD query
	// param name, the handler must reject — the deprecated path is gone.
	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet,
		"/ws?access_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.x.y", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"the legacy ?access_token= param must NOT be honored — we deliberately removed it to stop tokens in proxy logs")
}

func TestHandler_Serve_ConsumedTicketReusedOnceMore_Fails(t *testing.T) {
	store := NewMemTicketStore(60 * time.Second)
	h := &Handler{
		tickets: store, allowedOrigs: []string{"*"}, authz: fakeAuthz{},
		bundle: handlerTestBundle(), log: nopLog(),
	}

	tkt, err := store.Issue(context.Background(), uuid.New(), "teacher")
	require.NoError(t, err)

	// First Serve will fail at the upgrader (httptest.ResponseRecorder doesn't
	// implement http.Hijacker), but it should still consume the ticket.
	first := httptest.NewRecorder()
	h.Serve(first, httptest.NewRequest(http.MethodGet, "/ws?ticket="+tkt.Raw, nil))
	// We don't assert on `first` — we care only that the ticket was consumed.

	second := httptest.NewRecorder()
	h.Serve(second, httptest.NewRequest(http.MethodGet, "/ws?ticket="+tkt.Raw, nil))
	assert.Equal(t, http.StatusUnauthorized, second.Code,
		"a ticket may only be consumed once — the second use must 401 even if the first one's upgrade itself succeeded")
}

// --- CheckOrigin matrix ---

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
