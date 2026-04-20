package hass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// clientID used for HA's OAuth2 indieauth flow during onboarding. HA validates
// that the client_id is a parseable URL but doesn't care about its value
// beyond that, so a stable identifier is fine.
const oauthClientID = "https://smartclass.local/"

type Client struct {
	http    *http.Client
	baseURL string
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		// 120s accounts for slow multi-call flow steps — xiaomi_home's OAuth
		// step, for instance, chains a Xiaomi token exchange, a device-list
		// fetch and a cert provisioning call, each with its own MiHome API
		// round-trip. 20s used to time out mid-flow with "Home Assistant did
		// not answer" even though HA was still working.
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) OnboardingStatus(ctx context.Context) (OnboardingStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/onboarding", nil)
	if err != nil {
		return OnboardingStatus{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return OnboardingStatus{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	// HA unloads the /api/onboarding route entirely once onboarding completes
	// (owner created via UI), so a 404 here means "already done".
	if resp.StatusCode == http.StatusNotFound {
		return OnboardingStatus{OwnerDone: true, CoreConfigDone: true, IntegrationDone: true, AnalyticsDone: true}, nil
	}
	if resp.StatusCode >= 400 {
		return OnboardingStatus{}, fmt.Errorf("%w: onboarding status %d", ErrUpstream, resp.StatusCode)
	}
	var steps []struct {
		Step string `json:"step"`
		Done bool   `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&steps); err != nil {
		return OnboardingStatus{}, fmt.Errorf("%w: decode onboarding: %v", ErrUpstream, err)
	}
	out := OnboardingStatus{}
	for _, s := range steps {
		switch s.Step {
		case "user":
			out.OwnerDone = s.Done
		case "core_config":
			out.CoreConfigDone = s.Done
		case "integration":
			out.IntegrationDone = s.Done
		case "analytics":
			out.AnalyticsDone = s.Done
		}
	}
	return out, nil
}

type onboardUserReq struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Language string `json:"language"`
}

type onboardUserResp struct {
	AuthCode string `json:"auth_code"`
}

// CreateOwner performs the HA onboarding "user" step. Returns an auth code to
// be exchanged for tokens. Fails with ErrAlreadyOnboarded if someone already
// completed this step (HA returns 403 in that case).
func (c *Client) CreateOwner(ctx context.Context, name, username, password, lang string) (string, error) {
	body, _ := json.Marshal(onboardUserReq{
		ClientID: oauthClientID,
		Name:     name, Username: username, Password: password, Language: lang,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/onboarding/users", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusForbidden {
		return "", ErrAlreadyOnboarded
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%w: onboarding/users %d: %s", ErrOnboardingFailed, resp.StatusCode, string(raw))
	}
	var out onboardUserResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%w: decode: %v", ErrOnboardingFailed, err)
	}
	if out.AuthCode == "" {
		return "", fmt.Errorf("%w: empty auth_code", ErrOnboardingFailed)
	}
	return out.AuthCode, nil
}

// TokenSet is what HA's /auth/token endpoint returns. Modern HA (2026.4+)
// removed the long-lived access token REST endpoint, so we keep the OAuth
// refresh token and rotate access tokens before they expire.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode trades an onboarding auth_code for an access + refresh token pair.
func (c *Client) ExchangeCode(ctx context.Context, authCode string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", authCode)
	form.Set("client_id", oauthClientID)
	return c.tokenRequest(ctx, form)
}

// LoginWithPassword runs HA's interactive login flow for the homeassistant
// auth provider, then exchanges the resulting auth_code for tokens. Used as a
// recovery path when the owner exists already (e.g. backend crashed mid-
// bootstrap or restarted after onboarding) so we can still fetch a fresh
// token set without manual intervention.
func (c *Client) LoginWithPassword(ctx context.Context, username, password string) (*TokenSet, error) {
	// 1. open a login flow
	startBody, _ := json.Marshal(map[string]any{
		"client_id":    oauthClientID,
		"handler":      []any{"homeassistant", nil},
		"redirect_uri": oauthClientID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login_flow", bytes.NewReader(startBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: login_flow %d: %s", ErrUpstream, resp.StatusCode, truncate(string(raw), 240))
	}
	var startResp struct {
		FlowID string `json:"flow_id"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(raw, &startResp); err != nil || startResp.FlowID == "" {
		return nil, fmt.Errorf("%w: login_flow open: %s", ErrUpstream, truncate(string(raw), 200))
	}

	// 2. submit the credentials
	stepBody, _ := json.Marshal(map[string]any{
		"username":  username,
		"password":  password,
		"client_id": oauthClientID,
	})
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login_flow/"+startResp.FlowID, bytes.NewReader(stepBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	raw, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: login submit %d: %s", ErrUpstream, resp.StatusCode, truncate(string(raw), 240))
	}
	var stepResp struct {
		Type   string `json:"type"`
		Result string `json:"result"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &stepResp); err != nil {
		return nil, fmt.Errorf("%w: login submit decode: %v", ErrUpstream, err)
	}
	if stepResp.Type != "create_entry" || stepResp.Result == "" {
		return nil, fmt.Errorf("%w: login refused: %s", ErrOnboardingFailed, truncate(string(raw), 200))
	}

	// 3. exchange the auth_code for tokens
	return c.ExchangeCode(ctx, stepResp.Result)
}

// RefreshAccessToken exchanges a refresh token for a new access token. HA
// rotates only the access token; the refresh token stays valid until revoked.
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", oauthClientID)
	ts, err := c.tokenRequest(ctx, form)
	if err != nil {
		return nil, err
	}
	if ts.RefreshToken == "" {
		ts.RefreshToken = refreshToken
	}
	return ts, nil
}

func (c *Client) tokenRequest(ctx context.Context, form url.Values) (*TokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: token %d: %s", ErrOnboardingFailed, resp.StatusCode, string(raw))
	}
	var out tokenResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: decode token: %v", ErrOnboardingFailed, err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("%w: empty access_token", ErrOnboardingFailed)
	}
	expires := time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UTC()
	return &TokenSet{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, ExpiresAt: expires}, nil
}

// FinishOnboarding completes every remaining HA onboarding step
// (core_config, integration, analytics) — NOT best-effort anymore. Without
// this the HA UI stays stuck on the "Welcome" wizard and, critically,
// /api/config/config_entries/flow_handlers returns empty/errors so our own
// "Найти IoT" wizard shows all brand tiles greyed out. Loops with status
// re-checks because HA marks each step done only when the POST succeeds, and
// occasionally returns transient 4xx right after onboarding/users completes
// (race with translation loading).
func (c *Client) FinishOnboarding(ctx context.Context, accessToken string) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		status, err := c.OnboardingStatus(ctx)
		if err != nil {
			lastErr = err
		} else if status.CoreConfigDone && status.IntegrationDone && status.AnalyticsDone {
			return nil
		} else {
			lastErr = nil
			if !status.CoreConfigDone {
				if err := c.postOnboardingStep(ctx, accessToken, "/api/onboarding/core_config", map[string]any{}); err != nil {
					lastErr = err
				}
			}
			if !status.IntegrationDone {
				// HA's /integration endpoint expects the client_id we used for
				// the user step and a redirect_uri inside that client_id's
				// origin — it returns an auth_code that we ignore (our own
				// long-lived token from ExchangeCode is enough).
				if err := c.postOnboardingStep(ctx, accessToken, "/api/onboarding/integration", map[string]any{
					"client_id":    oauthClientID,
					"redirect_uri": oauthClientID + "auth_callback.html",
				}); err != nil {
					lastErr = err
				}
			}
			if !status.AnalyticsDone {
				// Explicit opt-out of all analytics — harmless, keeps the badge
				// off the HA dashboard. Some HA versions want just {}; sending
				// explicit preferences works on every version.
				if err := c.postOnboardingStep(ctx, accessToken, "/api/onboarding/analytics", map[string]any{
					"preferences": map[string]bool{
						"base":        false,
						"diagnostics": false,
						"usage":       false,
						"statistics":  false,
					},
				}); err != nil {
					lastErr = err
				}
			}
		}
		// Backoff between passes — HA needs a moment to register each step.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return fmt.Errorf("%w: %v", ErrOnboardingFailed, lastErr)
	}
	return fmt.Errorf("%w: onboarding did not complete after %d attempts", ErrOnboardingFailed, maxAttempts)
}

func (c *Client) postOnboardingStep(ctx context.Context, token, path string, body any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	// 403 with body "unknown_step" means HA already marked this step done —
	// success from our perspective. Any other 4xx/5xx is surfaced so the
	// FinishOnboarding loop can decide to retry.
	if resp.StatusCode == http.StatusForbidden && strings.Contains(string(raw), "unknown") {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("onboarding %s: %d %s", path, resp.StatusCode, truncate(string(raw), 160))
	}
	return nil
}

func (c *Client) ListFlowHandlers(ctx context.Context, token string) ([]FlowHandler, error) {
	// HA returns this endpoint as a flat []string of domain names (e.g.
	// ["xiaomi_miio", "tuya", "hue"]), not the richer object array older
	// integrations documentation showed. Decode as strings and project each
	// domain into a FlowHandler with ConfigFlow=true so the frontend's brand
	// catalog can still match on domain.
	var domains []string
	if err := c.getJSON(ctx, token, "/api/config/config_entries/flow_handlers", &domains); err != nil {
		return nil, err
	}
	out := make([]FlowHandler, 0, len(domains))
	for _, d := range domains {
		out = append(out, FlowHandler{Domain: d, Name: d, ConfigFlow: true})
	}
	return out, nil
}

func (c *Client) StartFlow(ctx context.Context, token, handler string) (*FlowStep, error) {
	body, _ := json.Marshal(map[string]any{
		"handler":      handler,
		"show_advanced_options": false,
	})
	var step FlowStep
	if err := c.requestJSON(ctx, http.MethodPost, token, "/api/config/config_entries/flow", bytes.NewReader(body), &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (c *Client) StepFlow(ctx context.Context, token, flowID string, data map[string]any) (*FlowStep, error) {
	if data == nil {
		data = map[string]any{}
	}
	body, _ := json.Marshal(data)
	var step FlowStep
	if err := c.requestJSON(ctx, http.MethodPost, token, "/api/config/config_entries/flow/"+flowID, bytes.NewReader(body), &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (c *Client) AbortFlow(ctx context.Context, token, flowID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/config/config_entries/flow/"+flowID, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrFlowNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: abort %d", ErrUpstream, resp.StatusCode)
	}
	return nil
}

type haState struct {
	EntityID   string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

func (c *Client) ListEntities(ctx context.Context, token string) ([]Entity, error) {
	var states []haState
	if err := c.getJSON(ctx, token, "/api/states", &states); err != nil {
		return nil, err
	}
	out := make([]Entity, 0, len(states))
	for _, s := range states {
		domain := s.EntityID
		if i := strings.IndexByte(domain, '.'); i >= 0 {
			domain = domain[:i]
		}
		friendly, _ := s.Attributes["friendly_name"].(string)
		out = append(out, Entity{
			EntityID:     s.EntityID,
			State:        s.State,
			Domain:       domain,
			FriendlyName: friendly,
			Attributes:   s.Attributes,
		})
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, token, path string, out any) error {
	return c.requestJSON(ctx, http.MethodGet, token, path, nil, out)
}

func (c *Client) requestJSON(ctx context.Context, method, token, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return ErrFlowNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: %s %d: %s", ErrUpstream, path, resp.StatusCode, truncate(string(raw), 240))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode %s: %v", ErrUpstream, path, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
