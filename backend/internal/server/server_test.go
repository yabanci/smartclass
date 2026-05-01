package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestMetricsEndpoint_ServesOurRegistry(t *testing.T) {
	metrics.Reset()
	metrics.AuthLogins.WithLabelValues("ok").Inc()

	r := chi.NewRouter()
	r.Mount("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{Registry: metrics.Registry}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "cctv_smartclass_auth_logins_total",
		"the metric name we just incremented must appear in /metrics output — "+
			"if not, the registry mount is wrong")
	assert.Contains(t, body, `cctv_smartclass_auth_logins_total{result="ok"} 1`,
		"counter value must be exposed exactly")
}
