package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestRegistry_HasAllMetricsRegistered(t *testing.T) {
	// Re-register all handles under a fresh Registry — proves init wired them
	// (a typo would surface as "metric not found" or "already registered").
	metrics.Reset()

	// Sample one of each kind to confirm registration.
	metrics.HTTPRequests.WithLabelValues("GET", "/x", "200").Inc()
	metrics.DBQueries.WithLabelValues("test.op", "ok").Inc()
	metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok").Inc()
	metrics.HassCalls.WithLabelValues("ListEntities", "ok").Inc()
	metrics.WSConnected.Inc()
	metrics.WSMessagesPublished.WithLabelValues("user").Inc()
	metrics.AuthLogins.WithLabelValues("ok").Inc()
	metrics.AuthRefresh.WithLabelValues("ok").Inc()
	metrics.AuthReplayDetected.Inc()
	metrics.NotificationsCreated.WithLabelValues("warning").Inc()
	metrics.ScenesRun.WithLabelValues("ok").Inc()

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/x", "200")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.DBQueries.WithLabelValues("test.op", "ok")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("ListEntities", "ok")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.WSConnected))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.WSMessagesPublished.WithLabelValues("user")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.AuthLogins.WithLabelValues("ok")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.AuthRefresh.WithLabelValues("ok")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.AuthReplayDetected))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.NotificationsCreated.WithLabelValues("warning")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.ScenesRun.WithLabelValues("ok")))
}

func TestReset_ZeroesEveryCounter(t *testing.T) {
	metrics.Reset()
	metrics.AuthLogins.WithLabelValues("ok").Inc()
	metrics.AuthLogins.WithLabelValues("ok").Inc()
	require.Equal(t, 2.0, testutil.ToFloat64(metrics.AuthLogins.WithLabelValues("ok")))

	metrics.Reset()
	assert.Equal(t, 0.0, testutil.ToFloat64(metrics.AuthLogins.WithLabelValues("ok")),
		"Reset() must rebuild every metric so tests start from zero counters")
}
