package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/hasher"
	"smartclass/internal/user"
	"smartclass/internal/user/usertest"
)

func newTestSvc(t *testing.T) (*user.Service, *usertest.MemRepo, *hasher.Bcrypt) {
	t.Helper()
	repo := usertest.NewMemRepo()
	h := hasher.NewBcrypt(4)
	return user.NewService(repo, h), repo, h
}

func seedUser(t *testing.T, repo *usertest.MemRepo, h *hasher.Bcrypt, password string) *user.User {
	t.Helper()
	hash, err := h.Hash(password)
	require.NoError(t, err)
	u := &user.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: hash,
		FullName:     "Test User",
		Role:         user.RoleTeacher,
		Language:     "en",
	}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func TestService_Get(t *testing.T) {
	svc, repo, h := newTestSvc(t)
	seeded := seedUser(t, repo, h, "pw-pw-pw-pw")

	t.Run("found", func(t *testing.T) {
		u, err := svc.Get(context.Background(), seeded.ID)
		require.NoError(t, err)
		assert.Equal(t, seeded.Email, u.Email)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Get(context.Background(), uuid.New())
		require.Error(t, err)
		assert.ErrorIs(t, err, user.ErrDomainNotFound)
	})
}

func TestService_UpdateProfile(t *testing.T) {
	svc, repo, h := newTestSvc(t)
	seeded := seedUser(t, repo, h, "pw-pw-pw-pw")

	name := "New Name"
	lang := "ru"
	u, err := svc.UpdateProfile(context.Background(), seeded.ID, user.UpdateProfileInput{
		FullName: &name,
		Language: &lang,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", u.FullName)
	assert.Equal(t, "ru", u.Language)
}

func TestService_ChangePassword(t *testing.T) {
	svc, repo, h := newTestSvc(t)
	seeded := seedUser(t, repo, h, "old-password-1")

	cases := []struct {
		name        string
		current     string
		next        string
		expectedErr error
	}{
		{"ok", "old-password-1", "new-password-1", nil},
		{"weak", "old-password-1", "short", user.ErrWeakPassword},
		{"wrong current", "wrong-one-1", "new-password-1", user.ErrPasswordMismatch},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := svc.ChangePassword(context.Background(), seeded.ID, c.current, c.next)
			if c.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, c.expectedErr), "got %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
