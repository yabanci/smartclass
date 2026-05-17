package devicetoken_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicetoken"
	"smartclass/internal/devicetoken/devicetokentest"
)

func TestService_Register_SavesToken(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	tok, err := svc.Register(context.Background(), userID, "tok-abc", devicetoken.PlatformAndroid)
	require.NoError(t, err)

	assert.Equal(t, userID, tok.UserID)
	assert.Equal(t, "tok-abc", tok.Token)
	assert.Equal(t, devicetoken.PlatformAndroid, tok.Platform)
	assert.NotEqual(t, uuid.Nil, tok.ID)
	assert.False(t, tok.CreatedAt.IsZero())
}

func TestService_Register_UpsertBumpsLastSeen(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	first, err := svc.Register(context.Background(), userID, "same-tok", devicetoken.PlatformIOS)
	require.NoError(t, err)

	second, err := svc.Register(context.Background(), userID, "same-tok", devicetoken.PlatformIOS)
	require.NoError(t, err)

	// Same token → same row, ID preserved.
	assert.Equal(t, first.ID, second.ID,
		"upsert must not create a second row for the same (user_id, token) pair")

	// last_seen_at must not regress.
	assert.False(t, second.LastSeenAt.Before(first.LastSeenAt),
		"re-registering the same token must bump last_seen_at")
}

func TestService_Unregister_RemovesToken(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	_, err := svc.Register(context.Background(), userID, "tok-del", devicetoken.PlatformWeb)
	require.NoError(t, err)

	require.NoError(t, svc.Unregister(context.Background(), userID, "tok-del"))

	tokens, err := svc.GetByUser(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, tokens,
		"after Unregister, ListByUser must return no tokens for this user")
}

func TestService_Unregister_Idempotent(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	userID := uuid.New()
	err := svc.Unregister(context.Background(), userID, "nonexistent")
	assert.NoError(t, err,
		"unregistering a token that does not exist must not error — callers must be able to call it safely on logout")
}

func TestService_GetByUser_ScopedToUser(t *testing.T) {
	repo := devicetokentest.NewMemRepo()
	svc := devicetoken.NewService(repo)

	uid1 := uuid.New()
	uid2 := uuid.New()
	_, err := svc.Register(context.Background(), uid1, "tok-u1", devicetoken.PlatformAndroid)
	require.NoError(t, err)
	_, err = svc.Register(context.Background(), uid2, "tok-u2", devicetoken.PlatformWeb)
	require.NoError(t, err)

	tokens, err := svc.GetByUser(context.Background(), uid1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, "tok-u1", tokens[0].Token,
		"GetByUser must only return tokens belonging to the requested user")
}
