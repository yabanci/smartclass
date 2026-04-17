package notification

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("notification: not found")

type ListFilter struct {
	UserID     uuid.UUID
	OnlyUnread bool
	Limit      int
	Offset     int
}

type Repository interface {
	Create(ctx context.Context, n *Notification) error
	CreateBatch(ctx context.Context, items []*Notification) error
	List(ctx context.Context, f ListFilter) ([]*Notification, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}
