package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/user-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("user-service")
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
		ServiceName: "user-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- MongoDB ---
	mongoClient, err := pkgDatabase.NewMongoClientFromURL(ctx, cfg.MongoDB.URL)
	if err != nil {
		return fmt.Errorf("mongodb: %w", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(ctx); err != nil {
			logger.Error("failed to disconnect mongodb", zap.Error(err))
		}
	}()
	logger.Info("connected to mongodb")

	// --- MongoDB Collection + Indexes ---
	db := mongoClient.Database(cfg.MongoDB.Database)
	usersCollection := db.Collection("users")

	if err := implementations.EnsureIndexes(ctx, usersCollection); err != nil {
		return fmt.Errorf("mongodb indexes: %w", err)
	}
	logger.Info("mongodb indexes ensured")

	// --- Repositories ---
	readRepo := implementations.NewUserReadRepository(usersCollection, logger)
	writeRepo := implementations.NewUserWriteRepository(usersCollection, logger)

	// --- Auth-service gRPC client ---
	authServiceAddr := fmt.Sprintf("%s:%s", cfg.AuthService.Address, cfg.AuthService.Port)
	authClient, err := client.NewAuthServiceClient(authServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("auth-service client: %w", err)
	}
	defer authClient.Close() //nolint:errcheck
	logger.Info("auth-service client ready", zap.String("address", authServiceAddr))

	// --- File-service gRPC client ---
	fileServiceAddr := fmt.Sprintf("%s:%s", cfg.FileService.Address, cfg.FileService.Port)
	fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("file-service client: %w", err)
	}
	defer fileClient.Close() //nolint:errcheck
	logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- User service ---
	userService := service.NewUserService(readRepo, writeRepo, authClient, fileClient, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewUserServer(cfg, userService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("user-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
