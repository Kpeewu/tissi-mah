package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/booking-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("booking-service")
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
		ServiceName: "booking-service",
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

	// --- Trip-service client ---
	tripClient, err := client.NewTripServiceClient(cfg.TripService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("trips-service client: %w", err)
	}
	defer tripClient.Close()
	logger.Info("trips-service client ready", zap.String("address", cfg.TripService.Addr()))

	// --- User-service client ---
	userClient, err := client.NewUserServiceClient(cfg.UserService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close()
	logger.Info("user-service client ready", zap.String("address", cfg.UserService.Addr()))

	// --- Payment-service client ---
	paymentClient, err := client.NewPaymentServiceClient(cfg.PaymentService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("payment-service client: %w", err)
	}
	defer paymentClient.Close()
	logger.Info("payment-service client ready", zap.String("address", cfg.PaymentService.Addr()))

	// --- Redis (cache, graceful degradation) ---
	var bookingCache *cache.BookingCache
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		logger.Warn("redis unavailable, running without cache", zap.Error(err))
	} else {
		defer redisClient.Close()
		bookingCache = cache.NewBookingCache(redisClient, logger)
		logger.Info("redis connected, cache enabled")
	}

	// --- Repositories ---
	readRepo := implementations.NewBookingReadRepository(pool, logger)
	writeRepo := implementations.NewBookingWriteRepository(pool, logger)

	// --- Booking service ---
	bookingService := service.NewBookingService(
		readRepo, writeRepo,
		tripClient, userClient, paymentClient,
		bookingCache,
		cfg.ServiceFee.Percent,
		logger,
	)

	// --- Job de réconciliation (background) ---
	if impl := service.AsImpl(bookingService); impl != nil {
		go service.StartReconciliationJob(ctx, impl, cfg.Reconciliation.IntervalSeconds, logger)
		go service.StartPaymentReleaseJob(ctx, impl, cfg.Payment.ReleaseWorkerIntervalSeconds, cfg.Payment.ContestationDelaySeconds, logger)
	}

	// --- gRPC server ---
	srv, err := grpcServer.NewBookingServer(cfg, bookingService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("booking-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
