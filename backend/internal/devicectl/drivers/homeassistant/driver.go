// Package homeassistant implements devicectl.Driver on top of the Home
// Assistant REST API. Home Assistant integrates 2000+ device ecosystems
// (Xiaomi Mi Home, Samsung SmartThings, Tuya, Aqara, Sonoff, Shelly, Matter,
// Zigbee, HomeKit, …), so a single driver lets our backend control almost
// any smart device the user has paired with their HA instance.
//
// Config:
//
//	{
//	  "baseUrl": "http://homeassistant.local:8123",
//	  "token":   "<long-lived access token>",
//	  "entityId": "switch.classroom_lamp"
//	}
//
// The domain portion of entityId (switch, light, cover, climate, ...) is used
// to pick the right service call (switch.turn_on, cover.open_cover, etc.).
package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"smartclass/internal/devicectl"
	"smartclass/internal/platform/metrics"
)

const Name = "homeassistant"

// TokenProvider supplies a fresh HA access token at call time so the driver
// never uses a stale snapshot stored in the device config row.
type TokenProvider interface {
	CurrentToken(ctx context.Context) (string, error)
}

type Driver struct {
	client   *http.Client
	provider TokenProvider
}

func New(client *http.Client, provider TokenProvider) *Driver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Driver{client: client, provider: provider}
}

func (d *Driver) Name() string { return Name }

type config struct {
	BaseURL  string `json:"baseUrl"`
	Token    string `json:"token"`
	EntityID string `json:"entityId"`
}

func parseConfig(raw map[string]any) (*config, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	c := &config{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	if c.BaseURL == "" || c.EntityID == "" {
		return nil, fmt.Errorf("%w: baseUrl and entityId required", devicectl.ErrInvalidConfig)
	}
	if !strings.Contains(c.EntityID, ".") {
		return nil, fmt.Errorf("%w: entityId must be of the form <domain>.<object>", devicectl.ErrInvalidConfig)
	}
	return c, nil
}

// resolveToken returns a fresh token from the provider when available, falling
// back to the token baked into the device config row. Returns ErrInvalidConfig
// when neither source has a token (catches drivers registered without a
// provider AND old config rows that predate the provider migration).
func (d *Driver) resolveToken(ctx context.Context, cfg *config) (string, error) {
	if d.provider != nil {
		tok, err := d.provider.CurrentToken(ctx)
		if err != nil {
			return "", err
		}
		return tok, nil
	}
	if cfg.Token == "" {
		return "", fmt.Errorf("%w: no token and no provider configured", devicectl.ErrInvalidConfig)
	}
	return cfg.Token, nil
}

type serviceCall struct {
	domain  string
	service string
	extra   map[string]any
}

func mapCommand(cmd devicectl.Command, entityDomain string) (serviceCall, error) {
	sc := serviceCall{domain: entityDomain}
	switch cmd.Type {
	case devicectl.CmdOn:
		sc.service = "turn_on"
	case devicectl.CmdOff:
		sc.service = "turn_off"
	case devicectl.CmdOpen:
		switch entityDomain {
		case "cover":
			sc.service = "open_cover"
		case "lock":
			sc.service = "unlock"
		default:
			sc.service = "turn_on"
		}
	case devicectl.CmdClose:
		switch entityDomain {
		case "cover":
			sc.service = "close_cover"
		case "lock":
			sc.service = "lock"
		default:
			sc.service = "turn_off"
		}
	case devicectl.CmdSetValue:
		switch entityDomain {
		case "light":
			sc.service = "turn_on"
			sc.extra = map[string]any{"brightness_pct": cmd.Value}
		case "cover":
			sc.service = "set_cover_position"
			sc.extra = map[string]any{"position": cmd.Value}
		case "climate":
			sc.service = "set_temperature"
			sc.extra = map[string]any{"temperature": cmd.Value}
		case "fan":
			sc.service = "set_percentage"
			sc.extra = map[string]any{"percentage": cmd.Value}
		default:
			return serviceCall{}, fmt.Errorf("%w: SET_VALUE not supported for domain %q", devicectl.ErrUnsupportedCommand, entityDomain)
		}
	default:
		return serviceCall{}, fmt.Errorf("%w: %s", devicectl.ErrUnsupportedCommand, cmd.Type)
	}
	return sc, nil
}

func (d *Driver) Execute(ctx context.Context, t devicectl.Target, cmd devicectl.Command) (devicectl.Result, error) {
	var out devicectl.Result
	err := metrics.TrackDriver(ctx, Name, string(cmd.Type), func(ctx context.Context) error {
		cfg, err := parseConfig(t.Config)
		if err != nil {
			return err
		}
		token, err := d.resolveToken(ctx, cfg)
		if err != nil {
			return err
		}
		domain := entityDomain(cfg.EntityID)
		sc, err := mapCommand(cmd, domain)
		if err != nil {
			return err
		}
		body := map[string]any{"entity_id": cfg.EntityID}
		for k, v := range sc.extra {
			body[k] = v
		}
		if err := d.callService(ctx, cfg.BaseURL, token, sc.domain, sc.service, body); err != nil {
			out = devicectl.Result{Online: false}
			return err
		}
		out = devicectl.Result{
			Status:   inferStatus(cmd.Type, domain),
			Online:   true,
			LastSeen: time.Now().UTC(),
		}
		return nil
	})
	return out, err
}

func (d *Driver) Probe(ctx context.Context, t devicectl.Target) (devicectl.Result, error) {
	cfg, err := parseConfig(t.Config)
	if err != nil {
		return devicectl.Result{}, err
	}
	token, err := d.resolveToken(ctx, cfg)
	if err != nil {
		return devicectl.Result{Online: false}, err
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/api/states/" + cfg.EntityID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return devicectl.Result{Online: false}, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := d.client.Do(req)
	if err != nil {
		return devicectl.Result{Online: false}, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return devicectl.Result{Online: false}, fmt.Errorf("%w: HA %d: %s", devicectl.ErrUnavailable, resp.StatusCode, string(raw))
	}
	var out struct {
		State      string         `json:"state"`
		Attributes map[string]any `json:"attributes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return devicectl.Result{Online: true, Status: devicectl.StatusUnknown, LastSeen: time.Now().UTC()}, nil
	}
	return devicectl.Result{
		Status:   stateToStatus(out.State),
		Online:   out.State != "unavailable" && out.State != "unknown" && out.State != "",
		LastSeen: time.Now().UTC(),
		Raw:      out.Attributes,
	}, nil
}

func (d *Driver) callService(ctx context.Context, baseURL, token, domain, service string, body map[string]any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	url := strings.TrimRight(baseURL, "/") + "/api/services/" + domain + "/" + service
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: HA %d: %s", devicectl.ErrUnavailable, resp.StatusCode, string(raw))
	}
	return nil
}

func entityDomain(entityID string) string {
	if i := strings.Index(entityID, "."); i > 0 {
		return entityID[:i]
	}
	return "switch"
}

func stateToStatus(s string) devicectl.Status {
	switch strings.ToLower(s) {
	case "on":
		return devicectl.StatusOn
	case "off":
		return devicectl.StatusOff
	case "open", "opening":
		return devicectl.StatusOpen
	case "closed", "closing":
		return devicectl.StatusClosed
	}
	return devicectl.StatusUnknown
}

func inferStatus(cmd devicectl.CommandType, domain string) devicectl.Status {
	switch cmd {
	case devicectl.CmdOn:
		return devicectl.StatusOn
	case devicectl.CmdOff:
		return devicectl.StatusOff
	case devicectl.CmdOpen:
		if domain == "cover" {
			return devicectl.StatusOpen
		}
		return devicectl.StatusOn
	case devicectl.CmdClose:
		if domain == "cover" {
			return devicectl.StatusClosed
		}
		return devicectl.StatusOff
	}
	return devicectl.StatusUnknown
}
