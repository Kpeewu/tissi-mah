package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("vehicle-service")
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
		ServiceName: "vehicle-service",
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

	// --- File-service client ---
	fileServiceAddr := cfg.FileService.Address()
	fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("file-service client: %w", err)
	}
	defer fileClient.Close() //nolint:errcheck
	logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- Repositories ---
	readRepo := implementations.NewVehicleReadRepository(pool, logger)
	writeRepo := implementations.NewVehicleWriteRepository(pool, logger)

	// --- Vehicle service ---
	vehicleService := service.NewVehicleService(readRepo, writeRepo, fileClient, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewVehicleServer(cfg, vehicleService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("vehicle-service ready",
		zap.String("port", cfg.Server.Port),
		zap.String("file-service", fileServiceAddr),
	)

	return srv.Serve(ctx)
}
