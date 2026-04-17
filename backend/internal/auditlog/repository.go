package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Query struct {
	ActorID    *uuid.UUID
	EntityType EntityType
	EntityID   *uuid.UUID
	Action     Action
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

type Repository interface {
	Insert(ctx context.Context, entries []Entry) error
	List(ctx context.Context, q Query) ([]Entry, error)
}
