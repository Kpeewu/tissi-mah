package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/push-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/push-service/internal/fcm"
	grpcServer "github.com/Kpeewu/tissi-mah/services/push-service/internal/grpc"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("push-service")
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
		ServiceName: "push-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Firebase FCM client ---
	fcmClient, err := fcm.NewClient(cfg.Firebase.CredentialsPath, logger)
	if err != nil {
		return fmt.Errorf("fcm client: %w", err)
	}
	defer fcmClient.Close() //nolint:errcheck

	logger.Info("fcm client initialized")

	// --- gRPC server ---
	srv, err := grpcServer.NewPushServer(cfg, fcmClient, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("push-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
