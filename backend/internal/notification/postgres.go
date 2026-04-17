package notification

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, n *Notification) error {
	return r.CreateBatch(ctx, []*Notification{n})
}

func (r *PostgresRepository) CreateBatch(ctx context.Context, items []*Notification) error {
	if len(items) == 0 {
		return nil
	}
	const cols = 8
	args := make([]any, 0, len(items)*cols)
	values := make([]string, 0, len(items))
	now := time.Now().UTC()
	for i, n := range items {
		if n.ID == uuid.Nil {
			n.ID = uuid.New()
		}
		if n.CreatedAt.IsZero() {
			n.CreatedAt = now
		}
		meta, err := json.Marshal(n.Metadata)
		if err != nil {
			return err
		}
		base := i * cols
		values = append(values, "($"+strconv.Itoa(base+1)+",$"+strconv.Itoa(base+2)+",$"+strconv.Itoa(base+3)+",$"+strconv.Itoa(base+4)+",$"+strconv.Itoa(base+5)+",$"+strconv.Itoa(base+6)+",$"+strconv.Itoa(base+7)+",$"+strconv.Itoa(base+8)+")")
		args = append(args, n.ID, n.UserID, n.ClassroomID, string(n.Type), n.Title, n.Message, meta, n.CreatedAt)
	}
	q := "INSERT INTO notifications (id, user_id, classroom_id, type, title, message, metadata, created_at) VALUES " + strings.Join(values, ",")
	_, err := r.pool.Exec(ctx, q, args...)
	return err
}

func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]*Notification, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT id, user_id, classroom_id, type, title, message, metadata, read_at, created_at FROM notifications WHERE user_id=$1")
	args := []any{f.UserID}
	if f.OnlyUnread {
		sb.WriteString(" AND read_at IS NULL")
	}
	sb.WriteString(" ORDER BY created_at DESC")
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	args = append(args, limit)
	sb.WriteString(" LIMIT $" + strconv.Itoa(len(args)))
	if f.Offset > 0 {
		args = append(args, f.Offset)
		sb.WriteString(" OFFSET $" + strconv.Itoa(len(args)))
	}
	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Notification
	for rows.Next() {
		n := &Notification{}
		var typeStr string
		var meta []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.ClassroomID, &typeStr, &n.Title, &n.Message, &meta, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Type = Type(typeStr)
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &n.Metadata)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL", userID).Scan(&n)
	return n, err
}

func (r *PostgresRepository) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, "UPDATE notifications SET read_at=now() WHERE id=$1 AND user_id=$2 AND read_at IS NULL", id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "UPDATE notifications SET read_at=now() WHERE user_id=$1 AND read_at IS NULL", userID)
	return err
}
