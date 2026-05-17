package devicetokentest

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/devicetoken"
)

type MemRepo struct {
	mu    sync.Mutex
	items []*devicetoken.Token
}

func NewMemRepo() *MemRepo { return &MemRepo{} }

func (r *MemRepo) Save(_ context.Context, t *devicetoken.Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, existing := range r.items {
		if existing.UserID == t.UserID && existing.Token == t.Token {
			existing.LastSeenAt = now
			t.ID = existing.ID
			t.CreatedAt = existing.CreatedAt
			t.LastSeenAt = existing.LastSeenAt
			return nil
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.LastSeenAt = now
	c := *t
	r.items = append(r.items, &c)
	return nil
}

func (r *MemRepo) DeleteByToken(_ context.Context, userID uuid.UUID, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.items[:0]
	for _, t := range r.items {
		if t.UserID != userID || t.Token != token {
			out = append(out, t)
		}
	}
	r.items = out
	return nil
}

func (r *MemRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*devicetoken.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*devicetoken.Token
	for _, t := range r.items {
		if t.UserID == userID {
			c := *t
			out = append(out, &c)
		}
	}
	return out, nil
}
