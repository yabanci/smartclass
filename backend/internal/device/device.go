package device

import (
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID          uuid.UUID
	ClassroomID uuid.UUID
	Name        string
	Type        string
	Brand       string
	Driver      string
	Config      map[string]any
	Status      string
	Online      bool
	LastSeenAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
