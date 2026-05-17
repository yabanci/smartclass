package devicetoken

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("devicetoken: not found")

type Repository interface {
	Save(ctx context.Context, t *Token) error
	DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error)
}
