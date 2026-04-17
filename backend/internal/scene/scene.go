package scene

import (
	"time"

	"github.com/google/uuid"
)

type Step struct {
	DeviceID uuid.UUID `json:"deviceId"`
	Command  string    `json:"command"`
	Value    any       `json:"value,omitempty"`
}

type Scene struct {
	ID          uuid.UUID
	ClassroomID uuid.UUID
	Name        string
	Description string
	Steps       []Step
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
