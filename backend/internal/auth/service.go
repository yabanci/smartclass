package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/user"
)

var (
	ErrInvalidCredentials = httpx.NewDomainError("invalid_credentials", http.StatusUnauthorized, "auth.invalid_credentials")
	ErrEmailTaken         = httpx.NewDomainError("email_taken", http.StatusConflict, "auth.email_taken")
	ErrInvalidRefresh     = httpx.NewDomainError("invalid_refresh", http.StatusUnauthorized, "auth.invalid_refresh")
)

type Service struct {
	users   user.Repository
	hash    hasher.Hasher
	issuer  tokens.Issuer
}

func NewService(users user.Repository, hash hasher.Hasher, issuer tokens.Issuer) *Service {
	return &Service{users: users, hash: hash, issuer: issuer}
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
	Role     user.Role
	Language string
	Phone    string
}

type LoginResult struct {
	User   *user.User
	Tokens tokens.Pair
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*LoginResult, error) {
	email := normalizeEmail(in.Email)
	if !in.Role.Valid() {
		return nil, httpx.ErrBadRequest
	}
	hash, err := s.hash.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	lang := in.Language
	if lang == "" {
		lang = "kz"
	}
	u := &user.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		FullName:     strings.TrimSpace(in.FullName),
		Role:         in.Role,
		Language:     lang,
		Phone:        in.Phone,
	}
	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	pair, err := s.issuer.Issue(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: pair}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = normalizeEmail(email)
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := s.hash.Compare(u.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	pair, err := s.issuer.Issue(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: pair}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	claims, err := s.issuer.Parse(refreshToken)
	if err != nil || claims.Kind != tokens.KindRefresh {
		return nil, ErrInvalidRefresh
	}
	u, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}
	pair, err := s.issuer.Issue(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: pair}, nil
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}
