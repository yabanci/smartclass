package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"smartclass/internal/platform/httpx"
)

// ReadinessCheck represents one named dependency check. Implementations are
// small: postgres pings the pool, hass GETs /api/. New ones are added by
// passing a new struct into Deps.ReadinessChecks.
type ReadinessCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// ReadinessReport is the JSON body shape returned by /readyz.
type ReadinessReport struct {
	Status string                 `json:"status"`
	Checks map[string]CheckResult `json:"checks"`
}

// CheckResult is one row of the per-check status table.
type CheckResult struct {
	Status  string `json:"status"`
	Latency string `json:"latency"`
	Error   string `json:"error,omitempty"`
}

// readyzHandler runs every ReadinessCheck with a per-check 2-second timeout
// and assembles a JSON report. Any failing check downgrades the overall
// status to "unready" and the HTTP response to 503 — that's the signal to
// the orchestrator's load balancer to stop sending traffic.
func readyzHandler(checks []ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := ReadinessReport{
			Status: "ok",
			Checks: make(map[string]CheckResult, len(checks)),
		}
		for _, c := range checks {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			start := time.Now()
			err := c.Check(ctx)
			cancel()
			res := CheckResult{Latency: time.Since(start).Round(time.Millisecond).String()}
			if err != nil {
				res.Status = "fail"
				res.Error = err.Error()
				report.Status = "unready"
			} else {
				res.Status = "ok"
			}
			report.Checks[c.Name()] = res
		}
		status := http.StatusOK
		if report.Status == "unready" {
			status = http.StatusServiceUnavailable
		}
		httpx.JSON(w, status, report)
	}
}

// PostgresCheck wraps a DB pool's Ready method.
type PostgresCheck struct {
	DB pinger
}

type pinger interface {
	Ready(ctx context.Context) error
}

func (PostgresCheck) Name() string                      { return "postgres" }
func (p PostgresCheck) Check(ctx context.Context) error { return p.DB.Ready(ctx) }

// HassCheck wraps a lightweight HTTP probe at the configured Home Assistant
// base URL. We hit GET /api/ which HA serves as a small JSON banner — any
// status under 500 means the HA process is alive (401 means "alive but auth
// required", which is fine for liveness).
//
// Registered only when HA is enabled in config; including it unconditionally
// would surface a permanent "fail" entry on deployments without HA.
type HassCheck struct {
	BaseURL string
	Client  *http.Client
}

func (HassCheck) Name() string { return "homeassistant" }

func (h HassCheck) Check(ctx context.Context) error {
	if h.BaseURL == "" {
		return fmt.Errorf("homeassistant: BaseURL not configured")
	}
	cli := h.Client
	if cli == nil {
		cli = &http.Client{Timeout: 2 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/api/", nil) // #nosec G107 -- operator-configured URL
	if err != nil {
		return fmt.Errorf("homeassistant: build request: %w", err)
	}
	resp, err := cli.Do(req) // #nosec G107
	if err != nil {
		return fmt.Errorf("homeassistant: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("homeassistant: status %d", resp.StatusCode)
	}
	return nil
}
