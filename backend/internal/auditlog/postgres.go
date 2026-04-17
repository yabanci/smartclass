package auditlog

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	const cols = 6
	args := make([]any, 0, len(entries)*cols)
	values := make([]string, 0, len(entries))
	for i, e := range entries {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now().UTC()
		}
		meta, err := json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
		base := i * cols
		values = append(values,
			"($"+strconv.Itoa(base+1)+",$"+strconv.Itoa(base+2)+",$"+strconv.Itoa(base+3)+",$"+strconv.Itoa(base+4)+",$"+strconv.Itoa(base+5)+",$"+strconv.Itoa(base+6)+")")
		args = append(args, e.ActorID, string(e.EntityType), e.EntityID, string(e.Action), meta, e.CreatedAt)
	}
	q := "INSERT INTO action_logs (actor_id, entity_type, entity_id, action, metadata, created_at) VALUES " + strings.Join(values, ",")
	_, err := r.pool.Exec(ctx, q, args...)
	return err
}

func (r *PostgresRepository) List(ctx context.Context, q Query) ([]Entry, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT id, actor_id, entity_type, entity_id, action, metadata, created_at FROM action_logs WHERE 1=1")
	args := []any{}
	if q.ActorID != nil {
		args = append(args, *q.ActorID)
		sb.WriteString(" AND actor_id=$" + strconv.Itoa(len(args)))
	}
	if q.EntityType != "" {
		args = append(args, string(q.EntityType))
		sb.WriteString(" AND entity_type=$" + strconv.Itoa(len(args)))
	}
	if q.EntityID != nil {
		args = append(args, *q.EntityID)
		sb.WriteString(" AND entity_id=$" + strconv.Itoa(len(args)))
	}
	if q.Action != "" {
		args = append(args, string(q.Action))
		sb.WriteString(" AND action=$" + strconv.Itoa(len(args)))
	}
	if q.From != nil {
		args = append(args, *q.From)
		sb.WriteString(" AND created_at >= $" + strconv.Itoa(len(args)))
	}
	if q.To != nil {
		args = append(args, *q.To)
		sb.WriteString(" AND created_at <= $" + strconv.Itoa(len(args)))
	}
	sb.WriteString(" ORDER BY created_at DESC")
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	args = append(args, limit)
	sb.WriteString(" LIMIT $" + strconv.Itoa(len(args)))
	if q.Offset > 0 {
		args = append(args, q.Offset)
		sb.WriteString(" OFFSET $" + strconv.Itoa(len(args)))
	}
	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var et, act string
		var meta []byte
		if err := rows.Scan(&e.ID, &e.ActorID, &et, &e.EntityID, &act, &meta, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.EntityType = EntityType(et)
		e.Action = Action(act)
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &e.Metadata)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
