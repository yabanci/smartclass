package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestHTTPMiddleware_CountsByRouteAndStatus(t *testing.T) {
	metrics.Reset()
	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Get("/items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/abc", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/items/{id}", "200"))
	assert.Equal(t, 1.0, got,
		"label `route` must use chi's matched pattern, never the raw URL — otherwise "+
			"every unique id explodes label cardinality")
}

func TestHTTPMiddleware_RecordsDuration(t *testing.T) {
	metrics.Reset()
	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	count := testutil.CollectAndCount(metrics.HTTPDuration)
	assert.GreaterOrEqual(t, count, 1, "duration histogram must record at least one observation")
}

func TestHTTPMiddleware_FallsBackToRawPathOutsideChi(t *testing.T) {
	metrics.Reset()
	// Wrap directly without a chi router — middleware must not panic.
	h := metrics.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	// Without chi we fall back to URL.Path. Pet-project healthz/metrics endpoints
	// take this path; confirm they get recorded as the raw path, not panic.
	got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/probe", "200"))
	assert.Equal(t, 1.0, got)
}
