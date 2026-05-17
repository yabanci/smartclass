package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"smartclass/internal/platform/metrics"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const uniqueViolation = "23505"

func (r *PostgresRepository) Create(ctx context.Context, u *User) error {
	const q = `
INSERT INTO users (id, email, password_hash, full_name, role, language, avatar_url, phone, birth_date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	now := time.Now().UTC()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt, u.UpdatedAt = now, now
	_, err := r.pool.Exec(metrics.WithDBOp(ctx, "users.Create"), q,
		u.ID, u.Email, u.PasswordHash, u.FullName, string(u.Role),
		u.Language, u.AvatarURL, u.Phone, u.BirthDate, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == uniqueViolation {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return r.getByColumn(metrics.WithDBOp(ctx, "users.GetByID"), "id", id)
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.getByColumn(metrics.WithDBOp(ctx, "users.GetByEmail"), "email", email)
}

// getByColumn fetches a single user by a known-safe column name. The column
// parameter is restricted to an explicit allowlist to prevent SQL injection
// from future callers inadvertently passing user-controlled input.
func (r *PostgresRepository) getByColumn(ctx context.Context, column string, value any) (*User, error) {
	switch column {
	case "id", "email":
		// allowed
	default:
		return nil, fmt.Errorf("users: getByColumn: unknown column %q", column)
	}
	q := `
SELECT id, email, password_hash, full_name, role, language, avatar_url, phone, birth_date, fcm_token, created_at, updated_at
FROM users WHERE ` + column + ` = $1 LIMIT 1`
	u := &User{}
	var roleStr string
	err := r.pool.QueryRow(ctx, q, value).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &roleStr,
		&u.Language, &u.AvatarURL, &u.Phone, &u.BirthDate, &u.FCMToken, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Role = Role(roleStr)
	return u, nil
}

func (r *PostgresRepository) UpdateFCMToken(ctx context.Context, id uuid.UUID, token string) error {
	const q = `UPDATE users SET fcm_token=$2, updated_at=$3 WHERE id=$1`
	tag, err := r.pool.Exec(metrics.WithDBOp(ctx, "users.UpdateFCMToken"), q, id, token, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, u *User) error {
	const q = `
UPDATE users SET full_name=$2, language=$3, avatar_url=$4, phone=$5, birth_date=$6, role=$7, updated_at=$8
WHERE id=$1`
	u.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(metrics.WithDBOp(ctx, "users.Update"), q, u.ID, u.FullName, u.Language, u.AvatarURL, u.Phone, u.BirthDate, string(u.Role), u.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	const q = `UPDATE users SET password_hash=$2, updated_at=$3 WHERE id=$1`
	tag, err := r.pool.Exec(metrics.WithDBOp(ctx, "users.UpdatePassword"), q, id, hash, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
