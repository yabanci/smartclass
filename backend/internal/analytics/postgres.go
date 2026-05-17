package analytics

import (
	"context"
	"fmt"
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

func (r *PostgresRepository) SensorSeries(ctx context.Context, classroomID uuid.UUID, metric string, bucket Bucket, from, to time.Time) ([]TimePoint, error) {
	if !bucket.Valid() {
		return nil, fmt.Errorf("invalid bucket: %s", bucket)
	}
	const q = `
SELECT date_trunc($1, sr.recorded_at) AS bucket,
       AVG(sr.value) AS avg,
       MIN(sr.value) AS min,
       MAX(sr.value) AS max,
       COUNT(*)      AS count
FROM sensor_readings sr
JOIN devices d ON d.id = sr.device_id
WHERE d.classroom_id = $2
  AND sr.metric = $3
  AND sr.recorded_at BETWEEN $4 AND $5
GROUP BY 1
ORDER BY 1`
	rows, err := r.pool.Query(ctx, q, string(bucket), classroomID, metric, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimePoint
	for rows.Next() {
		var p TimePoint
		if err := rows.Scan(&p.Bucket, &p.Avg, &p.Min, &p.Max, &p.Count); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) DeviceUsage(ctx context.Context, classroomID uuid.UUID, from, to time.Time) ([]DeviceUsage, error) {
	const q = `
SELECT al.entity_id, COUNT(*) AS cnt
FROM action_logs al
JOIN devices d ON d.id = al.entity_id
WHERE al.entity_type = 'device'
  AND al.action = 'command'
  AND d.classroom_id = $1
  AND al.created_at BETWEEN $2 AND $3
GROUP BY al.entity_id
ORDER BY cnt DESC`
	rows, err := r.pool.Query(ctx, q, classroomID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceUsage
	for rows.Next() {
		var u DeviceUsage
		if err := rows.Scan(&u.DeviceID, &u.CommandCount); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) EnergyTotal(ctx context.Context, classroomID uuid.UUID, from, to time.Time) (float64, error) {
	const q = `
SELECT COALESCE(SUM(sr.value), 0)
FROM sensor_readings sr
JOIN devices d ON d.id = sr.device_id
WHERE d.classroom_id = $1
  AND sr.metric = 'energy'
  AND sr.recorded_at BETWEEN $2 AND $3`
	var total float64
	err := r.pool.QueryRow(ctx, q, classroomID, from, to).Scan(&total)
	return total, err
}
