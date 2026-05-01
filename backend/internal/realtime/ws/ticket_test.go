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
