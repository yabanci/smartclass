// Package smartthings implements devicectl.Driver on top of Samsung's
// SmartThings REST API (https://api.smartthings.com/v1). SmartThings is
// Samsung's cloud platform — all paired devices (SmartThings-branded, plus
// everything exposed through Matter / SmartThings Schema connectors) become
// available through a single official, token-based REST endpoint.
//
// Config (create a Personal Access Token at https://account.smartthings.com/tokens):
//
//	{
//	  "token":    "pat-...",
//	  "deviceId": "<uuid>",
//	  "component": "main"   // optional, defaults to "main"
//	}
//
// Capability mapping is inferred from the command:
//
//	ON/OFF     -> capability=switch,       command=on/off
//	OPEN/CLOSE -> capability=windowShade,  command=open/close  (override via "capability")
//	SET_VALUE  -> capability=switchLevel,  command=setLevel    (override via "capability"+"setCommand")
//
// All three parameters can be overridden from config for non-default
// capabilities (locks, thermostats, dimmers, colour bulbs, etc.).
package smartthings

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
)

const (
	Name           = "smartthings"
	defaultBaseURL = "https://api.smartthings.com/v1"
)

type Driver struct {
	client  *http.Client
	baseURL string
}

type Option func(*Driver)

func WithBaseURL(url string) Option { return func(d *Driver) { d.baseURL = url } }

func New(client *http.Client, opts ...Option) *Driver {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	d := &Driver{client: client, baseURL: defaultBaseURL}
	for _, o := range opts {
		o(d)
	}
	return d
}

func (d *Driver) Name() string { return Name }

type config struct {
	Token      string `json:"token"`
	DeviceID   string `json:"deviceId"`
	Component  string `json:"component"`
	Capability string `json:"capability"`
	SetCommand string `json:"setCommand"`
	// BaseURL optionally overrides the SmartThings API host per-device.
	// Useful for tests, custom gateways, and SmartThings Schema connectors.
	BaseURL string `json:"baseUrl"`
}

func parseConfig(raw map[string]any) (*config, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	c := &config{Component: "main"}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	if c.Token == "" || c.DeviceID == "" {
		return nil, fmt.Errorf("%w: token and deviceId required", devicectl.ErrInvalidConfig)
	}
	if c.Component == "" {
		c.Component = "main"
	}
	return c, nil
}

type stCommand struct {
	Component  string `json:"component"`
	Capability string `json:"capability"`
	Command    string `json:"command"`
	Arguments  []any  `json:"arguments,omitempty"`
}

func mapCommand(cmd devicectl.Command, cfg *config) (stCommand, devicectl.Status, error) {
	out := stCommand{Component: cfg.Component}
	switch cmd.Type {
	case devicectl.CmdOn:
		out.Capability = override(cfg.Capability, "switch")
		out.Command = "on"
		return out, devicectl.StatusOn, nil
	case devicectl.CmdOff:
		out.Capability = override(cfg.Capability, "switch")
		out.Command = "off"
		return out, devicectl.StatusOff, nil
	case devicectl.CmdOpen:
		out.Capability = override(cfg.Capability, "windowShade")
		if out.Capability == "lock" {
			out.Command = "unlock"
		} else {
			out.Command = "open"
		}
		return out, devicectl.StatusOpen, nil
	case devicectl.CmdClose:
		out.Capability = override(cfg.Capability, "windowShade")
		if out.Capability == "lock" {
			out.Command = "lock"
		} else {
			out.Command = "close"
		}
		return out, devicectl.StatusClosed, nil
	case devicectl.CmdSetValue:
		out.Capability = override(cfg.Capability, "switchLevel")
		out.Command = override(cfg.SetCommand, "setLevel")
		if cmd.Value != nil {
			out.Arguments = []any{cmd.Value}
		}
		return out, devicectl.StatusUnknown, nil
	}
	return stCommand{}, devicectl.StatusUnknown, fmt.Errorf("%w: %s", devicectl.ErrUnsupportedCommand, cmd.Type)
}

func override(custom, fallback string) string {
	if custom != "" {
		return custom
	}
	return fallback
}

func (d *Driver) Execute(ctx context.Context, t devicectl.Target, cmd devicectl.Command) (devicectl.Result, error) {
	cfg, err := parseConfig(t.Config)
	if err != nil {
		return devicectl.Result{}, err
	}
	stCmd, expected, err := mapCommand(cmd, cfg)
	if err != nil {
		return devicectl.Result{}, err
	}
	body := map[string]any{"commands": []stCommand{stCmd}}
	raw, err := d.postJSON(ctx, cfg, "/devices/"+cfg.DeviceID+"/commands", body)
	if err != nil {
		return devicectl.Result{Online: false}, err
	}
	_ = raw
	return devicectl.Result{
		Status:   expected,
		Online:   true,
		LastSeen: time.Now().UTC(),
	}, nil
}

func (d *Driver) Probe(ctx context.Context, t devicectl.Target) (devicectl.Result, error) {
	cfg, err := parseConfig(t.Config)
	if err != nil {
		return devicectl.Result{}, err
	}
	raw, err := d.getJSON(ctx, cfg, "/devices/"+cfg.DeviceID+"/status")
	if err != nil {
		return devicectl.Result{Online: false}, err
	}
	return devicectl.Result{
		Status:   extractStatus(raw, cfg),
		Online:   true,
		LastSeen: time.Now().UTC(),
		Raw:      raw,
	}, nil
}

func (d *Driver) postJSON(ctx context.Context, cfg *config, path string, body any) (map[string]any, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	return d.do(ctx, http.MethodPost, cfg, path, bytes.NewReader(bodyJSON))
}

func (d *Driver) getJSON(ctx context.Context, cfg *config, path string) (map[string]any, error) {
	return d.do(ctx, http.MethodGet, cfg, path, nil)
}

func (d *Driver) baseFor(cfg *config) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	return d.baseURL
}

func (d *Driver) do(ctx context.Context, method string, cfg *config, path string, body io.Reader) (map[string]any, error) {
	url := strings.TrimRight(d.baseFor(cfg), "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: SmartThings %d: %s", devicectl.ErrUnavailable, resp.StatusCode, string(raw))
	}
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	if parsed == nil {
		parsed = map[string]any{}
	}
	return parsed, nil
}

// extractStatus walks components.<component>.switch.switch.value for the common
// switch capability. For other capabilities callers can inspect Result.Raw.
func extractStatus(raw map[string]any, cfg *config) devicectl.Status {
	comps, ok := raw["components"].(map[string]any)
	if !ok {
		return devicectl.StatusUnknown
	}
	comp, ok := comps[cfg.Component].(map[string]any)
	if !ok {
		return devicectl.StatusUnknown
	}
	cap := override(cfg.Capability, "switch")
	capBlock, ok := comp[cap].(map[string]any)
	if !ok {
		return devicectl.StatusUnknown
	}
	inner, ok := capBlock[cap].(map[string]any)
	if !ok {
		return devicectl.StatusUnknown
	}
	switch v := inner["value"].(type) {
	case string:
		return mapState(v)
	}
	return devicectl.StatusUnknown
}

func mapState(s string) devicectl.Status {
	switch strings.ToLower(s) {
	case "on":
		return devicectl.StatusOn
	case "off":
		return devicectl.StatusOff
	case "open":
		return devicectl.StatusOpen
	case "closed":
		return devicectl.StatusClosed
	}
	return devicectl.StatusUnknown
}
