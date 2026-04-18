package hass

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) Repository { return &pgRepo{pool: pool} }

func (r *pgRepo) Load(ctx context.Context) (*Credentials, error) {
	row := r.pool.QueryRow(ctx, `SELECT base_url, token, refresh_token, expires_at, onboarded, updated_at FROM hass_config WHERE id = 1`)
	c := &Credentials{}
	var expires *time.Time
	if err := row.Scan(&c.BaseURL, &c.Token, &c.RefreshToken, &expires, &c.Onboarded, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNoRow
		}
		return nil, err
	}
	if expires != nil {
		c.ExpiresAt = *expires
	}
	return c, nil
}

func (r *pgRepo) Save(ctx context.Context, c *Credentials) error {
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().UTC()
	}
	var expires any
	if !c.ExpiresAt.IsZero() {
		expires = c.ExpiresAt
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO hass_config (id, base_url, token, refresh_token, expires_at, onboarded, updated_at)
VALUES (1, $1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
  base_url = EXCLUDED.base_url,
  token = EXCLUDED.token,
  refresh_token = EXCLUDED.refresh_token,
  expires_at = EXCLUDED.expires_at,
  onboarded = EXCLUDED.onboarded,
  updated_at = EXCLUDED.updated_at
`, c.BaseURL, c.Token, c.RefreshToken, expires, c.Onboarded, c.UpdatedAt)
	return err
}
