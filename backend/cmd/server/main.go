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
	"smartclass/internal/classroom"
	"smartclass/internal/config"
	"smartclass/internal/device"
	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/generic"
	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/postgres"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/platform/validation"
	"smartclass/internal/realtime"
	"smartclass/internal/realtime/ws"
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
	classroomRepo := classroom.NewPostgresRepository(db.Pool)
	deviceRepo := device.NewPostgresRepository(db.Pool)

	hash := hasher.NewBcrypt(cfg.Bcrypt.Cost)
	issuer := tokens.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.Issuer)
	valid := validation.New()

	factory := devicectl.NewFactory()
	factory.Register(generic.New(nil))
	logger.Info("device drivers registered", zap.Strings("drivers", factory.Names()))

	hub := ws.NewHub(logger)
	var broker realtime.Broker = hub

	authSvc := auth.NewService(userRepo, hash, issuer)
	userSvc := user.NewService(userRepo, hash)
	classroomSvc := classroom.NewService(classroomRepo)
	deviceSvc := device.NewService(deviceRepo, classroomSvc, factory, broker)

	authH := auth.NewHandler(authSvc, valid, bundle)
	userH := user.NewHandler(userSvc, valid, bundle)
	classroomH := classroom.NewHandler(classroomSvc, valid, bundle)
	deviceH := device.NewHandler(deviceSvc, valid, bundle)
	wsH := ws.NewHandler(hub, logger, bundle)

	srv := server.New(server.Deps{
		Cfg:              cfg,
		Logger:           logger,
		Bundle:           bundle,
		Issuer:           issuer,
		AuthHandler:      authH,
		UserHandler:      userH,
		ClassroomHandler: classroomH,
		DeviceHandler:    deviceH,
		WSHandler:        wsH,
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
