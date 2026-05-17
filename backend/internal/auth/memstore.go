package auth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemRefreshStore is an in-memory RefreshStore for unit tests. Production
// code uses PostgresRefreshStore. The semantics match the SQL implementation:
// MarkUsed is once-only and returns ErrRefreshAlreadyUsed on the second
// call; RevokeUser is idempotent.
type MemRefreshStore struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]*RefreshStatus
}

func NewMemRefreshStore() *MemRefreshStore {
	return &MemRefreshStore{tokens: map[uuid.UUID]*RefreshStatus{}}
}

func (m *MemRefreshStore) Track(_ context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[jti] = &RefreshStatus{UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (m *MemRefreshStore) Status(_ context.Context, jti uuid.UUID) (RefreshStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.tokens[jti]
	if !ok {
		return RefreshStatus{}, ErrRefreshUnknown
	}
	// Return a copy so callers can't mutate the store.
	return *st, nil
}

func (m *MemRefreshStore) MarkUsed(_ context.Context, jti uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.tokens[jti]
	if !ok {
		return ErrRefreshUnknown
	}
	if st.UsedAt != nil || st.RevokedAt != nil {
		return ErrRefreshAlreadyUsed
	}
	now := time.Now()
	st.UsedAt = &now
	return nil
}

func (m *MemRefreshStore) RevokeUser(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, st := range m.tokens {
		if st.UserID == userID && st.UsedAt == nil && st.RevokedAt == nil {
			st.RevokedAt = &now
		}
	}
	return nil
}

// CountLive returns the number of refresh tokens for userID that are still
// valid (not used, not revoked, not expired). Test-only helper.
func (m *MemRefreshStore) CountLive(userID uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	n := 0
	for _, st := range m.tokens {
		if st.UserID == userID && st.IsLive(now) {
			n++
		}
	}
	return n
}
