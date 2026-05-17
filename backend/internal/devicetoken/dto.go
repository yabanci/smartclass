package devicetoken

import "time"

// RegisterRequest is the JSON body for POST /me/device-tokens.
type RegisterRequest struct {
	Token    string `json:"token"    validate:"required,max=4096"`
	Platform string `json:"platform" validate:"required,oneof=android ios web"`
}

// TokenDTO is the JSON representation returned in responses.
type TokenDTO struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Platform   string    `json:"platform"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

func ToDTO(t *Token) TokenDTO {
	return TokenDTO{
		ID:         t.ID.String(),
		Token:      t.Token,
		Platform:   string(t.Platform),
		CreatedAt:  t.CreatedAt,
		LastSeenAt: t.LastSeenAt,
	}
}
