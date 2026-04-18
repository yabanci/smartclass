package hass

import "context"

type Repository interface {
	Load(ctx context.Context) (*Credentials, error)
	Save(ctx context.Context, c *Credentials) error
}
