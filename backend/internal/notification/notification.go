package notification

import (
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeInfo    Type = "info"
	TypeWarning Type = "warning"
	TypeError   Type = "error"
)

func (t Type) Valid() bool {
	switch t {
	case TypeInfo, TypeWarning, TypeError:
		return true
	}
	return false
}

type Notification struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ClassroomID *uuid.UUID
	Type        Type
	Title       string
	Message     string
	Metadata    map[string]any
	ReadAt      *time.Time
	CreatedAt   time.Time
}
