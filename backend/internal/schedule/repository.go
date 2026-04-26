package schedule

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("schedule: not found")
	// ErrConflict is returned by CreateIfNoOverlap / UpdateIfNoOverlap when the
	// new time range intersects an existing lesson. Mapped to ErrOverlap by the
	// service layer so HTTP callers receive a typed 409.
	ErrConflict = errors.New("schedule: time overlap")
)

type Repository interface {
	// CreateIfNoOverlap atomically checks for overlaps and inserts the lesson.
	// Returns ErrConflict if an existing lesson's time range intersects l's.
	CreateIfNoOverlap(ctx context.Context, l *Lesson) error
	// UpdateIfNoOverlap atomically checks for overlaps (excluding l.ID) and updates.
	// Returns ErrConflict on collision, ErrNotFound if l.ID does not exist.
	UpdateIfNoOverlap(ctx context.Context, l *Lesson) error
	GetByID(ctx context.Context, id uuid.UUID) (*Lesson, error)
	ListByClassroom(ctx context.Context, classroomID uuid.UUID) ([]*Lesson, error)
	ListByClassroomAndDay(ctx context.Context, classroomID uuid.UUID, day DayOfWeek) ([]*Lesson, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
