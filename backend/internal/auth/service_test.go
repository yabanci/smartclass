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

func newSvc(t *testing.T) (*auth.Service, *usertest.MemRepo) {
	t.Helper()
	repo := usertest.NewMemRepo()
	h := hasher.NewBcrypt(4)
	iss := tokens.NewJWT("test-secret-key-1234567890", time.Minute, time.Hour, "test")
	return auth.NewService(repo, h, iss), repo
}

func TestService_Register(t *testing.T) {
	svc, repo := newSvc(t)

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
	svc, _ := newSvc(t)

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

func TestService_Refresh(t *testing.T) {
	svc, repo := newSvc(t)

	res, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "r@example.com", Password: "password1", FullName: "R", Role: user.RoleAdmin,
	})
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		refreshed, err := svc.Refresh(context.Background(), res.Tokens.Refresh)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshed.Tokens.Access)
		assert.Equal(t, res.User.ID, refreshed.User.ID)
	})

	t.Run("access token rejected as refresh", func(t *testing.T) {
		_, err := svc.Refresh(context.Background(), res.Tokens.Access)
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})

	t.Run("garbage", func(t *testing.T) {
		_, err := svc.Refresh(context.Background(), "garbage")
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})

	t.Run("deleted user", func(t *testing.T) {
		repo.Delete(res.User.ID)
		_, err := svc.Refresh(context.Background(), res.Tokens.Refresh)
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})
}
