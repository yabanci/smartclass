# Observability — Design Spec

**Date:** 2026-05-01
**Topic:** Add Prometheus metrics + readiness depth + structured-log enrichment to the smartclass backend.
**Status:** Approved (ready for plan)
**Source:** `docs/superpowers/audits/2026-05-01-deep-audit.md` finding F-029 (zero metrics/traces), F-013 (request-ID landed — trace correlation foundation), F-011 (readyz extension).

## 1. Purpose

Today the backend has structured logs and a Postgres-only `/readyz`. Anything else — request rates, latency percentiles, error rates per dependency, business KPIs — requires grepping logs by hand. This spec installs the smallest set of moving parts that makes the backend operationally observable.

Out of scope: distributed tracing (single process), Grafana docker-compose stack (separate one-day follow-up), `/metrics` authentication (pet project, localhost), alert rules (separate spec).

## 2. Scope

**In scope**
- New package `backend/internal/platform/metrics` exposing typed counters/gauges/histograms.
- HTTP middleware that records every request (method, route, status, latency).
- pgx `QueryTracer` that records every SQL query (op, latency, ok/err).
- Manual instrumentation in 3 device drivers + HA client.
- WebSocket hub gauges + publish counter.
- 5 business KPI counters (logins, refresh, replay, notifications, scenes).
- `/metrics` endpoint via `promhttp.Handler()` mounted on the same listener.
- `/readyz` extended: per-dependency JSON, pluggable checks, 2s per-check timeout, optional HA check.
- Structured-log enrichment: `user_id`, `role` from Authn middleware; `subsystem` field on every package's logger.
- Sample `prometheus.yml` and Grafana dashboard JSON in `docs/observability/`.
- README "how to scrape locally" snippet.

**Out of scope (with rationale)**
- Distributed tracing / OpenTelemetry — one process, no service mesh; revisit when a second service exists.
- Grafana + Prometheus in `docker-compose.yml` — separate spec; this one ships only `/metrics`.
- `/metrics` auth — pet project, localhost only; production would lock down at proxy.
- Alert rules / PagerDuty wiring — separate spec.
- MQTT readiness check — no Go consumer exists, so "broker reachable" is not a meaningful liveness signal for this service yet.

## 3. Architecture

One new package owns every counter/gauge/histogram. Existing code calls typed helpers (`metrics.Notifications.Inc("warning")`); never touches the underlying Prometheus types directly.

### File layout

```
backend/internal/platform/metrics/
├── metrics.go          — Registry singleton + typed metric handles (HTTP, DB, Driver, HA, WS, Business)
├── http.go             — chi-compatible middleware: request count + duration histogram
├── db.go               — pgx.QueryTracer implementation: query count + duration histogram per op
├── reset.go            — Reset() helper for unit tests (rebuilds the registry; testutil-friendly)
└── metrics_test.go     — 15+ unit tests verifying every metric increments under expected conditions

backend/internal/server/server.go
├── r.Use(metrics.HTTPMiddleware)        — wraps every chi route
└── r.Mount("/metrics", promhttp.HandlerFor(metrics.Registry, ...))

backend/internal/server/health.go        ← extracted; new ReadinessReport + per-check struct
backend/internal/server/health_test.go   ← new

backend/cmd/server/main.go
├── pgxpool.Config.ConnConfig.Tracer = metrics.NewDBTracer()    — instrument pool
└── d.Readiness wired with []ReadinessCheck{postgres, hass?}

backend/internal/devicectl/drivers/{generic,homeassistant,smartthings}/driver.go
└── single helper call replacing http.Do in each — adds counter+histogram

backend/internal/hass/client.go
└── helper around http.Do — counter+histogram, label = op name

backend/internal/notification/service.go    ← +1 line: metrics.NotificationsCreated.WithLabelValues(string(in.Type)).Inc()
backend/internal/scene/service.go           ← +1 line: metrics.ScenesRun.WithLabelValues(result).Inc()
backend/internal/auth/service.go            ← +3 lines (login, refresh, replay)
backend/internal/realtime/ws/hub.go         ← gauge inc/dec on Register/Unregister; publish counter

docs/observability/
├── prometheus.yml      — sample scrape config
└── dashboard.json      — sample Grafana dashboard
```

### Single-Registry Design

`metrics.Registry` is a package-level `*prometheus.Registry` (not the global default). Three benefits:

1. **Tests can call `metrics.Reset()`** which rebuilds the registry; no global state leaking between test packages.
2. **No collision with future packages** that import another instrumented library which uses the default registry.
3. **The `/metrics` handler reads exactly our registry** — no surprise metrics from third-party libs we didn't intend to expose.

Caller-facing API is typed; raw `prometheus.NewCounter` etc. live only inside the package.

```go
// metrics.go (sketch)
package metrics

import "github.com/prometheus/client_golang/prometheus"

var Registry = prometheus.NewRegistry()

var (
    HTTPRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "cctv_smartclass",
            Subsystem: "http",
            Name:      "requests_total",
            Help:      "HTTP requests handled, partitioned by method/route/status.",
        },
        []string{"method", "route", "status"},
    )
    HTTPDuration = prometheus.NewHistogramVec(...)
    DBQueries    = prometheus.NewCounterVec(...)
    // ... 13 more
)

func init() { Registry.MustRegister(HTTPRequests, HTTPDuration, ...) }
```

## 4. Metrics catalogue

Naming: `cctv_smartclass_<subsystem>_<noun>_<unit>`. Counters end in `_total`; histograms end in `_duration_seconds` (per Prometheus convention). Help text mandatory for each.

| Subsystem | Name | Type | Labels | Recorded by |
|---|---|---|---|---|
| http | `http_requests_total` | Counter | method, route, status | HTTPMiddleware |
| http | `http_request_duration_seconds` | Histogram | method, route | HTTPMiddleware |
| db | `db_queries_total` | Counter | op, result | DBTracer |
| db | `db_query_duration_seconds` | Histogram | op | DBTracer |
| driver | `driver_calls_total` | Counter | driver, command, result | each driver wrapper |
| driver | `driver_call_duration_seconds` | Histogram | driver, command | each driver wrapper |
| hass | `hass_calls_total` | Counter | op, result | hass.Client wrapper |
| hass | `hass_call_duration_seconds` | Histogram | op | hass.Client wrapper |
| ws | `ws_connected` | Gauge | — | hub.Register/Unregister |
| ws | `ws_messages_published_total` | Counter | topic_kind | hub.Publish |
| auth | `auth_logins_total` | Counter | result (ok\|invalid) | auth.Service.Login |
| auth | `auth_refresh_total` | Counter | result (ok\|invalid\|replay) | auth.Service.Refresh |
| auth | `auth_replay_detected_total` | Counter | — | auth.Service.Refresh (replay branch) |
| notification | `notifications_created_total` | Counter | type (warning\|info) | notification.Service.Create* |
| scene | `scenes_run_total` | Counter | result (ok\|partial\|err) | scene.Service.Run |

Histogram bucket strategy:
- HTTP/driver/HA: `prometheus.ExponentialBuckets(0.005, 2, 12)` — 5ms → ~10s, 12 buckets. Catches both fast routes (≤50ms) and slow external calls (HA flow at 2-3s).
- DB: `prometheus.ExponentialBuckets(0.001, 2, 12)` — 1ms → 2s. DB queries should be sub-second; the long tail is the alert signal.

### Label cardinality bound

`route` uses chi's matched pattern (`/api/v1/classrooms/{id}`), not the raw URL — bounded by route count. `op` for DB is a constant string per call site (e.g., `users.GetByEmail`), bounded by call-site count. `command` for drivers is the `devicectl.CommandType` enum. **No user-supplied input ever lands in a label.** This keeps Prometheus storage bounded and prevents a malicious user from causing label-cardinality explosion.

## 5. Readiness extension

Replace the single-checker `/readyz` with a list of named checks.

```go
// server/health.go
type ReadinessCheck interface {
    Name() string
    Check(ctx context.Context) error
}

type ReadinessReport struct {
    Status string                  `json:"status"`
    Checks map[string]CheckResult  `json:"checks"`
}

type CheckResult struct {
    Status  string `json:"status"`            // "ok" | "fail"
    Latency string `json:"latency"`           // "12ms"
    Error   string `json:"error,omitempty"`
}

func readyzHandler(checks []ReadinessCheck) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        report := ReadinessReport{Status: "ok", Checks: map[string]CheckResult{}}
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
```

Concrete checks ship in this spec:
- **`postgres`** — wraps `*postgres.DB.Pool.Ping(ctx)`. Always present.
- **`homeassistant`** — wraps a lightweight `GET /api/` against the configured HA base URL. **Only registered when `cfg.Hass.Enabled`** (otherwise the check would always fail and confuse operators).

Liveness (`/healthz`) stays a simple unconditional 200. Different consumer (orchestrator restart vs. load-balancer trafficking).

## 6. Structured-log enrichment

Two changes; small surface, high value.

**a) Add `user_id` + `role` to logs after Authn.**

The `Authn` middleware puts a `Principal` in the request context, but it does so on the *child* context from `r.WithContext(ctx)`. The outer `RequestLogger` middleware never sees that — by the time `next.ServeHTTP` returns, `RequestLogger`'s `r.Context()` is still the original parent without the principal.

The fix is a "principal slot" — a mutable container `RequestLogger` puts in the context up front, that `Authn` writes into when it succeeds:

```go
// middleware/principal.go (new tiny file)
type principalSlot struct {
    Principal Principal
    Set       bool
}
type principalSlotKey struct{}

// In RequestLogger (BEFORE next.ServeHTTP):
slot := &principalSlot{}
ctx := context.WithValue(r.Context(), principalSlotKey{}, slot)
next.ServeHTTP(rec, r.WithContext(ctx))
// After: slot may have been populated by Authn down the chain.

// In Authn (after successful JWT parse):
if slot, ok := r.Context().Value(principalSlotKey{}).(*principalSlot); ok {
    slot.Principal = Principal{UserID: claims.UserID, Role: claims.Role}
    slot.Set = true
}
ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
ctx = context.WithValue(ctx, ctxKeyRole, claims.Role)
next.ServeHTTP(w, r.WithContext(ctx))
```

The slot is *mutable* (pointer), so writes from a child context are visible to the parent's `RequestLogger`. The existing `PrincipalFrom` helper for downstream handlers is unchanged — it still reads from immutable context values.

```go
// Final log assembly in RequestLogger:
fields := []zap.Field{...}
if id := RequestIDFrom(r.Context()); id != "" {
    fields = append(fields, zap.String("request_id", id))
}
if slot.Set {
    fields = append(fields, zap.Stringer("user_id", slot.Principal.UserID),
                            zap.String("role", slot.Principal.Role))
}
logger.Info("http", fields...)
```

401s and unauthenticated routes naturally log without `user_id` — the slot stays unset, no special branch needed.

**b) Per-subsystem logger field.**

Every service that owns a logger today (`auth`, `notification`, `scene`, `device`, `sensor`, `auditlog`, `hass`, `realtime/ws`) builds its logger with one extra `.With(zap.String("subsystem", "<name>"))` at construction. Two-line change per package; lets `grep subsystem=scene` filter logs without false-positives.

## 7. Testing

For every metric in §4, at least one unit test verifies it increments under the expected condition.

Pattern (using `prometheus/client_golang/prometheus/testutil`):

```go
func TestNotificationCounter_IncrementsOnCreate(t *testing.T) {
    metrics.Reset()
    svc := newTestSvc(t)
    _, err := svc.CreateForUser(ctx, Input{Type: TypeWarning, ...})
    require.NoError(t, err)
    got := testutil.ToFloat64(metrics.NotificationsCreated.WithLabelValues("warning"))
    assert.Equal(t, 1.0, got, "creating one warning notification must increment the counter exactly once")
}
```

Test count estimate:
- HTTP middleware: 4 tests (200, 404, 5xx via test handler, panic recovery)
- DB tracer: 3 tests (ok query, err query, connect-failure)
- Drivers: 1 test per driver × 3 = 3 tests
- HA client: 2 tests (ok, err)
- WS hub: 2 tests (gauge inc on Register / dec on Unregister, publish counter)
- Business KPIs: 5 tests (login, refresh-ok, refresh-replay, notification, scene)
- Readiness: 4 tests (all-ok, postgres-down, ha-down, ha-disabled-not-checked)

**Total: ~25 new unit tests.**

The `metrics.Reset()` helper is the seam that makes this clean — every test gets a fresh registry, so test order and parallelism don't leak counter state.

## 8. Deliverables

1. `backend/internal/platform/metrics/` package (4 files + tests).
2. Wired-in middleware + pgx tracer + driver/HA wrappers + business inc'es.
3. New `backend/internal/server/health.go` with multi-check readiness.
4. `docs/observability/prometheus.yml` — sample scrape config (15s interval, target `host.docker.internal:8080`).
5. `docs/observability/dashboard.json` — Grafana dashboard JSON (HTTP rates, p95 latency, DB error rate, driver error rate, WS connected count, business KPIs over time).
6. README addition: 8-10 line section "Local observability" with `docker run -p 9090:9090 -v $PWD/docs/observability/prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus`.
7. CI: no new step needed — existing `go test` covers the new tests; staticcheck/govulncheck/gosec already on.

## 9. Effort estimate

1–2 working days end-to-end:
- Day 1 morning: package skeleton + HTTP middleware + DB tracer + tests.
- Day 1 afternoon: drivers + HA + WS + business KPIs + tests.
- Day 2 morning: `/readyz` rebuild + tests + log enrichment.
- Day 2 afternoon: docs/observability/ artifacts + README + smoke test.

## 10. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Label cardinality explosion (e.g., user UUID accidentally as label) | All labels are static enums or chi route patterns. Code review checklist: **never** put user-supplied data in a label. |
| `metrics.Reset()` race in parallel tests | Reset rebuilds the package-global Registry; tests within one package run sequentially by default. Tests across packages don't share Registry once Reset runs at the start of each test. |
| `/metrics` endpoint accidentally exposed publicly in prod | Documented as "lock at proxy / firewall in prod"; no auth in pet-project default. |
| pgx tracer interface change with library upgrade | Pin pgx in go.mod; CI's `govulncheck` will flag. |
| HA readiness check spams logs when HA goes down | Check returns error; report logs ERROR once per `/readyz` call (typically 10s scrape interval). Acceptable. |
| Histogram buckets wrong for our actual traffic distribution | Buckets are tunable; first iteration ships sane defaults; future spec can re-bucket based on real data from week 1. |

## 11. Done criteria

- `curl http://localhost:8080/metrics` returns Prometheus-format text including all 15 metrics from §4.
- `curl http://localhost:8080/readyz` returns the multi-check JSON shape from §5; correct 200/503 split.
- Every metric in §4 has at least one passing unit test that asserts it increments.
- `flutter test` still 59/59; `go test -race ./...` still all-pass; staticcheck/gosec/govulncheck still 0.
- README has a 5-line section telling the user how to scrape with Prometheus locally.

## 12. After this spec

`superpowers:writing-plans` produces the implementation plan. Next-cycle specs already queued in audit: **WS auth + contract versioning** (next P2) and **test coverage hardening** (next P1 if we find more handler-level issues).
