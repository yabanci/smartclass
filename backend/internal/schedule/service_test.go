package schedule_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/schedule"
	"smartclass/internal/schedule/scheduletest"
	"smartclass/internal/user"
)

func newSvc(t *testing.T) (*schedule.Service, *schedule.Service, classroom.Principal, uuid.UUID) {
	t.Helper()
	clsRepo := classroomtest.NewMemRepo()
	clsSvc := classroom.NewService(clsRepo)
	ownerID := uuid.New()
	p := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	cls, err := clsSvc.Create(context.Background(), classroom.CreateInput{Name: "R", CreatedBy: ownerID})
	require.NoError(t, err)

	svc := schedule.NewService(scheduletest.NewMemRepo(), clsSvc)
	return svc, svc, p, cls.ID
}

func TestService_Create_Overlap(t *testing.T) {
	svc, _, p, cid := newSvc(t)
	ctx := context.Background()

	mk := func(start, end string) schedule.CreateInput {
		s, _ := schedule.ParseTimeOfDay(start)
		e, _ := schedule.ParseTimeOfDay(end)
		return schedule.CreateInput{
			ClassroomID: cid, Subject: "Math", DayOfWeek: schedule.Monday,
			StartsAt: s, EndsAt: e,
		}
	}

	_, err := svc.Create(ctx, p, mk("09:00", "10:00"))
	require.NoError(t, err)

	_, err = svc.Create(ctx, p, mk("09:30", "10:30"))
	require.ErrorIs(t, err, schedule.ErrOverlap)

	_, err = svc.Create(ctx, p, mk("10:00", "11:00"))
	require.NoError(t, err, "touching end is not overlap")

	_, err = svc.Create(ctx, p, mk("11:00", "10:30"))
	require.ErrorIs(t, err, schedule.ErrInvalidTime)
}

func TestService_Week(t *testing.T) {
	svc, _, p, cid := newSvc(t)
	ctx := context.Background()
	mk := func(day schedule.DayOfWeek, start, end string) {
		s, _ := schedule.ParseTimeOfDay(start)
		e, _ := schedule.ParseTimeOfDay(end)
		_, err := svc.Create(ctx, p, schedule.CreateInput{
			ClassroomID: cid, Subject: "S", DayOfWeek: day, StartsAt: s, EndsAt: e,
		})
		require.NoError(t, err)
	}
	mk(schedule.Monday, "09:00", "10:00")
	mk(schedule.Monday, "10:00", "11:00")
	mk(schedule.Wednesday, "08:00", "09:00")

	week, err := svc.Week(ctx, p, cid)
	require.NoError(t, err)
	assert.Len(t, week[schedule.Monday], 2)
	assert.Len(t, week[schedule.Tuesday], 0)
	assert.Len(t, week[schedule.Wednesday], 1)
}

func TestService_Current(t *testing.T) {
	svc, _, p, cid := newSvc(t)
	ctx := context.Background()

	s, _ := schedule.ParseTimeOfDay("10:00")
	e, _ := schedule.ParseTimeOfDay("11:00")
	_, err := svc.Create(ctx, p, schedule.CreateInput{
		ClassroomID: cid, Subject: "Now", DayOfWeek: schedule.Wednesday, StartsAt: s, EndsAt: e,
	})
	require.NoError(t, err)

	frozen := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC) // Wed
	svc.WithClock(func() time.Time { return frozen })

	l, err := svc.Current(ctx, p, cid)
	require.NoError(t, err)
	require.NotNil(t, l)
	assert.Equal(t, "Now", l.Subject)

	svc.WithClock(func() time.Time { return frozen.Add(2 * time.Hour) })
	l, err = svc.Current(ctx, p, cid)
	require.NoError(t, err)
	assert.Nil(t, l)
}

// TestService_ConcurrentCreate verifies the TOCTOU fix: two goroutines racing
// to create overlapping lessons should result in exactly one success and one
// ErrOverlap. The MemRepo mutex provides the same serialization guarantee that
// the Postgres FOR UPDATE transaction provides in production.
func TestService_ConcurrentCreate(t *testing.T) {
	svc, _, p, cid := newSvc(t)
	ctx := context.Background()
	s, _ := schedule.ParseTimeOfDay("10:00")
	e, _ := schedule.ParseTimeOfDay("11:00")
	inp := schedule.CreateInput{
		ClassroomID: cid, Subject: "Math", DayOfWeek: schedule.Monday,
		StartsAt: s, EndsAt: e,
	}

	const N = 20
	errs := make([]error, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := svc.Create(ctx, p, inp)
			errs[i] = err
		}()
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else {
			assert.ErrorIs(t, err, schedule.ErrOverlap, "unexpected error type: %v", err)
		}
	}
	assert.Equal(t, 1, successes, "exactly one concurrent create should succeed")
}

func TestService_UpdateOverlap(t *testing.T) {
	svc, _, p, cid := newSvc(t)
	ctx := context.Background()
	s, _ := schedule.ParseTimeOfDay("09:00")
	e, _ := schedule.ParseTimeOfDay("10:00")
	a, err := svc.Create(ctx, p, schedule.CreateInput{ClassroomID: cid, Subject: "A", DayOfWeek: schedule.Monday, StartsAt: s, EndsAt: e})
	require.NoError(t, err)
	s2, _ := schedule.ParseTimeOfDay("10:00")
	e2, _ := schedule.ParseTimeOfDay("11:00")
	_, err = svc.Create(ctx, p, schedule.CreateInput{ClassroomID: cid, Subject: "B", DayOfWeek: schedule.Monday, StartsAt: s2, EndsAt: e2})
	require.NoError(t, err)

	bad, _ := schedule.ParseTimeOfDay("10:30")
	_, err = svc.Update(ctx, p, a.ID, schedule.UpdateInput{EndsAt: &bad})
	require.ErrorIs(t, err, schedule.ErrOverlap)
}
