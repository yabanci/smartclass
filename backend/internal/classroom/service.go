package classroom

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"smartclass/internal/platform/httpx"
	"smartclass/internal/user"
)

var (
	ErrDomainNotFound = httpx.NewDomainError("classroom_not_found", http.StatusNotFound, "classroom.not_found")
	ErrForbidden      = httpx.NewDomainError("classroom_forbidden", http.StatusForbidden, "forbidden")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name        string
	Description string
	CreatedBy   uuid.UUID
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Classroom, error) {
	c := &Classroom{
		ID:          uuid.New(),
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Classroom, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) ListForPrincipal(ctx context.Context, p Principal, limit, offset int) ([]*Classroom, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if p.Role == user.RoleAdmin {
		return s.repo.List(ctx, limit, offset)
	}
	return s.repo.ListForUser(ctx, p.UserID, limit, offset)
}

type UpdateInput struct {
	Name        *string
	Description *string
}

func (s *Service) Update(ctx context.Context, p Principal, id uuid.UUID, in UpdateInput) (*Classroom, error) {
	c, err := s.authorize(ctx, p, id, true)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		c.Description = *in.Description
	}
	if err := s.repo.Update(ctx, c); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) Delete(ctx context.Context, p Principal, id uuid.UUID) error {
	if _, err := s.authorize(ctx, p, id, true); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrDomainNotFound
		}
		return err
	}
	return nil
}

func (s *Service) Assign(ctx context.Context, p Principal, classroomID, userID uuid.UUID) error {
	if _, err := s.authorize(ctx, p, classroomID, true); err != nil {
		return err
	}
	return s.repo.Assign(ctx, classroomID, userID)
}

func (s *Service) Unassign(ctx context.Context, p Principal, classroomID, userID uuid.UUID) error {
	if _, err := s.authorize(ctx, p, classroomID, true); err != nil {
		return err
	}
	return s.repo.Unassign(ctx, classroomID, userID)
}

func (s *Service) Members(ctx context.Context, p Principal, classroomID uuid.UUID) ([]uuid.UUID, error) {
	if _, err := s.authorize(ctx, p, classroomID, false); err != nil {
		return nil, err
	}
	return s.repo.Members(ctx, classroomID)
}

type Principal struct {
	UserID uuid.UUID
	Role   user.Role
}

func (s *Service) Authorize(ctx context.Context, p Principal, classroomID uuid.UUID, mutate bool) error {
	_, err := s.authorize(ctx, p, classroomID, mutate)
	return err
}

func (s *Service) authorize(ctx context.Context, p Principal, classroomID uuid.UUID, mutate bool) (*Classroom, error) {
	c, err := s.Get(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if p.Role == user.RoleAdmin {
		return c, nil
	}
	if mutate && c.CreatedBy != p.UserID {
		return nil, ErrForbidden
	}
	member, err := s.repo.IsMember(ctx, classroomID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !member && c.CreatedBy != p.UserID {
		return nil, ErrForbidden
	}
	return c, nil
}
