// Package usertest provides in-memory fakes for the user repository,
// intended for unit tests in packages that depend on user.Repository.
package usertest

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"smartclass/internal/user"
)

type MemRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*user.User
	byEmail map[string]*user.User
}

func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]*user.User{}, byEmail: map[string]*user.User{}}
}

func (r *MemRepo) Create(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[u.Email]; exists {
		return user.ErrEmailTaken
	}
	copy := *u
	r.byID[u.ID] = &copy
	r.byEmail[u.Email] = &copy
	return nil
}

func (r *MemRepo) GetByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	copy := *u
	return &copy, nil
}

func (r *MemRepo) GetByEmail(_ context.Context, email string) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byEmail[email]
	if !ok {
		return nil, user.ErrNotFound
	}
	copy := *u
	return &copy, nil
}

func (r *MemRepo) Update(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[u.ID]
	if !ok {
		return user.ErrNotFound
	}
	copy := *u
	copy.Email = existing.Email
	copy.PasswordHash = existing.PasswordHash
	r.byID[u.ID] = &copy
	r.byEmail[existing.Email] = &copy
	return nil
}

func (r *MemRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return user.ErrNotFound
	}
	u.PasswordHash = hash
	return nil
}

func (r *MemRepo) Delete(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[id]; ok {
		delete(r.byEmail, u.Email)
		delete(r.byID, id)
	}
}
