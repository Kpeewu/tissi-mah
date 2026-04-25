// geolocation-service — routing OSRM + geocoding Nominatim + cache Redis.
// Service stateless appelé par le front pendant la création d'un trajet.
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("geolocation-service")
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
		ServiceName: "geolocation-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Service (stub en Phase 1.3 — wiring complet en Phase 1.6) ---
	geoService := service.NewGeolocationService(logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewGeolocationServer(cfg, geoService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("geolocation-service ready",
		zap.String("port", cfg.Server.Port),
		zap.String("osrm", cfg.OSRM.URL),
		zap.String("nominatim", cfg.Nominatim.URL),
	)

	return srv.Serve(ctx)
}
