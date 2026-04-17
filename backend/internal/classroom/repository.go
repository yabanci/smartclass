package classroom

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("classroom: not found")

type Repository interface {
	Create(ctx context.Context, c *Classroom) error
	GetByID(ctx context.Context, id uuid.UUID) (*Classroom, error)
	List(ctx context.Context, limit, offset int) ([]*Classroom, error)
	ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Classroom, error)
	Update(ctx context.Context, c *Classroom) error
	Delete(ctx context.Context, id uuid.UUID) error

	Assign(ctx context.Context, classroomID, userID uuid.UUID) error
	Unassign(ctx context.Context, classroomID, userID uuid.UUID) error
	IsMember(ctx context.Context, classroomID, userID uuid.UUID) (bool, error)
	Members(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error)
}
