package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/auth-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("auth-service")
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
		ServiceName: "auth-service",
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

	// --- Repositories ---
	readRepo := implementations.NewAuthReadRepository(pool)
	writeRepo := implementations.NewAuthWriteRepository(pool)

	// --- User-service gRPC client ---
	userServiceAddr := fmt.Sprintf("%s:%s", cfg.UserService.Address, cfg.UserService.Port)
	userClient, err := client.NewUserServiceClient(userServiceAddr)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close() //nolint:errcheck
	logger.Info("user-service client ready", zap.String("address", userServiceAddr))

	// --- Auth service ---
	authService := service.NewAuthService(readRepo, writeRepo, userClient, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewAuthServer(cfg, authService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("auth-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
