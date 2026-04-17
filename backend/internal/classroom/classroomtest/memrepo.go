package classroomtest

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"smartclass/internal/classroom"
)

type MemRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*classroom.Classroom
	members map[uuid.UUID]map[uuid.UUID]struct{}
}

func NewMemRepo() *MemRepo {
	return &MemRepo{
		byID:    map[uuid.UUID]*classroom.Classroom{},
		members: map[uuid.UUID]map[uuid.UUID]struct{}{},
	}
}

func (r *MemRepo) Create(_ context.Context, c *classroom.Classroom) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *c
	r.byID[c.ID] = &copy
	r.members[c.ID] = map[uuid.UUID]struct{}{}
	return nil
}

func (r *MemRepo) GetByID(_ context.Context, id uuid.UUID) (*classroom.Classroom, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok {
		return nil, classroom.ErrNotFound
	}
	copy := *c
	return &copy, nil
}

func (r *MemRepo) List(_ context.Context, limit, offset int) ([]*classroom.Classroom, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]*classroom.Classroom, 0, len(r.byID))
	for _, c := range r.byID {
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	return page(all, limit, offset), nil
}

func (r *MemRepo) ListForUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*classroom.Classroom, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*classroom.Classroom
	for id, mems := range r.members {
		if _, ok := mems[userID]; ok {
			if c, ok := r.byID[id]; ok {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return page(out, limit, offset), nil
}

func page(src []*classroom.Classroom, limit, offset int) []*classroom.Classroom {
	if offset >= len(src) {
		return nil
	}
	end := offset + limit
	if end > len(src) {
		end = len(src)
	}
	return src[offset:end]
}

func (r *MemRepo) Update(_ context.Context, c *classroom.Classroom) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[c.ID]; !ok {
		return classroom.ErrNotFound
	}
	copy := *c
	r.byID[c.ID] = &copy
	return nil
}

func (r *MemRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return classroom.ErrNotFound
	}
	delete(r.byID, id)
	delete(r.members, id)
	return nil
}

func (r *MemRepo) Assign(_ context.Context, classroomID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.members[classroomID]
	if !ok {
		m = map[uuid.UUID]struct{}{}
		r.members[classroomID] = m
	}
	m[userID] = struct{}{}
	return nil
}

func (r *MemRepo) Unassign(_ context.Context, classroomID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.members[classroomID]; ok {
		delete(m, userID)
	}
	return nil
}

func (r *MemRepo) IsMember(_ context.Context, classroomID, userID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.members[classroomID][userID]
	return ok, nil
}

func (r *MemRepo) Members(_ context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.members[classroomID]
	out := make([]uuid.UUID, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}
