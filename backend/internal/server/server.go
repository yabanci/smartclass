package server

import (
	"context"
	"net/http"
	"net/netip"
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
	http     *http.Server
	cfg      config.Config
	stopRLs  []func() // cleanup functions for rate-limiter goroutines
}

type Deps struct {
	Cfg                 config.Config
	Logger              *zap.Logger
	Bundle              *i18n.Bundle
	Issuer              tokens.Issuer
	ReadinessChecks     []ReadinessCheck
	// TrustedProxies restricts X-Forwarded-For trust to these CIDRs.
	// Empty = any loopback/private address is trusted (default).
	TrustedProxies      []netip.Prefix
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
	WSTicketHandler     http.Handler
}

func New(d Deps) *Server {
	r := chi.NewRouter()

	rl := mw.NewRateLimiter(d.Cfg.RateLimit.RPS, d.Cfg.RateLimit.Burst).
		WithTrustedProxies(d.TrustedProxies)
	// Strict limiter for auth endpoints: 5 RPS burst 10 per IP prevents brute-force.
	// Auth limiter inherits the same proxy trust configuration.
	authRL := mw.NewRateLimiter(5, 10).WithTrustedProxies(d.TrustedProxies)

	r.Use(mw.Recoverer(d.Logger))
	r.Use(mw.RequestID)
	r.Use(mw.RequestLogger(d.Logger))
	r.Use(metrics.HTTPMiddleware)
	r.Use(mw.CORS(d.Cfg.CORS.Origins))
	r.Use(mw.Language)
	r.Use(mw.BodyLimit(mw.MaxBodyBytes))
	r.Use(rl.Middleware())

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyzHandler(d.ReadinessChecks))
	// /metrics is intentionally unauthenticated. The endpoint is meant to be
	// scraped by an in-cluster Prometheus; in production this listener must
	// be locked down at the proxy/firewall layer (see README §"Local
	// observability"). For pet-project localhost use, no auth is fine.
	r.Mount("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{
		Registry:          metrics.Registry,
		EnableOpenMetrics: true,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		// /auth has two distinct middleware regimes that both need to live at
		// the same URL prefix: the unauthenticated entry points (register/
		// login/refresh, behind the strict authRL limiter) and the
		// authenticated ones (logout). chi panics if you call r.Route("/auth")
		// twice on the same mux, so both groups nest INSIDE one /auth Route.
		r.Route("/auth", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(authRL.Middleware())
				d.AuthHandler.Routes(r)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.Authn(d.Issuer, d.Bundle))
				d.AuthHandler.AuthenticatedRoutes(r)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(mw.Authn(d.Issuer, d.Bundle))

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

			// /ws/ticket needs the principal (JWT-authenticated). The actual
			// /ws upgrade authenticates via the ticket itself, so it is
			// registered OUTSIDE this Authn-protected group below.
			if d.WSTicketHandler != nil {
				r.Post("/ws/ticket", d.WSTicketHandler.ServeHTTP)
			}
		})

		// /ws is authenticated by the single-use ticket query param, NOT by
		// Bearer JWT — sits outside the Authn-protected group so the upgrade
		// handshake doesn't 401 before the ticket is even consumed.
		if d.WSHandler != nil {
			r.Get("/ws", d.WSHandler.Serve)
		}
	})

	srv := &http.Server{
		Addr:              d.Cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return &Server{
		http:    srv,
		cfg:     d.Cfg,
		stopRLs: []func(){rl.Stop, authRL.Stop},
	}
}

func (s *Server) ListenAndServe() error { return s.http.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error {
	for _, stop := range s.stopRLs {
		stop()
	}
	return s.http.Shutdown(ctx)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
