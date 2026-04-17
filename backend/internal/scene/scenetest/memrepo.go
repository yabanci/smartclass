package scenetest

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"smartclass/internal/scene"
)

type MemRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*scene.Scene
}

func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]*scene.Scene{}}
}

func (r *MemRepo) Create(_ context.Context, s *scene.Scene) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *s
	r.byID[s.ID] = &c
	return nil
}
func (r *MemRepo) GetByID(_ context.Context, id uuid.UUID) (*scene.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok {
		return nil, scene.ErrNotFound
	}
	c := *s
	return &c, nil
}
func (r *MemRepo) ListByClassroom(_ context.Context, classroomID uuid.UUID) ([]*scene.Scene, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*scene.Scene
	for _, s := range r.byID {
		if s.ClassroomID == classroomID {
			c := *s
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (r *MemRepo) Update(_ context.Context, s *scene.Scene) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[s.ID]; !ok {
		return scene.ErrNotFound
	}
	c := *s
	r.byID[s.ID] = &c
	return nil
}
func (r *MemRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return scene.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
