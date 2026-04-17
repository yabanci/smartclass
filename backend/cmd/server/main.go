package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"smartclass/internal/auth"
	"smartclass/internal/config"
	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/postgres"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/platform/validation"
	"smartclass/internal/server"
	"smartclass/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger, err := newLogger(cfg.Env)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		logger.Fatal("postgres connect", zap.Error(err))
	}
	defer db.Close()

	if err := db.Migrate(cfg.Paths.Migrations); err != nil {
		logger.Fatal("migrate", zap.Error(err))
	}
	logger.Info("migrations applied", zap.String("dir", cfg.Paths.Migrations))

	bundle := i18n.MustLoadDir(cfg.Paths.I18n)

	userRepo := user.NewPostgresRepository(db.Pool)
	hash := hasher.NewBcrypt(cfg.Bcrypt.Cost)
	issuer := tokens.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.Issuer)
	valid := validation.New()

	authSvc := auth.NewService(userRepo, hash, issuer)
	userSvc := user.NewService(userRepo, hash)

	authH := auth.NewHandler(authSvc, valid, bundle)
	userH := user.NewHandler(userSvc, valid, bundle)

	srv := server.New(server.Deps{
		Cfg:         cfg,
		Logger:      logger,
		Bundle:      bundle,
		Issuer:      issuer,
		AuthHandler: authH,
		UserHandler: userH,
	})

	go func() {
		logger.Info("server listening", zap.String("addr", cfg.HTTP.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", zap.Error(err))
	}
}

func newLogger(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
