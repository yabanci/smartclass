package devicetest

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/device"
)

type MemRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*device.Device
}

func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]*device.Device{}}
}

func (r *MemRepo) Create(_ context.Context, d *device.Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *d
	r.byID[d.ID] = &c
	return nil
}

func (r *MemRepo) GetByID(_ context.Context, id uuid.UUID) (*device.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return nil, device.ErrNotFound
	}
	c := *d
	return &c, nil
}

func (r *MemRepo) ListByClassroom(_ context.Context, classroomID uuid.UUID) ([]*device.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*device.Device
	for _, d := range r.byID {
		if d.ClassroomID == classroomID {
			c := *d
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *MemRepo) Update(_ context.Context, d *device.Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[d.ID]; !ok {
		return device.ErrNotFound
	}
	c := *d
	r.byID[d.ID] = &c
	return nil
}

func (r *MemRepo) UpdateState(_ context.Context, id uuid.UUID, status string, online bool, lastSeen *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return device.ErrNotFound
	}
	d.Status = status
	d.Online = online
	d.LastSeenAt = lastSeen
	return nil
}

func (r *MemRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return device.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
