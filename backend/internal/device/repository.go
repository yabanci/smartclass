package device

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("device: not found")

type Repository interface {
	Create(ctx context.Context, d *Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*Device, error)
	ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Device, error)
	Update(ctx context.Context, d *Device) error
	UpdateState(ctx context.Context, id uuid.UUID, status string, online bool, lastSeen *time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
}
