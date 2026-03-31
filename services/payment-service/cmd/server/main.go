package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	grpcServer "github.com/Kpeewu/tissi-mah/services/payment-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("payment-service")
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
		ServiceName: "payment-service",
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

	// --- Booking-service client ---
	bookingClient, err := client.NewBookingServiceClient(cfg.BookingService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("booking-service client: %w", err)
	}
	defer bookingClient.Close()
	logger.Info("booking-service client ready", zap.String("address", cfg.BookingService.Addr()))

	// --- User-service client ---
	userClient, err := client.NewUserServiceClient(cfg.UserService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close()
	logger.Info("user-service client ready", zap.String("address", cfg.UserService.Addr()))

	// --- FedaPay client ---
	fedapayClient := fedapay.NewClient(cfg.FedaPay.APIURL, cfg.FedaPay.APIKey, cfg.FedaPay.WebhookSecret, logger)
	logger.Info("fedapay client ready", zap.String("apiURL", cfg.FedaPay.APIURL))

	// --- Redis (cache + payout lock, graceful degradation) ---
	var paymentCache *cache.PaymentCache
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		logger.Warn("redis unavailable, running without cache", zap.Error(err))
	} else {
		defer redisClient.Close()
		paymentCache = cache.NewPaymentCache(redisClient, logger)
		logger.Info("redis connected, cache enabled")
	}

	// --- Repositories ---
	paymentReadRepo := implementations.NewPaymentReadRepository(pool, logger)
	paymentWriteRepo := implementations.NewPaymentWriteRepository(pool, logger)
	refundReadRepo := implementations.NewRefundReadRepository(pool, logger)
	refundWriteRepo := implementations.NewRefundWriteRepository(pool, logger)

	// --- Payout repositories ---
	payoutReadRepo := implementations.NewPayoutReadRepository(pool, logger)
	payoutWriteRepo := implementations.NewPayoutWriteRepository(pool, logger)

	// --- Payment service ---
	paymentService := service.NewPaymentService(
		paymentReadRepo, paymentWriteRepo,
		refundReadRepo, refundWriteRepo,
		payoutReadRepo, payoutWriteRepo,
		bookingClient, userClient,
		fedapayClient,
		paymentCache,
		cfg,
		logger,
	)

	// --- Workers (background) ---
	if impl := service.AsImpl(paymentService); impl != nil {
		go service.StartPayoutWorker(ctx, impl, cfg.Payout.IntervalSeconds, logger)
		go service.StartExpirationWorker(ctx, impl, cfg.Expiration.IntervalSeconds, cfg.Expiration.PaymentTimeoutMinutes, logger)
	}

	// --- gRPC server ---
	srv, err := grpcServer.NewPaymentServer(cfg, paymentService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("payment-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
