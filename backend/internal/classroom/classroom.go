package classroom

import (
	"time"

	"github.com/google/uuid"
)

type Classroom struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Assignment struct {
	ClassroomID uuid.UUID
	UserID      uuid.UUID
	AssignedAt  time.Time
}
