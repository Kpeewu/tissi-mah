package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
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
	_ = ctx // Utilisé par srv.Serve(ctx) ci-dessous

	// --- File-service gRPC client ---
	// TODO: implémenter le client gRPC concret vers file-service
	// fileServiceAddr := fmt.Sprintf("%s:%s", cfg.FileService.Address, cfg.FileService.Port)
	// fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	// if err != nil {
	// 	return fmt.Errorf("file-service client: %w", err)
	// }
	// defer fileClient.Close()
	// logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- Persona HTTP client ---
	// TODO: implémenter le client HTTP concret vers l'API Persona
	// personaClient, err := client.NewPersonaClient(cfg.Persona.APIKey, logger)
	// if err != nil {
	// 	return fmt.Errorf("persona client: %w", err)
	// }
	// logger.Info("persona client ready")

	// --- KYC Service ---
	// TODO: remplacer nil par les clients concrets une fois implémentés
	kycService := service.NewKYCService(
		nil, // fileClient
		nil, // personaClient
		cfg.Persona.TemplateID,
		cfg.Persona.WebhookSecret,
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
