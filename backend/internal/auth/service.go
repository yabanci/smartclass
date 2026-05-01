package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/metrics"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/user"
)

// timeNow is a package-level seam so tests can freeze the clock without
// reaching into the service struct. Production code uses time.Now; tests
// override it via SetClockForTest.
var timeNow = time.Now

// SetClockForTest swaps the clock seam used by Refresh's IsLive check.
// Tests must call it inside t.Cleanup to restore the default.
func SetClockForTest(now func() time.Time) func() {
	prev := timeNow
	timeNow = now
	return func() { timeNow = prev }
}

var (
	ErrInvalidCredentials = httpx.NewDomainError("invalid_credentials", http.StatusUnauthorized, "auth.invalid_credentials")
	ErrEmailTaken         = httpx.NewDomainError("email_taken", http.StatusConflict, "auth.email_taken")
	ErrInvalidRefresh     = httpx.NewDomainError("invalid_refresh", http.StatusUnauthorized, "auth.invalid_refresh")
)

type Service struct {
	users   user.Repository
	hash    hasher.Hasher
	issuer  tokens.Issuer
	store   RefreshStore
	logger  *zap.Logger
}

// NewService wires the auth service. The RefreshStore is required: without
// it refresh tokens cannot be rotated, revoked, or replay-checked. Callers
// that genuinely need a stateless setup (tests) can pass a NoopRefreshStore.
func NewService(users user.Repository, hash hasher.Hasher, issuer tokens.Issuer, store RefreshStore, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{users: users, hash: hash, issuer: issuer, store: store, logger: logger.With(zap.String("subsystem", "auth"))}
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
	pair, err := s.issuePair(ctx, u)
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
			metrics.AuthLogins.WithLabelValues("invalid").Inc()
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := s.hash.Compare(u.PasswordHash, password); err != nil {
		metrics.AuthLogins.WithLabelValues("invalid").Inc()
		return nil, ErrInvalidCredentials
	}
	pair, err := s.issuePair(ctx, u)
	if err != nil {
		return nil, err
	}
	metrics.AuthLogins.WithLabelValues("ok").Inc()
	return &LoginResult{User: u, Tokens: pair}, nil
}

// Refresh rotates the refresh token: it consumes the presented refresh JWT
// and issues a fresh access+refresh pair. The presented refresh-jti is
// marked used; presenting it again returns ErrInvalidRefresh and triggers
// blanket revocation of every other refresh token for the same user
// (replay detection).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	claims, err := s.issuer.Parse(refreshToken)
	if err != nil || claims.Kind != tokens.KindRefresh {
		metrics.AuthRefresh.WithLabelValues("invalid").Inc()
		return nil, ErrInvalidRefresh
	}
	jti := claims.JTI()
	if jti == uuid.Nil {
		metrics.AuthRefresh.WithLabelValues("invalid").Inc()
		return nil, ErrInvalidRefresh
	}

	status, err := s.store.Status(ctx, jti)
	if err != nil {
		if errors.Is(err, ErrRefreshUnknown) {
			metrics.AuthRefresh.WithLabelValues("invalid").Inc()
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}
	if status.IsUsed() {
		// Replay attempt: legitimate user already rotated this token, so
		// whoever is presenting it now is unauthorized. Burn every live
		// refresh token for this user; force them to log in again.
		s.logger.Warn("refresh replay detected — revoking all sessions",
			zap.Stringer("user_id", status.UserID), zap.Stringer("jti", jti))
		_ = s.store.RevokeUser(ctx, status.UserID)
		metrics.AuthRefresh.WithLabelValues("replay").Inc()
		metrics.AuthReplayDetected.Inc()
		return nil, ErrInvalidRefresh
	}
	if !status.IsLive(timeNow()) {
		metrics.AuthRefresh.WithLabelValues("invalid").Inc()
		return nil, ErrInvalidRefresh
	}

	if err := s.store.MarkUsed(ctx, jti); err != nil {
		if errors.Is(err, ErrRefreshAlreadyUsed) {
			// Lost a race to another consumer of the same token (or a
			// concurrent replay). Revoke and reject.
			s.logger.Warn("refresh race lost — revoking all sessions",
				zap.Stringer("user_id", status.UserID), zap.Stringer("jti", jti))
			_ = s.store.RevokeUser(ctx, status.UserID)
			metrics.AuthRefresh.WithLabelValues("replay").Inc()
			metrics.AuthReplayDetected.Inc()
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}

	u, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			metrics.AuthRefresh.WithLabelValues("invalid").Inc()
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}
	pair, err := s.issuePair(ctx, u)
	if err != nil {
		return nil, err
	}
	metrics.AuthRefresh.WithLabelValues("ok").Inc()
	return &LoginResult{User: u, Tokens: pair}, nil
}

// Logout revokes every still-live refresh token belonging to userID. The
// access token (which we don't track in DB to keep the hot path stateless)
// remains valid until its short TTL expires; that's the trade-off for a
// stateless access path.
func (s *Service) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.store.RevokeUser(ctx, userID)
}

// issuePair signs a fresh access+refresh pair and persists the refresh-jti
// so it can later be rotated/revoked. Failing to persist invalidates the
// pair: better to refuse a login than to issue a token we cannot revoke.
func (s *Service) issuePair(ctx context.Context, u *user.User) (tokens.Pair, error) {
	pair, err := s.issuer.Issue(u.ID, string(u.Role))
	if err != nil {
		return tokens.Pair{}, err
	}
	if err := s.store.Track(ctx, pair.RefreshJTI, u.ID, pair.RefreshExpiresAt); err != nil {
		return tokens.Pair{}, err
	}
	return pair, nil
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}
