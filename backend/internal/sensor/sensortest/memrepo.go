package sensortest

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"smartclass/internal/sensor"
)

type MemRepo struct {
	mu      sync.Mutex
	items   []sensor.Reading
	counter int64
	// device -> classroom (required for LatestByClassroom)
	deviceClassroom map[uuid.UUID]uuid.UUID
}

func NewMemRepo() *MemRepo {
	return &MemRepo{deviceClassroom: map[uuid.UUID]uuid.UUID{}}
}

func (r *MemRepo) SetDeviceClassroom(deviceID, classroomID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deviceClassroom[deviceID] = classroomID
}

func (r *MemRepo) Insert(_ context.Context, readings []sensor.Reading) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rd := range readings {
		r.counter++
		rd.ID = r.counter
		r.items = append(r.items, rd)
	}
	return nil
}

func (r *MemRepo) List(_ context.Context, q sensor.Query) ([]sensor.Reading, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []sensor.Reading
	for _, it := range r.items {
		if it.DeviceID != q.DeviceID {
			continue
		}
		if q.Metric != "" && it.Metric != q.Metric {
			continue
		}
		if q.From != nil && it.RecordedAt.Before(*q.From) {
			continue
		}
		if q.To != nil && it.RecordedAt.After(*q.To) {
			continue
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.After(out[j].RecordedAt) })
	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *MemRepo) LatestByClassroom(_ context.Context, classroomID uuid.UUID) ([]sensor.Reading, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latest := map[string]sensor.Reading{}
	for _, it := range r.items {
		if r.deviceClassroom[it.DeviceID] != classroomID {
			continue
		}
		key := it.DeviceID.String() + "|" + string(it.Metric)
		cur, ok := latest[key]
		if !ok || it.RecordedAt.After(cur.RecordedAt) {
			latest[key] = it
		}
	}
	out := make([]sensor.Reading, 0, len(latest))
	for _, v := range latest {
		out = append(out, v)
	}
	return out, nil
}
