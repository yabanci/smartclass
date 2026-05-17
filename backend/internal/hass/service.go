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

	// flowMu guards lastFlowByHandler. Every StartFlow records the flow_id
	// it received so the next StartFlow for the same handler can abort it
	// first — HA core raises `already_in_progress` when any flow with the
	// same unique_id is still pending, and its WS `flow/progress` channel
	// hides USER-source flows so we can't discover them any other way.
	flowMu             sync.Mutex
	lastFlowByHandler  map[string]string
}

func NewService(cfg Config, repo Repository, client *Client, devices *device.Service, logger *zap.Logger) *Service {
	if cfg.OwnerName == "" {
		cfg.OwnerName = "Smart Classroom"
	}
	if cfg.OwnerUsername == "" {
		cfg.OwnerUsername = "smartclass"
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	if logger != nil {
		logger = logger.With(zap.String("subsystem", "hass"))
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
		// Re-run FinishOnboarding when HA reports steps still pending — covers
		// the case where a previous backend crashed between user-create and the
		// later steps, leaving a valid token in Postgres but HA stuck on its
		// welcome wizard (then every brand in our UI comes back greyed because
		// /api/config/config_entries/flow_handlers 404s).
		if st, err := s.client.OnboardingStatus(ctx); err == nil &&
			(!st.CoreConfigDone || !st.IntegrationDone || !st.AnalyticsDone) {
			if err := s.client.FinishOnboarding(ctx, stored.Token); err != nil && s.logger != nil {
				s.logger.Warn("hass: resume finish onboarding failed", zap.Error(err))
			}
		}
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
	// Drive HA out of onboarding mode — REQUIRED, not best-effort. Without all
	// four steps (user/core_config/integration/analytics) done, HA's config-
	// flow endpoints (`/api/config/config_entries/flow_handlers`) are missing,
	// which makes our "Найти IoT" wizard show every brand tile greyed out.
	// We don't fail the bootstrap if finish errors — the token is still valid
	// and REST reads work — but we log it loudly so retries surface the issue.
	if err := s.client.FinishOnboarding(ctx, tokens.AccessToken); err != nil && s.logger != nil {
		s.logger.Warn("hass: finish onboarding failed; UI may stay on welcome wizard", zap.Error(err))
	}
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
// After a successful bootstrap it runs the end-to-end SelfCheck and logs a
// clear banner so operators know at a glance whether the whole HA integration
// stack is healthy without needing to click through the UI.
func (s *Service) BootstrapWithRetry(ctx context.Context) {
	delay := 3 * time.Second
	const maxDelay = 60 * time.Second
	for {
		if _, err := s.Bootstrap(ctx); err == nil {
			s.logSelfCheck(ctx)
			return
		} else if s.logger != nil {
			s.logger.Warn("hass: bootstrap attempt failed; will retry", zap.Error(err), zap.Duration("in", delay))
		}
		// Stop retrying on ErrAlreadyOnboarded — it's a permanent state that
		// requires admin intervention (SetToken) to resolve.
		s.mu.Lock()
		bootstrapErr := s.bootstrapErr
		s.mu.Unlock()
		if errors.Is(bootstrapErr, ErrAlreadyOnboarded) {
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
	s.creds.Token = tokens.AccessToken
	s.creds.RefreshToken = tokens.RefreshToken
	s.creds.ExpiresAt = tokens.ExpiresAt
	s.creds.UpdatedAt = time.Now().UTC()
	credsSnapshot := *s.creds
	s.mu.Unlock()

	// Persist the refreshed token with a short retry so transient DB blips
	// don't leave a permanently stale row. Backoffs: 100 ms → 500 ms → 2 s.
	saveBackoffs := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}
	var saveErr error
	for attempt := 1; attempt <= len(saveBackoffs)+1; attempt++ {
		if saveErr = s.repo.Save(ctx, &credsSnapshot); saveErr == nil {
			break
		}
		if attempt <= len(saveBackoffs) {
			if s.logger != nil {
				s.logger.Error("hass: persist refreshed token failed; will retry",
					zap.Error(saveErr),
					zap.Int("attempt", attempt),
				)
			}
			select {
			case <-ctx.Done():
				// Context cancelled during retry backoff; return the in-memory
				// snapshot — the token is live even though DB persistence failed.
				return &credsSnapshot, nil
			case <-time.After(saveBackoffs[attempt-1]):
			}
		}
	}
	if saveErr != nil {
		if s.logger != nil {
			s.logger.Error("hass: persist refreshed token failed after all attempts; in-memory token may drift from DB",
				zap.Error(saveErr),
				zap.Int("attempts", len(saveBackoffs)+1),
			)
		}
		s.mu.Lock()
		s.bootstrapErr = saveErr
		s.mu.Unlock()
	}

	return &credsSnapshot, nil
}

// Status reports whether HA is reachable and we have a usable token.
type Status struct {
	BaseURL    string `json:"baseUrl"`
	Configured bool   `json:"configured"`
	Onboarded  bool   `json:"onboarded"`
	Reason     string `json:"reason,omitempty"`
}

// SelfCheck is a per-subsystem readiness snapshot. Each check has a pass/fail
// and a short diagnostic so one curl call tells a user whether the whole
// integration stack is healthy without needing to click through the UI. We
// run this automatically at the end of BootstrapWithRetry and expose it at
// GET /api/v1/hass/selftest.
type SelfCheck struct {
	OK     bool              `json:"ok"`
	Checks []SelfCheckResult `json:"checks"`
}

type SelfCheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// RunSelfCheck verifies the end-to-end HA integration: credentials, onboarding
// state, flow_handlers discovery, xiaomi_home installed, and that a StartFlow
// for xiaomi_home decodes (catches regressions like the dict-shaped options
// bug we just fixed). Does not require a real Mi account — it aborts the
// probe flow right after the first form is returned.
func (s *Service) RunSelfCheck(ctx context.Context) SelfCheck {
	out := SelfCheck{OK: true}
	add := func(name, msg string, ok bool) {
		out.Checks = append(out.Checks, SelfCheckResult{Name: name, OK: ok, Message: msg})
		if !ok {
			out.OK = false
		}
	}

	creds, err := s.Credentials(ctx)
	if err != nil {
		add("credentials", err.Error(), false)
		return out
	}
	add("credentials", "token "+trimToken(creds.Token)+" @ "+creds.BaseURL, true)

	st, err := s.client.OnboardingStatus(ctx)
	if err != nil {
		add("onboarding", err.Error(), false)
	} else {
		allDone := st.OwnerDone && st.CoreConfigDone && st.IntegrationDone && st.AnalyticsDone
		msg := fmt.Sprintf("user=%v core=%v integration=%v analytics=%v",
			st.OwnerDone, st.CoreConfigDone, st.IntegrationDone, st.AnalyticsDone)
		add("onboarding", msg, allDone)
	}

	handlers, err := s.client.ListFlowHandlers(ctx, creds.Token)
	if err != nil {
		add("flow_handlers", err.Error(), false)
	} else {
		add("flow_handlers", fmt.Sprintf("%d integrations discoverable", len(handlers)), len(handlers) > 0)
	}

	hasXiaomi := false
	for _, h := range handlers {
		if h.Domain == "xiaomi_home" {
			hasXiaomi = true
			break
		}
	}
	add("xiaomi_home", "custom_components/xiaomi_home present in HA", hasXiaomi)

	if hasXiaomi {
		step, err := s.client.StartFlow(ctx, creds.Token, "xiaomi_home")
		if err != nil {
			add("xiaomi_home.startflow", err.Error(), false)
		} else {
			add("xiaomi_home.startflow",
				fmt.Sprintf("step_id=%s type=%s fields=%d", step.StepID, step.Type, len(step.DataSchema)),
				step.FlowID != "" && step.Type != "abort")
			if step.FlowID != "" {
				_ = s.client.AbortFlow(ctx, creds.Token, step.FlowID)
			}
		}
	}

	return out
}

func trimToken(t string) string {
	if len(t) < 12 {
		return "(short)"
	}
	return t[:6] + "..." + t[len(t)-4:]
}

// logSelfCheck runs the full HA readiness probe after bootstrap and prints a
// single-line banner. Green path: INFO "hass: READY (credentials=ok ...)".
// Red path: WARN "hass: DEGRADED (<first failing check>)" with per-check
// details at DEBUG level. Never blocks — if the probe itself errors we just
// log the failure.
func (s *Service) logSelfCheck(ctx context.Context) {
	if s.logger == nil {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result := s.RunSelfCheck(probeCtx)
	fields := make([]zap.Field, 0, len(result.Checks))
	for _, c := range result.Checks {
		status := "ok"
		if !c.OK {
			status = "FAIL"
		}
		fields = append(fields, zap.String(c.Name, status+": "+c.Message))
	}
	if result.OK {
		s.logger.Info("hass: READY — smart-classroom integration stack is healthy", fields...)
		return
	}
	s.logger.Warn("hass: DEGRADED — run `curl http://localhost:8080/api/v1/hass/selftest` for full report", fields...)
}

// CurrentToken implements homeassistant.TokenProvider. It returns a fresh,
// auto-refreshed HA access token so device drivers never use a stale snapshot.
func (s *Service) CurrentToken(ctx context.Context) (string, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return "", err
	}
	return c.Token, nil
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

// StartFlow scrubs any in-progress flow for the same handler before starting a
// new one. Two sources of stale flows exist:
//   1. The flow we ourselves started last time and never finished — HA keeps
//      it alive with source=USER, which `flow/progress` hides, so we track
//      its id in `lastFlowByHandler` and DELETE it directly.
//   2. Secondary non-user flows xiaomi_home (and similar OAuth integrations)
//      spawn from their callback — those DO show up in `flow/progress`, so
//      we list them over the WS and abort everything matching the handler.
// If the new flow still comes back as abort/already_in_progress (e.g. race
// with an in-flight callback), we scrub once more and retry the start.
func (s *Service) StartFlow(ctx context.Context, handler string) (*FlowStep, error) {
	c, err := s.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	s.scrubFlows(ctx, c.Token, handler)
	step, err := s.client.StartFlow(ctx, c.Token, handler)
	if err == nil && step != nil && step.Type == "abort" && step.Reason == "already_in_progress" {
		s.scrubFlows(ctx, c.Token, handler)
		step, err = s.client.StartFlow(ctx, c.Token, handler)
	}
	if err == nil && step != nil && step.FlowID != "" {
		s.rememberFlow(handler, step.FlowID)
	}
	return step, err
}

func (s *Service) rememberFlow(handler, flowID string) {
	s.flowMu.Lock()
	defer s.flowMu.Unlock()
	if s.lastFlowByHandler == nil {
		s.lastFlowByHandler = make(map[string]string)
	}
	s.lastFlowByHandler[handler] = flowID
}

func (s *Service) popLastFlow(handler string) string {
	s.flowMu.Lock()
	defer s.flowMu.Unlock()
	if s.lastFlowByHandler == nil {
		return ""
	}
	id := s.lastFlowByHandler[handler]
	delete(s.lastFlowByHandler, handler)
	return id
}

// scrubFlows aborts every in-progress flow whose handler matches the given
// domain. First we DELETE the last flow_id we know about (covers USER-source
// flows HA refuses to list), then we walk `flow/progress` for any remaining
// non-user orphans. All errors are logged but never fatal: a 404 means the
// flow already disappeared, anything else we'd rather ignore than block.
func (s *Service) scrubFlows(ctx context.Context, token, handler string) {
	if prev := s.popLastFlow(handler); prev != "" {
		if err := s.client.AbortFlow(ctx, token, prev); err != nil && !errors.Is(err, ErrFlowNotFound) && s.logger != nil {
			s.logger.Warn("hass: scrub prev flow failed",
				zap.Error(err),
				zap.String("flow_id", prev),
				zap.String("handler", handler),
			)
		}
	}
	flows, err := s.client.ListInProgressFlows(ctx, token)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("hass: scrub list failed", zap.Error(err), zap.String("handler", handler))
		}
		return
	}
	for _, f := range flows {
		if f.Handler != handler {
			continue
		}
		if err := s.client.AbortFlow(ctx, token, f.FlowID); err != nil && !errors.Is(err, ErrFlowNotFound) && s.logger != nil {
			s.logger.Warn("hass: scrub abort failed",
				zap.Error(err),
				zap.String("flow_id", f.FlowID),
				zap.String("handler", handler),
			)
		}
	}
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
			"entityId": in.EntityID,
			// token intentionally omitted — driver fetches a fresh token via
			// hass.Service.CurrentToken() at every Execute/Probe call, so stored
			// device rows never hold a stale access token.
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

