package pushnotif

import (
	"context"

	"github.com/google/uuid"

	"smartclass/internal/notification"
)

// NotifPusher adapts NotificationPusher to the notification.Pusher interface.
type NotifPusher struct {
	inner *NotificationPusher
}

func NewNotifPusher(inner *NotificationPusher) *NotifPusher {
	return &NotifPusher{inner: inner}
}

func (a *NotifPusher) Send(ctx context.Context, userID uuid.UUID, p notification.PushPayload) error {
	return a.inner.Send(ctx, userID, Payload{
		Title: p.Title,
		Body:  p.Body,
		Data:  p.Data,
	})
}
