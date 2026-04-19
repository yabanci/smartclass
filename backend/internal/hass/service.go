package hass

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"smartclass/internal/classroom"
	"smartclass/internal/device"
	"smartclass/internal/devicectl/drivers/homeassistant"
)

type Config struct {
	BaseURL       string
	OwnerName     string
	OwnerUsername string
	OwnerPassword string
	Language      string
}

type Service struct {
	cfg      Config
	repo     Repository
	client   *Client
	logger   *zap.Logger
	devices  *device.Service

	mu           sync.Mutex
	creds        *Credentials
	bootstrapErr error

	// refreshMu serialises access-token refreshes. A bare sync.Mutex lets
	// two goroutines both observe NeedsRefresh() and both call HA's /auth/token
	// concurrently, leaving one of the two refreshed access tokens orphaned
	// (and the next request racing to find which one landed in s.creds).
	// Under refreshMu we double-check and refresh at most once.
	refreshMu sync.Mutex
}

func NewService(cfg Config, repo Repository, client *Client, devices *device.Service, logger *zap.Logger) *Service {
	if cfg.OwnerName == "" {
		cfg.OwnerName = "Smart Classroom"
	}
	if cfg.OwnerUsername == "" {
		cfg.OwnerUsername = "smartclass"
	}
	if cfg.OwnerPassword == "" {
		cfg.OwnerPassword = "smartclass1234"
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	return &Service{cfg: cfg, repo: repo, client: client, logger: logger, devices: devices}
}

// Bootstrap is safe to call repeatedly and concurrently — it short-circuits
// once the DB has a usable token. Returns the active credentials on success.
func (s *Service) Bootstrap(ctx context.Context) (*Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.creds != nil && s.creds.Token != "" {
		return s.creds, nil
	}
	// 1. try to load a previously-stored token
	if stored, err := s.repo.Load(ctx); err == nil {
		s.creds = stored
		return stored, nil
	} else if !errors.Is(err, errNoRow) {
		return nil, err
	}
	// 2. run HA onboarding end-to-end
	status, err := s.client.OnboardingStatus(ctx)
	if err != nil {
		s.bootstrapErr = err
		return nil, err
	}
	var tokens *TokenSet
	if status.OwnerDone {
		// HA already has an owner — could be a previous partial bootstrap or a
		// manual setup. Try logging in with our configured creds first; only
		// surface ErrAlreadyOnboarded if those don't work (admin will then need
		// to paste a token via SetToken).
		t, err := s.client.LoginWithPassword(ctx, s.cfg.OwnerUsername, s.cfg.OwnerPassword)
		if err != nil {
			s.bootstrapErr = ErrAlreadyOnboarded
			return nil, ErrAlreadyOnboarded
		}
		tokens = t
	} else {
		authCode, err := s.client.CreateOwner(ctx, s.cfg.OwnerName, s.cfg.OwnerUsername, s.cfg.OwnerPassword, s.cfg.Language)
		if err != nil {
			s.bootstrapErr = err
			return nil, err
		}
		t, err := s.client.ExchangeCode(ctx, authCode)
		if err != nil {
			s.bootstrapErr = err
			return nil, err
		}
		tokens = t
	}
	// best-effort — drives the HA UI out of onboarding mode but not required for REST access
	s.client.FinishOnboarding(ctx, tokens.AccessToken)
	c := &Credentials{
		BaseURL:      s.client.BaseURL(),
		Token:        tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		Onboarded:    true,
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return nil, err
	}
	s.creds = c
	if s.logger != nil {
		s.logger.Info("hass: onboarded and token persisted", zap.String("base_url", c.BaseURL))
	}
	return c, nil
}

// BootstrapWithRetry is intended to run in the background from main. Tries
// Bootstrap on a growing backoff until it succeeds or ctx is cancelled — HA
// takes 30-60s to become reachable on a cold boot so we can't block startup.
func (s *Service) BootstrapWithRetry(ctx context.Context) {
	delay := 3 * time.Second
	const maxDelay = 60 * time.Second
	for {
		if _, err := s.Bootstrap(ctx); err == nil {
			return
		} else if s.logger != nil {
			s.logger.Warn("hass: bootstrap attempt failed; will retry", zap.Error(err), zap.Duration("in", delay))
		}
		// Stop retrying on ErrAlreadyOnboarded — it's a permanent state that
		// requires admin intervention (SetToken) to resolve.
		if errors.Is(s.bootstrapErr, ErrAlreadyOnboarded) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

// Credentials returns the cached creds, forces a bootstrap if we don't have
// them, and refreshes the OAuth access token under the lock when it's about
// to expire so callers always get something usable.
func (s *Service) Credentials(ctx context.Context) (*Credentials, error) {
	s.mu.Lock()
	if s.creds == nil || s.creds.Token == "" {
		s.mu.Unlock()
		return s.Bootstrap(ctx)
	}
	if !s.creds.NeedsRefresh() {
		c := *s.creds
		s.mu.Unlock()
		return &c, nil
	}
	if s.creds.RefreshToken == "" {
		// manually-set long-lived token; no refresh path
		c := *s.creds
		s.mu.Unlock()
		return &c, nil
	}
	s.mu.Unlock()

	// Serialise refreshes so N parallel expired callers trigger exactly one
	// /auth/token round-trip; the later ones re-read the cached token.
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	s.mu.Lock()
	if s.creds == nil || s.creds.Token == "" {
		s.mu.Unlock()
		return s.Bootstrap(ctx)
	}
	if !s.creds.NeedsRefresh() {
		c := *s.creds
		s.mu.Unlock()
		return &c, nil
	}
	refresh := s.creds.RefreshToken
	s.mu.Unlock()

	tokens, err := s.client.RefreshAccessToken(ctx, refresh)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds.Token = tokens.AccessToken
	s.creds.RefreshToken = tokens.RefreshToken
	s.creds.ExpiresAt = tokens.ExpiresAt
	s.creds.UpdatedAt = time.Now().UTC()
	if err := s.repo.Save(ctx, s.creds); err != nil && s.logger != nil {
		s.logger.Warn("hass: persist refreshed token failed", zap.Error(err))
	}
	c := *s.creds
	return &c, nil
}

// Status reports whether HA is reachable and we have a usable token.
type Status struct {
	BaseURL    string `json:"baseUrl"`
	Configured bool   `json:"configured"`
	Onboarded  bool   `json:"onboarded"`
	Reason     string `json:"reason,omitempty"`
}

func (s *Service) Status(ctx context.Context) Status {
	c, err := s.Credentials(ctx)
	if err != nil {
		return Status{BaseURL: s.client.BaseURL(), Configured: false, Reason: err.Error()}
	}
	return Status{BaseURL: c.BaseURL, Configured: true, Onboarded: c.Onboarded}
}

// SetToken lets an admin bypass auto-onboarding by providing an existing HA
// long-lived access token (e.g. when HA was already set up manually). We
// verify it works by listing entities before persisting.
func (s *Service) SetToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("%w: empty token", ErrNotConfigured)
	}
	if _, err := s.client.ListEntities(ctx, token); err != nil {
		return err
	}
	c := &Credentials{
		BaseURL:   s.client.BaseURL(),
		Token:     token,
		Onboarded: true,
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return err
	}
	s.mu.Lock()
	s.creds = c
	s.bootstrapErr = nil
	s.mu.Unlock()
	return nil
}

func (s *Service) ListIntegrations(ctx context.Context) ([]FlowHandler, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("hass: credentials failed before ListIntegrations", zap.Error(err))
		}
		return nil, err
	}
	out, err := s.client.ListFlowHandlers(ctx, c.Token)
	if err != nil && s.logger != nil {
		s.logger.Error("hass: ListFlowHandlers upstream failed",
			zap.Error(err),
			zap.String("base_url", c.BaseURL),
			zap.Int("token_len", len(c.Token)),
		)
	}
	return out, err
}

func (s *Service) StartFlow(ctx context.Context, handler string) (*FlowStep, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	return s.client.StartFlow(ctx, c.Token, handler)
}

func (s *Service) StepFlow(ctx context.Context, flowID string, data map[string]any) (*FlowStep, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	return s.client.StepFlow(ctx, c.Token, flowID, data)
}

func (s *Service) AbortFlow(ctx context.Context, flowID string) error {
	c, err := s.Credentials(ctx)
	if err != nil {
		return err
	}
	return s.client.AbortFlow(ctx, c.Token, flowID)
}

func (s *Service) ListEntities(ctx context.Context) ([]Entity, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.client.ListEntities(ctx, c.Token)
	if err != nil {
		return nil, err
	}
	// Only surface domains our homeassistant driver knows how to control.
	out := make([]Entity, 0, len(all))
	for _, e := range all {
		if supportedDomain(e.Domain) {
			out = append(out, e)
		}
	}
	return out, nil
}

func supportedDomain(d string) bool {
	switch d {
	case "switch", "light", "cover", "lock", "climate", "fan":
		return true
	}
	return false
}

// AdoptInput ties an HA entity to a classroom as a new Device.
type AdoptInput struct {
	EntityID    string
	ClassroomID uuid.UUID
	Name        string
	Brand       string
}

func (s *Service) Adopt(ctx context.Context, p classroom.Principal, in AdoptInput) (*device.Device, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	domain := in.EntityID
	if i := strings.IndexByte(domain, '.'); i >= 0 {
		domain = domain[:i]
	}
	typeGuess := deviceTypeFromDomain(domain)
	name := in.Name
	if name == "" {
		name = in.EntityID
	}
	brand := in.Brand
	if brand == "" {
		brand = "generic"
	}
	return s.devices.Create(ctx, p, device.CreateInput{
		ClassroomID: in.ClassroomID,
		Name:        name,
		Type:        typeGuess,
		Brand:       brand,
		Driver:      homeassistant.Name,
		Config: map[string]any{
			"baseUrl":  c.BaseURL,
			"token":    c.Token,
			"entityId": in.EntityID,
		},
	})
}

func deviceTypeFromDomain(d string) string {
	switch d {
	case "light":
		return "light"
	case "switch":
		return "switch"
	case "cover":
		return "cover"
	case "lock":
		return "lock"
	case "climate":
		return "climate"
	case "fan":
		return "fan"
	default:
		return "generic"
	}
}

