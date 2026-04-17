package scheduletest

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"smartclass/internal/schedule"
)

type MemRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*schedule.Lesson
}

func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]*schedule.Lesson{}}
}

func (r *MemRepo) Create(_ context.Context, l *schedule.Lesson) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *l
	r.byID[l.ID] = &c
	return nil
}

func (r *MemRepo) GetByID(_ context.Context, id uuid.UUID) (*schedule.Lesson, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return nil, schedule.ErrNotFound
	}
	c := *l
	return &c, nil
}

func (r *MemRepo) ListByClassroom(_ context.Context, classroomID uuid.UUID) ([]*schedule.Lesson, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*schedule.Lesson
	for _, l := range r.byID {
		if l.ClassroomID == classroomID {
			c := *l
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DayOfWeek != out[j].DayOfWeek {
			return out[i].DayOfWeek < out[j].DayOfWeek
		}
		return out[i].StartsAt < out[j].StartsAt
	})
	return out, nil
}

func (r *MemRepo) ListByClassroomAndDay(_ context.Context, classroomID uuid.UUID, day schedule.DayOfWeek) ([]*schedule.Lesson, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*schedule.Lesson
	for _, l := range r.byID {
		if l.ClassroomID == classroomID && l.DayOfWeek == day {
			c := *l
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt < out[j].StartsAt })
	return out, nil
}

func (r *MemRepo) Update(_ context.Context, l *schedule.Lesson) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[l.ID]; !ok {
		return schedule.ErrNotFound
	}
	c := *l
	r.byID[l.ID] = &c
	return nil
}

func (r *MemRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return schedule.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
