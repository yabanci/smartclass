//go:build integration

package schedule_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/testsupport"
	"smartclass/internal/schedule"
	"smartclass/internal/user"
)

// seedScheduleFixture creates a user + classroom and returns both IDs.
func seedScheduleFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (classroomID, teacherID uuid.UUID) {
	t.Helper()
	teacherID = uuid.New()
	classroomID = uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, role) VALUES ($1, $2, $3, $4, $5)`,
		teacherID, teacherID.String()+"@test.local", "ph", "Teacher", string(user.RoleTeacher),
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO classrooms (id, name, created_by) VALUES ($1, $2, $3)`,
		classroomID, "Schedule Room", teacherID,
	)
	require.NoError(t, err)
	return
}

func lesson(classroomID uuid.UUID, teacherID *uuid.UUID, day schedule.DayOfWeek, start, end schedule.TimeOfDay) *schedule.Lesson {
	return &schedule.Lesson{
		ClassroomID: classroomID,
		Subject:     "Math",
		TeacherID:   teacherID,
		DayOfWeek:   day,
		StartsAt:    start,
		EndsAt:      end,
	}
}

func TestPostgresScheduleRepository_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	repo := schedule.NewPostgresRepository(pool)

	t.Run("CreateIfNoOverlap and GetByID round-trip", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l))
		assert.NotEqual(t, uuid.Nil, l.ID)

		got, err := repo.GetByID(ctx, l.ID)
		require.NoError(t, err)
		assert.Equal(t, schedule.Monday, got.DayOfWeek)
		assert.Equal(t, schedule.TimeOfDay(9*60), got.StartsAt)
		assert.Equal(t, schedule.TimeOfDay(10*60), got.EndsAt)
		assert.Equal(t, classroomID, got.ClassroomID)
	})

	t.Run("GetByID on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, schedule.ErrNotFound)
	})

	t.Run("CreateIfNoOverlap rejects overlapping slot in same classroom+day", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l1 := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l1))

		// 09:30-10:30 overlaps 09:00-10:00.
		l2 := lesson(classroomID, &teacherID, schedule.Monday, 9*60+30, 10*60+30)
		err := repo.CreateIfNoOverlap(ctx, l2)
		assert.ErrorIs(t, err, schedule.ErrConflict)
	})

	t.Run("Adjacent slots do not overlap (boundary condition)", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l1 := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l1))

		// 10:00-11:00 is exactly adjacent — not an overlap.
		l2 := lesson(classroomID, &teacherID, schedule.Monday, 10*60, 11*60)
		assert.NoError(t, repo.CreateIfNoOverlap(ctx, l2))
	})

	t.Run("Same slot on different days is not a conflict", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l1 := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l1))

		l2 := lesson(classroomID, &teacherID, schedule.Tuesday, 9*60, 10*60)
		assert.NoError(t, repo.CreateIfNoOverlap(ctx, l2))
	})

	t.Run("ListByClassroom returns lessons ordered by day_of_week, starts_at", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l1 := lesson(classroomID, &teacherID, schedule.Wednesday, 14*60, 15*60)
		l2 := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		l3 := lesson(classroomID, &teacherID, schedule.Monday, 11*60, 12*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l1))
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l2))
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l3))

		list, err := repo.ListByClassroom(ctx, classroomID)
		require.NoError(t, err)
		require.Len(t, list, 3)
		// Ordered: Mon 09:00, Mon 11:00, Wed 14:00.
		assert.Equal(t, schedule.Monday, list[0].DayOfWeek)
		assert.Equal(t, schedule.TimeOfDay(9*60), list[0].StartsAt)
		assert.Equal(t, schedule.Monday, list[1].DayOfWeek)
		assert.Equal(t, schedule.Wednesday, list[2].DayOfWeek)
	})

	t.Run("ListByClassroomAndDay filters to single day (C-002 contract)", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		monLesson := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		tueLesson := lesson(classroomID, &teacherID, schedule.Tuesday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, monLesson))
		require.NoError(t, repo.CreateIfNoOverlap(ctx, tueLesson))

		list, err := repo.ListByClassroomAndDay(ctx, classroomID, schedule.Monday)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, schedule.Monday, list[0].DayOfWeek)
	})

	t.Run("UpdateIfNoOverlap changes timing; same slot (update self) is allowed", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l := lesson(classroomID, &teacherID, schedule.Friday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l))

		l.StartsAt = 10 * 60
		l.EndsAt = 11 * 60
		require.NoError(t, repo.UpdateIfNoOverlap(ctx, l))

		got, err := repo.GetByID(ctx, l.ID)
		require.NoError(t, err)
		assert.Equal(t, schedule.TimeOfDay(10*60), got.StartsAt)
	})

	t.Run("UpdateIfNoOverlap rejects conflict with another lesson", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l1 := lesson(classroomID, &teacherID, schedule.Thursday, 9*60, 10*60)
		l2 := lesson(classroomID, &teacherID, schedule.Thursday, 11*60, 12*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l1))
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l2))

		// Move l2 to overlap with l1.
		l2.StartsAt = 9*60 + 30
		l2.EndsAt = 10*60 + 30
		err := repo.UpdateIfNoOverlap(ctx, l2)
		assert.ErrorIs(t, err, schedule.ErrConflict)
	})

	t.Run("Delete removes lesson; GetByID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l))

		require.NoError(t, repo.Delete(ctx, l.ID))
		_, err := repo.GetByID(ctx, l.ID)
		assert.ErrorIs(t, err, schedule.ErrNotFound)
	})

	t.Run("Classroom DELETE cascades to lessons", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		classroomID, teacherID := seedScheduleFixture(t, ctx, pool)
		l := lesson(classroomID, &teacherID, schedule.Monday, 9*60, 10*60)
		require.NoError(t, repo.CreateIfNoOverlap(ctx, l))

		_, err := pool.Exec(ctx, "DELETE FROM classrooms WHERE id=$1", classroomID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, l.ID)
		assert.ErrorIs(t, err, schedule.ErrNotFound)
	})
}
