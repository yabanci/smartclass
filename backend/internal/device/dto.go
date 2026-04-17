package device

import "time"

type DTO struct {
	ID          string         `json:"id"`
	ClassroomID string         `json:"classroomId"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Brand       string         `json:"brand"`
	Driver      string         `json:"driver"`
	Config      map[string]any `json:"config"`
	Status      string         `json:"status"`
	Online      bool           `json:"online"`
	LastSeenAt  *time.Time     `json:"lastSeenAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

func ToDTO(d *Device) DTO {
	return DTO{
		ID: d.ID.String(), ClassroomID: d.ClassroomID.String(),
		Name: d.Name, Type: d.Type, Brand: d.Brand, Driver: d.Driver,
		Config: d.Config, Status: d.Status, Online: d.Online,
		LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type CreateRequest struct {
	ClassroomID string         `json:"classroomId" validate:"required,uuid"`
	Name        string         `json:"name" validate:"required,min=1,max=120"`
	Type        string         `json:"type" validate:"required,min=1,max=40"`
	Brand       string         `json:"brand" validate:"required,min=1,max=40"`
	Driver      string         `json:"driver" validate:"required,min=1,max=40"`
	Config      map[string]any `json:"config"`
}

type UpdateRequest struct {
	Name   *string         `json:"name,omitempty" validate:"omitempty,min=1,max=120"`
	Type   *string         `json:"type,omitempty" validate:"omitempty,min=1,max=40"`
	Brand  *string         `json:"brand,omitempty" validate:"omitempty,min=1,max=40"`
	Driver *string         `json:"driver,omitempty" validate:"omitempty,min=1,max=40"`
	Config *map[string]any `json:"config,omitempty"`
}

type CommandRequest struct {
	Type  string `json:"type" validate:"required,oneof=ON OFF OPEN CLOSE SET_VALUE"`
	Value any    `json:"value,omitempty"`
}
