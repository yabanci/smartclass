package pushnotif

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/devicetoken"
	"smartclass/internal/platform/metrics"
)

// NotificationPusher fans out a push to all registered tokens for a user.
// On ErrInvalidToken the stale token is removed from the database.
type NotificationPusher struct {
	client *Client
	tokens *devicetoken.Service
	log    *zap.Logger
}

func NewNotificationPusher(client *Client, tokens *devicetoken.Service, log *zap.Logger) *NotificationPusher {
	if log == nil {
		log = zap.NewNop()
	}
	return &NotificationPusher{client: client, tokens: tokens, log: log}
}

// Send dispatches p to all FCM tokens registered for userID.
// Stale tokens (ErrInvalidToken) are deleted; other errors are logged and skipped.
func (p *NotificationPusher) Send(ctx context.Context, userID uuid.UUID, payload Payload) error {
	toks, err := p.tokens.GetByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, t := range toks {
		if err := p.client.Send(ctx, t.Token, payload); err != nil {
			if errors.Is(err, ErrInvalidToken) {
				metrics.PushSends.WithLabelValues("invalid_token").Inc()
				p.log.Info("pushnotif: removing stale token",
					zap.String("token_prefix", safePrefix(t.Token)))
				if delErr := p.tokens.Unregister(ctx, userID, t.Token); delErr != nil {
					p.log.Warn("pushnotif: failed to delete stale token", zap.Error(delErr))
				}
			} else {
				metrics.PushSends.WithLabelValues("err").Inc()
				p.log.Warn("pushnotif: send failed",
					zap.Error(err),
					zap.Stringer("userID", userID))
			}
		} else {
			metrics.PushSends.WithLabelValues("ok").Inc()
		}
	}
	return nil
}
