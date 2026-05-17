//go:build integration

package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/analytics"
	"smartclass/internal/platform/testsupport"
	"smartclass/internal/user"
)

// analyticsFixture creates user + classroom + device and returns their IDs.
func analyticsFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (classroomID, deviceID uuid.UUID) {
	t.Helper()
	ownerID := uuid.New()
	classroomID = uuid.New()
	deviceID = uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, role) VALUES ($1,$2,$3,$4,$5)`,
		ownerID, ownerID.String()+"@test.local", "ph", "Owner", string(user.RoleTeacher),
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO classrooms (id, name, created_by) VALUES ($1,$2,$3)`,
		classroomID, "Analytics Room", ownerID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO devices (id, classroom_id, name, type, brand, driver, config, status, online)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		deviceID, classroomID, "Sensor 1", "sensor", "Acme", "generic", `{}`, "unknown", false,
	)
	require.NoError(t, err)

	return
}

// insertReading inserts a single sensor_readings row at the given time.
func insertReading(t *testing.T, ctx context.Context, pool *pgxpool.Pool, deviceID uuid.UUID, metric string, value float64, at time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO sensor_readings (device_id, metric, value, unit, recorded_at, raw)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		deviceID, metric, value, "", at, `{}`,
	)
	require.NoError(t, err)
}

func TestPostgresAnalyticsRepository_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	repo := analytics.NewPostgresRepository(pool)

	t.Run("SensorSeries returns bucketed averages for hour bucket", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, deviceID := analyticsFixture(t, ctx, pool)

		base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		// Two readings in hour 10 → avg = 22.
		insertReading(t, ctx, pool, deviceID, "temperature", 20, base.Add(10*time.Minute))
		insertReading(t, ctx, pool, deviceID, "temperature", 24, base.Add(20*time.Minute))
		// One reading in hour 11.
		insertReading(t, ctx, pool, deviceID, "temperature", 30, base.Add(70*time.Minute))

		from := base
		to := base.Add(2 * time.Hour)
		pts, err := repo.SensorSeries(ctx, classroomID, "temperature", analytics.BucketHour, from, to)
		require.NoError(t, err)
		require.Len(t, pts, 2, "expected two hour buckets")

		// Bucket ordering: hour 10 first.
		assert.InDelta(t, 22.0, pts[0].Avg, 0.01)
		assert.Equal(t, int64(2), pts[0].Count)
		assert.InDelta(t, 20.0, pts[0].Min, 0.01)
		assert.InDelta(t, 24.0, pts[0].Max, 0.01)

		assert.InDelta(t, 30.0, pts[1].Avg, 0.01)
		assert.Equal(t, int64(1), pts[1].Count)
	})

	t.Run("SensorSeries filters by classroom — other classroom's device excluded", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		c1, d1 := analyticsFixture(t, ctx, pool)
		_, d2 := analyticsFixture(t, ctx, pool) // different classroom

		base := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
		insertReading(t, ctx, pool, d1, "temperature", 25, base)
		insertReading(t, ctx, pool, d2, "temperature", 99, base) // must not appear in c1 result

		pts, err := repo.SensorSeries(ctx, c1, "temperature", analytics.BucketHour,
			base.Add(-time.Minute), base.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, pts, 1)
		assert.InDelta(t, 25.0, pts[0].Avg, 0.01)
	})

	t.Run("SensorSeries returns empty slice when no readings in range", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, _ := analyticsFixture(t, ctx, pool)
		base := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

		pts, err := repo.SensorSeries(ctx, classroomID, "temperature", analytics.BucketDay, base, base.Add(24*time.Hour))
		require.NoError(t, err)
		assert.Empty(t, pts)
	})

	t.Run("SensorSeries rejects invalid bucket", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, _ := analyticsFixture(t, ctx, pool)
		base := time.Now()
		_, err := repo.SensorSeries(ctx, classroomID, "temperature", analytics.Bucket("millisecond"), base, base.Add(time.Hour))
		assert.Error(t, err)
	})

	t.Run("EnergyTotal sums energy readings across multiple devices", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, d1 := analyticsFixture(t, ctx, pool)
		// Add a second device in the same classroom.
		d2 := uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO devices (id, classroom_id, name, type, brand, driver, config, status, online)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			d2, classroomID, "Sensor 2", "sensor", "Acme", "generic", `{}`, "unknown", false,
		)
		require.NoError(t, err)

		base := time.Date(2024, 4, 1, 8, 0, 0, 0, time.UTC)
		insertReading(t, ctx, pool, d1, "energy", 100, base)
		insertReading(t, ctx, pool, d2, "energy", 200, base)

		total, err := repo.EnergyTotal(ctx, classroomID, base.Add(-time.Minute), base.Add(time.Minute))
		require.NoError(t, err)
		assert.InDelta(t, 300.0, total, 0.01)
	})

	t.Run("EnergyTotal returns 0 when no energy readings", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, _ := analyticsFixture(t, ctx, pool)
		base := time.Now()
		total, err := repo.EnergyTotal(ctx, classroomID, base.Add(-time.Hour), base)
		require.NoError(t, err)
		assert.Equal(t, 0.0, total)
	})

	t.Run("DeviceUsage ranks devices by command count", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, d1 := analyticsFixture(t, ctx, pool)
		d2 := uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO devices (id, classroom_id, name, type, brand, driver, config, status, online)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			d2, classroomID, "Device 2", "light", "Acme", "generic", `{}`, "unknown", false,
		)
		require.NoError(t, err)

		base := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)
		// d1: 3 commands, d2: 1 command.
		for i := 0; i < 3; i++ {
			_, err = pool.Exec(ctx,
				`INSERT INTO action_logs (actor_id, entity_type, entity_id, action, metadata, created_at)
				 VALUES ($1,$2,$3,$4,$5,$6)`,
				uuid.New(), "device", d1, "command", `{}`, base,
			)
			require.NoError(t, err)
		}
		_, err = pool.Exec(ctx,
			`INSERT INTO action_logs (actor_id, entity_type, entity_id, action, metadata, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			uuid.New(), "device", d2, "command", `{}`, base,
		)
		require.NoError(t, err)

		usages, err := repo.DeviceUsage(ctx, classroomID, base.Add(-time.Minute), base.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, usages, 2)
		// Ordered by count DESC.
		assert.Equal(t, d1, usages[0].DeviceID)
		assert.Equal(t, int64(3), usages[0].CommandCount)
		assert.Equal(t, d2, usages[1].DeviceID)
		assert.Equal(t, int64(1), usages[1].CommandCount)
	})
}
