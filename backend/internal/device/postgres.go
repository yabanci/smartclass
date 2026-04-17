package device

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, d *Device) error {
	const q = `
INSERT INTO devices (id, classroom_id, name, type, brand, driver, config, status, online, last_seen_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	d.CreatedAt, d.UpdatedAt = now, now
	cfg, err := encodeJSON(d.Config)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, q,
		d.ID, d.ClassroomID, d.Name, d.Type, d.Brand, d.Driver, cfg,
		d.Status, d.Online, d.LastSeenAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Device, error) {
	const q = `
SELECT id, classroom_id, name, type, brand, driver, config, status, online, last_seen_at, created_at, updated_at
FROM devices WHERE id=$1`
	return r.scanOne(r.pool.QueryRow(ctx, q, id))
}

func (r *PostgresRepository) ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Device, error) {
	const q = `
SELECT id, classroom_id, name, type, brand, driver, config, status, online, last_seen_at, created_at, updated_at
FROM devices WHERE classroom_id=$1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		d, err := r.scanOne(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dst ...any) error
}

func (r *PostgresRepository) scanOne(s scanner) (*Device, error) {
	d := &Device{}
	var cfg []byte
	err := s.Scan(&d.ID, &d.ClassroomID, &d.Name, &d.Type, &d.Brand, &d.Driver,
		&cfg, &d.Status, &d.Online, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	d.Config, err = decodeJSON(cfg)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *PostgresRepository) Update(ctx context.Context, d *Device) error {
	const q = `
UPDATE devices SET name=$2, type=$3, brand=$4, driver=$5, config=$6, updated_at=$7 WHERE id=$1`
	d.UpdatedAt = time.Now().UTC()
	cfg, err := encodeJSON(d.Config)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, q, d.ID, d.Name, d.Type, d.Brand, d.Driver, cfg, d.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdateState(ctx context.Context, id uuid.UUID, status string, online bool, lastSeen *time.Time) error {
	const q = `UPDATE devices SET status=$2, online=$3, last_seen_at=$4, updated_at=$5 WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, status, online, lastSeen, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM devices WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func encodeJSON(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func decodeJSON(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}
