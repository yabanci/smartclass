package scene

import "time"

type StepDTO struct {
	DeviceID string `json:"deviceId"`
	Command  string `json:"command"`
	Value    any    `json:"value,omitempty"`
}

type DTO struct {
	ID          string    `json:"id"`
	ClassroomID string    `json:"classroomId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Steps       []StepDTO `json:"steps"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ToDTO(s *Scene) DTO {
	steps := make([]StepDTO, 0, len(s.Steps))
	for _, st := range s.Steps {
		steps = append(steps, StepDTO{DeviceID: st.DeviceID.String(), Command: st.Command, Value: st.Value})
	}
	return DTO{
		ID: s.ID.String(), ClassroomID: s.ClassroomID.String(),
		Name: s.Name, Description: s.Description, Steps: steps,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type CreateRequest struct {
	ClassroomID string         `json:"classroomId" validate:"required,uuid"`
	Name        string         `json:"name" validate:"required,min=1,max=120"`
	Description string         `json:"description" validate:"max=2000"`
	Steps       []StepRequest  `json:"steps" validate:"dive"`
}

type StepRequest struct {
	DeviceID string `json:"deviceId" validate:"required,uuid"`
	Command  string `json:"command" validate:"required,oneof=ON OFF OPEN CLOSE SET_VALUE"`
	Value    any    `json:"value,omitempty"`
}

type UpdateRequest struct {
	Name        *string        `json:"name,omitempty" validate:"omitempty,min=1,max=120"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=2000"`
	Steps       *[]StepRequest `json:"steps,omitempty" validate:"omitempty,dive"`
}
