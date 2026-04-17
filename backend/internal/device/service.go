package device

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/devicectl"
	"smartclass/internal/platform/httpx"
	"smartclass/internal/realtime"
)

var (
	ErrDomainNotFound   = httpx.NewDomainError("device_not_found", http.StatusNotFound, "device.not_found")
	ErrUnknownDriver    = httpx.NewDomainError("unknown_driver", http.StatusBadRequest, "device.unknown_driver")
	ErrCommandFailed    = httpx.NewDomainError("device_command_failed", http.StatusBadGateway, "device.command_failed")
	ErrUnsupportedCmd   = httpx.NewDomainError("unsupported_command", http.StatusBadRequest, "device.unsupported_command")
)

type Service struct {
	repo      Repository
	classroom *classroom.Service
	factory   *devicectl.Factory
	broker    realtime.Broker
}

func NewService(repo Repository, cls *classroom.Service, f *devicectl.Factory, broker realtime.Broker) *Service {
	if broker == nil {
		broker = realtime.Noop{}
	}
	return &Service{repo: repo, classroom: cls, factory: f, broker: broker}
}

type CreateInput struct {
	ClassroomID uuid.UUID
	Name        string
	Type        string
	Brand       string
	Driver      string
	Config      map[string]any
}

func (s *Service) Create(ctx context.Context, p classroom.Principal, in CreateInput) (*Device, error) {
	if err := s.classroom.Authorize(ctx, p, in.ClassroomID, true); err != nil {
		return nil, err
	}
	if _, err := s.factory.Get(in.Driver); err != nil {
		return nil, ErrUnknownDriver
	}
	d := &Device{
		ID:          uuid.New(),
		ClassroomID: in.ClassroomID,
		Name:        in.Name,
		Type:        in.Type,
		Brand:       in.Brand,
		Driver:      in.Driver,
		Config:      in.Config,
		Status:      string(devicectl.StatusUnknown),
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	s.publish(ctx, d, "device.created")
	return d, nil
}

func (s *Service) Get(ctx context.Context, p classroom.Principal, id uuid.UUID) (*Device, error) {
	d, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, d.ClassroomID, false); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Service) ListByClassroom(ctx context.Context, p classroom.Principal, classroomID uuid.UUID) ([]*Device, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	return s.repo.ListByClassroom(ctx, classroomID)
}

type UpdateInput struct {
	Name   *string
	Type   *string
	Brand  *string
	Driver *string
	Config *map[string]any
}

func (s *Service) Update(ctx context.Context, p classroom.Principal, id uuid.UUID, in UpdateInput) (*Device, error) {
	d, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, d.ClassroomID, true); err != nil {
		return nil, err
	}
	if in.Name != nil {
		d.Name = *in.Name
	}
	if in.Type != nil {
		d.Type = *in.Type
	}
	if in.Brand != nil {
		d.Brand = *in.Brand
	}
	if in.Driver != nil {
		if _, err := s.factory.Get(*in.Driver); err != nil {
			return nil, ErrUnknownDriver
		}
		d.Driver = *in.Driver
	}
	if in.Config != nil {
		d.Config = *in.Config
	}
	if err := s.repo.Update(ctx, d); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	s.publish(ctx, d, "device.updated")
	return d, nil
}

func (s *Service) Delete(ctx context.Context, p classroom.Principal, id uuid.UUID) error {
	d, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if err := s.classroom.Authorize(ctx, p, d.ClassroomID, true); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publish(ctx, d, "device.deleted")
	return nil
}

func (s *Service) Execute(ctx context.Context, p classroom.Principal, id uuid.UUID, cmd devicectl.Command) (*Device, error) {
	if !cmd.Type.Valid() {
		return nil, ErrUnsupportedCmd
	}
	d, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, d.ClassroomID, false); err != nil {
		return nil, err
	}
	driver, err := s.factory.Get(d.Driver)
	if err != nil {
		return nil, ErrUnknownDriver
	}
	target := devicectl.Target{ID: d.ID, Brand: d.Brand, Config: d.Config}
	res, err := driver.Execute(ctx, target, cmd)
	if err != nil {
		if errors.Is(err, devicectl.ErrUnsupportedCommand) {
			return nil, ErrUnsupportedCmd
		}
		_ = s.repo.UpdateState(ctx, d.ID, d.Status, false, d.LastSeenAt)
		d.Online = false
		s.publish(ctx, d, "device.unavailable")
		return nil, fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	d.Status = string(res.Status)
	d.Online = res.Online
	ls := res.LastSeen
	d.LastSeenAt = &ls
	if err := s.repo.UpdateState(ctx, d.ID, d.Status, d.Online, d.LastSeenAt); err != nil {
		return nil, err
	}
	s.publish(ctx, d, "device.state_changed")
	return d, nil
}

func (s *Service) load(ctx context.Context, id uuid.UUID) (*Device, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return d, nil
}

func (s *Service) publish(ctx context.Context, d *Device, eventType string) {
	_ = s.broker.Publish(ctx, realtime.Event{
		Topic: fmt.Sprintf("classroom:%s:devices", d.ClassroomID.String()),
		Type:  eventType,
		Payload: map[string]any{
			"id":         d.ID.String(),
			"classroomId": d.ClassroomID.String(),
			"name":       d.Name,
			"status":     d.Status,
			"online":     d.Online,
			"lastSeenAt": d.LastSeenAt,
			"updatedAt":  time.Now().UTC(),
		},
	})
}
