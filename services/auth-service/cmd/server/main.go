package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/auth-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("auth-service")
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
		ServiceName: "auth-service",
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

	// --- Notification Redis (stream publication) ---
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.NotificationRedis.URL)
	if err != nil {
		return fmt.Errorf("notification redis: %w", err)
	}
	defer redisClient.Close()
	logger.Info("connected to notification redis")

	// --- Suspension Redis (clés suspended:{firebaseUID} lues par l'api-gateway) ---
	suspensionRedis, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("suspension redis: %w", err)
	}
	defer suspensionRedis.Close()
	logger.Info("connected to suspension redis")

	// --- Repositories ---
	readRepo := implementations.NewAuthReadRepository(pool, logger)
	writeRepo := implementations.NewAuthWriteRepository(pool, logger)

	// --- User-service gRPC client ---
	userServiceAddr := fmt.Sprintf("%s:%s", cfg.UserService.Address, cfg.UserService.Port)
	userClient, err := client.NewUserServiceClient(userServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("user-service client: %w", err)
	}
	defer userClient.Close() //nolint:errcheck
	logger.Info("user-service client ready", zap.String("address", userServiceAddr))

	// --- Trips-service gRPC client ---
	tripsServiceAddr := fmt.Sprintf("%s:%s", cfg.TripsService.Address, cfg.TripsService.Port)
	tripsClient, err := client.NewTripsServiceClient(tripsServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("trips-service client: %w", err)
	}
	defer tripsClient.Close() //nolint:errcheck
	logger.Info("trips-service client ready", zap.String("address", tripsServiceAddr))

	// --- Booking-service gRPC client ---
	bookingServiceAddr := fmt.Sprintf("%s:%s", cfg.BookingService.Address, cfg.BookingService.Port)
	bookingClient, err := client.NewBookingServiceClient(bookingServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("booking-service client: %w", err)
	}
	defer bookingClient.Close() //nolint:errcheck
	logger.Info("booking-service client ready", zap.String("address", bookingServiceAddr))

	// --- Payment-service gRPC client ---
	paymentServiceAddr := fmt.Sprintf("%s:%s", cfg.PaymentService.Address, cfg.PaymentService.Port)
	paymentClient, err := client.NewPaymentServiceClient(paymentServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("payment-service client: %w", err)
	}
	defer paymentClient.Close() //nolint:errcheck
	logger.Info("payment-service client ready", zap.String("address", paymentServiceAddr))

	// --- Chat-service gRPC client ---
	chatServiceAddr := fmt.Sprintf("%s:%s", cfg.ChatService.Address, cfg.ChatService.Port)
	chatClient, err := client.NewChatServiceClient(chatServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("chat-service client: %w", err)
	}
	defer chatClient.Close() //nolint:errcheck
	logger.Info("chat-service client ready", zap.String("address", chatServiceAddr))

	// --- File-service gRPC client ---
	fileServiceAddr := fmt.Sprintf("%s:%s", cfg.FileService.Address, cfg.FileService.Port)
	fileClient, err := client.NewFileServiceClient(fileServiceAddr, logger)
	if err != nil {
		return fmt.Errorf("file-service client: %w", err)
	}
	defer fileClient.Close() //nolint:errcheck
	logger.Info("file-service client ready", zap.String("address", fileServiceAddr))

	// --- Auth service ---
	authService := service.NewAuthService(readRepo, writeRepo, userClient, tripsClient, bookingClient, paymentClient, chatClient, fileClient, redisClient, suspensionRedis, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewAuthServer(cfg, authService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("auth-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
