package tokens

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_IssueAndParse(t *testing.T) {
	j := NewJWT("super-secret-test-key", time.Minute, time.Hour, "smartclass")
	uid := uuid.New()

	pair, err := j.Issue(uid, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, pair.Access)
	require.NotEmpty(t, pair.Refresh)
	require.NotEqual(t, pair.Access, pair.Refresh)

	claims, err := j.Parse(pair.Access)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, KindAccess, claims.Kind)

	refreshClaims, err := j.Parse(pair.Refresh)
	require.NoError(t, err)
	assert.Equal(t, KindRefresh, refreshClaims.Kind)
}

func TestJWT_RejectsExpiredToken(t *testing.T) {
	frozen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	j := NewJWT("secret-secret", time.Minute, time.Minute, "iss", WithClock(func() time.Time { return frozen }))

	pair, err := j.Issue(uuid.New(), "user")
	require.NoError(t, err)

	j2 := NewJWT("secret-secret", time.Minute, time.Minute, "iss", WithClock(func() time.Time { return frozen.Add(2 * time.Minute) }))
	_, err = j2.Parse(pair.Access)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWT_RejectsWrongSecret(t *testing.T) {
	a := NewJWT("aaa-aaa-aaa", time.Minute, time.Minute, "iss")
	b := NewJWT("bbb-bbb-bbb", time.Minute, time.Minute, "iss")

	pair, err := a.Issue(uuid.New(), "user")
	require.NoError(t, err)
	_, err = b.Parse(pair.Access)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWT_RejectsGarbage(t *testing.T) {
	j := NewJWT("secret-secret", time.Minute, time.Minute, "iss")
	_, err := j.Parse("not-a-real-token")
	require.ErrorIs(t, err, ErrInvalidToken)
}
