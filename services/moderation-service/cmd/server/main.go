package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/moderation-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/provider/perspective"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/provider/vision"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("moderation-service")
	defer bootstrapLogger.Sync() //nolint:errcheck

	if err := run(bootstrapLogger); err != nil {
		bootstrapLogger.Fatal("service terminated with error", zap.Error(err))
	}
}

func run(bootstrapLogger *zap.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger, err := pkgLogger.New(pkgLogger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment.Mode,
		ServiceName: "moderation-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- PostgreSQL ---
	pool, err := pkgDatabase.NewPostgresPoolFromURL(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	// --- Repositories ---
	logRepo := implementations.NewModerationLogRepository(pool, logger)
	violationRepo := implementations.NewViolationRepository(pool, logger)

	// --- Providers externes (optionnels) ---
	var textProvider = perspective.NewClient(cfg.Moderation.PerspectiveAPIKey, logger)
	var imageProvider = vision.NewClient(cfg.Moderation.GoogleAppCreds, logger)

	// --- Clients inter-services ---
	userServiceAddr := fmt.Sprintf("%s:%s", cfg.UserService.Address, cfg.UserService.Port)
	userClient, err := client.NewUserServiceClient(userServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close() //nolint:errcheck
	logger.Info("user-service client ready", zap.String("address", userServiceAddr))

	authServiceAddr := fmt.Sprintf("%s:%s", cfg.AuthService.Address, cfg.AuthService.Port)
	authClient, err := client.NewAuthServiceClient(authServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("auth-service client: %w", err)
	}
	defer authClient.Close() //nolint:errcheck
	logger.Info("auth-service client ready", zap.String("address", authServiceAddr))

	// --- Service ---
	moderationSvc := service.NewModerationService(service.Config{
		LogRepo:       logRepo,
		ViolationRepo: violationRepo,
		TextProvider:  textProvider,
		ImageProvider: imageProvider,
		UserClient:    userClient,
		AuthClient:    authClient,
		BlockedThresh: float32(cfg.Moderation.TextBlockedThreshold),
		FlaggedThresh: float32(cfg.Moderation.TextFlaggedThreshold),
		FailClosed:    cfg.Moderation.FailClosed,
		Logger:        logger,
	})

	// --- gRPC server ---
	srv, err := grpcServer.NewModerationServer(cfg, moderationSvc, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("moderation-service ready", zap.String("port", cfg.Server.Port))
	return srv.Serve(ctx)
}
