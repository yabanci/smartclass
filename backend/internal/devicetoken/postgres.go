package devicetoken

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"smartclass/internal/platform/metrics"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Save upserts a device token. On conflict (user_id, token) it bumps last_seen_at.
func (r *PostgresRepository) Save(ctx context.Context, t *Token) error {
	const q = `
INSERT INTO device_tokens (id, user_id, token, platform, created_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, token)
DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
RETURNING id, created_at, last_seen_at`

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.LastSeenAt = now

	return r.pool.QueryRow(metrics.WithDBOp(ctx, "devicetoken.Save"), q,
		t.ID, t.UserID, t.Token, string(t.Platform), t.CreatedAt, t.LastSeenAt,
	).Scan(&t.ID, &t.CreatedAt, &t.LastSeenAt)
}

func (r *PostgresRepository) DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error {
	_, err := r.pool.Exec(metrics.WithDBOp(ctx, "devicetoken.DeleteByToken"),
		"DELETE FROM device_tokens WHERE user_id=$1 AND token=$2",
		userID, token)
	return err
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error) {
	const q = `
SELECT id, user_id, token, platform, created_at, last_seen_at
FROM device_tokens
WHERE user_id=$1
ORDER BY last_seen_at DESC`

	rows, err := r.pool.Query(metrics.WithDBOp(ctx, "devicetoken.ListByUser"), q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Token
	for rows.Next() {
		tk := &Token{}
		var platform string
		if err := rows.Scan(&tk.ID, &tk.UserID, &tk.Token, &platform,
			&tk.CreatedAt, &tk.LastSeenAt); err != nil {
			return nil, err
		}
		tk.Platform = Platform(platform)
		out = append(out, tk)
	}
	return out, rows.Err()
}
