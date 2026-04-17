package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"smartclass/internal/analytics"
	"smartclass/internal/auditlog"
	"smartclass/internal/auth"
	"smartclass/internal/classroom"
	"smartclass/internal/config"
	"smartclass/internal/device"
	"smartclass/internal/notification"
	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
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

type Deps struct {
	Cfg                 config.Config
	Logger              *zap.Logger
	Bundle              *i18n.Bundle
	Issuer              tokens.Issuer
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
	WSHandler           *ws.Handler
}

func New(d Deps) *Server {
	r := chi.NewRouter()

	rl := mw.NewRateLimiter(d.Cfg.RateLimit.RPS, d.Cfg.RateLimit.Burst)

	r.Use(mw.Recoverer(d.Logger))
	r.Use(mw.RequestLogger(d.Logger))
	r.Use(mw.CORS(d.Cfg.CORS.Origins))
	r.Use(mw.Language)
	r.Use(rl.Middleware())

	r.Get("/healthz", healthz)
	r.Get("/readyz", healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", d.AuthHandler.Routes)

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
