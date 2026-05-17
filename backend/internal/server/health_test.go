package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCheck struct {
	name string
	err  error
}

func (f fakeCheck) Name() string                  { return f.name }
func (f fakeCheck) Check(_ context.Context) error { return f.err }

type slowCheck struct {
	name string
	d    time.Duration
}

func (s slowCheck) Name() string { return s.name }
func (s slowCheck) Check(ctx context.Context) error {
	select {
	case <-time.After(s.d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// envelope mirrors the shape that httpx.JSON wraps responses in: {"data": <body>}.
type envelope struct {
	Data ReadinessReport `json:"data"`
}

func TestReadyz_AllOk(t *testing.T) {
	h := readyzHandler([]ReadinessCheck{fakeCheck{name: "postgres"}, fakeCheck{name: "ha"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var got envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "ok", got.Data.Status)
	assert.Equal(t, "ok", got.Data.Checks["postgres"].Status)
	assert.Equal(t, "ok", got.Data.Checks["ha"].Status)
}

func TestReadyz_OneCheckFails_Returns503(t *testing.T) {
	h := readyzHandler([]ReadinessCheck{
		fakeCheck{name: "postgres"},
		fakeCheck{name: "ha", err: errors.New("connection refused")},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"any failing check must downgrade the whole report to 503 — that's the load-balancer's signal to stop sending traffic")

	var got envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "unready", got.Data.Status)
	assert.Equal(t, "ok", got.Data.Checks["postgres"].Status)
	assert.Equal(t, "fail", got.Data.Checks["ha"].Status)
	assert.Equal(t, "connection refused", got.Data.Checks["ha"].Error)
}

func TestReadyz_PerCheckTimeout_TwoSeconds(t *testing.T) {
	slow := slowCheck{name: "ha", d: 5 * time.Second}
	h := readyzHandler([]ReadinessCheck{slow})

	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	elapsed := time.Since(start)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Less(t, elapsed, 3*time.Second,
		"each check has its own 2s timeout, so /readyz returns within ~2s even when a check would block forever")
}

func TestReadyz_NoChecksRegistered_ReportsOk(t *testing.T) {
	h := readyzHandler(nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHassCheck_OkOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := HassCheck{BaseURL: srv.URL, Client: srv.Client()}
	require.NoError(t, c.Check(context.Background()))
}

func TestHassCheck_OkOn401_StillReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := HassCheck{BaseURL: srv.URL, Client: srv.Client()}
	assert.NoError(t, c.Check(context.Background()),
		"401 means HA is alive but auth is required — readiness only cares about reachability")
}

func TestHassCheck_FailOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := HassCheck{BaseURL: srv.URL, Client: srv.Client()}
	assert.Error(t, c.Check(context.Background()))
}
