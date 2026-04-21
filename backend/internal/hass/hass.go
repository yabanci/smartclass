// Package hass drives Home Assistant via its REST + auth APIs so users can
// onboard HA, pair IoT devices, and adopt discovered entities — all without
// leaving our UI. The flow is:
//
//  1. Bootstrap (idempotent): call /api/onboarding to check status; if the
//     owner user has not been created, POST /api/onboarding/users, exchange
//     the returned auth code for an access token at /auth/token, then mint a
//     long-lived access token at /auth/long_lived_access_token. Store the
//     token in postgres so subsequent backend restarts reuse it.
//  2. Config flows: proxy /api/config/config_entries/flow so the UI can drive
//     HA's dynamic integration wizard (e.g. Tuya account, Xiaomi cloud, MQTT).
//  3. Entity adoption: list /api/states and turn a chosen entity_id into a
//     Device row in our DB with driver=homeassistant and a ready-to-use
//     config pointing at the shared HA token.
package hass

import (
	"errors"
	"net/http"
	"time"

	"smartclass/internal/platform/httpx"
)

var (
	ErrNotConfigured   = httpx.NewDomainError("hass_not_configured", http.StatusServiceUnavailable, "hass.not_configured")
	ErrAlreadyOnboarded = httpx.NewDomainError("hass_already_onboarded", http.StatusConflict, "hass.already_onboarded")
	ErrOnboardingFailed = httpx.NewDomainError("hass_onboarding_failed", http.StatusBadGateway, "hass.onboarding_failed")
	ErrUpstream         = httpx.NewDomainError("hass_upstream", http.StatusBadGateway, "hass.upstream")
	ErrFlowNotFound     = httpx.NewDomainError("hass_flow_not_found", http.StatusNotFound, "hass.flow_not_found")
)

// sentinel used for "no row yet" — handled at the repo layer.
var errNoRow = errors.New("hass: no row")

// ErrSentinelNoRow exposes the internal "no row" sentinel so alternative
// Repository implementations (e.g. in-memory fakes in tests) return the same
// value the Postgres repo uses, keeping service-layer error matching uniform.
func ErrSentinelNoRow() error { return errNoRow }

type Credentials struct {
	BaseURL      string
	Token        string    // current access token (or a manually-set long-lived token)
	RefreshToken string    // empty when Token is a long-lived (manual) token
	ExpiresAt    time.Time // zero when Token never expires
	Onboarded    bool
	UpdatedAt    time.Time
}

// NeedsRefresh is true when the access token has less than 60 seconds of life
// left. Long-lived (manual) tokens have a zero ExpiresAt and never refresh.
func (c *Credentials) NeedsRefresh() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Until(c.ExpiresAt) < 60*time.Second
}

type OnboardingStatus struct {
	OwnerDone       bool
	CoreConfigDone  bool
	IntegrationDone bool
	AnalyticsDone   bool
}

type FlowHandler struct {
	Domain         string   `json:"domain"`
	Name           string   `json:"name"`
	Integration    string   `json:"integration,omitempty"`
	IotClass       string   `json:"iot_class,omitempty"`
	SupportedBy    string   `json:"supported_by,omitempty"`
	ConfigFlow     bool     `json:"config_flow"`
	DependsOn      []string `json:"depends_on,omitempty"`
}

type FlowStep struct {
	FlowID              string         `json:"flow_id,omitempty"`
	Handler             string         `json:"handler,omitempty"`
	Type                string         `json:"type"`              // "form" | "create_entry" | "abort" | "progress" | "external_step"
	StepID              string         `json:"step_id,omitempty"`
	DataSchema          []SchemaField  `json:"data_schema,omitempty"`
	Errors              map[string]string `json:"errors,omitempty"`
	Description         string         `json:"description,omitempty"`
	DescriptionPlaceholders map[string]any `json:"description_placeholders,omitempty"`
	Reason              string         `json:"reason,omitempty"`
	Title               string         `json:"title,omitempty"`
	Result              map[string]any `json:"result,omitempty"`
}

// SchemaField is our simplified render of HA's voluptuous data_schema as
// delivered over REST. HA returns each field like
// {"type":"string","name":"host","required":true} — we keep that shape and
// expose it to the frontend 1:1 so the form renderer stays dumb.
//
// `Options` is deliberately typed `any` because HA's selector encoding varies:
// voluptuous.In(["cn","sg"]) serializes as `["cn","sg"]` (flat array),
// voluptuous.In({"cn":"China","sg":"Singapore"}) serializes as a dict, and
// selector({"options":[{"value":"cn","label":"China"}]}) serializes as an
// array of objects. xiaomi_home uses the dict form for `cloud_server`, so
// pinning Options to `[]any` made decoding the whole step fail with
// `cannot unmarshal object into Go struct field ... of type []interface {}`.
// The frontend normalizes all three shapes in SchemaFieldInput.
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Default  any    `json:"default,omitempty"`
	Options  any    `json:"options,omitempty"`
}

// FlowProgress is HA's summary of one in-progress config flow returned from
// GET /api/config/config_entries/flow. We only care about flow_id + handler:
// on a fresh StartFlow we scrub any stuck flow for the target handler so a
// crashed/abandoned OAuth session doesn't block the next attempt with
// `already_in_progress` (happens constantly with xiaomi_home because its
// OAuth callback spawns a secondary flow HA then refuses to close).
type FlowProgress struct {
	FlowID  string `json:"flow_id"`
	Handler string `json:"handler"`
	StepID  string `json:"step_id"`
}

type Entity struct {
	EntityID     string         `json:"entity_id"`
	State        string         `json:"state"`
	Domain       string         `json:"domain"`
	FriendlyName string         `json:"friendly_name"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}
