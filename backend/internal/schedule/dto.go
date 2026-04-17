package schedule

import "time"

type DTO struct {
	ID          string    `json:"id"`
	ClassroomID string    `json:"classroomId"`
	Subject     string    `json:"subject"`
	TeacherID   *string   `json:"teacherId,omitempty"`
	DayOfWeek   int       `json:"dayOfWeek"`
	StartsAt    string    `json:"startsAt"`
	EndsAt      string    `json:"endsAt"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ToDTO(l *Lesson) DTO {
	var tid *string
	if l.TeacherID != nil {
		s := l.TeacherID.String()
		tid = &s
	}
	return DTO{
		ID: l.ID.String(), ClassroomID: l.ClassroomID.String(),
		Subject: l.Subject, TeacherID: tid, DayOfWeek: int(l.DayOfWeek),
		StartsAt: l.StartsAt.String(), EndsAt: l.EndsAt.String(),
		Notes: l.Notes, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

type CreateRequest struct {
	ClassroomID string  `json:"classroomId" validate:"required,uuid"`
	Subject     string  `json:"subject" validate:"required,min=1,max=160"`
	TeacherID   *string `json:"teacherId,omitempty" validate:"omitempty,uuid"`
	DayOfWeek   int     `json:"dayOfWeek" validate:"required,min=1,max=7"`
	StartsAt    string  `json:"startsAt" validate:"required"`
	EndsAt      string  `json:"endsAt" validate:"required"`
	Notes       string  `json:"notes" validate:"max=2000"`
}

type UpdateRequest struct {
	Subject      *string `json:"subject,omitempty" validate:"omitempty,min=1,max=160"`
	TeacherID    *string `json:"teacherId,omitempty" validate:"omitempty,uuid"`
	ClearTeacher bool    `json:"clearTeacher,omitempty"`
	DayOfWeek    *int    `json:"dayOfWeek,omitempty" validate:"omitempty,min=1,max=7"`
	StartsAt     *string `json:"startsAt,omitempty"`
	EndsAt       *string `json:"endsAt,omitempty"`
	Notes        *string `json:"notes,omitempty" validate:"omitempty,max=2000"`
}
