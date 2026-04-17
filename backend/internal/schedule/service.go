package schedule

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/platform/httpx"
)

var (
	ErrDomainNotFound = httpx.NewDomainError("lesson_not_found", http.StatusNotFound, "schedule.not_found")
	ErrOverlap        = httpx.NewDomainError("lesson_overlaps", http.StatusConflict, "schedule.overlap")
	ErrInvalidTime    = httpx.NewDomainError("invalid_time", http.StatusBadRequest, "schedule.invalid_time")
)

type Service struct {
	repo      Repository
	classroom *classroom.Service
	clock     func() time.Time
}

func NewService(repo Repository, cls *classroom.Service) *Service {
	return &Service{repo: repo, classroom: cls, clock: func() time.Time { return time.Now() }}
}

func (s *Service) WithClock(now func() time.Time) *Service {
	s.clock = now
	return s
}

type CreateInput struct {
	ClassroomID uuid.UUID
	Subject     string
	TeacherID   *uuid.UUID
	DayOfWeek   DayOfWeek
	StartsAt    TimeOfDay
	EndsAt      TimeOfDay
	Notes       string
}

func (s *Service) Create(ctx context.Context, p classroom.Principal, in CreateInput) (*Lesson, error) {
	if err := s.classroom.Authorize(ctx, p, in.ClassroomID, true); err != nil {
		return nil, err
	}
	if !in.DayOfWeek.Valid() || !in.StartsAt.Valid() || !in.EndsAt.Valid() || in.EndsAt <= in.StartsAt {
		return nil, ErrInvalidTime
	}
	existing, err := s.repo.ListByClassroomAndDay(ctx, in.ClassroomID, in.DayOfWeek)
	if err != nil {
		return nil, err
	}
	candidate := Lesson{DayOfWeek: in.DayOfWeek, StartsAt: in.StartsAt, EndsAt: in.EndsAt}
	for _, e := range existing {
		if e.Overlaps(candidate) {
			return nil, ErrOverlap
		}
	}
	l := &Lesson{
		ID:          uuid.New(),
		ClassroomID: in.ClassroomID,
		Subject:     in.Subject,
		TeacherID:   in.TeacherID,
		DayOfWeek:   in.DayOfWeek,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		Notes:       in.Notes,
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

type UpdateInput struct {
	Subject   *string
	TeacherID *uuid.UUID
	ClearTeacher bool
	DayOfWeek *DayOfWeek
	StartsAt  *TimeOfDay
	EndsAt    *TimeOfDay
	Notes     *string
}

func (s *Service) Update(ctx context.Context, p classroom.Principal, id uuid.UUID, in UpdateInput) (*Lesson, error) {
	l, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, l.ClassroomID, true); err != nil {
		return nil, err
	}
	if in.Subject != nil {
		l.Subject = *in.Subject
	}
	if in.ClearTeacher {
		l.TeacherID = nil
	} else if in.TeacherID != nil {
		l.TeacherID = in.TeacherID
	}
	if in.DayOfWeek != nil {
		l.DayOfWeek = *in.DayOfWeek
	}
	if in.StartsAt != nil {
		l.StartsAt = *in.StartsAt
	}
	if in.EndsAt != nil {
		l.EndsAt = *in.EndsAt
	}
	if in.Notes != nil {
		l.Notes = *in.Notes
	}
	if !l.DayOfWeek.Valid() || !l.StartsAt.Valid() || !l.EndsAt.Valid() || l.EndsAt <= l.StartsAt {
		return nil, ErrInvalidTime
	}
	peers, err := s.repo.ListByClassroomAndDay(ctx, l.ClassroomID, l.DayOfWeek)
	if err != nil {
		return nil, err
	}
	for _, e := range peers {
		if e.ID == l.ID {
			continue
		}
		if e.Overlaps(*l) {
			return nil, ErrOverlap
		}
	}
	if err := s.repo.Update(ctx, l); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return l, nil
}

func (s *Service) Delete(ctx context.Context, p classroom.Principal, id uuid.UUID) error {
	l, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.classroom.Authorize(ctx, p, l.ClassroomID, true); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) Week(ctx context.Context, p classroom.Principal, classroomID uuid.UUID) (map[DayOfWeek][]*Lesson, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	list, err := s.repo.ListByClassroom(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	out := make(map[DayOfWeek][]*Lesson, 5)
	for d := Monday; d <= Friday; d++ {
		out[d] = []*Lesson{}
	}
	for _, l := range list {
		out[l.DayOfWeek] = append(out[l.DayOfWeek], l)
	}
	return out, nil
}

func (s *Service) Day(ctx context.Context, p classroom.Principal, classroomID uuid.UUID, d DayOfWeek) ([]*Lesson, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	if !d.Valid() {
		return nil, ErrInvalidTime
	}
	return s.repo.ListByClassroomAndDay(ctx, classroomID, d)
}

func (s *Service) Current(ctx context.Context, p classroom.Principal, classroomID uuid.UUID) (*Lesson, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	now := s.clock()
	today := FromTime(now)
	mins := TimeOfDay(now.Hour()*60 + now.Minute())
	list, err := s.repo.ListByClassroomAndDay(ctx, classroomID, today)
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		if mins >= l.StartsAt && mins < l.EndsAt {
			return l, nil
		}
	}
	return nil, nil
}

func (s *Service) get(ctx context.Context, id uuid.UUID) (*Lesson, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return l, nil
}
