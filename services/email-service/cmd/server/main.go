package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/email-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/email-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/email-service/internal/provider"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("email-service")
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
		ServiceName: "email-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Email provider ---
	emailProvider := provider.NewSMTPProvider(
		cfg.SMTP.Host, cfg.SMTP.Port,
		cfg.SMTP.Username, cfg.SMTP.Password,
		cfg.Email.From, logger,
	)

	logger.Info("email provider initialized", zap.String("provider", emailProvider.Name()))

	// --- gRPC server ---
	srv, err := grpcServer.NewEmailServer(cfg, emailProvider, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("email-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
