package sensor

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
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

func (r *PostgresRepository) Insert(ctx context.Context, readings []Reading) error {
	if len(readings) == 0 {
		return nil
	}
	const cols = 6
	args := make([]any, 0, len(readings)*cols)
	values := make([]string, 0, len(readings))
	for i, rd := range readings {
		if rd.RecordedAt.IsZero() {
			rd.RecordedAt = time.Now().UTC()
		}
		raw, err := json.Marshal(rd.Raw)
		if err != nil {
			return err
		}
		base := i * cols
		values = append(values, "($"+itoa(base+1)+",$"+itoa(base+2)+",$"+itoa(base+3)+",$"+itoa(base+4)+",$"+itoa(base+5)+",$"+itoa(base+6)+")")
		args = append(args, rd.DeviceID, string(rd.Metric), rd.Value, rd.Unit, rd.RecordedAt, raw)
	}
	q := "INSERT INTO sensor_readings (device_id, metric, value, unit, recorded_at, raw) VALUES " + strings.Join(values, ",")
	_, err := r.pool.Exec(metrics.WithDBOp(ctx, "sensor.Insert"), q, args...)
	return err
}

func (r *PostgresRepository) List(ctx context.Context, q Query) ([]Reading, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT id, device_id, metric, value, unit, recorded_at, raw FROM sensor_readings WHERE device_id=$1")
	args := []any{q.DeviceID}
	if q.Metric != "" {
		args = append(args, string(q.Metric))
		sb.WriteString(" AND metric=$")
		sb.WriteString(itoa(len(args)))
	}
	if q.From != nil {
		args = append(args, *q.From)
		sb.WriteString(" AND recorded_at >= $")
		sb.WriteString(itoa(len(args)))
	}
	if q.To != nil {
		args = append(args, *q.To)
		sb.WriteString(" AND recorded_at <= $")
		sb.WriteString(itoa(len(args)))
	}
	sb.WriteString(" ORDER BY recorded_at DESC")
	limit := q.Limit
	if limit <= 0 || limit > 10000 {
		limit = 500
	}
	args = append(args, limit)
	sb.WriteString(" LIMIT $")
	sb.WriteString(itoa(len(args)))

	rows, err := r.pool.Query(metrics.WithDBOp(ctx, "sensor.List"), sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Reading
	for rows.Next() {
		var rd Reading
		var metric string
		var raw []byte
		if err := rows.Scan(&rd.ID, &rd.DeviceID, &metric, &rd.Value, &rd.Unit, &rd.RecordedAt, &raw); err != nil {
			return nil, err
		}
		rd.Metric = Metric(metric)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rd.Raw)
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) LatestByClassroom(ctx context.Context, classroomID uuid.UUID) ([]Reading, error) {
	const q = `
SELECT DISTINCT ON (sr.device_id, sr.metric)
  sr.id, sr.device_id, sr.metric, sr.value, sr.unit, sr.recorded_at, sr.raw
FROM sensor_readings sr
JOIN devices d ON d.id = sr.device_id
WHERE d.classroom_id = $1
ORDER BY sr.device_id, sr.metric, sr.recorded_at DESC`
	rows, err := r.pool.Query(metrics.WithDBOp(ctx, "sensor.LatestByClassroom"), q, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reading
	for rows.Next() {
		var rd Reading
		var metric string
		var raw []byte
		if err := rows.Scan(&rd.ID, &rd.DeviceID, &metric, &rd.Value, &rd.Unit, &rd.RecordedAt, &raw); err != nil {
			return nil, err
		}
		rd.Metric = Metric(metric)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rd.Raw)
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

func itoa(i int) string { return strconv.Itoa(i) }
