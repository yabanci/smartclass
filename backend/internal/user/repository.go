package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("user: not found")
	ErrEmailTaken = errors.New("user: email taken")
)

type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
	UpdateFCMToken(ctx context.Context, id uuid.UUID, token string) error
}
