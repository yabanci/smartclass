//go:build integration

package device_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/device"
	"smartclass/internal/platform/testsupport"
	"smartclass/internal/user"
)

// seedClassroom creates the minimal user + classroom hierarchy required by
// device's FK device.classroom_id → classrooms.id.
func seedClassroom(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ownerID := uuid.New()
	classroomID := uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, role) VALUES ($1, $2, $3, $4, $5)`,
		ownerID, ownerID.String()+"@test.local", "ph", "Owner", string(user.RoleTeacher),
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO classrooms (id, name, created_by) VALUES ($1, $2, $3)`,
		classroomID, "Test Room", ownerID,
	)
	require.NoError(t, err)

	return classroomID
}

func newDevice(classroomID uuid.UUID) *device.Device {
	return &device.Device{
		ClassroomID: classroomID,
		Name:        "Projector",
		Type:        "projector",
		Brand:       "BenQ",
		Driver:      "generic",
		Config:      map[string]any{"ip": "192.168.1.10"},
		Status:      "unknown",
		Online:      false,
	}
}

func TestPostgresDeviceRepository_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	repo := device.NewPostgresRepository(pool)

	t.Run("Create and GetByID round-trip including JSONB config", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID := seedClassroom(t, ctx, pool)
		d := newDevice(classroomID)
		require.NoError(t, repo.Create(ctx, d))
		assert.NotEqual(t, uuid.Nil, d.ID)

		got, err := repo.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, d.Name, got.Name)
		assert.Equal(t, d.ClassroomID, got.ClassroomID)
		assert.Equal(t, "192.168.1.10", got.Config["ip"])
	})

	t.Run("GetByID on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, device.ErrNotFound)
	})

	t.Run("ListByClassroom returns only devices in that classroom", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		c1 := seedClassroom(t, ctx, pool)
		c2 := seedClassroom(t, ctx, pool)

		d1 := newDevice(c1)
		d2 := newDevice(c1)
		d2.Name = "Light"
		d3 := newDevice(c2)
		d3.Name = "AC"

		require.NoError(t, repo.Create(ctx, d1))
		require.NoError(t, repo.Create(ctx, d2))
		require.NoError(t, repo.Create(ctx, d3))

		list, err := repo.ListByClassroom(ctx, c1)
		require.NoError(t, err)
		assert.Len(t, list, 2)

		list, err = repo.ListByClassroom(ctx, c2)
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, "AC", list[0].Name)
	})

	t.Run("Update persists metadata changes and bumps updated_at", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID := seedClassroom(t, ctx, pool)
		d := newDevice(classroomID)
		require.NoError(t, repo.Create(ctx, d))
		originalUpdatedAt := d.UpdatedAt

		d.Name = "Smart Projector"
		d.Config = map[string]any{"ip": "10.0.0.5", "model": "W2700"}
		require.NoError(t, repo.Update(ctx, d))

		got, err := repo.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, "Smart Projector", got.Name)
		assert.Equal(t, "10.0.0.5", got.Config["ip"])
		assert.Equal(t, "W2700", got.Config["model"])
		assert.True(t, got.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("Update on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		d := &device.Device{
			ID:     uuid.New(),
			Name:   "Ghost",
			Type:   "x",
			Brand:  "x",
			Driver: "generic",
			Config: map[string]any{},
		}
		assert.ErrorIs(t, repo.Update(ctx, d), device.ErrNotFound)
	})

	t.Run("UpdateState persists online flag and last_seen_at", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID := seedClassroom(t, ctx, pool)
		d := newDevice(classroomID)
		require.NoError(t, repo.Create(ctx, d))

		now := time.Now().UTC()
		require.NoError(t, repo.UpdateState(ctx, d.ID, "on", true, &now))

		got, err := repo.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, "on", got.Status)
		assert.True(t, got.Online)
		require.NotNil(t, got.LastSeenAt)
		assert.WithinDuration(t, now, *got.LastSeenAt, time.Second)
	})

	t.Run("Delete removes device; GetByID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID := seedClassroom(t, ctx, pool)
		d := newDevice(classroomID)
		require.NoError(t, repo.Create(ctx, d))

		require.NoError(t, repo.Delete(ctx, d.ID))
		_, err := repo.GetByID(ctx, d.ID)
		assert.ErrorIs(t, err, device.ErrNotFound)
	})

	t.Run("Delete on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		assert.ErrorIs(t, repo.Delete(ctx, uuid.New()), device.ErrNotFound)
	})

	t.Run("Classroom CASCADE delete removes its devices", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID := seedClassroom(t, ctx, pool)
		d := newDevice(classroomID)
		require.NoError(t, repo.Create(ctx, d))

		_, err := pool.Exec(ctx, "DELETE FROM classrooms WHERE id=$1", classroomID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, d.ID)
		assert.ErrorIs(t, err, device.ErrNotFound)
	})
}
