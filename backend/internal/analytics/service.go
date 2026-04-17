package analytics

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/platform/httpx"
)

var ErrInvalidBucket = httpx.NewDomainError("invalid_bucket", http.StatusBadRequest, "analytics.invalid_bucket")

type Service struct {
	repo      Repository
	classroom *classroom.Service
}

func NewService(repo Repository, cls *classroom.Service) *Service {
	return &Service{repo: repo, classroom: cls}
}

func (s *Service) SensorSeries(ctx context.Context, p classroom.Principal, classroomID uuid.UUID, metric string, bucket Bucket, from, to time.Time) ([]TimePoint, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	if !bucket.Valid() {
		return nil, ErrInvalidBucket
	}
	if from.IsZero() {
		from = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	return s.repo.SensorSeries(ctx, classroomID, metric, bucket, from, to)
}

func (s *Service) DeviceUsage(ctx context.Context, p classroom.Principal, classroomID uuid.UUID, from, to time.Time) ([]DeviceUsage, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	if from.IsZero() {
		from = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	return s.repo.DeviceUsage(ctx, classroomID, from, to)
}

func (s *Service) EnergyTotal(ctx context.Context, p classroom.Principal, classroomID uuid.UUID, from, to time.Time) (float64, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return 0, err
	}
	if from.IsZero() {
		from = time.Now().UTC().Add(-30 * 24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	return s.repo.EnergyTotal(ctx, classroomID, from, to)
}
