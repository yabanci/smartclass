package scene

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("scene: not found")

type Repository interface {
	Create(ctx context.Context, s *Scene) error
	GetByID(ctx context.Context, id uuid.UUID) (*Scene, error)
	ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Scene, error)
	Update(ctx context.Context, s *Scene) error
	Delete(ctx context.Context, id uuid.UUID) error
}
