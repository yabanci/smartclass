package user

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/httpx"
)

var (
	ErrDomainNotFound   = httpx.NewDomainError("user_not_found", http.StatusNotFound, "user.not_found")
	ErrPasswordMismatch = httpx.NewDomainError("password_mismatch", http.StatusBadRequest, "user.password_mismatch")
	ErrWeakPassword     = httpx.NewDomainError("weak_password", http.StatusBadRequest, "auth.weak_password")
)

type Service struct {
	repo Repository
	hash hasher.Hasher
}

func NewService(repo Repository, hash hasher.Hasher) *Service {
	return &Service{repo: repo, hash: hash}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return u, nil
}

type UpdateProfileInput struct {
	FullName  *string
	Language  *string
	AvatarURL *string
	Phone     *string
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*User, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.FullName != nil {
		u.FullName = strings.TrimSpace(*in.FullName)
	}
	if in.Language != nil {
		u.Language = strings.ToLower(strings.TrimSpace(*in.Language))
	}
	if in.AvatarURL != nil {
		u.AvatarURL = *in.AvatarURL
	}
	if in.Phone != nil {
		u.Phone = *in.Phone
	}
	if err := s.repo.Update(ctx, u); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return u, nil
}

func (s *Service) UpdateFCMToken(ctx context.Context, userID uuid.UUID, token string) error {
	if err := s.repo.UpdateFCMToken(ctx, userID, token); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrDomainNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, id uuid.UUID, current, next string) error {
	if len(next) < 8 {
		return ErrWeakPassword
	}
	u, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.hash.Compare(u.PasswordHash, current); err != nil {
		return ErrPasswordMismatch
	}
	newHash, err := s.hash.Hash(next)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, id, newHash); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrDomainNotFound
		}
		return err
	}
	return nil
}
