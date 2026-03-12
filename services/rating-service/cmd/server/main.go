package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/rating-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("rating-service")
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
		ServiceName: "rating-service",
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
	userServiceAddr := fmt.Sprintf("%s:%s", cfg.UserService.Address, cfg.UserService.Port)
	userClient, err := client.NewUserServiceClient(userServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close()
	logger.Info("user-service client ready", zap.String("address", userServiceAddr))

	// --- Redis (cache) ---
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		// Redis non bloquant — le service démarre sans cache si Redis est indisponible
		logger.Warn("redis not reachable, cache disabled", zap.Error(err))
		redisClient = nil
	} else {
		logger.Info("connected to redis for caching")
	}

	var ratingCache *cache.RatingCache
	if redisClient != nil {
		ratingCache = cache.NewRatingCache(redisClient, logger)
		defer redisClient.Close() //nolint:errcheck
	}

	// --- Repositories ---
	readRepo := implementations.NewRatingReadRepository(pool, logger)
	writeRepo := implementations.NewRatingWriteRepository(pool, logger)

	// --- Rating service ---
	ratingService := service.NewRatingService(readRepo, writeRepo, userClient, ratingCache, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewRatingServer(cfg, ratingService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("rating-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
