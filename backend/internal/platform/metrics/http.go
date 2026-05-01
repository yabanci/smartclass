package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// HTTPMiddleware records two metrics per request: a request counter labeled
// by method/route/status, and a duration histogram labeled by method/route.
//
// `route` is chi's matched pattern (e.g., `/api/v1/devices/{id}`) — never the
// raw URL — so /devices/abc and /devices/xyz collapse into one label series.
// Without this the cardinality of `route` would be unbounded.
//
// When called outside a chi router (e.g., in unit tests or for the bare
// /healthz endpoint), `chi.RouteContext` returns nil and we fall back to the
// raw URL path. That's acceptable because the outside-chi paths are short and
// fixed in our codebase (/healthz, /readyz, /metrics).
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &httpStatusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			route = rctx.RoutePattern()
		}
		status := strconv.Itoa(rec.status)

		HTTPRequests.WithLabelValues(r.Method, route, status).Inc()
		HTTPDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// httpStatusRecorder captures the response status so we can label the counter
// with it. We intentionally don't try to be a full http.Hijacker/Flusher
// shim — RequestLogger already does that, and HTTPMiddleware is installed
// AFTER RequestLogger in the stack, so its statusRecorder sits between us
// and the real ResponseWriter.
type httpStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *httpStatusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
