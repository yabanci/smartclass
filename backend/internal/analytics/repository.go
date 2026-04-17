package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Bucket string

const (
	BucketHour  Bucket = "hour"
	BucketDay   Bucket = "day"
	BucketWeek  Bucket = "week"
	BucketMonth Bucket = "month"
)

func (b Bucket) Valid() bool {
	switch b {
	case BucketHour, BucketDay, BucketWeek, BucketMonth:
		return true
	}
	return false
}

type TimePoint struct {
	Bucket   time.Time `json:"bucket"`
	Avg      float64   `json:"avg"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Count    int64     `json:"count"`
}

type DeviceUsage struct {
	DeviceID    uuid.UUID `json:"deviceId"`
	CommandCount int64    `json:"commandCount"`
}

type Repository interface {
	SensorSeries(ctx context.Context, classroomID uuid.UUID, metric string, bucket Bucket, from, to time.Time) ([]TimePoint, error)
	DeviceUsage(ctx context.Context, classroomID uuid.UUID, from, to time.Time) ([]DeviceUsage, error)
	EnergyTotal(ctx context.Context, classroomID uuid.UUID, from, to time.Time) (float64, error)
}
