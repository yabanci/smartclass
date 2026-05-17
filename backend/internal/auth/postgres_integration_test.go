//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/auth"
	"smartclass/internal/platform/testsupport"
	"smartclass/internal/user"
)

// insertTestUser inserts a minimal user row to satisfy the refresh_tokens FK.
func insertTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	const q = `INSERT INTO users (id, email, password_hash, full_name, role)
	            VALUES ($1, $2, $3, $4, $5)`
	_, err := pool.Exec(ctx, q,
		id, id.String()+"@test.local", "bcrypt-placeholder", "Test User", string(user.RoleTeacher),
	)
	require.NoError(t, err)
}

func TestPostgresRefreshStore_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	store := auth.NewPostgresRefreshStore(pool)

	t.Run("Track then Status returns live token", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		userID := uuid.New()
		insertTestUser(t, ctx, pool, userID)
		jti := uuid.New()
		exp := time.Now().Add(time.Hour).UTC()

		require.NoError(t, store.Track(ctx, jti, userID, exp))

		st, err := store.Status(ctx, jti)
		require.NoError(t, err)
		assert.Equal(t, userID, st.UserID)
		assert.Nil(t, st.UsedAt)
		assert.Nil(t, st.RevokedAt)
		assert.True(t, st.IsLive(time.Now()))
	})

	t.Run("Status on unknown jti returns ErrRefreshUnknown", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		_, err := store.Status(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrRefreshUnknown)
	})

	t.Run("MarkUsed marks token used; second call returns ErrRefreshAlreadyUsed", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		userID := uuid.New()
		insertTestUser(t, ctx, pool, userID)
		jti := uuid.New()

		require.NoError(t, store.Track(ctx, jti, userID, time.Now().Add(time.Hour)))
		require.NoError(t, store.MarkUsed(ctx, jti))

		st, err := store.Status(ctx, jti)
		require.NoError(t, err)
		assert.True(t, st.IsUsed())
		assert.False(t, st.IsLive(time.Now()))

		// Replay attempt — must fail.
		assert.ErrorIs(t, store.MarkUsed(ctx, jti), auth.ErrRefreshAlreadyUsed)
	})

	t.Run("RevokeUser invalidates all live tokens for a user", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		userID := uuid.New()
		insertTestUser(t, ctx, pool, userID)

		jti1, jti2 := uuid.New(), uuid.New()
		require.NoError(t, store.Track(ctx, jti1, userID, time.Now().Add(time.Hour)))
		require.NoError(t, store.Track(ctx, jti2, userID, time.Now().Add(time.Hour)))

		require.NoError(t, store.RevokeUser(ctx, userID))

		for _, jti := range []uuid.UUID{jti1, jti2} {
			st, err := store.Status(ctx, jti)
			require.NoError(t, err)
			assert.NotNil(t, st.RevokedAt, "token %s must be revoked", jti)
			assert.False(t, st.IsLive(time.Now()))
		}

		// Revoked tokens cannot be redeemed.
		assert.ErrorIs(t, store.MarkUsed(ctx, jti1), auth.ErrRefreshAlreadyUsed)
	})

	t.Run("RevokeUser is idempotent with no live tokens", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		userID := uuid.New()
		insertTestUser(t, ctx, pool, userID)

		// No tokens — must not error.
		assert.NoError(t, store.RevokeUser(ctx, userID))
	})

	t.Run("RevokeUser WHERE clause excludes already-used tokens", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		userID := uuid.New()
		insertTestUser(t, ctx, pool, userID)
		jti := uuid.New()

		require.NoError(t, store.Track(ctx, jti, userID, time.Now().Add(time.Hour)))
		require.NoError(t, store.MarkUsed(ctx, jti))
		// used_at IS NOT NULL — RevokeUser WHERE clause skips it.
		require.NoError(t, store.RevokeUser(ctx, userID))

		st, err := store.Status(ctx, jti)
		require.NoError(t, err)
		assert.NotNil(t, st.UsedAt)
		assert.Nil(t, st.RevokedAt, "used token must not be double-revoked")
	})
}
