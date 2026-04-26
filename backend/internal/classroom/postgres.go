package classroom

import (
	"context"
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

func (r *PostgresRepository) Create(ctx context.Context, c *Classroom) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const insertClassroom = `INSERT INTO classrooms (id, name, description, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, insertClassroom, c.ID, c.Name, c.Description, c.CreatedBy, c.CreatedAt, c.UpdatedAt); err != nil {
		return err
	}
	// Auto-assign the creator atomically so they always appear in Members().
	const insertMember = `INSERT INTO classroom_users (classroom_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := tx.Exec(ctx, insertMember, c.ID, c.CreatedBy); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Classroom, error) {
	const q = `SELECT id, name, description, created_by, created_at, updated_at FROM classrooms WHERE id=$1`
	c := &Classroom{}
	err := r.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.Name, &c.Description, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]*Classroom, error) {
	const q = `SELECT id, name, description, created_by, created_at, updated_at FROM classrooms ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	return r.queryList(ctx, q, limit, offset)
}

func (r *PostgresRepository) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Classroom, error) {
	const q = `
SELECT c.id, c.name, c.description, c.created_by, c.created_at, c.updated_at
FROM classrooms c
JOIN classroom_users cu ON cu.classroom_id = c.id
WHERE cu.user_id = $1
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3`
	return r.queryList(ctx, q, userID, limit, offset)
}

func (r *PostgresRepository) queryList(ctx context.Context, q string, args ...any) ([]*Classroom, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Classroom
	for rows.Next() {
		c := &Classroom{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, c *Classroom) error {
	const q = `UPDATE classrooms SET name=$2, description=$3, updated_at=$4 WHERE id=$1`
	c.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q, c.ID, c.Name, c.Description, c.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM classrooms WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Assign(ctx context.Context, classroomID, userID uuid.UUID) error {
	const q = `INSERT INTO classroom_users (classroom_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.pool.Exec(ctx, q, classroomID, userID)
	return err
}

func (r *PostgresRepository) Unassign(ctx context.Context, classroomID, userID uuid.UUID) error {
	const q = `DELETE FROM classroom_users WHERE classroom_id=$1 AND user_id=$2`
	_, err := r.pool.Exec(ctx, q, classroomID, userID)
	return err
}

func (r *PostgresRepository) IsMember(ctx context.Context, classroomID, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM classroom_users WHERE classroom_id=$1 AND user_id=$2)`
	var ok bool
	err := r.pool.QueryRow(ctx, q, classroomID, userID).Scan(&ok)
	return ok, err
}

func (r *PostgresRepository) Members(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	const q = `SELECT user_id FROM classroom_users WHERE classroom_id=$1 ORDER BY assigned_at`
	rows, err := r.pool.Query(ctx, q, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
