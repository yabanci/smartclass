package schedule

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("schedule: not found")

type Repository interface {
	Create(ctx context.Context, l *Lesson) error
	GetByID(ctx context.Context, id uuid.UUID) (*Lesson, error)
	ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Lesson, error)
	ListByClassroomAndDay(ctx context.Context, classroomID uuid.UUID, day DayOfWeek) ([]*Lesson, error)
	Update(ctx context.Context, l *Lesson) error
	Delete(ctx context.Context, id uuid.UUID) error
}
