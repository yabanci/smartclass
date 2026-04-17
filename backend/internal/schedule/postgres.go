package schedule

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

func (r *PostgresRepository) Create(ctx context.Context, l *Lesson) error {
	const q = `
INSERT INTO lessons (id, classroom_id, subject, teacher_id, day_of_week, starts_at, ends_at, notes, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	now := time.Now().UTC()
	l.CreatedAt, l.UpdatedAt = now, now
	_, err := r.pool.Exec(ctx, q,
		l.ID, l.ClassroomID, l.Subject, l.TeacherID,
		int(l.DayOfWeek), minutesToPgTime(l.StartsAt), minutesToPgTime(l.EndsAt),
		l.Notes, l.CreatedAt, l.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Lesson, error) {
	const q = `SELECT id, classroom_id, subject, teacher_id, day_of_week, starts_at, ends_at, notes, created_at, updated_at FROM lessons WHERE id=$1`
	return r.scan(r.pool.QueryRow(ctx, q, id))
}

func (r *PostgresRepository) ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Lesson, error) {
	const q = `SELECT id, classroom_id, subject, teacher_id, day_of_week, starts_at, ends_at, notes, created_at, updated_at
FROM lessons WHERE classroom_id=$1 ORDER BY day_of_week, starts_at`
	rows, err := r.pool.Query(ctx, q, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.collect(rows)
}

func (r *PostgresRepository) ListByClassroomAndDay(ctx context.Context, classroomID uuid.UUID, day DayOfWeek) ([]*Lesson, error) {
	const q = `SELECT id, classroom_id, subject, teacher_id, day_of_week, starts_at, ends_at, notes, created_at, updated_at
FROM lessons WHERE classroom_id=$1 AND day_of_week=$2 ORDER BY starts_at`
	rows, err := r.pool.Query(ctx, q, classroomID, int(day))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.collect(rows)
}

func (r *PostgresRepository) Update(ctx context.Context, l *Lesson) error {
	const q = `UPDATE lessons SET subject=$2, teacher_id=$3, day_of_week=$4, starts_at=$5, ends_at=$6, notes=$7, updated_at=$8 WHERE id=$1`
	l.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q, l.ID, l.Subject, l.TeacherID, int(l.DayOfWeek),
		minutesToPgTime(l.StartsAt), minutesToPgTime(l.EndsAt), l.Notes, l.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM lessons WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id)
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

func (r *PostgresRepository) scan(s scanner) (*Lesson, error) {
	l := &Lesson{}
	var day int
	var starts, ends time.Time
	err := s.Scan(&l.ID, &l.ClassroomID, &l.Subject, &l.TeacherID, &day, &starts, &ends, &l.Notes, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	l.DayOfWeek = DayOfWeek(day)
	l.StartsAt = TimeOfDay(starts.Hour()*60 + starts.Minute())
	l.EndsAt = TimeOfDay(ends.Hour()*60 + ends.Minute())
	return l, nil
}

func (r *PostgresRepository) collect(rows pgx.Rows) ([]*Lesson, error) {
	var out []*Lesson
	for rows.Next() {
		l, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func minutesToPgTime(t TimeOfDay) time.Time {
	return time.Date(0, 1, 1, int(t)/60, int(t)%60, 0, 0, time.UTC)
}
