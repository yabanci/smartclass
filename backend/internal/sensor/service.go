package sensor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/classroom"
	"smartclass/internal/device"
	"smartclass/internal/platform/httpx"
	"smartclass/internal/realtime"
)

var ErrInvalidMetric = httpx.NewDomainError("invalid_metric", http.StatusBadRequest, "sensor.invalid_metric")

type Trigger interface {
	OnSensorReading(ctx context.Context, classroomID, deviceID uuid.UUID, metric string, value float64, unit string)
}

type noopTrigger struct{}

func (noopTrigger) OnSensorReading(context.Context, uuid.UUID, uuid.UUID, string, float64, string) {
}

type Service struct {
	repo      Repository
	classroom *classroom.Service
	devices   *device.Service
	broker    realtime.Broker
	trigger   Trigger
	log       *zap.Logger
}

func NewService(repo Repository, cls *classroom.Service, devices *device.Service, broker realtime.Broker) *Service {
	if broker == nil {
		broker = realtime.Noop{}
	}
	return &Service{repo: repo, classroom: cls, devices: devices, broker: broker, trigger: noopTrigger{}, log: zap.NewNop()}
}

func (s *Service) WithLogger(l *zap.Logger) *Service {
	if l != nil {
		s.log = l.With(zap.String("subsystem", "sensor"))
	}
	return s
}

func (s *Service) WithTrigger(t Trigger) *Service {
	if t != nil {
		s.trigger = t
	}
	return s
}

type IngestItem struct {
	DeviceID   uuid.UUID
	Metric     Metric
	Value      float64
	Unit       string
	RecordedAt *time.Time
	Raw        map[string]any
}

func (s *Service) Ingest(ctx context.Context, p classroom.Principal, items []IngestItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	readings := make([]Reading, 0, len(items))
	for _, it := range items {
		if !it.Metric.Valid() {
			return 0, ErrInvalidMetric
		}
		d, err := s.devices.Get(ctx, p, it.DeviceID)
		if err != nil {
			return 0, err
		}
		rec := time.Now().UTC()
		if it.RecordedAt != nil {
			rec = it.RecordedAt.UTC()
		}
		readings = append(readings, Reading{
			DeviceID:   it.DeviceID,
			Metric:     it.Metric,
			Value:      it.Value,
			Unit:       it.Unit,
			RecordedAt: rec,
			Raw:        it.Raw,
		})
		s.publish(ctx, d.ClassroomID, it.DeviceID, it.Metric, it.Value, it.Unit, rec)
		s.trigger.OnSensorReading(ctx, d.ClassroomID, it.DeviceID, string(it.Metric), it.Value, it.Unit)
	}
	if err := s.repo.Insert(ctx, readings); err != nil {
		return 0, err
	}
	return len(readings), nil
}

func (s *Service) History(ctx context.Context, p classroom.Principal, deviceID uuid.UUID, metric Metric, from, to *time.Time, limit int) ([]Reading, error) {
	d, err := s.devices.Get(ctx, p, deviceID)
	if err != nil {
		return nil, err
	}
	if metric != "" && !metric.Valid() {
		return nil, ErrInvalidMetric
	}
	return s.repo.List(ctx, Query{
		DeviceID: d.ID, Metric: metric, From: from, To: to, Limit: limit,
	})
}

func (s *Service) LatestForClassroom(ctx context.Context, p classroom.Principal, classroomID uuid.UUID) ([]Reading, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	return s.repo.LatestByClassroom(ctx, classroomID)
}

func (s *Service) publish(ctx context.Context, classroomID, deviceID uuid.UUID, metric Metric, value float64, unit string, at time.Time) {
	if err := s.broker.Publish(ctx, realtime.Event{
		Version: 1,
		Topic:   fmt.Sprintf("classroom:%s:sensors", classroomID.String()),
		Type:    "sensor.reading",
		Payload: map[string]any{
			"deviceId":   deviceID.String(),
			"metric":     string(metric),
			"value":      value,
			"unit":       unit,
			"recordedAt": at,
		},
	}); err != nil {
		s.log.Warn("sensor: broker publish failed", zap.Error(err))
	}
}
