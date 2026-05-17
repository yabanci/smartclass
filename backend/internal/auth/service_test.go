package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/auth"
	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/user"
	"smartclass/internal/user/usertest"
)

func newSvc(t *testing.T) (*auth.Service, *usertest.MemRepo, *auth.MemRefreshStore) {
	t.Helper()
	repo := usertest.NewMemRepo()
	h := hasher.NewBcrypt(4)
	iss := tokens.NewJWT("test-secret-key-1234567890", time.Minute, time.Hour, "test")
	store := auth.NewMemRefreshStore()
	return auth.NewService(repo, h, iss, store, nil), repo, store
}

func TestService_Register(t *testing.T) {
	svc, repo, _ := newSvc(t)

	t.Run("creates user and issues tokens", func(t *testing.T) {
		res, err := svc.Register(context.Background(), auth.RegisterInput{
			Email:    "A@Example.COM",
			Password: "password1",
			FullName: "Alice",
			Role:     user.RoleTeacher,
			Language: "ru",
		})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "a@example.com", res.User.Email, "email normalized")
		assert.Equal(t, user.RoleTeacher, res.User.Role)
		assert.NotEmpty(t, res.Tokens.Access)
		assert.NotEmpty(t, res.Tokens.Refresh)

		stored, err := repo.GetByEmail(context.Background(), "a@example.com")
		require.NoError(t, err)
		assert.NotEqual(t, "password1", stored.PasswordHash, "password hashed")
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		_, err := svc.Register(context.Background(), auth.RegisterInput{
			Email:    "a@example.com",
			Password: "password1",
			FullName: "Bob",
			Role:     user.RoleAdmin,
		})
		require.ErrorIs(t, err, auth.ErrEmailTaken)
	})

	t.Run("rejects invalid role", func(t *testing.T) {
		_, err := svc.Register(context.Background(), auth.RegisterInput{
			Email:    "b@example.com",
			Password: "password1",
			FullName: "Bob",
			Role:     user.Role("pirate"),
		})
		require.Error(t, err)
	})
}

func TestService_Login(t *testing.T) {
	svc, _, _ := newSvc(t)

	_, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "l@example.com", Password: "password1", FullName: "L", Role: user.RoleTeacher,
	})
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		res, err := svc.Login(context.Background(), "L@Example.com", "password1")
		require.NoError(t, err)
		assert.Equal(t, "l@example.com", res.User.Email)
		assert.NotEmpty(t, res.Tokens.Access)
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := svc.Login(context.Background(), "l@example.com", "wrong-password")
		require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})

	t.Run("unknown email", func(t *testing.T) {
		_, err := svc.Login(context.Background(), "nobody@example.com", "whatever")
		require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})
}

func TestService_Refresh_RotatesAndDetectsReplay(t *testing.T) {
	svc, _, store := newSvc(t)

	res, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "r@example.com", Password: "password1", FullName: "R", Role: user.RoleAdmin,
	})
	require.NoError(t, err)
	originalRefresh := res.Tokens.Refresh

	t.Run("first use rotates the pair", func(t *testing.T) {
		refreshed, err := svc.Refresh(context.Background(), originalRefresh)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshed.Tokens.Access)
		assert.NotEqual(t, originalRefresh, refreshed.Tokens.Refresh,
			"a successful refresh must yield a new refresh token, not echo the consumed one")
		assert.Equal(t, res.User.ID, refreshed.User.ID)
		assert.Equal(t, 1, store.CountLive(res.User.ID),
			"after rotation exactly one refresh token (the new one) is live")
	})

	t.Run("second use of the same token is a replay and revokes all sessions", func(t *testing.T) {
		_, err := svc.Refresh(context.Background(), originalRefresh)
		require.ErrorIs(t, err, auth.ErrInvalidRefresh,
			"presenting the same refresh token twice must fail authentication")
		assert.Equal(t, 0, store.CountLive(res.User.ID),
			"replay detection must revoke every other live refresh token for the user — "+
				"the legitimate user logs in again, the attacker is locked out")
	})
}

func TestService_Refresh_RejectsAccessToken(t *testing.T) {
	svc, _, _ := newSvc(t)

	res, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "a@example.com", Password: "password1", FullName: "A", Role: user.RoleAdmin,
	})
	require.NoError(t, err)

	_, err = svc.Refresh(context.Background(), res.Tokens.Access)
	require.ErrorIs(t, err, auth.ErrInvalidRefresh,
		"access tokens must not be accepted on /refresh — the kind claim is the discriminator")
}

func TestService_Refresh_RejectsGarbage(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Refresh(context.Background(), "garbage")
	require.ErrorIs(t, err, auth.ErrInvalidRefresh)
}

func TestService_Refresh_RejectsDeletedUser(t *testing.T) {
	svc, repo, _ := newSvc(t)
	res, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "d@example.com", Password: "password1", FullName: "D", Role: user.RoleAdmin,
	})
	require.NoError(t, err)
	repo.Delete(res.User.ID)

	_, err = svc.Refresh(context.Background(), res.Tokens.Refresh)
	require.ErrorIs(t, err, auth.ErrInvalidRefresh,
		"a refresh token must fail once the underlying user has been deleted")
}

func TestService_Logout_RevokesEveryLiveRefreshToken(t *testing.T) {
	svc, _, store := newSvc(t)

	// Two parallel sessions for the same user (two devices).
	r1, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "lo@example.com", Password: "password1", FullName: "Lo", Role: user.RoleTeacher,
	})
	require.NoError(t, err)
	r2, err := svc.Login(context.Background(), "lo@example.com", "password1")
	require.NoError(t, err)
	require.Equal(t, 2, store.CountLive(r1.User.ID))

	require.NoError(t, svc.Logout(context.Background(), r1.User.ID))
	assert.Equal(t, 0, store.CountLive(r1.User.ID),
		"Logout must revoke every still-live refresh token belonging to the user "+
			"so a stolen token can be killed without waiting for its TTL to expire")

	_, err = svc.Refresh(context.Background(), r2.Tokens.Refresh)
	require.ErrorIs(t, err, auth.ErrInvalidRefresh,
		"a refresh token explicitly revoked by Logout must be rejected")
}
