package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/trips-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/service"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("trips-service")
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
		ServiceName: "trips-service",
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

	// --- Booking-service client (graceful degradation) ---
	var bookingClient client.BookingClient
	bc, err := client.NewBookingServiceClient(cfg.BookingService.Addr(), logger)
	if err != nil {
		logger.Warn("booking-service client unavailable, running without booking integration", zap.Error(err))
	} else {
		defer bc.Close()
		bookingClient = bc
		logger.Info("booking-service client ready", zap.String("address", cfg.BookingService.Addr()))
	}

	// --- Rating-service client (graceful degradation) ---
	var ratingClient client.RatingClient
	rc, err := client.NewRatingServiceClient(cfg.RatingService.Addr(), logger)
	if err != nil {
		logger.Warn("rating-service client unavailable, running without rating enrichment", zap.Error(err))
	} else {
		defer rc.Close()
		ratingClient = rc
		logger.Info("rating-service client ready", zap.String("address", cfg.RatingService.Addr()))
	}

	// --- Redis (cache, graceful degradation) ---
	var tripCache *cache.TripCache
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		logger.Warn("redis unavailable, running without cache", zap.Error(err))
	} else {
		defer redisClient.Close()
		tripCache = cache.NewTripCache(redisClient, logger)
		logger.Info("redis connected, cache enabled")
	}

	// --- Notification Redis (stream publication) ---
	var notifRedis *redis.Client
	if cfg.NotificationRedis.URL != "" {
		nr, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.NotificationRedis.URL)
		if err != nil {
			logger.Warn("notification redis unavailable, TRIP_MODIFIED notifications disabled", zap.Error(err))
		} else {
			defer nr.Close()
			notifRedis = nr
			logger.Info("connected to notification redis")
		}
	}

	// --- Repositories ---
	readRepo := implementations.NewTripReadRepository(pool, logger)
	writeRepo := implementations.NewTripWriteRepository(pool, logger)

	// --- Trip service ---
	tripService := service.NewTripService(readRepo, writeRepo, userClient, vehicleClient, bookingClient, ratingClient, tripCache, notifRedis, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewTripServer(cfg, tripService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("trips-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
