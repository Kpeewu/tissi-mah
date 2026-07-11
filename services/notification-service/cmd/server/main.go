package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/consumer"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/dispatcher"
	grpcServer "github.com/Kpeewu/tissi-mah/services/notification-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/worker"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("notification-service")
	defer bootstrapLogger.Sync() //nolint:errcheck

	if err := run(bootstrapLogger); err != nil {
		bootstrapLogger.Fatal("service terminated with error", zap.Error(err))
	}
}

func run(bootstrapLogger *zap.Logger) error {
	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// --- Logger ---
	logger, err := pkgLogger.New(pkgLogger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment.Mode,
		ServiceName: "notification-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- PostgreSQL ---
	pool, err := database.NewPostgresPoolFromURL(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()

	// --- Redis ---
	rdb, err := database.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer rdb.Close()

	// --- Repositories ---
	routingRepo := implementations.NewRoutingRepository(pool, logger)
	templateRepo := implementations.NewTemplateRepository(pool, logger)
	notifRepo := implementations.NewNotificationRepository(pool, logger)
	inboxRepo := implementations.NewInboxRepository(pool, logger)
	preferenceRepo := implementations.NewPreferenceRepository(pool, logger)
	deviceRepo := implementations.NewDeviceTokenRepository(pool, logger)

	// --- gRPC Clients ---
	userClient, err := client.NewUserServiceClient(cfg.UserService.Address(), logger)
	if err != nil {
		return fmt.Errorf("user client: %w", err)
	}
	defer userClient.Close()

	pushClient, err := client.NewPushServiceClient(cfg.PushService.Address(), logger)
	if err != nil {
		return fmt.Errorf("push client: %w", err)
	}
	defer pushClient.Close()

	emailClient, err := client.NewEmailServiceClient(cfg.EmailService.Address(), logger)
	if err != nil {
		return fmt.Errorf("email client: %w", err)
	}
	defer emailClient.Close()

	// --- Service ---
	notifService := service.NewNotificationService(inboxRepo, preferenceRepo, deviceRepo, userClient, logger)

	// --- Dispatcher ---
	disp := dispatcher.NewDispatcher(
		routingRepo, templateRepo, notifRepo, inboxRepo,
		preferenceRepo, deviceRepo,
		pushClient, emailClient, userClient,
		logger,
	)

	// --- Workers (goroutines) ---
	retryWorker := worker.NewRetryWorker(notifRepo, pushClient, emailClient, logger)
	retentionWorker := worker.NewRetentionWorker(notifRepo, inboxRepo, logger)

	go retryWorker.Start(ctx)
	go retentionWorker.Start(ctx)

	// --- Redis Stream Consumer (goroutine) ---
	eventConsumer := consumer.NewConsumer(rdb, disp, logger)
	go func() {
		if err := eventConsumer.Start(ctx); err != nil {
			logger.Error("consumer error", zap.Error(err))
		}
	}()

	// --- gRPC Server ---
	srv, err := grpcServer.NewNotificationServer(cfg, notifService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("notification-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
