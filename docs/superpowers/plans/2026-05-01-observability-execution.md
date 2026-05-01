# Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Prometheus metrics on every external call + business KPI, multi-dependency `/readyz` JSON, log enrichment with `user_id`/`role`/`subsystem`, plus `docs/observability/` scrape config and dashboard.

**Architecture:** Single owning package `backend/internal/platform/metrics/` exposes typed metric handles backed by a private `*prometheus.Registry`. Existing call sites get tiny instrumentation hooks (HTTP middleware, pgx `QueryTracer`, driver/HA wrappers, business `Inc()` calls). `/readyz` is rebuilt around a `ReadinessCheck` interface. Log enrichment uses a "principal slot" pointer in the request context so the outermost `RequestLogger` middleware can read principal data written later by `Authn`.

**Tech Stack:** Go 1.25 · pgx v5.9.2 · chi v5.2.5 · zap · `github.com/prometheus/client_golang` (new dep).

**Source spec:** `docs/superpowers/specs/2026-05-01-observability-design.md`

---

## File map

```
backend/
├── go.mod                                                         ← +1 dep
├── internal/platform/metrics/                                     ← NEW PACKAGE
│   ├── metrics.go            — Registry + 15 typed handles + Reset()
│   ├── http.go               — chi-compatible HTTPMiddleware
│   ├── db.go                 — pgx.QueryTracer + WithDBOp(ctx, op)
│   ├── driver.go             — TrackDriver helper used by every driver
│   └── *_test.go             — 25+ unit tests
├── internal/platform/httpx/middleware/
│   ├── principal.go          — NEW: principal-slot context pattern
│   ├── auth.go               — write principal into slot when Authn succeeds
│   ├── logger.go             — read principal slot, log user_id/role
│   └── principal_test.go     — NEW
├── internal/server/
│   ├── server.go             — Mount /metrics; new readyz handler
│   ├── health.go             — NEW: ReadinessCheck interface + handler
│   └── health_test.go        — NEW
├── internal/devicectl/drivers/{generic,homeassistant,smartthings}/driver.go
│                             — wrap http.Do calls in metrics.TrackDriver
├── internal/hass/client.go   — wrap requestJSON / AbortFlow in TrackHass
├── internal/notification/service.go      ← +1 line
├── internal/scene/service.go             ← +1-2 lines
├── internal/auth/service.go              ← +3-4 lines
├── internal/realtime/ws/hub.go           ← gauge inc/dec, publish counter
├── cmd/server/main.go        — wire DB tracer, register readiness checks
└── (every service that has a logger) — `.With(zap.String("subsystem", "..."))` in constructor

docs/observability/                                                 ← NEW
├── prometheus.yml            — sample scrape config
└── dashboard.json            — sample Grafana dashboard

README.md                     ← +1 section "Local observability"
```

**Per-task discipline.** Each task is its own commit. After every task: run `go build ./... && go test -race ./<changed-package>/...` from `backend/`. Final task runs full regression (`go test -race ./...`, `staticcheck`, `gosec`, `govulncheck`, `flutter test`, `flutter analyze`).

**Working directory.** All `backend/`-prefixed paths are relative to `/Users/arsenozhetov/Projects/pet/smartclass`. Run go commands from `backend/` unless otherwise stated.

---

## Task 1: Add prometheus dependency + bootstrap empty metrics package

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`
- Create: `backend/internal/platform/metrics/metrics.go` (placeholder)

- [ ] **Step 1: Add the dependency**

```bash
cd backend && go get github.com/prometheus/client_golang@latest
go mod tidy
```

Expected: `go.mod` lists `github.com/prometheus/client_golang vX.Y.Z` under `require`.

- [ ] **Step 2: Create empty package skeleton**

Create `backend/internal/platform/metrics/metrics.go`:

```go
// Package metrics owns every counter, gauge, and histogram the smartclass
// backend exposes on /metrics. It uses a private *prometheus.Registry rather
// than the global default to keep our metrics surface explicit and to let
// tests rebuild the registry between cases.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Registry is the package-private Prometheus registry. The HTTP /metrics
// handler reads exactly this registry, so accidentally-imported third-party
// metrics don't leak through.
var Registry *prometheus.Registry

func init() {
	Registry = prometheus.NewRegistry()
}
```

- [ ] **Step 3: Verify it builds**

```bash
cd backend && go build ./...
```

Expected: exit 0, no output.

- [ ] **Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/platform/metrics/metrics.go
git -c commit.gpgsign=false commit -m "feat(metrics): add prometheus/client_golang dep + empty package"
```

---

## Task 2: Define all 15 typed metric handles + Reset() for tests

**Files:**
- Modify: `backend/internal/platform/metrics/metrics.go`
- Create: `backend/internal/platform/metrics/metrics_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/platform/metrics/metrics_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/platform/metrics/...
```

Expected: FAIL — undefined: `metrics.HTTPRequests` etc.

- [ ] **Step 3: Implement metric handles**

Replace `backend/internal/platform/metrics/metrics.go` with:

```go
// Package metrics owns every counter, gauge, and histogram the smartclass
// backend exposes on /metrics. It uses a private *prometheus.Registry rather
// than the global default to keep our metrics surface explicit and to let
// tests rebuild the registry between cases.
package metrics

import "github.com/prometheus/client_golang/prometheus"

const namespace = "cctv_smartclass"

// Bucket strategies. HTTP/Driver/HA buckets cover 5ms → ~10s in 12 exponential
// steps; DB buckets cover 1ms → ~2s. These match the observed latency span on
// the smartclass backend (sub-50ms hot routes vs. multi-second HA flow steps).
var (
	httpBuckets   = prometheus.ExponentialBuckets(0.005, 2, 12)
	dbBuckets     = prometheus.ExponentialBuckets(0.001, 2, 12)
	driverBuckets = prometheus.ExponentialBuckets(0.005, 2, 12)
	hassBuckets   = prometheus.ExponentialBuckets(0.005, 2, 12)
)

// Registry is the package-private Prometheus registry. The HTTP /metrics
// handler reads exactly this registry, so accidentally-imported third-party
// metrics don't leak through.
var Registry *prometheus.Registry

// All metric handles below. Constructed in init() and re-constructed in
// Reset(). Tests call Reset() to start from zero counters.
var (
	HTTPRequests         *prometheus.CounterVec
	HTTPDuration         *prometheus.HistogramVec
	DBQueries            *prometheus.CounterVec
	DBDuration           *prometheus.HistogramVec
	DriverCalls          *prometheus.CounterVec
	DriverDuration       *prometheus.HistogramVec
	HassCalls            *prometheus.CounterVec
	HassDuration         *prometheus.HistogramVec
	WSConnected          prometheus.Gauge
	WSMessagesPublished  *prometheus.CounterVec
	AuthLogins           *prometheus.CounterVec
	AuthRefresh          *prometheus.CounterVec
	AuthReplayDetected   prometheus.Counter
	NotificationsCreated *prometheus.CounterVec
	ScenesRun            *prometheus.CounterVec
)

func init() { build() }

// Reset rebuilds the Registry and every metric handle. Tests call this in
// their setup so counter state from a previous test doesn't leak in. Safe to
// call from a single test goroutine; not safe to call concurrently with
// metric writes.
func Reset() { build() }

func build() {
	Registry = prometheus.NewRegistry()

	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_total",
			Help: "HTTP requests handled, partitioned by method, matched route, and status.",
		},
		[]string{"method", "route", "status"},
	)
	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
			Help: "HTTP request latency, partitioned by method and matched route.", Buckets: httpBuckets,
		},
		[]string{"method", "route"},
	)

	DBQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "db", Name: "queries_total",
			Help: "SQL queries executed, partitioned by op name and result (ok|err).",
		},
		[]string{"op", "result"},
	)
	DBDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "db", Name: "query_duration_seconds",
			Help: "SQL query latency, partitioned by op name.", Buckets: dbBuckets,
		},
		[]string{"op"},
	)

	DriverCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "driver", Name: "calls_total",
			Help: "Device-driver command calls, partitioned by driver, command type, and result.",
		},
		[]string{"driver", "command", "result"},
	)
	DriverDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "driver", Name: "call_duration_seconds",
			Help: "Device-driver command latency, partitioned by driver and command type.", Buckets: driverBuckets,
		},
		[]string{"driver", "command"},
	)

	HassCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "hass", Name: "calls_total",
			Help: "Home Assistant API calls, partitioned by op name and result.",
		},
		[]string{"op", "result"},
	)
	HassDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "hass", Name: "call_duration_seconds",
			Help: "Home Assistant API call latency, partitioned by op name.", Buckets: hassBuckets,
		},
		[]string{"op"},
	)

	WSConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "ws", Name: "connected",
			Help: "Number of currently-connected WebSocket clients.",
		},
	)
	WSMessagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "ws", Name: "messages_published_total",
			Help: "WebSocket events published to subscribers, partitioned by topic kind (user|classroom).",
		},
		[]string{"topic_kind"},
	)

	AuthLogins = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "auth", Name: "logins_total",
			Help: "Login attempts, partitioned by result (ok|invalid).",
		},
		[]string{"result"},
	)
	AuthRefresh = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "auth", Name: "refresh_total",
			Help: "Refresh-token attempts, partitioned by result (ok|invalid|replay).",
		},
		[]string{"result"},
	)
	AuthReplayDetected = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "auth", Name: "replay_detected_total",
			Help: "Refresh-token replay attempts detected (the user's other sessions are revoked).",
		},
	)

	NotificationsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "notification", Name: "created_total",
			Help: "Notifications created, partitioned by type (warning|info).",
		},
		[]string{"type"},
	)

	ScenesRun = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "scene", Name: "run_total",
			Help: "Scene runs, partitioned by overall result (ok|partial|err).",
		},
		[]string{"result"},
	)

	Registry.MustRegister(
		HTTPRequests, HTTPDuration,
		DBQueries, DBDuration,
		DriverCalls, DriverDuration,
		HassCalls, HassDuration,
		WSConnected, WSMessagesPublished,
		AuthLogins, AuthRefresh, AuthReplayDetected,
		NotificationsCreated, ScenesRun,
	)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/platform/metrics/...
```

Expected: PASS — 2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/metrics/
git -c commit.gpgsign=false commit -m "feat(metrics): define 15 typed handles + Reset() for tests"
```

---

## Task 3: HTTP middleware (count + histogram per request)

**Files:**
- Create: `backend/internal/platform/metrics/http.go`
- Create: `backend/internal/platform/metrics/http_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/platform/metrics/http_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failure**

```bash
cd backend && go test ./internal/platform/metrics/... -run HTTPMiddleware
```

Expected: FAIL — undefined `metrics.HTTPMiddleware`.

- [ ] **Step 3: Implement middleware**

Create `backend/internal/platform/metrics/http.go`:

```go
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
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
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

// statusRecorder captures the response status so we can label the counter
// with it. We intentionally don't try to be a full `http.Hijacker`/`Flusher`
// shim — the `RequestLogger` already does that, and we install
// `HTTPMiddleware` AFTER `RequestLogger` in the stack, so its statusRecorder
// sits between us and the real ResponseWriter.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/platform/metrics/...
```

Expected: PASS — 5 tests total (2 from Task 2 + 3 new).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/metrics/http.go backend/internal/platform/metrics/http_test.go
git -c commit.gpgsign=false commit -m "feat(metrics): chi-aware HTTP middleware + 3 unit tests"
```

---

## Task 4: pgx QueryTracer with context-based op name

**Files:**
- Create: `backend/internal/platform/metrics/db.go`
- Create: `backend/internal/platform/metrics/db_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/platform/metrics/db_test.go`:

```go
package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestDBTracer_RecordsOkQuery(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "users.GetByEmail")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT * FROM users WHERE email=$1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.DBQueries.WithLabelValues("users.GetByEmail", "ok")))
	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DBDuration), 1)
}

func TestDBTracer_RecordsErrQuery(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "users.Insert")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("constraint violation")})

	got := testutil.ToFloat64(metrics.DBQueries.WithLabelValues("users.Insert", "err"))
	assert.Equal(t, 1.0, got, "queries returning a non-nil Err must be counted under result=err")
}

func TestDBTracer_FallsBackToUnknownWhenOpMissing(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	// No WithDBOp — bare context. The tracer must not panic and must label
	// with "unknown" so we can still see *something* in dashboards while
	// repos are progressively annotated.
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DBQueries.WithLabelValues("unknown", "ok")),
		"queries without an op annotation must fall back to op=unknown so dashboards still show traffic")
}

func TestDBTracer_DurationStartsAtTraceQueryStart(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "slow.op")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{})
	time.Sleep(10 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	// We can't read histogram bucket bounds via testutil cleanly, so just
	// confirm an observation was recorded.
	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DBDuration), 1)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd backend && go test ./internal/platform/metrics/... -run TestDBTracer
```

Expected: FAIL — undefined `metrics.NewDBTracer`, `metrics.WithDBOp`.

- [ ] **Step 3: Implement tracer**

Create `backend/internal/platform/metrics/db.go`:

```go
package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DBTracer satisfies pgx.QueryTracer. It records a counter per query result
// and a duration histogram. The op name comes from the context — repo
// functions call WithDBOp(ctx, "users.GetByEmail") before issuing a query.
// Queries without an op annotation fall back to op="unknown" so dashboards
// still show traffic while we progressively annotate repos.
type DBTracer struct{}

func NewDBTracer() *DBTracer { return &DBTracer{} }

type dbOpCtxKey struct{}
type dbStartCtxKey struct{}

// WithDBOp annotates ctx with the op name the next pgx query should report
// under. Repos call this just before pool.Query/QueryRow/Exec.
func WithDBOp(ctx context.Context, op string) context.Context {
	return context.WithValue(ctx, dbOpCtxKey{}, op)
}

func dbOpFrom(ctx context.Context) string {
	if op, ok := ctx.Value(dbOpCtxKey{}).(string); ok && op != "" {
		return op
	}
	return "unknown"
}

func (DBTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, dbStartCtxKey{}, time.Now())
}

func (DBTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	op := dbOpFrom(ctx)
	result := "ok"
	if data.Err != nil {
		result = "err"
	}
	DBQueries.WithLabelValues(op, result).Inc()
	if start, ok := ctx.Value(dbStartCtxKey{}).(time.Time); ok {
		DBDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/platform/metrics/...
```

Expected: PASS — 9 tests total.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/metrics/db.go backend/internal/platform/metrics/db_test.go
git -c commit.gpgsign=false commit -m "feat(metrics): pgx QueryTracer + WithDBOp(ctx, op) context helper"
```

---

## Task 5: Driver instrumentation helper

**Files:**
- Create: `backend/internal/platform/metrics/driver.go`
- Create: `backend/internal/platform/metrics/driver_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/platform/metrics/driver_test.go`:

```go
package metrics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestTrackDriver_OkPath(t *testing.T) {
	metrics.Reset()
	err := metrics.TrackDriver(context.Background(), "generic_http", "ON", func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok")))
	assert.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DriverDuration), 1)
}

func TestTrackDriver_ErrPath_LabelsAsErr(t *testing.T) {
	metrics.Reset()
	wantErr := errors.New("boom")
	err := metrics.TrackDriver(context.Background(), "homeassistant", "OFF", func(ctx context.Context) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr,
		"TrackDriver must return the inner function's error verbatim — instrumentation never swallows errors")
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("homeassistant", "OFF", "err")))
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd backend && go test ./internal/platform/metrics/... -run TestTrackDriver
```

Expected: FAIL — undefined `metrics.TrackDriver`.

- [ ] **Step 3: Implement helper**

Create `backend/internal/platform/metrics/driver.go`:

```go
package metrics

import (
	"context"
	"time"
)

// TrackDriver wraps a single driver command call. Drivers call it with the
// driver name, the command type, and a closure that performs the actual HTTP
// request to the device/integration. It increments the calls counter with
// the right `result` label, observes the duration histogram, and returns the
// inner error verbatim — instrumentation never silences errors.
func TrackDriver(ctx context.Context, driver, command string, fn func(ctx context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	result := "ok"
	if err != nil {
		result = "err"
	}
	DriverCalls.WithLabelValues(driver, command, result).Inc()
	DriverDuration.WithLabelValues(driver, command).Observe(time.Since(start).Seconds())
	return err
}

// TrackHass wraps a single Home Assistant API call (StartFlow, StepFlow,
// AbortFlow, requestJSON). Same shape as TrackDriver, different metric.
func TrackHass(ctx context.Context, op string, fn func(ctx context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	result := "ok"
	if err != nil {
		result = "err"
	}
	HassCalls.WithLabelValues(op, result).Inc()
	HassDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
	return err
}
```

- [ ] **Step 4: Add a TrackHass test**

Append to `backend/internal/platform/metrics/driver_test.go`:

```go

func TestTrackHass_Counts(t *testing.T) {
	metrics.Reset()
	_ = metrics.TrackHass(context.Background(), "ListEntities", func(_ context.Context) error { return nil })
	_ = metrics.TrackHass(context.Background(), "AbortFlow", func(_ context.Context) error { return errors.New("404") })

	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("ListEntities", "ok")))
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("AbortFlow", "err")))
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test ./internal/platform/metrics/...
```

Expected: PASS — 12 tests total.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/platform/metrics/driver.go backend/internal/platform/metrics/driver_test.go
git -c commit.gpgsign=false commit -m "feat(metrics): TrackDriver + TrackHass helpers"
```

---

## Task 6: Wire HTTP middleware + DB tracer + /metrics endpoint

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Mount /metrics + add HTTPMiddleware**

In `backend/internal/server/server.go`, add to imports:

```go
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"smartclass/internal/platform/metrics"
```

Then change the middleware stack and add the metrics mount. Find this block:

```go
	r.Use(mw.Recoverer(d.Logger))
	r.Use(mw.RequestID)
	r.Use(mw.RequestLogger(d.Logger))
	r.Use(mw.CORS(d.Cfg.CORS.Origins))
	r.Use(mw.Language)
	r.Use(mw.BodyLimit(mw.MaxBodyBytes))
	r.Use(rl.Middleware())

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(d.Readiness))
```

Replace with:

```go
	r.Use(mw.Recoverer(d.Logger))
	r.Use(mw.RequestID)
	r.Use(mw.RequestLogger(d.Logger))
	r.Use(metrics.HTTPMiddleware)
	r.Use(mw.CORS(d.Cfg.CORS.Origins))
	r.Use(mw.Language)
	r.Use(mw.BodyLimit(mw.MaxBodyBytes))
	r.Use(rl.Middleware())

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(d.Readiness))
	// /metrics is intentionally unauthenticated. The endpoint is meant to be
	// scraped by an in-cluster Prometheus; in production this listener must
	// be locked down at the proxy/firewall layer (see README §"Local
	// observability"). For pet-project localhost use, no auth is fine.
	r.Mount("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{
		Registry:          metrics.Registry,
		EnableOpenMetrics: true,
	}))
```

- [ ] **Step 2: Wire pgx tracer in main.go**

In `backend/cmd/server/main.go`, find:

```go
	db, err := postgres.Connect(ctx, cfg.DB.DSN())
```

Above that line, add a comment + capture the tracer:

```go
	// Pass the metrics tracer into the pool so every SQL query gets counted.
	// Repos progressively annotate their queries via metrics.WithDBOp(ctx,
	// "<op>"); unannotated queries fall back to op="unknown".
	dbTracer := metrics.NewDBTracer()
```

Add to imports:

```go
	"smartclass/internal/platform/metrics"
```

Now update `backend/internal/platform/postgres/db.go` `Connect` function. Find:

```go
func Connect(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}

	cfg.MaxConns = 20
```

Change the signature to accept an optional tracer:

```go
func Connect(ctx context.Context, dsn string, tracer pgx.QueryTracer) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
	}

	cfg.MaxConns = 20
```

Update import in `db.go`:

```go
import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)
```

Now update the call site in `main.go`. Find:

```go
	db, err := postgres.Connect(ctx, cfg.DB.DSN())
```

Replace with:

```go
	db, err := postgres.Connect(ctx, cfg.DB.DSN(), dbTracer)
```

- [ ] **Step 3: Build + test**

```bash
cd backend && go build ./... && go test -count=1 ./internal/server/... ./internal/platform/postgres/... ./internal/platform/metrics/...
```

Expected: build OK; all tests pass.

- [ ] **Step 4: Smoke-test /metrics endpoint**

We can't run a live server in CI, but we can unit-test that promhttp serves our registry. Add to `backend/internal/server/server_test.go` — create the file:

```go
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
```

- [ ] **Step 5: Run smoke test**

```bash
cd backend && go test ./internal/server/... -run TestMetricsEndpoint
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/server/server.go backend/internal/server/server_test.go backend/internal/platform/postgres/db.go backend/cmd/server/main.go
git -c commit.gpgsign=false commit -m "feat(server): mount /metrics + wire pgx QueryTracer"
```

---

## Task 7: Annotate auth Login + Refresh queries with WithDBOp

**Files:**
- Modify: `backend/internal/auth/service.go` (only the queries that hit the DB through user.Repository — actually they go through repo, so we annotate user repo)
- Modify: `backend/internal/user/postgres.go`

The auth flow is the highest-leverage place to annotate first because of F-009 (refresh-token replay). After this task, every `auth_*` metric will pair up with a meaningful db `users.*` op label.

- [ ] **Step 1: Read user/postgres.go to find the queries**

```bash
grep -n "QueryRow\|Query\b\|Exec\b" backend/internal/user/postgres.go
```

- [ ] **Step 2: Annotate each query**

Open `backend/internal/user/postgres.go`. For each function that does a DB call, replace `r.pool.QueryRow(ctx, q, ...)` with the WithDBOp-annotated form.

Example: in `GetByEmail`:

```go
// before
err := r.pool.QueryRow(ctx, q, email).Scan(...)

// after — annotate ctx so the pgx tracer labels this op
err := r.pool.QueryRow(metrics.WithDBOp(ctx, "users.GetByEmail"), q, email).Scan(...)
```

Add to imports:

```go
	"smartclass/internal/platform/metrics"
```

Apply to **every** function in `user/postgres.go`. Use the function name as the op (`users.GetByEmail`, `users.GetByID`, `users.Create`, `users.Update`, `users.UpdatePassword`, `users.UpdateFCMToken`, `users.Delete`).

- [ ] **Step 3: Annotate the new auth refresh-token postgres store**

In `backend/internal/auth/postgres.go`, similarly wrap each query:

- `Track` → `auth.refresh.Track`
- `Status` → `auth.refresh.Status`
- `MarkUsed` → `auth.refresh.MarkUsed`
- `RevokeUser` → `auth.refresh.RevokeUser`

Example for `Track`:

```go
// before
if _, err := s.pool.Exec(ctx, q, jti, userID, expiresAt); err != nil {

// after
if _, err := s.pool.Exec(metrics.WithDBOp(ctx, "auth.refresh.Track"), q, jti, userID, expiresAt); err != nil {
```

Add the metrics import.

- [ ] **Step 4: Build + run auth tests**

```bash
cd backend && go build ./... && go test -count=1 ./internal/auth/... ./internal/user/...
```

Expected: PASS — existing tests don't observe the metric, so they keep passing.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/user/postgres.go backend/internal/auth/postgres.go
git -c commit.gpgsign=false commit -m "feat(metrics): annotate user + auth.refresh queries with WithDBOp"
```

---

## Task 8: Instrument all 3 device drivers

**Files:**
- Modify: `backend/internal/devicectl/drivers/generic/driver.go`
- Modify: `backend/internal/devicectl/drivers/homeassistant/driver.go`
- Modify: `backend/internal/devicectl/drivers/smartthings/driver.go`

Each driver has a single `Apply` (or similarly-named) method that issues an HTTP request. Wrap the actual HTTP call in `metrics.TrackDriver`.

- [ ] **Step 1: Read the three drivers to find call sites**

```bash
grep -n "func .*Apply\|client\.Do\|http\.Do" backend/internal/devicectl/drivers/generic/driver.go backend/internal/devicectl/drivers/homeassistant/driver.go backend/internal/devicectl/drivers/smartthings/driver.go
```

- [ ] **Step 2: Generic driver**

Open `backend/internal/devicectl/drivers/generic/driver.go`. Find the `Apply` method body (something like `func (d *Driver) Apply(ctx, cfg, cmd) error`). Wrap the inner work:

```go
// before
func (d *Driver) Apply(ctx context.Context, cfg map[string]any, cmd devicectl.Command) error {
	// ... build request, do request, parse response
}

// after
func (d *Driver) Apply(ctx context.Context, cfg map[string]any, cmd devicectl.Command) error {
	return metrics.TrackDriver(ctx, "generic_http", string(cmd.Type), func(ctx context.Context) error {
		// ... existing body unchanged ...
	})
}
```

Add to imports:

```go
	"smartclass/internal/platform/metrics"
```

- [ ] **Step 3: Homeassistant driver**

Same pattern, but driver name `"homeassistant"`:

```go
return metrics.TrackDriver(ctx, "homeassistant", string(cmd.Type), func(ctx context.Context) error {
    // ... existing body ...
})
```

- [ ] **Step 4: Smartthings driver**

Same pattern, driver name `"smartthings"`.

- [ ] **Step 5: Build + run all driver tests**

```bash
cd backend && go build ./... && go test -count=1 ./internal/devicectl/...
```

Expected: PASS — driver tests don't assert metrics; they keep passing.

- [ ] **Step 6: Add an end-to-end driver-metrics test**

Create `backend/internal/devicectl/drivers/generic/metrics_test.go`:

```go
package generic_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/generic"
	"smartclass/internal/platform/metrics"
)

func TestGenericDriver_IncrementsMetricOnApply(t *testing.T) {
	metrics.Reset()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := generic.New(nil)
	cfg := map[string]any{"baseUrl": server.URL, "onPath": "/relay/0?turn=on"}
	err := d.Apply(context.Background(), cfg, devicectl.Command{Type: devicectl.CommandTypeOn})
	require.NoError(t, err)

	got := testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok"))
	require.Equal(t, 1.0, got,
		"a successful Apply call must increment the driver counter labeled (generic_http, ON, ok)")
}
```

- [ ] **Step 7: Run the new test**

```bash
cd backend && go test -count=1 ./internal/devicectl/drivers/generic/... -run TestGenericDriver_Increments
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/devicectl/drivers/
git -c commit.gpgsign=false commit -m "feat(metrics): instrument generic_http/homeassistant/smartthings drivers"
```

---

## Task 9: Instrument hass.Client

**Files:**
- Modify: `backend/internal/hass/client.go`

The HA client has 4 methods that issue HTTP: `CreateOwner`, `StartFlow`, `StepFlow`, `AbortFlow`, `ListEntities`, `requestJSON`. Wrap each.

- [ ] **Step 1: Wrap requestJSON (the shared helper)**

Open `backend/internal/hass/client.go`. Find `func (c *Client) requestJSON(...)`. Wrap the body:

```go
// before
func (c *Client) requestJSON(ctx context.Context, method, token, path string, body io.Reader, out any) error {
	// #nosec G704 -- baseURL is operator-configured (HA endpoint), path is internal/validated input.
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	// ... rest unchanged ...
}

// after
func (c *Client) requestJSON(ctx context.Context, method, token, path string, body io.Reader, out any) error {
	return metrics.TrackHass(ctx, "requestJSON", func(ctx context.Context) error {
		// #nosec G704 -- baseURL is operator-configured (HA endpoint), path is internal/validated input.
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		// ... rest of original body ...
	})
}
```

- [ ] **Step 2: Wrap AbortFlow (special-cased above the shared helper)**

```go
func (c *Client) AbortFlow(ctx context.Context, token, flowID string) error {
	if !flowIDPattern.MatchString(flowID) {
		return ErrInvalidFlowID
	}
	return metrics.TrackHass(ctx, "AbortFlow", func(ctx context.Context) error {
		// #nosec G704 -- baseURL is operator-configured (HA endpoint); flowID is regex-validated above.
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/config/config_entries/flow/"+flowID, nil)
		// ... existing body ...
	})
}
```

- [ ] **Step 3: Wrap CreateOwner**

```go
func (c *Client) CreateOwner(ctx context.Context, name, username, password, lang string) (string, error) {
	var out string
	err := metrics.TrackHass(ctx, "CreateOwner", func(ctx context.Context) error {
		// existing body, but instead of `return ...` use `out = ..., return nil` etc.
		// ... existing body ...
		out = code
		return nil
	})
	return out, err
}
```

For functions that already return `(value, error)`, capture into a local `out` then return it.

- [ ] **Step 4: Add metrics import**

```go
import (
	// ... existing ...
	"smartclass/internal/platform/metrics"
)
```

- [ ] **Step 5: Build + run hass tests**

```bash
cd backend && go build ./... && go test -count=1 ./internal/hass/...
```

Expected: PASS — the existing tests use `httptest.NewServer`, so they exercise the wrapped paths.

- [ ] **Step 6: Add a hass metrics test**

Append to `backend/internal/hass/service_test.go` (or create a small `client_metrics_test.go`):

```go
package hass

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestClient_RequestJSON_RecordsHassCallMetric(t *testing.T) {
	metrics.Reset()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound) // triggers ErrFlowNotFound on AbortFlow
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	_ = c.AbortFlow(context.Background(), "tok", "abc-123")

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("AbortFlow", "ok")),
		"AbortFlow returning ErrFlowNotFound is not an err for metrics — the upstream signaled the expected 404 cleanly. "+
			"That semantic is set by the wrapped function returning nil-or-domain-error vs transport-error.")
}
```

(Note: if AbortFlow's internal logic returns nil for 404, the result label is "ok"; if it returns ErrFlowNotFound, depending on how the wrapper sees errors, this may need adjustment. Verify by reading the post-wrap code.)

- [ ] **Step 7: Run + commit**

```bash
cd backend && go test -count=1 ./internal/hass/...
git add backend/internal/hass/
git -c commit.gpgsign=false commit -m "feat(metrics): instrument hass.Client requestJSON/AbortFlow/CreateOwner"
```

---

## Task 10: Instrument WebSocket hub (gauge + publish counter)

**Files:**
- Modify: `backend/internal/realtime/ws/hub.go`

- [ ] **Step 1: Add gauge inc/dec**

In `backend/internal/realtime/ws/hub.go`, find `func (h *Hub) Register(c *Client)` and `func (h *Hub) Unregister(c *Client)`. Add metric updates:

```go
import (
	// ...existing...
	"smartclass/internal/platform/metrics"
)

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byID[c.ID] = c
	for t := range c.topics {
		if h.byTopic[t] == nil {
			h.byTopic[t] = map[string]*Client{}
		}
		h.byTopic[t][c.ID] = c
	}
	metrics.WSConnected.Inc()
	h.log.Debug("ws: client registered", zap.String("id", c.ID), zap.Int("topics", len(c.topics)))
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.byID, c.ID)
	for t := range c.topics {
		if subs, ok := h.byTopic[t]; ok {
			delete(subs, c.ID)
			if len(subs) == 0 {
				delete(h.byTopic, t)
			}
		}
	}
	h.mu.Unlock()
	c.close()
	metrics.WSConnected.Dec()
	h.log.Debug("ws: client unregistered", zap.String("id", c.ID))
}
```

- [ ] **Step 2: Add publish counter**

In `func (h *Hub) Publish(...)`, after the topic lookup:

```go
func (h *Hub) Publish(_ context.Context, event realtime.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// ... existing fan-out logic ...

	// Count the publish regardless of subscriber count (a 0-subscriber publish
	// is still a published event for the producer's perspective).
	metrics.WSMessagesPublished.WithLabelValues(topicKind(event.Topic)).Inc()
	return nil
}

// topicKind extracts "user" or "classroom" or "other" from a topic string.
// Bounded label cardinality: only those 3 values can be returned.
func topicKind(topic string) string {
	switch {
	case strings.HasPrefix(topic, "user:"):
		return "user"
	case strings.HasPrefix(topic, "classroom:"):
		return "classroom"
	default:
		return "other"
	}
}
```

(`strings` already imported in `hub.go`? If not add it.)

- [ ] **Step 3: Add a hub-metrics test**

Append to `backend/internal/realtime/ws/hub_test.go`:

```go
func TestHub_RegisterIncrementsConnectedGauge(t *testing.T) {
	metrics.Reset()
	hub := NewHub(nil)

	a := newClient("a", []string{"user:x:notifications"})
	hub.Register(a)
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.WSConnected),
		"Register must bump the connected gauge so dashboards show live count")

	hub.Unregister(a)
	require.Equal(t, 0.0, testutil.ToFloat64(metrics.WSConnected),
		"Unregister must decrement the gauge — gauge would otherwise leak forever")
}

func TestHub_PublishIncrementsByTopicKind(t *testing.T) {
	metrics.Reset()
	hub := NewHub(nil)
	_ = hub.Publish(context.Background(), realtime.Event{Topic: "user:abc:notifications", Type: "x"})
	_ = hub.Publish(context.Background(), realtime.Event{Topic: "classroom:c1:devices", Type: "y"})

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.WSMessagesPublished.WithLabelValues("user")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.WSMessagesPublished.WithLabelValues("classroom")))
}
```

Add to imports:
```go
	"github.com/prometheus/client_golang/prometheus/testutil"
	"smartclass/internal/platform/metrics"
```

- [ ] **Step 4: Run + commit**

```bash
cd backend && go test -race -count=1 ./internal/realtime/...
git add backend/internal/realtime/ws/hub.go backend/internal/realtime/ws/hub_test.go
git -c commit.gpgsign=false commit -m "feat(metrics): WS connected gauge + messages_published counter"
```

---

## Task 11: Wire all 5 business KPIs

**Files:**
- Modify: `backend/internal/auth/service.go`
- Modify: `backend/internal/notification/service.go`
- Modify: `backend/internal/scene/service.go`

- [ ] **Step 1: auth.Service — login/refresh/replay**

In `backend/internal/auth/service.go`, find each path and add an `Inc()`:

```go
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = normalizeEmail(email)
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			metrics.AuthLogins.WithLabelValues("invalid").Inc()
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := s.hash.Compare(u.PasswordHash, password); err != nil {
		metrics.AuthLogins.WithLabelValues("invalid").Inc()
		return nil, ErrInvalidCredentials
	}
	pair, err := s.issuePair(ctx, u)
	if err != nil {
		return nil, err
	}
	metrics.AuthLogins.WithLabelValues("ok").Inc()
	return &LoginResult{User: u, Tokens: pair}, nil
}
```

For `Refresh`, add increments at the right branches (rejection paths get `invalid` or `replay`; happy path gets `ok`):

```go
// at top of Refresh, after the JWT parse error path:
if err != nil || claims.Kind != tokens.KindRefresh {
	metrics.AuthRefresh.WithLabelValues("invalid").Inc()
	return nil, ErrInvalidRefresh
}

// in the replay branch (status.IsUsed() or MarkUsed returns ErrRefreshAlreadyUsed):
metrics.AuthRefresh.WithLabelValues("replay").Inc()
metrics.AuthReplayDetected.Inc()
// ... existing revoke + return ErrInvalidRefresh ...

// in the unknown / unknown-user / not-live branches:
metrics.AuthRefresh.WithLabelValues("invalid").Inc()
return nil, ErrInvalidRefresh

// at the end on success, just before `return &LoginResult{...}`:
metrics.AuthRefresh.WithLabelValues("ok").Inc()
return &LoginResult{User: u, Tokens: pair}, nil
```

Add `"smartclass/internal/platform/metrics"` import.

- [ ] **Step 2: notification.Service — created counter**

In `backend/internal/notification/service.go`, in `CreateForUser` and `CreateForClassroom`, after the successful repo write:

```go
// CreateForUser
if err := s.repo.Create(ctx, n); err != nil {
	return nil, err
}
metrics.NotificationsCreated.WithLabelValues(string(in.Type)).Inc()
s.publish(ctx, n)
return n, nil

// CreateForClassroom (after batch insert)
if err := s.repo.CreateBatch(ctx, items); err != nil {
	return nil, err
}
for range items {
	metrics.NotificationsCreated.WithLabelValues(string(in.Type)).Inc()
}
for _, n := range items {
	s.publish(ctx, n)
}
return items, nil
```

Add `"smartclass/internal/platform/metrics"` import.

- [ ] **Step 3: scene.Service — run counter**

In `backend/internal/scene/service.go`, at the end of `Run`, add result classification + counter inc:

```go
out := &RunResult{SceneID: sc.ID, Steps: results}
if firstErr != nil {
	failedCount := 0
	for _, r := range results {
		if !r.Success {
			failedCount++
		}
	}
	if failedCount == len(results) {
		metrics.ScenesRun.WithLabelValues("err").Inc()
	} else {
		metrics.ScenesRun.WithLabelValues("partial").Inc()
	}
	return out, fmt.Errorf("%w: %v", ErrStepFailed, firstErr)
}
metrics.ScenesRun.WithLabelValues("ok").Inc()
return out, nil
```

Add `"smartclass/internal/platform/metrics"` import.

- [ ] **Step 4: Update auth tests to verify counters**

In `backend/internal/auth/service_test.go`, after each scenario test, add a counter check. Pick one representative — e.g., in `TestService_Refresh_RotatesAndDetectsReplay` "second use" subtest, append:

```go
		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.AuthRefresh.WithLabelValues("replay")),
			"replay attempt must be counted under result=replay so we can alert on it")
		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.AuthReplayDetected),
			"the dedicated replay counter must increment too — it's the alerting handle")
```

Add imports + reset call at top of the test:

```go
import (
	// ... existing ...
	"github.com/prometheus/client_golang/prometheus/testutil"
	"smartclass/internal/platform/metrics"
)

func TestService_Refresh_RotatesAndDetectsReplay(t *testing.T) {
	metrics.Reset()
	// ...
}
```

- [ ] **Step 5: Build + test**

```bash
cd backend && go build ./... && go test -race -count=1 ./internal/auth/... ./internal/notification/... ./internal/scene/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/auth/service.go backend/internal/notification/service.go backend/internal/scene/service.go backend/internal/auth/service_test.go
git -c commit.gpgsign=false commit -m "feat(metrics): wire auth/notification/scene business KPI counters"
```

---

## Task 12: Multi-check /readyz

**Files:**
- Create: `backend/internal/server/health.go`
- Create: `backend/internal/server/health_test.go`
- Modify: `backend/internal/server/server.go` (replace existing readyz)
- Modify: `backend/cmd/server/main.go` (build the check list)

- [ ] **Step 1: Write failing test**

Create `backend/internal/server/health_test.go`:

```go
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

func (f fakeCheck) Name() string                              { return f.name }
func (f fakeCheck) Check(_ context.Context) error             { return f.err }
func (f fakeCheck) Slow(d time.Duration) ReadinessCheck       { return slowCheck{name: f.name, d: d} }

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

func TestReadyz_AllOk(t *testing.T) {
	h := readyzHandler([]ReadinessCheck{fakeCheck{name: "postgres"}, fakeCheck{name: "ha"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Data ReadinessReport `json:"data"`
	}
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

	var got struct {
		Data ReadinessReport `json:"data"`
	}
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
```

- [ ] **Step 2: Run test to verify failure**

```bash
cd backend && go test ./internal/server/... -run TestReadyz
```

Expected: FAIL — undefined `readyzHandler`, `ReadinessCheck`, `ReadinessReport`.

- [ ] **Step 3: Implement health.go**

Create `backend/internal/server/health.go`:

```go
package server

import (
	"context"
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
	Status  string `json:"status"`            // "ok" or "fail"
	Latency string `json:"latency"`           // "12ms"
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

// PostgresCheck wraps a DB pool's Ping. The DB type already implements a
// Ready(ctx) method from F-011; we adapt it to the Name+Check shape.
type PostgresCheck struct {
	DB pinger
}

type pinger interface {
	Ready(ctx context.Context) error
}

func (PostgresCheck) Name() string                       { return "postgres" }
func (p PostgresCheck) Check(ctx context.Context) error  { return p.DB.Ready(ctx) }
```

- [ ] **Step 4: Replace readyz in server.go**

Open `backend/internal/server/server.go`. Find the existing `readyz` function and `Readiness ReadinessChecker` field on `Deps`:

```go
type Deps struct {
	// ...
	Readiness           ReadinessChecker
	// ...
}
```

Replace with the multi-check field:

```go
type Deps struct {
	// ...
	ReadinessChecks []ReadinessCheck   // ordered list of named dependency checks
	// ...
}
```

Delete the old `ReadinessChecker` interface (single-check) — now obsolete. Delete the old `readyz` function in server.go (the new one is in health.go).

Find the line:

```go
	r.Get("/readyz", readyz(d.Readiness))
```

Replace with:

```go
	r.Get("/readyz", readyzHandler(d.ReadinessChecks))
```

- [ ] **Step 5: Wire from main.go**

In `backend/cmd/server/main.go`, find the `Readiness: db,` line in the `server.Deps{...}` literal. Replace:

```go
		Readiness:           db,
```

with:

```go
		ReadinessChecks:     []server.ReadinessCheck{server.PostgresCheck{DB: db}},
```

- [ ] **Step 6: Build + test**

```bash
cd backend && go build ./... && go test -count=1 ./internal/server/...
```

Expected: PASS — 4 new readyz tests + the existing /metrics test all pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/server/health.go backend/internal/server/health_test.go backend/internal/server/server.go backend/cmd/server/main.go
git -c commit.gpgsign=false commit -m "feat(server): multi-check /readyz with per-check 2s timeout"
```

---

## Task 13: HA readiness check (conditional)

**Files:**
- Modify: `backend/internal/server/health.go` (add HassCheck)
- Modify: `backend/cmd/server/main.go` (register conditionally)

- [ ] **Step 1: Add HassCheck**

Append to `backend/internal/server/health.go`:

```go
// HassCheck wraps a lightweight HTTP probe at the configured Home Assistant
// base URL. We hit GET /api/ which HA serves as a small JSON banner — any 2xx
// or 401 (unauth but reachable) means the HA process is alive. 5xx, network
// errors, or timeouts mean it's not.
//
// Registered only when cfg.Hass.Enabled — otherwise the check would always
// fail and confuse operators reading /readyz.
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
	// Any < 500 is "alive" — even 401 (auth required) means the HA process
	// is up and responding. We're checking liveness, not authorization.
	if resp.StatusCode >= 500 {
		return fmt.Errorf("homeassistant: status %d", resp.StatusCode)
	}
	return nil
}
```

Add to imports:

```go
import (
	// ... existing ...
	"fmt"
	"net/http"
)
```

- [ ] **Step 2: Add a HassCheck unit test**

Append to `backend/internal/server/health_test.go`:

```go
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
		"401 means HA is alive but auth is required — readiness only cares about reachability, not authz")
}

func TestHassCheck_FailOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := HassCheck{BaseURL: srv.URL, Client: srv.Client()}
	assert.Error(t, c.Check(context.Background()))
}
```

- [ ] **Step 3: Wire conditionally in main.go**

In `backend/cmd/server/main.go`, replace:

```go
		ReadinessChecks:     []server.ReadinessCheck{server.PostgresCheck{DB: db}},
```

with:

```go
		ReadinessChecks:     buildReadinessChecks(cfg, db),
```

Then add the helper at file scope (e.g., near the end of `main.go`):

```go
// buildReadinessChecks composes the list of checks to expose at /readyz.
// Postgres is always present. HA is added only when cfg.Hass.Enabled — when
// HA is disabled, including the check would surface a permanent "fail" entry
// which confuses operators reading the report.
func buildReadinessChecks(cfg config.Config, db *postgres.DB) []server.ReadinessCheck {
	checks := []server.ReadinessCheck{server.PostgresCheck{DB: db}}
	if cfg.Hass.Enabled && cfg.Hass.URL != "" {
		checks = append(checks, server.HassCheck{BaseURL: cfg.Hass.URL})
	}
	return checks
}
```

(Adjust `cfg.Hass.Enabled` / `cfg.Hass.URL` to whatever the actual config field names are in `internal/config`. If not present, infer from the existing HA wiring in main.go and use the same condition.)

- [ ] **Step 4: Build + test**

```bash
cd backend && go build ./... && go test -count=1 ./internal/server/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/health.go backend/internal/server/health_test.go backend/cmd/server/main.go
git -c commit.gpgsign=false commit -m "feat(server): conditional HA readiness check (3 unit tests)"
```

---

## Task 14: Principal-slot pattern for log enrichment

**Files:**
- Create: `backend/internal/platform/httpx/middleware/principal.go`
- Create: `backend/internal/platform/httpx/middleware/principal_test.go`
- Modify: `backend/internal/platform/httpx/middleware/auth.go` (write to slot)
- Modify: `backend/internal/platform/httpx/middleware/logger.go` (read from slot)

- [ ] **Step 1: Write failing test**

Create `backend/internal/platform/httpx/middleware/principal_test.go`:

```go
package middleware

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrincipalSlot_RoundTrip(t *testing.T) {
	slot := &PrincipalSlot{}
	ctx := WithPrincipalSlot(context.Background(), slot)

	got := PrincipalSlotFrom(ctx)
	require.NotNil(t, got)
	assert.False(t, got.Set, "fresh slot starts unset")

	uid := uuid.New()
	got.Principal = Principal{UserID: uid, Role: "teacher"}
	got.Set = true

	// Re-fetch from ctx to confirm the slot is the same pointer (mutable
	// shared container, not a copy) — that's what makes the outer logger see
	// updates the inner Authn made.
	again := PrincipalSlotFrom(ctx)
	require.True(t, again.Set, "the slot is a shared pointer; writes from any holder must be visible to all")
	assert.Equal(t, uid, again.Principal.UserID)
	assert.Equal(t, "teacher", again.Principal.Role)
}

func TestPrincipalSlotFrom_ReturnsNilWhenAbsent(t *testing.T) {
	got := PrincipalSlotFrom(context.Background())
	assert.Nil(t, got, "callers without WithPrincipalSlot must safely get nil — no panic")
}
```

- [ ] **Step 2: Run failing**

```bash
cd backend && go test ./internal/platform/httpx/middleware/... -run TestPrincipalSlot
```

Expected: FAIL — undefined.

- [ ] **Step 3: Implement principal.go**

Create `backend/internal/platform/httpx/middleware/principal.go`:

```go
package middleware

import "context"

// PrincipalSlot is a mutable container for principal information set by
// Authn and read by RequestLogger. It exists because the outermost
// RequestLogger can't see the principal that an inner Authn writes via
// `r.WithContext(...)` — that's a child context, invisible to the parent.
//
// The slot solves this with a pointer: the outer middleware puts a fresh
// *PrincipalSlot in the context up front; Authn writes into it; the outer
// middleware reads after next.ServeHTTP returns.
type PrincipalSlot struct {
	Principal Principal
	Set       bool
}

type principalSlotKey struct{}

// WithPrincipalSlot returns a context carrying slot. RequestLogger calls
// this before invoking next.ServeHTTP.
func WithPrincipalSlot(ctx context.Context, slot *PrincipalSlot) context.Context {
	return context.WithValue(ctx, principalSlotKey{}, slot)
}

// PrincipalSlotFrom returns the slot pointer, or nil if WithPrincipalSlot
// was never called for this request. Returning nil (not a zero struct) lets
// callers distinguish "no slot configured" from "slot present but empty".
func PrincipalSlotFrom(ctx context.Context) *PrincipalSlot {
	slot, _ := ctx.Value(principalSlotKey{}).(*PrincipalSlot)
	return slot
}
```

- [ ] **Step 4: Update RequestLogger to install + read the slot**

Open `backend/internal/platform/httpx/middleware/logger.go`. In `RequestLogger`, add the slot:

```go
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			// Install a mutable slot so an inner Authn middleware can write
			// the principal, and we read it back here for log enrichment.
			slot := &PrincipalSlot{}
			ctx := WithPrincipalSlot(r.Context(), slot)

			next.ServeHTTP(rec, r.WithContext(ctx))

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Int("bytes", rec.bytes),
				zap.Duration("took", time.Since(start)),
				zap.String("remote", r.RemoteAddr),
			}
			if id := RequestIDFrom(r.Context()); id != "" {
				fields = append(fields, zap.String("request_id", id))
			}
			if slot.Set {
				fields = append(fields, zap.Stringer("user_id", slot.Principal.UserID),
					zap.String("role", slot.Principal.Role))
			}
			logger.Info("http", fields...)
		})
	}
}
```

- [ ] **Step 5: Update Authn to write into the slot**

Open `backend/internal/platform/httpx/middleware/auth.go`. Find `func Authn(...)`. After the JWT successfully parses and Kind matches, write into the slot:

```go
func Authn(issuer tokens.Issuer, bundle *i18n.Bundle) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "unauthorized"), nil)
				return
			}
			claims, err := issuer.Parse(token)
			if err != nil || claims.Kind != tokens.KindAccess {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "auth.invalid_token"), nil)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxKeyRole, claims.Role)

			// Mirror the principal into the slot so the outer RequestLogger
			// can include user_id+role on the log line. The downstream
			// PrincipalFrom helper still reads from the immutable ctx values,
			// unchanged.
			if slot := PrincipalSlotFrom(r.Context()); slot != nil {
				slot.Principal = Principal{UserID: claims.UserID, Role: claims.Role}
				slot.Set = true
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 6: Add an integration test in middleware package**

Append to `backend/internal/platform/httpx/middleware/principal_test.go`:

```go

import (
	// ... above imports
	"net/http"
	"net/http/httptest"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLogger_IncludesUserIDAfterAuthn(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	// Compose: RequestLogger (outer) → Authn-imitator (inner) → handler.
	authnImitator := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slot := PrincipalSlotFrom(r.Context()); slot != nil {
				slot.Principal = Principal{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Role: "teacher"}
				slot.Set = true
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := RequestLogger(logger)(authnImitator(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, 1, logs.Len(), "exactly one http log line per request")
	entry := logs.All()[0]
	fields := map[string]any{}
	for _, f := range entry.Context {
		fields[f.Key] = f.String
	}
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", fields["user_id"],
		"user_id must be on the log line — that's the whole point of the slot pattern")
	assert.Equal(t, "teacher", fields["role"])
}
```

- [ ] **Step 7: Run all middleware tests**

```bash
cd backend && go test -count=1 ./internal/platform/httpx/middleware/...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/platform/httpx/middleware/
git -c commit.gpgsign=false commit -m "feat(observability): principal-slot for log enrichment with user_id/role"
```

---

## Task 15: Per-subsystem logger fields

**Files:**
- Modify: `backend/internal/auth/service.go`, `backend/internal/notification/{service.go,trigger.go}`, `backend/internal/scene/service.go`, `backend/internal/device/service.go`, `backend/internal/sensor/service.go`, `backend/internal/auditlog/service.go`, `backend/internal/hass/service.go`, `backend/internal/realtime/ws/{hub.go,handler.go}`

For every service that owns a `*zap.Logger`, in its constructor (`NewService`/`NewEngine`/`NewHub`), wrap the supplied logger with a `subsystem` field.

- [ ] **Step 1: auth.Service**

In `backend/internal/auth/service.go`, find the place in `NewService` where `logger` is assigned to `s.logger` (or where it falls back to Nop):

```go
// before
func NewService(users user.Repository, hash hasher.Hasher, issuer tokens.Issuer, store RefreshStore, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{users: users, hash: hash, issuer: issuer, store: store, logger: logger}
}

// after
func NewService(users user.Repository, hash hasher.Hasher, issuer tokens.Issuer, store RefreshStore, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{users: users, hash: hash, issuer: issuer, store: store, logger: logger.With(zap.String("subsystem", "auth"))}
}
```

- [ ] **Step 2: notification.Service + Engine**

In `backend/internal/notification/service.go`, in `NewService` (and any `WithLogger` setter), wrap with subsystem:

```go
func (s *Service) WithLogger(l *zap.Logger) *Service {
	if l != nil {
		s.log = l.With(zap.String("subsystem", "notification"))
	}
	return s
}
```

Same for `Engine.WithLogger` in `trigger.go`:

```go
func (e *Engine) WithLogger(l *zap.Logger) *Engine {
	if l != nil {
		e.log = l.With(zap.String("subsystem", "notification.trigger"))
	}
	return e
}
```

- [ ] **Step 3-7: Apply to scene, device, sensor, auditlog, hass services**

For each constructor or WithLogger setter, wrap with `subsystem`:

- `scene.Service.WithLogger` → `"scene"`
- `device.Service.WithLogger` → `"device"`
- `sensor.Service.WithLogger` → `"sensor"` (if it exists; otherwise skip)
- `auditlog.Service.WithLogger` → `"auditlog"`
- `hass.Service.WithLogger` → `"hass"`

(Use `grep -rn "WithLogger\|s\.log =" backend/internal --include="*.go"` to find all sites.)

- [ ] **Step 8: ws.Hub**

In `backend/internal/realtime/ws/hub.go`:

```go
func NewHub(log *zap.Logger) *Hub {
	if log == nil {
		log = zap.NewNop()
	}
	return &Hub{
		byID:    map[string]*Client{},
		byTopic: map[string]map[string]*Client{},
		log:     log.With(zap.String("subsystem", "ws.hub")),
	}
}
```

In `backend/internal/realtime/ws/handler.go` `NewHandler`:

```go
return &Handler{hub: hub, log: log.With(zap.String("subsystem", "ws")), bundle: bundle, authz: authz}
```

(Adjust for the `if log == nil` branch.)

- [ ] **Step 9: Build + run all tests**

```bash
cd backend && go build ./... && go test -race -count=1 ./...
```

Expected: PASS — adding a `With` field is purely additive; nothing in any test asserts log fields by content.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/
git -c commit.gpgsign=false commit -m "feat(observability): per-subsystem logger fields for log filtering"
```

---

## Task 16: docs/observability artifacts + README

**Files:**
- Create: `docs/observability/prometheus.yml`
- Create: `docs/observability/dashboard.json`
- Modify: `README.md`

- [ ] **Step 1: Create prometheus.yml**

Create `docs/observability/prometheus.yml`:

```yaml
# Sample Prometheus scrape config for local smartclass observability.
#
# Run from repo root:
#   docker run --rm -p 9090:9090 \
#     -v "$PWD/docs/observability/prometheus.yml:/etc/prometheus/prometheus.yml" \
#     prom/prometheus
#
# Then open http://localhost:9090 and try queries like:
#   sum(rate(cctv_smartclass_http_requests_total[1m])) by (status)
#   histogram_quantile(0.95, sum(rate(cctv_smartclass_http_request_duration_seconds_bucket[5m])) by (le, route))
#   increase(cctv_smartclass_auth_replay_detected_total[1h])

global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'smartclass'
    metrics_path: '/metrics'
    static_configs:
      # When running Prometheus in Docker on macOS/Windows, host.docker.internal
      # resolves to the host machine. On Linux replace with localhost or the
      # backend container's IP if running everything in one compose network.
      - targets: ['host.docker.internal:8080']
```

- [ ] **Step 2: Create dashboard.json**

Create `docs/observability/dashboard.json` with a minimal Grafana dashboard. (This is a long JSON; including a working skeleton.)

```json
{
  "title": "Smartclass — Operations",
  "uid": "smartclass-ops",
  "schemaVersion": 39,
  "panels": [
    {
      "type": "timeseries",
      "title": "HTTP request rate (per status)",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
      "targets": [{"expr": "sum by (status) (rate(cctv_smartclass_http_requests_total[1m]))"}]
    },
    {
      "type": "timeseries",
      "title": "HTTP p95 latency by route",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
      "targets": [{"expr": "histogram_quantile(0.95, sum by (le, route) (rate(cctv_smartclass_http_request_duration_seconds_bucket[5m])))"}]
    },
    {
      "type": "timeseries",
      "title": "DB query rate by op",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
      "targets": [{"expr": "sum by (op) (rate(cctv_smartclass_db_queries_total[1m]))"}]
    },
    {
      "type": "timeseries",
      "title": "DB error ratio",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
      "targets": [{"expr": "sum(rate(cctv_smartclass_db_queries_total{result=\"err\"}[5m])) / sum(rate(cctv_smartclass_db_queries_total[5m]))"}]
    },
    {
      "type": "stat",
      "title": "Active WebSocket clients",
      "gridPos": {"h": 4, "w": 6, "x": 0, "y": 16},
      "targets": [{"expr": "cctv_smartclass_ws_connected"}]
    },
    {
      "type": "stat",
      "title": "Replay detections (last 1h)",
      "gridPos": {"h": 4, "w": 6, "x": 6, "y": 16},
      "targets": [{"expr": "increase(cctv_smartclass_auth_replay_detected_total[1h])"}],
      "fieldConfig": {"defaults": {"thresholds": {"steps": [{"color": "green", "value": 0}, {"color": "red", "value": 1}]}}}
    },
    {
      "type": "timeseries",
      "title": "Driver call rate by driver/result",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
      "targets": [{"expr": "sum by (driver, result) (rate(cctv_smartclass_driver_calls_total[1m]))"}]
    }
  ]
}
```

- [ ] **Step 3: Update README**

Open `README.md`. Find a good insertion point (after the "Day-to-day" section). Insert:

```markdown
## Local observability

The backend exposes Prometheus metrics at `GET /metrics`. To scrape them
locally:

```bash
docker run --rm -p 9090:9090 \
  -v "$PWD/docs/observability/prometheus.yml:/etc/prometheus/prometheus.yml" \
  prom/prometheus
```

Then open <http://localhost:9090> and try queries like:

- `sum(rate(cctv_smartclass_http_requests_total[1m])) by (status)`
- `histogram_quantile(0.95, sum(rate(cctv_smartclass_http_request_duration_seconds_bucket[5m])) by (le, route))`
- `increase(cctv_smartclass_auth_replay_detected_total[1h])`

For graphs: `docs/observability/dashboard.json` is a Grafana dashboard you
can import (Grafana → Dashboards → Import → Upload JSON).

`/healthz` is liveness (always 200 if the process is alive). `/readyz` is
readiness — it returns 503 + a per-check JSON breakdown when any
dependency (Postgres, Home Assistant) is unreachable.
```

- [ ] **Step 4: Commit**

```bash
git add docs/observability/ README.md
git -c commit.gpgsign=false commit -m "docs(observability): sample prometheus.yml + Grafana dashboard + README"
```

---

## Task 17: Final regression run

**Files:** No changes — verification only.

- [ ] **Step 1: All Go checks**

```bash
export PATH=$PATH:$(go env GOPATH)/bin
cd backend
go vet ./... && echo "vet OK"
staticcheck ./... && echo "staticcheck OK"
govulncheck ./... 2>&1 | tail -3
gosec -quiet ./... 2>&1 | tail -7
go test -race -count=1 ./... 2>&1 | grep -E "FAIL|ok\s" | tail -25
```

Expected: each tool reports zero issues; all packages PASS under `-race`.

- [ ] **Step 2: Coverage**

```bash
cd backend && go test -coverprofile=/tmp/cov.out ./... > /dev/null 2>&1
go tool cover -func=/tmp/cov.out | tail -1
rm /tmp/cov.out
```

Expected: total ≥ 28% (was 27.3%; new metrics tests + readyz tests + principal-slot tests add ~1-2 percentage points).

- [ ] **Step 3: Mobile regression**

```bash
cd /Users/arsenozhetov/Projects/pet/smartclass
(cd mobile && flutter analyze && flutter test --reporter=compact 2>&1 | tail -3)
```

Expected: 59/59 Flutter tests pass; analyze clean.

- [ ] **Step 4: Live smoke**

(Optional, requires running stack.)

```bash
make up
sleep 5
curl -s http://localhost:8080/metrics | head -50
curl -s http://localhost:8080/readyz | jq
make down
```

Expected: `/metrics` returns Prometheus text format including `cctv_smartclass_http_requests_total`. `/readyz` returns `{"data": {"status": "ok", "checks": {"postgres": {"status": "ok", "latency": "..."}}}}`.

- [ ] **Step 5: Final commit if any tweaks were needed**

```bash
git status
# If clean, no commit needed.
# If tweaks made:
git add .
git -c commit.gpgsign=false commit -m "chore(observability): regression fixes from final run"
```

- [ ] **Step 6: Print summary**

Output to user: number of metrics now exposed (15), number of new tests (≥25), `/metrics` endpoint live, `/readyz` JSON shape. Reference the audit findings closed by this work: F-029 (zero metrics → 15), F-013 extension (user_id/role on logs), F-011 extension (multi-check readyz).

---

## Self-Review

**Spec coverage check (against `2026-05-01-observability-design.md`):**
- §3 Architecture / file layout → Tasks 1-2, 12, 14. ✓
- §4 Metrics catalogue (15 metrics) → Tasks 2, 3, 4, 5, 8, 9, 10, 11. ✓
- §5 Readiness extension → Tasks 12-13. ✓
- §6 Log enrichment via principal slot → Task 14. ✓
- §6 Per-subsystem logger fields → Task 15. ✓
- §7 Testing (~25 tests) → Tasks 2-6, 8-12, 14 each ship tests. ✓ (Total: 30+ new tests.)
- §8 Deliverables (prometheus.yml + dashboard.json + README) → Task 16. ✓
- §9 Done criteria → Task 17 verifies. ✓

**Placeholder scan:** No "TBD"/"TODO"/"fill in details" inside task steps. The README scrape config and dashboard JSON contain real values. ✓

**Type/name consistency:**
- `metrics.HTTPRequests`, `metrics.HTTPDuration`, `metrics.DBQueries`, etc. — same names used across Tasks 2 (declaration) and 3-11 (consumption). ✓
- `metrics.WithDBOp`, `metrics.NewDBTracer` — used consistently in Tasks 4 (declaration) and 7 (consumption). ✓
- `metrics.TrackDriver`, `metrics.TrackHass` — Task 5 declaration, Tasks 8-9 consumption. ✓
- `ReadinessCheck` interface, `PostgresCheck`, `HassCheck`, `ReadinessReport`, `CheckResult` — Tasks 12-13 self-consistent. ✓
- `PrincipalSlot`, `WithPrincipalSlot`, `PrincipalSlotFrom` — Task 14 self-consistent. ✓
- Per-subsystem `subsystem` field strings (`"auth"`, `"notification"`, etc.) — Task 15 self-consistent (one constant per service).

Plan ready.
