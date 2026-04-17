package classroom

import "time"

type DTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ToDTO(c *Classroom) DTO {
	return DTO{
		ID:          c.ID.String(),
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy.String(),
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

type CreateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=120"`
	Description string `json:"description" validate:"max=2000"`
}

type UpdateRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=120"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

type AssignRequest struct {
	UserID string `json:"userId" validate:"required,uuid"`
}
