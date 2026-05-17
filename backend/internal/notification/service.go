package notification

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/metrics"
	"smartclass/internal/realtime"
)

var ErrDomainNotFound = httpx.NewDomainError("notification_not_found", http.StatusNotFound, "not_found")

type MemberLookup interface {
	Members(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error)
}

// PushPayload holds the content sent to a device push notification.
type PushPayload struct {
	Title string
	Body  string
	Data  map[string]string
}

// Pusher sends a push notification to all FCM tokens registered for a user.
// Implementations must be safe for concurrent use.
type Pusher interface {
	Send(ctx context.Context, userID uuid.UUID, payload PushPayload) error
}

type Service struct {
	repo    Repository
	members MemberLookup
	broker  realtime.Broker
	pusher  Pusher
	log     *zap.Logger
}

func NewService(repo Repository, members MemberLookup, broker realtime.Broker) *Service {
	if broker == nil {
		broker = realtime.Noop{}
	}
	return &Service{repo: repo, members: members, broker: broker, log: zap.NewNop()}
}

func (s *Service) WithPusher(p Pusher) *Service {
	if p != nil {
		s.pusher = p
	}
	return s
}

func (s *Service) WithLogger(l *zap.Logger) *Service {
	if l != nil {
		s.log = l.With(zap.String("subsystem", "notification"))
	}
	return s
}

type Input struct {
	UserID      uuid.UUID
	ClassroomID *uuid.UUID
	Type        Type
	Title       string
	Message     string
	Metadata    map[string]any
}

func (s *Service) CreateForUser(ctx context.Context, in Input) (*Notification, error) {
	if !in.Type.Valid() {
		return nil, httpx.ErrBadRequest
	}
	n := toNotification(in)
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}
	metrics.NotificationsCreated.WithLabelValues(string(in.Type)).Inc()
	s.publish(ctx, n)
	if s.pusher != nil {
		// #nosec G118 — push fan-out intentionally outlives the request scope:
		// the HTTP 201 must return immediately while FCM round-trips complete
		// in the background. sendPush derives a child context from ctx so the
		// request_id propagates through logs while still enforcing the 10s cap.
		go s.sendPush(ctx, n)
	}
	return n, nil
}

func (s *Service) CreateForClassroom(ctx context.Context, classroomID uuid.UUID, in Input) ([]*Notification, error) {
	if !in.Type.Valid() {
		return nil, httpx.ErrBadRequest
	}
	members, err := s.members.Members(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	cid := classroomID
	items := make([]*Notification, 0, len(members))
	for _, uid := range members {
		n := toNotification(in)
		n.UserID = uid
		n.ClassroomID = &cid
		items = append(items, n)
	}
	if err := s.repo.CreateBatch(ctx, items); err != nil {
		return nil, err
	}
	for range items {
		metrics.NotificationsCreated.WithLabelValues(string(in.Type)).Inc()
	}
	for _, n := range items {
		s.publish(ctx, n)
	}
	if s.pusher != nil {
		for _, n := range items {
			n := n // capture loop var
			// #nosec G118 — see CreateForUser; same reasoning applies per item.
			go s.sendPush(ctx, n)
		}
	}
	return items, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, onlyUnread bool, limit, offset int) ([]*Notification, error) {
	return s.repo.List(ctx, ListFilter{UserID: userID, OnlyUnread: onlyUnread, Limit: limit, Offset: offset})
}

func (s *Service) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	if err := s.repo.MarkRead(ctx, userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrDomainNotFound
		}
		return err
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func toNotification(in Input) *Notification {
	return &Notification{
		ID:          uuid.New(),
		UserID:      in.UserID,
		ClassroomID: in.ClassroomID,
		Type:        in.Type,
		Title:       in.Title,
		Message:     in.Message,
		Metadata:    in.Metadata,
	}
}

func (s *Service) sendPush(parentCtx context.Context, n *Notification) {
	// Derive a child context from the request context so request_id propagates
	// through logs. context.Background() would drop it. The 10s cap is applied
	// here; cancel is always deferred so there is no goroutine leak.
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()
	payload := PushPayload{
		Title: n.Title,
		Body:  n.Message,
		Data: map[string]string{
			"notificationId": n.ID.String(),
			"type":           string(n.Type),
		},
	}
	if err := s.pusher.Send(ctx, n.UserID, payload); err != nil {
		s.log.Warn("notification: push failed",
			zap.Stringer("userID", n.UserID), zap.Error(err))
	}
}

func (s *Service) publish(ctx context.Context, n *Notification) {
	if err := s.broker.Publish(ctx, realtime.Event{
		Version: 1,
		Topic:   fmt.Sprintf("user:%s:notifications", n.UserID.String()),
		Type:    "notification.created",
		Payload: map[string]any{
			"id":        n.ID.String(),
			"type":      string(n.Type),
			"title":     n.Title,
			"message":   n.Message,
			"metadata":  n.Metadata,
			"createdAt": n.CreatedAt,
		},
	}); err != nil {
		s.log.Warn("notification: broker publish failed",
			zap.Stringer("userID", n.UserID), zap.Error(err))
	}
}
