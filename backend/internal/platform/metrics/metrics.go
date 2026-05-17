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
	WSTicketInvalid      prometheus.Counter
	AuthLogins           *prometheus.CounterVec
	AuthRefresh          *prometheus.CounterVec
	AuthReplayDetected   prometheus.Counter
	NotificationsCreated *prometheus.CounterVec
	ScenesRun            *prometheus.CounterVec
	PushSends            *prometheus.CounterVec
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
			Help: "WebSocket events published to subscribers, partitioned by topic kind (user|classroom|other).",
		},
		[]string{"topic_kind"},
	)
	WSTicketInvalid = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "ws", Name: "ticket_invalid_total",
			Help: "WebSocket upgrade attempts rejected due to invalid or expired ticket.",
		},
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

	PushSends = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "push", Name: "sends_total",
			Help: "FCM push-notification send attempts, partitioned by result (ok|invalid_token|err).",
		},
		[]string{"result"},
	)

	Registry.MustRegister(
		HTTPRequests, HTTPDuration,
		DBQueries, DBDuration,
		DriverCalls, DriverDuration,
		HassCalls, HassDuration,
		WSConnected, WSMessagesPublished, WSTicketInvalid,
		AuthLogins, AuthRefresh, AuthReplayDetected,
		NotificationsCreated, ScenesRun,
		PushSends,
	)
}
