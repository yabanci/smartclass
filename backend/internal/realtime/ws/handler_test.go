package ws

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	mw "smartclass/internal/platform/httpx/middleware"
)

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
