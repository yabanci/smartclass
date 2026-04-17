package auth

import "time"

type RegisterRequest struct {
	Email    string  `json:"email" validate:"required,email"`
	Password string  `json:"password" validate:"required,min=8,max=128"`
	FullName string  `json:"fullName" validate:"required,min=2,max=120"`
	Role     string  `json:"role" validate:"required,oneof=teacher admin technician"`
	Language string  `json:"language" validate:"omitempty,oneof=en ru kz"`
	Phone    *string `json:"phone,omitempty" validate:"omitempty,max=32"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

type TokenPairDTO struct {
	Access           string    `json:"accessToken"`
	Refresh          string    `json:"refreshToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
	TokenType        string    `json:"tokenType"`
}
