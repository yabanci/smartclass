// Package generic is a configurable HTTP driver suitable for devices that
// expose a plain REST API (Shelly Gen1, Sonoff LAN, custom relays, any
// simple webhook-style endpoint). Configuration lives in the device's
// Config map so no code change is needed per-device.
//
// Example Config:
//
//	{
//	  "baseUrl": "http://192.168.1.50",
//	  "headers": {"Authorization": "Bearer xxx"},
//	  "commands": {
//	    "ON":  {"method": "POST", "path": "/relay/0?turn=on"},
//	    "OFF": {"method": "POST", "path": "/relay/0?turn=off"}
//	  },
//	  "status": {"method": "GET", "path": "/status"}
//	}
package generic

import (
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

const Name = "generic_http"

type Driver struct {
	client *http.Client
}

func New(client *http.Client) *Driver {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Driver{client: client}
}

func (d *Driver) Name() string { return Name }

type endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body,omitempty"`
}

type config struct {
	BaseURL  string              `json:"baseUrl"`
	Headers  map[string]string   `json:"headers"`
	Commands map[string]endpoint `json:"commands"`
	Status   *endpoint           `json:"status"`
}

func parseConfig(raw map[string]any) (*config, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	cfg := &config{}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrInvalidConfig, err)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("%w: baseUrl required", devicectl.ErrInvalidConfig)
	}
	return cfg, nil
}

func (d *Driver) Execute(ctx context.Context, t devicectl.Target, cmd devicectl.Command) (devicectl.Result, error) {
	var out devicectl.Result
	err := metrics.TrackDriver(ctx, Name, string(cmd.Type), func(ctx context.Context) error {
		cfg, err := parseConfig(t.Config)
		if err != nil {
			return err
		}
		ep, ok := cfg.Commands[string(cmd.Type)]
		if !ok {
			return fmt.Errorf("%w: %s", devicectl.ErrUnsupportedCommand, cmd.Type)
		}
		body := substituteValue(ep.Body, cmd.Value)
		raw, err := d.do(ctx, cfg, ep, body)
		if err != nil {
			out = devicectl.Result{Online: false}
			return err
		}
		out = devicectl.Result{
			Status:   inferStatus(cmd.Type),
			Online:   true,
			LastSeen: time.Now().UTC(),
			Raw:      raw,
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
	if cfg.Status == nil {
		return devicectl.Result{Status: devicectl.StatusUnknown, Online: false}, nil
	}
	raw, err := d.do(ctx, cfg, *cfg.Status, "")
	if err != nil {
		return devicectl.Result{Status: devicectl.StatusUnknown, Online: false}, err
	}
	return devicectl.Result{
		Status:   parseRawStatus(raw),
		Online:   true,
		LastSeen: time.Now().UTC(),
		Raw:      raw,
	}, nil
}

func (d *Driver) do(ctx context.Context, cfg *config, ep endpoint, body string) (map[string]any, error) {
	if ep.Method == "" {
		ep.Method = http.MethodGet
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/" + strings.TrimLeft(ep.Path, "/")
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, ep.Method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", devicectl.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read: %v", devicectl.ErrUnavailable, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: http %d", devicectl.ErrUnavailable, resp.StatusCode)
	}
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	if parsed == nil {
		parsed = map[string]any{"raw": string(raw)}
	}
	return parsed, nil
}

func substituteValue(template string, value any) string {
	if template == "" {
		return ""
	}
	if value == nil {
		return template
	}
	return strings.ReplaceAll(template, "{{value}}", fmt.Sprintf("%v", value))
}

func inferStatus(cmd devicectl.CommandType) devicectl.Status {
	switch cmd {
	case devicectl.CmdOn:
		return devicectl.StatusOn
	case devicectl.CmdOff:
		return devicectl.StatusOff
	case devicectl.CmdOpen:
		return devicectl.StatusOpen
	case devicectl.CmdClose:
		return devicectl.StatusClosed
	}
	return devicectl.StatusUnknown
}

func parseRawStatus(raw map[string]any) devicectl.Status {
	if v, ok := raw["ison"].(bool); ok {
		if v {
			return devicectl.StatusOn
		}
		return devicectl.StatusOff
	}
	if v, ok := raw["state"].(string); ok {
		switch strings.ToLower(v) {
		case "on", "true":
			return devicectl.StatusOn
		case "off", "false":
			return devicectl.StatusOff
		case "open":
			return devicectl.StatusOpen
		case "closed", "close":
			return devicectl.StatusClosed
		}
	}
	return devicectl.StatusUnknown
}
