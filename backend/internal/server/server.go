package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"smartclass/internal/auth"
	"smartclass/internal/config"
	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/user"
)

type Server struct {
	http *http.Server
	cfg  config.Config
}

type Deps struct {
	Cfg         config.Config
	Logger      *zap.Logger
	Bundle      *i18n.Bundle
	Issuer      tokens.Issuer
	AuthHandler *auth.Handler
	UserHandler *user.Handler
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
