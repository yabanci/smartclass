package schedule_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/schedule"
	"smartclass/internal/schedule/scheduletest"
	"smartclass/internal/user"
)

// makeTOD is a readable shorthand for the package's TimeOfDay constructor:
// TimeOfDay represents minutes since midnight, so 09:30 == 9*60 + 30.
func makeTOD(h, m int) schedule.TimeOfDay { return schedule.TimeOfDay(h*60 + m) }

// TestService_Overlap_TableDriven exercises overlap detection at the
// boundaries that historically hide bugs: half-open intervals, same-minute
// boundaries, end-equal-start rejection, different days. Each case sets up
// the same anchor lesson (Monday 09:00-10:00) and tries to add a second.
func TestService_Overlap_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		day       schedule.DayOfWeek
		startsAt  schedule.TimeOfDay
		endsAt    schedule.TimeOfDay
		wantErr   error
		errReason string
	}{
		{
			name:      "edge_touch_after_anchor_is_ok_half_open",
			day:       schedule.Monday,
			startsAt:  makeTOD(10, 0),
			endsAt:    makeTOD(11, 0),
			wantErr:   nil,
			errReason: "anchor ends at 10:00 (exclusive); a new lesson starting AT 10:00 must NOT conflict with half-open semantics",
		},
		{
			name:      "overlap_in_middle_is_conflict",
			day:       schedule.Monday,
			startsAt:  makeTOD(9, 30),
			endsAt:    makeTOD(10, 30),
			wantErr:   schedule.ErrOverlap,
			errReason: "any minute inside [09:00, 10:00) must be rejected as overlapping the anchor",
		},
		{
			name:      "edge_touch_before_anchor_is_ok",
			day:       schedule.Monday,
			startsAt:  makeTOD(8, 0),
			endsAt:    makeTOD(9, 0),
			wantErr:   nil,
			errReason: "anchor starts at 09:00; a new lesson ending AT 09:00 is back-to-back, not overlapping",
		},
		{
			name:      "end_equals_start_is_invalid_time",
			day:       schedule.Monday,
			startsAt:  makeTOD(11, 0),
			endsAt:    makeTOD(11, 0),
			wantErr:   schedule.ErrInvalidTime,
			errReason: "zero-length lessons must be rejected at validation, before overlap is even consulted",
		},
		{
			name:      "different_day_no_conflict",
			day:       schedule.Tuesday,
			startsAt:  makeTOD(9, 0),
			endsAt:    makeTOD(10, 0),
			wantErr:   nil,
			errReason: "the same wall-clock window on a different weekday must not be a conflict — overlap is per-day",
		},
		{
			name:      "fully_inside_anchor_is_conflict",
			day:       schedule.Monday,
			startsAt:  makeTOD(9, 15),
			endsAt:    makeTOD(9, 45),
			wantErr:   schedule.ErrOverlap,
			errReason: "a window fully contained inside the anchor must be a conflict",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, principal, classroomID := newScheduleSvcWithAnchor(t)

			_, err := svc.Create(context.Background(), principal, schedule.CreateInput{
				ClassroomID: classroomID,
				Subject:     "second lesson",
				DayOfWeek:   tc.day,
				StartsAt:    tc.startsAt,
				EndsAt:      tc.endsAt,
			})

			if tc.wantErr == nil {
				require.NoError(t, err, tc.errReason)
			} else {
				require.ErrorIs(t, err, tc.wantErr, tc.errReason)
			}
		})
	}
}

// TestService_Overlap_EditSelf_NoChange covers the canonical "rename my own
// lesson" flow: updating a lesson with its existing (start, end, day) values
// must not be reported as overlapping itself.
func TestService_Overlap_EditSelf_NoChange(t *testing.T) {
	svc, principal, classroomID := newScheduleSvcWithAnchor(t)

	week, err := svc.Week(context.Background(), principal, classroomID)
	require.NoError(t, err)
	require.Len(t, week[schedule.Monday], 1, "exactly one anchor lesson seeded")
	anchor := week[schedule.Monday][0]

	newSubject := "renamed"
	updated, err := svc.Update(context.Background(), principal, anchor.ID, schedule.UpdateInput{
		Subject: &newSubject,
	})
	assert.NoError(t, err,
		"updating a lesson without changing day/start/end must succeed — overlap-with-self is the canonical false-positive bug")
	assert.Equal(t, "renamed", updated.Subject)
}

// newScheduleSvcWithAnchor wires schedule.Service over the in-memory repo,
// seeds one classroom owned by a teacher, and places one Monday 09:00-10:00
// anchor lesson. Returns (service, teacher principal, classroom id).
func newScheduleSvcWithAnchor(t *testing.T) (*schedule.Service, classroom.Principal, uuid.UUID) {
	t.Helper()
	classroomRepo := classroomtest.NewMemRepo()
	classroomSvc := classroom.NewService(classroomRepo)
	repo := scheduletest.NewMemRepo()
	svc := schedule.NewService(repo, classroomSvc)

	teacherID := uuid.New()
	principal := classroom.Principal{UserID: teacherID, Role: user.RoleTeacher}

	cls, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name:      "test-room",
		CreatedBy: teacherID,
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), principal, schedule.CreateInput{
		ClassroomID: cls.ID,
		Subject:     "anchor",
		DayOfWeek:   schedule.Monday,
		StartsAt:    makeTOD(9, 0),
		EndsAt:      makeTOD(10, 0),
	})
	require.NoError(t, err)

	return svc, principal, cls.ID
}
