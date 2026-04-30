package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRefreshStore is the single source of truth for refresh-token state.
// Schema is defined in migrations/00013_refresh_tokens.sql.
type PostgresRefreshStore struct {
	pool *pgxpool.Pool
}

func NewPostgresRefreshStore(pool *pgxpool.Pool) *PostgresRefreshStore {
	return &PostgresRefreshStore{pool: pool}
}

func (s *PostgresRefreshStore) Track(ctx context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error {
	const q = `INSERT INTO refresh_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3)`
	if _, err := s.pool.Exec(ctx, q, jti, userID, expiresAt); err != nil {
		return fmt.Errorf("auth: track refresh: %w", err)
	}
	return nil
}

func (s *PostgresRefreshStore) Status(ctx context.Context, jti uuid.UUID) (RefreshStatus, error) {
	const q = `SELECT user_id, expires_at, used_at, revoked_at FROM refresh_tokens WHERE jti = $1`
	row := s.pool.QueryRow(ctx, q, jti)
	var st RefreshStatus
	if err := row.Scan(&st.UserID, &st.ExpiresAt, &st.UsedAt, &st.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshStatus{}, ErrRefreshUnknown
		}
		return RefreshStatus{}, fmt.Errorf("auth: status refresh: %w", err)
	}
	return st, nil
}

func (s *PostgresRefreshStore) MarkUsed(ctx context.Context, jti uuid.UUID) error {
	// The WHERE clause enforces once-only redemption at the SQL layer:
	// `used_at IS NULL` makes the second concurrent caller see 0 rows
	// affected, which we map to ErrRefreshAlreadyUsed. This is what powers
	// replay detection.
	const q = `UPDATE refresh_tokens SET used_at = now() WHERE jti = $1 AND used_at IS NULL AND revoked_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, jti)
	if err != nil {
		return fmt.Errorf("auth: mark used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRefreshAlreadyUsed
	}
	return nil
}

func (s *PostgresRefreshStore) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now()
	            WHERE user_id = $1 AND revoked_at IS NULL AND used_at IS NULL`
	if _, err := s.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("auth: revoke user: %w", err)
	}
	return nil
}
