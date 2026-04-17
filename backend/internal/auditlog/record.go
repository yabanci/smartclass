// Package auditlog records structured domain events for audit/compliance. It
// is wired in as a Recorder interface into other services; a Noop{} is used
// when audit is disabled (tests / dev).
package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EntityType string

const (
	EntityUser      EntityType = "user"
	EntityClassroom EntityType = "classroom"
	EntityDevice    EntityType = "device"
	EntityScene     EntityType = "scene"
	EntityLesson    EntityType = "lesson"
)

type Action string

const (
	ActionCreate       Action = "create"
	ActionUpdate       Action = "update"
	ActionDelete       Action = "delete"
	ActionCommand      Action = "command"
	ActionSceneRun     Action = "scene_run"
	ActionLogin        Action = "login"
	ActionPasswordChange Action = "password_change"
)

type Entry struct {
	ID         int64
	ActorID    *uuid.UUID
	EntityType EntityType
	EntityID   *uuid.UUID
	Action     Action
	Metadata   map[string]any
	CreatedAt  time.Time
}

// Recorder uses plain strings for entity/action so that domain packages can
// depend on this tiny interface without importing auditlog.
type Recorder interface {
	Record(ctx context.Context, actor *uuid.UUID, entity string, entityID *uuid.UUID, action string, meta map[string]any)
}

type Noop struct{}

func (Noop) Record(context.Context, *uuid.UUID, string, *uuid.UUID, string, map[string]any) {}
