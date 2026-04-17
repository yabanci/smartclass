package sensor

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Query struct {
	DeviceID uuid.UUID
	Metric   Metric
	From     *time.Time
	To       *time.Time
	Limit    int
}

type Repository interface {
	Insert(ctx context.Context, readings []Reading) error
	List(ctx context.Context, q Query) ([]Reading, error)
	LatestByClassroom(ctx context.Context, classroomID uuid.UUID) ([]Reading, error)
}
