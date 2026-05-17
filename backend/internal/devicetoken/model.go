package devicetoken

import (
	"time"

	"github.com/google/uuid"
)

type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWeb     Platform = "web"
)

type Token struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Token      string
	Platform   Platform
	CreatedAt  time.Time
	LastSeenAt time.Time
}
