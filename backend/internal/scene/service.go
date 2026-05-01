package scene

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/classroom"
	"smartclass/internal/device"
	"smartclass/internal/devicectl"
	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/metrics"
	"smartclass/internal/realtime"
)

// stepTimeout caps how long a single device command inside a scene may run.
// Without a per-step ceiling, a single misbehaving device (slow network, dead
// HA instance, locked physical device) holds up the entire scene; with it,
// the scene degrades gracefully — the bad step fails fast and the next step
// gets its full budget.
const stepTimeout = 10 * time.Second

var (
	ErrDomainNotFound = httpx.NewDomainError("scene_not_found", http.StatusNotFound, "scene.not_found")
	ErrStepFailed     = httpx.NewDomainError("scene_step_failed", http.StatusBadGateway, "scene.step_failed")
)

type Service struct {
	repo      Repository
	classroom *classroom.Service
	devices   *device.Service
	broker    realtime.Broker
	log       *zap.Logger
}

func NewService(repo Repository, cls *classroom.Service, devices *device.Service, broker realtime.Broker) *Service {
	if broker == nil {
		broker = realtime.Noop{}
	}
	return &Service{repo: repo, classroom: cls, devices: devices, broker: broker, log: zap.NewNop()}
}

func (s *Service) WithLogger(l *zap.Logger) *Service {
	if l != nil {
		s.log = l
	}
	return s
}

type CreateInput struct {
	ClassroomID uuid.UUID
	Name        string
	Description string
	Steps       []Step
}

func (s *Service) Create(ctx context.Context, p classroom.Principal, in CreateInput) (*Scene, error) {
	if err := s.classroom.Authorize(ctx, p, in.ClassroomID, true); err != nil {
		return nil, err
	}
	sc := &Scene{
		ID: uuid.New(), ClassroomID: in.ClassroomID,
		Name: in.Name, Description: in.Description, Steps: in.Steps,
	}
	if err := s.repo.Create(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *Service) Get(ctx context.Context, p classroom.Principal, id uuid.UUID) (*Scene, error) {
	sc, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, sc.ClassroomID, false); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *Service) ListByClassroom(ctx context.Context, p classroom.Principal, classroomID uuid.UUID) ([]*Scene, error) {
	if err := s.classroom.Authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	return s.repo.ListByClassroom(ctx, classroomID)
}

type UpdateInput struct {
	Name        *string
	Description *string
	Steps       *[]Step
}

func (s *Service) Update(ctx context.Context, p classroom.Principal, id uuid.UUID, in UpdateInput) (*Scene, error) {
	sc, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, sc.ClassroomID, true); err != nil {
		return nil, err
	}
	if in.Name != nil {
		sc.Name = *in.Name
	}
	if in.Description != nil {
		sc.Description = *in.Description
	}
	if in.Steps != nil {
		sc.Steps = *in.Steps
	}
	if err := s.repo.Update(ctx, sc); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return sc, nil
}

func (s *Service) Delete(ctx context.Context, p classroom.Principal, id uuid.UUID) error {
	sc, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if err := s.classroom.Authorize(ctx, p, sc.ClassroomID, true); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

type StepResult struct {
	Step    Step           `json:"step"`
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
}

type RunResult struct {
	SceneID uuid.UUID    `json:"sceneId"`
	Steps   []StepResult `json:"steps"`
}

func (s *Service) Run(ctx context.Context, p classroom.Principal, id uuid.UUID) (*RunResult, error) {
	sc, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.classroom.Authorize(ctx, p, sc.ClassroomID, false); err != nil {
		return nil, err
	}

	results := make([]StepResult, 0, len(sc.Steps))
	var firstErr error
	for _, step := range sc.Steps {
		cmd := devicectl.Command{Type: devicectl.CommandType(step.Command), Value: step.Value}
		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		_, err := s.devices.Execute(stepCtx, p, step.DeviceID, cmd)
		cancel()
		r := StepResult{Step: step, Success: err == nil}
		if err != nil {
			r.Error = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
		results = append(results, r)
		// Honour caller cancellation: if the request was cancelled mid-scene,
		// stop spawning new step contexts.
		if ctx.Err() != nil {
			break
		}
	}

	if err := s.broker.Publish(ctx, realtime.Event{
		Topic: fmt.Sprintf("classroom:%s:scenes", sc.ClassroomID.String()),
		Type:  "scene.ran",
		Payload: map[string]any{
			"sceneId": sc.ID.String(),
			"name":    sc.Name,
			"steps":   results,
		},
	}); err != nil {
		s.log.Warn("scene: broker publish failed", zap.Stringer("sceneID", sc.ID), zap.Error(err))
	}

	out := &RunResult{SceneID: sc.ID, Steps: results}
	if firstErr != nil {
		failedCount := 0
		for _, r := range results {
			if !r.Success {
				failedCount++
			}
		}
		if failedCount == len(results) {
			metrics.ScenesRun.WithLabelValues("err").Inc()
		} else {
			metrics.ScenesRun.WithLabelValues("partial").Inc()
		}
		return out, fmt.Errorf("%w: %v", ErrStepFailed, firstErr)
	}
	metrics.ScenesRun.WithLabelValues("ok").Inc()
	return out, nil
}

func (s *Service) load(ctx context.Context, id uuid.UUID) (*Scene, error) {
	sc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return sc, nil
}
