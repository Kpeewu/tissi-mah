package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/cache"
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

	// --- Redis (cache) ---
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		// Redis non bloquant — le service démarre sans cache si Redis est indisponible
		logger.Warn("redis not reachable, cache disabled", zap.Error(err))
		redisClient = nil
	} else {
		logger.Info("connected to redis for caching")
	}

	var vehicleCache *cache.VehicleCache
	if redisClient != nil {
		vehicleCache = cache.NewVehicleCache(redisClient, logger)
		defer redisClient.Close() //nolint:errcheck
	}

	// --- File-service client ---
	fileServiceAddr := cfg.FileService.Address()
	fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("file-service client: %w", err)
	}
	defer fileClient.Close() //nolint:errcheck
	logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- Trips-service client ---
	tripsServiceAddr := cfg.TripsService.Address()
	tripsClient, err := client.NewTripsServiceClient(tripsServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("trips-service client: %w", err)
	}
	defer tripsClient.Close() //nolint:errcheck
	logger.Info("trips-service client ready", zap.String("address", tripsServiceAddr))

	// --- Repositories ---
	readRepo := implementations.NewVehicleReadRepository(pool, logger)
	writeRepo := implementations.NewVehicleWriteRepository(pool, logger)

	// --- Vehicle service ---
	vehicleService := service.NewVehicleService(readRepo, writeRepo, fileClient, tripsClient, vehicleCache, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewVehicleServer(cfg, vehicleService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("vehicle-service ready",
		zap.String("port", cfg.Server.Port),
		zap.String("file-service", fileServiceAddr),
		zap.String("trips-service", tripsServiceAddr),
		zap.Bool("cache-enabled", vehicleCache != nil),
	)

	return srv.Serve(ctx)
}
