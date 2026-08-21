package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("kyc-service")
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
		ServiceName: "kyc-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- File-service gRPC client ---
	fileServiceAddr := fmt.Sprintf("%s:%s", cfg.FileService.Address, cfg.FileService.Port)
	fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("file-service client: %w", err)
	}
	defer fileClient.Close()
	logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- User-service gRPC client ---
	userClient, err := client.NewUserServiceClient(cfg.UserService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close()
	logger.Info("user-service client ready", zap.String("address", cfg.UserService.Addr()))

	// --- Support-service gRPC client (enrichissement prénom/nom des agents) ---
	supportClient, err := client.NewSupportServiceClient(cfg.SupportService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("support-service client: %w", err)
	}
	defer supportClient.Close()
	logger.Info("support-service client ready", zap.String("address", cfg.SupportService.Addr()))

	// --- Vehicle-service gRPC client (propagation is_verified par véhicule) ---
	vehicleClient, err := client.NewVehicleServiceClient(cfg.VehicleService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("vehicle-service client: %w", err)
	}
	defer vehicleClient.Close()
	logger.Info("vehicle-service client ready", zap.String("address", cfg.VehicleService.Addr()))

	// --- Notification Redis (stream publication) ---
	notifRedis, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.NotificationRedis.URL)
	if err != nil {
		return fmt.Errorf("notification redis: %w", err)
	}
	defer notifRedis.Close()
	logger.Info("connected to notification redis")

	// --- KYC Service ---
	kycService := service.NewKYCService(
		fileClient,
		userClient,
		supportClient,
		vehicleClient,
		notifRedis,
		logger,
	)

	// --- gRPC server ---
	srv, err := grpcServer.NewKYCServer(cfg, kycService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("kyc-service ready",
		zap.String("port", cfg.Server.Port),
		zap.String("environment", cfg.Environment.Mode),
	)

	return srv.Serve(ctx)
}
