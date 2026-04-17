package notificationtest

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/notification"
)

type MemRepo struct {
	mu    sync.Mutex
	items []*notification.Notification
}

func NewMemRepo() *MemRepo {
	return &MemRepo{}
}

func (r *MemRepo) Create(_ context.Context, n *notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *n
	r.items = append(r.items, &c)
	return nil
}

func (r *MemRepo) CreateBatch(_ context.Context, items []*notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range items {
		c := *n
		r.items = append(r.items, &c)
	}
	return nil
}

func (r *MemRepo) List(_ context.Context, f notification.ListFilter) ([]*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*notification.Notification
	for _, n := range r.items {
		if n.UserID != f.UserID {
			continue
		}
		if f.OnlyUnread && n.ReadAt != nil {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if f.Offset >= len(out) {
		return nil, nil
	}
	end := f.Offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[f.Offset:end], nil
}

func (r *MemRepo) CountUnread(_ context.Context, userID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, item := range r.items {
		if item.UserID == userID && item.ReadAt == nil {
			n++
		}
	}
	return n, nil
}

func (r *MemRepo) MarkRead(_ context.Context, userID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.items {
		if n.ID == id && n.UserID == userID {
			if n.ReadAt == nil {
				now := time.Now().UTC()
				n.ReadAt = &now
			}
			return nil
		}
	}
	return notification.ErrNotFound
}

func (r *MemRepo) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, n := range r.items {
		if n.UserID == userID && n.ReadAt == nil {
			n.ReadAt = &now
		}
	}
	return nil
}
