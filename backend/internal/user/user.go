package user

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleTeacher    Role = "teacher"
	RoleAdmin      Role = "admin"
	RoleTechnician Role = "technician"
)

func (r Role) Valid() bool {
	switch r {
	case RoleTeacher, RoleAdmin, RoleTechnician:
		return true
	}
	return false
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	Role         Role
	Language     string
	AvatarURL    string
	Phone        string
	BirthDate    *time.Time
	FCMToken     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
