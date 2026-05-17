package devicetoken

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, userID uuid.UUID, token string, platform Platform) (*Token, error) {
	t := &Token{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	}
	if err := s.repo.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Unregister(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteByToken(ctx, userID, token)
}

func (s *Service) GetByUser(ctx context.Context, userID uuid.UUID) ([]*Token, error) {
	return s.repo.ListByUser(ctx, userID)
}
