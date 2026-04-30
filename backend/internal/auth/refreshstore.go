package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// RefreshStore is the persistence layer that turns stateless JWT refresh
// tokens into rotatable, revocable, replay-detectable credentials. Every
// issued refresh-jti must be Tracked; every consumed refresh-jti must be
// MarkUsed before a new pair is issued; explicit logout calls RevokeUser.
type RefreshStore interface {
	// Track records a freshly-issued refresh token. user_id and expires_at are
	// indexed for cleanup and for RevokeUser.
	Track(ctx context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error

	// Status reports the current state of a refresh-jti so callers can
	// distinguish "valid", "already used", and "revoked or unknown" with one
	// round-trip. Returning ErrRefreshUnknown lets callers map to a generic
	// 401 without leaking whether they had the right shape but wrong id.
	Status(ctx context.Context, jti uuid.UUID) (RefreshStatus, error)

	// MarkUsed transitions a token to "used" exactly once. The implementation
	// must enforce the once-only contract — a second MarkUsed for the same
	// jti returns ErrRefreshAlreadyUsed, which is the signal an auth server
	// uses to detect replay and torch all of that user's other refresh
	// tokens.
	MarkUsed(ctx context.Context, jti uuid.UUID) error

	// RevokeUser invalidates every still-valid refresh token for a user.
	// Used on logout and on replay detection. Idempotent: calling it on a
	// user with no live tokens is a no-op.
	RevokeUser(ctx context.Context, userID uuid.UUID) error
}

// RefreshStatus is the per-jti state visible to the auth service.
type RefreshStatus struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

// IsLive reports whether a refresh token is still spendable: it must exist,
// not be expired, not be used, not be revoked.
func (s RefreshStatus) IsLive(now time.Time) bool {
	if s.UsedAt != nil || s.RevokedAt != nil {
		return false
	}
	return now.Before(s.ExpiresAt)
}

// IsUsed reports whether a refresh token was already redeemed (the replay
// signal — a stolen token being presented after the legitimate user already
// rotated it).
func (s RefreshStatus) IsUsed() bool { return s.UsedAt != nil }

// ErrRefreshUnknown is returned by Status when no row matches. The caller
// should treat this as a generic 401 — it intentionally collapses
// "never-issued" and "expired-and-cleaned-up" into one signal.
var ErrRefreshUnknown = errors.New("auth: refresh token unknown")

// ErrRefreshAlreadyUsed is returned by MarkUsed when the token had already
// been redeemed. Callers must treat this as a replay attempt and revoke the
// affected user's other refresh tokens.
var ErrRefreshAlreadyUsed = errors.New("auth: refresh token already used")
