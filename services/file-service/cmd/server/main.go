package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("file-service")
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
		ServiceName: "file-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- PostgreSQL ---
	pool, err := pkgDatabase.NewPostgresPoolFromURL(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	// --- S3/MinIO storage client ---
	storageClient, err := storage.NewS3Client(ctx, cfg.S3, logger)
	if err != nil {
		return fmt.Errorf("s3 client: %w", err)
	}
	logger.Info("s3/minio client ready",
		zap.String("bucket", cfg.S3.Bucket),
		zap.String("endpoint", cfg.S3.Endpoint),
	)

	// --- User-service client ---
	userClient, err := client.NewUserServiceClient(cfg.UserService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close()
	logger.Info("user-service client ready", zap.String("address", cfg.UserService.Addr()))

	// --- Vehicle-service client ---
	vehicleClient, err := client.NewVehicleServiceClient(cfg.VehicleService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("vehicle-service client: %w", err)
	}
	defer vehicleClient.Close()
	logger.Info("vehicle-service client ready", zap.String("address", cfg.VehicleService.Addr()))

	// --- Repositories ---
	userDocRead := implementations.NewUserDocumentReadRepository(pool, logger)
	userDocWrite := implementations.NewUserDocumentWriteRepository(pool, logger)
	vehicleDocRead := implementations.NewVehicleDocumentReadRepository(pool, logger)
	vehicleDocWrite := implementations.NewVehicleDocumentWriteRepository(pool, logger)
	reviewRead := implementations.NewDocumentReviewReadRepository(pool, logger)
	reviewWrite := implementations.NewDocumentReviewWriteRepository(pool, logger)

	// --- Moderation client (optionnel) ---
	var moderationClient client.ModerationClient
	if cfg.ModerationEnabled {
		mc, err := client.NewModerationServiceClient(cfg.ModerationService.Addr(), cfg.ModerationFailClosed, logger)
		if err != nil {
			logger.Warn("moderation-service client init failed, moderation disabled", zap.Error(err))
		} else {
			moderationClient = mc
			defer mc.Close() //nolint:errcheck
			logger.Info("moderation-service client ready", zap.String("address", cfg.ModerationService.Addr()))
		}
	}

	// --- File service ---
	fileService := service.NewFileService(
		userDocRead, userDocWrite,
		vehicleDocRead, vehicleDocWrite,
		reviewRead, reviewWrite,
		storageClient,
		moderationClient,
		logger,
	)

	// --- gRPC server ---
	srv, err := grpcServer.NewFileServer(cfg, fileService, userClient, vehicleClient, storageClient, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("file-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
