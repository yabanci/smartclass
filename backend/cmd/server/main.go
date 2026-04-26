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

	"smartclass/internal/analytics"
	"smartclass/internal/auditlog"
	"smartclass/internal/auth"
	"smartclass/internal/classroom"
	"smartclass/internal/config"
	"smartclass/internal/device"
	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/generic"
	"smartclass/internal/devicectl/drivers/homeassistant"
	"smartclass/internal/devicectl/drivers/smartthings"
	"smartclass/internal/hass"
	"smartclass/internal/notification"
	"smartclass/internal/platform/hasher"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/postgres"
	"smartclass/internal/platform/tokens"
	"smartclass/internal/platform/validation"
	"smartclass/internal/realtime"
	"smartclass/internal/realtime/ws"
	"smartclass/internal/scene"
	"smartclass/internal/schedule"
	"smartclass/internal/sensor"
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
	scheduleRepo := schedule.NewPostgresRepository(db.Pool)
	sceneRepo := scene.NewPostgresRepository(db.Pool)
	sensorRepo := sensor.NewPostgresRepository(db.Pool)
	notificationRepo := notification.NewPostgresRepository(db.Pool)
	auditRepo := auditlog.NewPostgresRepository(db.Pool)
	analyticsRepo := analytics.NewPostgresRepository(db.Pool)
	hassRepo := hass.NewPostgresRepository(db.Pool)

	hash := hasher.NewBcrypt(cfg.Bcrypt.Cost)
	issuer := tokens.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.Issuer)
	valid := validation.New()

	factory := devicectl.NewFactory()
	factory.Register(generic.New(nil))
	factory.Register(smartthings.New(nil))

	hub := ws.NewHub(logger)
	var broker realtime.Broker = hub

	auditSvc := auditlog.NewService(auditRepo, logger)

	classroomSvc := classroom.NewService(classroomRepo)
	notificationSvc := notification.NewService(notificationRepo, classroomRepo, broker).WithLogger(logger)
	triggerEngine := notification.NewEngine(notificationSvc, notification.DefaultRules()).WithLogger(logger)

	authSvc := auth.NewService(userRepo, hash, issuer)
	userSvc := user.NewService(userRepo, hash)
	deviceSvc := device.NewService(deviceRepo, classroomSvc, factory, broker).
		WithLogger(logger).
		WithTrigger(triggerEngine).
		WithRecorder(auditSvc)
	scheduleSvc := schedule.NewService(scheduleRepo, classroomSvc)
	sceneSvc := scene.NewService(sceneRepo, classroomSvc, deviceSvc, broker).WithLogger(logger)
	sensorSvc := sensor.NewService(sensorRepo, classroomSvc, deviceSvc, broker).
		WithLogger(logger).
		WithTrigger(triggerEngine)
	analyticsSvc := analytics.NewService(analyticsRepo, classroomSvc)

	hassClient := hass.NewClient(cfg.Hass.URL, nil)
	hassSvc := hass.NewService(hass.Config{
		BaseURL:       cfg.Hass.URL,
		OwnerName:     cfg.Hass.OwnerName,
		OwnerUsername: cfg.Hass.OwnerUsername,
		OwnerPassword: cfg.Hass.OwnerPassword,
		Language:      cfg.Hass.Language,
	}, hassRepo, hassClient, deviceSvc, logger)
	factory.Register(homeassistant.New(nil, hassSvc))
	logger.Info("device drivers registered", zap.Strings("drivers", factory.Names()))
	go hassSvc.BootstrapWithRetry(ctx)

	authH := auth.NewHandler(authSvc, valid, bundle)
	userH := user.NewHandler(userSvc, valid, bundle)
	classroomH := classroom.NewHandler(classroomSvc, valid, bundle)
	deviceH := device.NewHandler(deviceSvc, valid, bundle)
	scheduleH := schedule.NewHandler(scheduleSvc, valid, bundle)
	sceneH := scene.NewHandler(sceneSvc, valid, bundle)
	sensorH := sensor.NewHandler(sensorSvc, valid, bundle)
	notificationH := notification.NewHandler(notificationSvc, bundle)
	auditH := auditlog.NewHandler(auditSvc, bundle)
	analyticsH := analytics.NewHandler(analyticsSvc, bundle)
	hassH := hass.NewHandler(hassSvc, valid, bundle)
	wsH := ws.NewHandler(hub, logger, bundle)

	srv := server.New(server.Deps{
		Cfg:                 cfg,
		Logger:              logger,
		Bundle:              bundle,
		Issuer:              issuer,
		AuthHandler:         authH,
		UserHandler:         userH,
		ClassroomHandler:    classroomH,
		DeviceHandler:       deviceH,
		ScheduleHandler:     scheduleH,
		SceneHandler:        sceneH,
		SensorHandler:       sensorH,
		NotificationHandler: notificationH,
		AuditHandler:        auditH,
		AnalyticsHandler:    analyticsH,
		HassHandler:         hassH,
		WSHandler:           wsH,
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
	if err := auditSvc.FlushSync(shutdownCtx); err != nil {
		logger.Warn("audit flush", zap.Error(err))
	}
}

func newLogger(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
