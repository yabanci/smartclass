package scene

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"smartclass/internal/platform/metrics"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, s *Scene) error {
	const q = `INSERT INTO scenes (id, classroom_id, name, description, steps, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	s.CreatedAt, s.UpdatedAt = now, now
	steps, err := json.Marshal(s.Steps)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(metrics.WithDBOp(ctx, "scene.Create"), q, s.ID, s.ClassroomID, s.Name, s.Description, steps, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Scene, error) {
	const q = `SELECT id, classroom_id, name, description, steps, created_at, updated_at FROM scenes WHERE id=$1`
	return r.scan(r.pool.QueryRow(metrics.WithDBOp(ctx, "scene.GetByID"), q, id))
}

func (r *PostgresRepository) ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Scene, error) {
	const q = `SELECT id, classroom_id, name, description, steps, created_at, updated_at FROM scenes WHERE classroom_id=$1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(metrics.WithDBOp(ctx, "scene.ListByClassroom"), q, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Scene
	for rows.Next() {
		s, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, s *Scene) error {
	const q = `UPDATE scenes SET name=$2, description=$3, steps=$4, updated_at=$5 WHERE id=$1`
	s.UpdatedAt = time.Now().UTC()
	steps, err := json.Marshal(s.Steps)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(metrics.WithDBOp(ctx, "scene.Update"), q, s.ID, s.Name, s.Description, steps, s.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM scenes WHERE id=$1`
	tag, err := r.pool.Exec(metrics.WithDBOp(ctx, "scene.Delete"), q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dst ...any) error
}

func (r *PostgresRepository) scan(s scanner) (*Scene, error) {
	out := &Scene{}
	var steps []byte
	err := s.Scan(&out.ID, &out.ClassroomID, &out.Name, &out.Description, &steps, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(steps) > 0 {
		if err := json.Unmarshal(steps, &out.Steps); err != nil {
			return nil, err
		}
	}
	return out, nil
}
