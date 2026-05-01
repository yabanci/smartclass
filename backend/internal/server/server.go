package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"smartclass/internal/analytics"
	"smartclass/internal/auditlog"
	"smartclass/internal/auth"
	"smartclass/internal/classroom"
	"smartclass/internal/config"
	"smartclass/internal/device"
	"smartclass/internal/hass"
	"smartclass/internal/notification"
	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/metrics"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/realtime/ws"
	"smartclass/internal/scene"
	"smartclass/internal/schedule"
	"smartclass/internal/sensor"
	"smartclass/internal/user"
)

type Server struct {
	http *http.Server
	cfg  config.Config
}

// ReadinessChecker is anything that can answer "are my critical dependencies
// reachable right now". The server's /readyz wires it up so a Kubernetes
// readiness probe (or our `make verify` script) sees a real signal — not a
// process-is-alive heartbeat that masks DB outages.
type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type Deps struct {
	Cfg                 config.Config
	Logger              *zap.Logger
	Bundle              *i18n.Bundle
	Issuer              tokens.Issuer
	Readiness           ReadinessChecker
	AuthHandler         *auth.Handler
	UserHandler         *user.Handler
	ClassroomHandler    *classroom.Handler
	DeviceHandler       *device.Handler
	ScheduleHandler     *schedule.Handler
	SceneHandler        *scene.Handler
	SensorHandler       *sensor.Handler
	NotificationHandler *notification.Handler
	AuditHandler        *auditlog.Handler
	AnalyticsHandler    *analytics.Handler
	HassHandler         *hass.Handler
	WSHandler           *ws.Handler
}

func New(d Deps) *Server {
	r := chi.NewRouter()

	rl := mw.NewRateLimiter(d.Cfg.RateLimit.RPS, d.Cfg.RateLimit.Burst)
	// Strict limiter for auth endpoints: 5 RPS burst 10 per IP prevents brute-force.
	authRL := mw.NewRateLimiter(5, 10)

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

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authRL.Middleware())
			r.Route("/auth", d.AuthHandler.Routes)
		})

		r.Group(func(r chi.Router) {
			r.Use(mw.Authn(d.Issuer, d.Bundle))

			r.Route("/auth", d.AuthHandler.AuthenticatedRoutes)

			r.Route("/users", d.UserHandler.Routes)

			r.Route("/classrooms", func(r chi.Router) {
				d.ClassroomHandler.Routes(r)
				r.Route("/{id}/devices", d.DeviceHandler.ClassroomRoutes)
				r.Route("/{id}/schedule", d.ScheduleHandler.ClassroomRoutes)
				r.Route("/{id}/scenes", d.SceneHandler.ClassroomRoutes)
				r.Route("/{id}/sensors", d.SensorHandler.ClassroomRoutes)
				r.Route("/{id}/analytics", d.AnalyticsHandler.ClassroomRoutes)
			})

			r.Route("/devices", func(r chi.Router) {
				d.DeviceHandler.Routes(r)
				r.Route("/{id}/sensors", d.SensorHandler.DeviceRoutes)
			})

			r.Route("/schedule", d.ScheduleHandler.Routes)
			r.Route("/scenes", d.SceneHandler.Routes)
			r.Route("/sensors", d.SensorHandler.Routes)
			r.Route("/notifications", d.NotificationHandler.Routes)
			r.Route("/logs", d.AuditHandler.Routes)
			if d.HassHandler != nil {
				r.Route("/hass", d.HassHandler.Routes)
			}

			if d.WSHandler != nil {
				r.Get("/ws", d.WSHandler.Serve)
			}
		})
	})

	srv := &http.Server{
		Addr:              d.Cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return &Server{http: srv, cfg: d.Cfg}
}

func (s *Server) ListenAndServe() error { return s.http.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz reports 503 when the server cannot serve traffic — for now that's
// "DB pool can't ping in 2s". Liveness (/healthz) stays decoupled: the
// process is up even when Postgres is down, which is exactly what
// orchestrators need to distinguish "restart me" from "stop sending traffic".
func readyz(checker ReadinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checker == nil {
			httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := checker.Ready(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unready",
				"reason": err.Error(),
			})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
