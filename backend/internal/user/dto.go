package user

import "time"

type ProfileDTO struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	FullName  string     `json:"fullName"`
	Role      string     `json:"role"`
	Language  string     `json:"language"`
	AvatarURL string     `json:"avatarUrl,omitempty"`
	Phone     string     `json:"phone,omitempty"`
	BirthDate *time.Time `json:"birthDate,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func ToDTO(u *User) ProfileDTO {
	return ProfileDTO{
		ID:        u.ID.String(),
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      string(u.Role),
		Language:  u.Language,
		AvatarURL: u.AvatarURL,
		Phone:     u.Phone,
		BirthDate: u.BirthDate,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

type UpdateProfileRequest struct {
	FullName  *string `json:"fullName,omitempty" validate:"omitempty,min=2,max=120"`
	Language  *string `json:"language,omitempty" validate:"omitempty,oneof=en ru kz"`
	AvatarURL *string `json:"avatarUrl,omitempty" validate:"omitempty,url"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,max=32"`
}

type ChangePasswordRequest struct {
	Current string `json:"currentPassword" validate:"required"`
	Next    string `json:"newPassword" validate:"required,min=8,max=128"`
}
