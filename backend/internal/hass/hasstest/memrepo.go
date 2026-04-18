// Package hasstest provides an in-memory hass.Repository for unit tests.
package hasstest

import (
	"context"
	"sync"

	"smartclass/internal/hass"
)

type MemRepo struct {
	mu   sync.RWMutex
	creds *hass.Credentials
}

func New() *MemRepo { return &MemRepo{} }

func (r *MemRepo) Load(_ context.Context) (*hass.Credentials, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.creds == nil {
		return nil, hass.ErrSentinelNoRow()
	}
	cp := *r.creds
	return &cp, nil
}

func (r *MemRepo) Save(_ context.Context, c *hass.Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.creds = &cp
	return nil
}
